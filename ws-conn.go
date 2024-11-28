package websvc

import (
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/misc"
	"github.com/gorilla/websocket"
)

type WebSocketConnection struct {
	Attachment    interface{}
	_waitGroup    sync.WaitGroup
	_quitChan     chan int
	_sendingQueue chan *misc.Buffer

	_conn            *websocket.Conn
	_pool            *sync.Pool
	_cfg             *WebSocketConfig
	_lastBeat        int64
	_refCount        int32
	_heartBeatTimout int64
	_triggerBeat     bool
}

func NewWebSocketConnection(cfg *WebSocketConfig) *WebSocketConnection {
	return &WebSocketConnection{
		_quitChan:     make(chan int, 5),
		_sendingQueue: make(chan *misc.Buffer, 100),
		_conn:         nil,
		_pool:         nil,
		_cfg:          cfg,
		_refCount:     1,
		_triggerBeat:  true,
	}
}

func (sc *WebSocketConnection) AddRef() *WebSocketConnection {
	atomic.AddInt32(&sc._refCount, 1)
	return sc
}

func (sc *WebSocketConnection) Release() {
	if atomic.AddInt32(&sc._refCount, -1) == 0 {
		if sc._pool != nil {
			sc._pool.Put(sc)
		}
	}
}

func (sc *WebSocketConnection) Connect(url string, qs chan os.Signal) bool {

	var headers http.Header
	if sc._cfg.Headers != nil {
		headers = sc._cfg.Headers(sc._cfg.Attachment)
	}

	conn, _, e := websocket.DefaultDialer.Dial(url, headers)
	if e != nil {
		logger.Error("websvc:ws:dial %s failed: %s", url, e.Error())
		if sc._cfg.OnDisconnected != nil {
			sc._cfg.OnDisconnected(sc, sc._cfg.Attachment)
		}
		return false
	} else {
		sc._conn = conn
		if sc._cfg.OnConnected != nil {
			sc._cfg.OnConnected(sc, sc._cfg.Attachment)
		}
		ret := sc.run(qs)
		if sc._cfg.OnDisconnected != nil {
			sc._cfg.OnDisconnected(sc, sc._cfg.Attachment)
		}
		return ret
	}
}

func (sc *WebSocketConnection) Close() {
	if sc._conn != nil {
		sc._conn.Close()
		sc._conn = nil
	}
}

func (sc *WebSocketConnection) Send(msg *misc.Buffer) {
	sc._sendingQueue <- msg
}

func (sc *WebSocketConnection) SendBinaryBuffer(msg *misc.Buffer) {
	msg.Tag = websocket.BinaryMessage
	sc._sendingQueue <- msg
}

func (sc *WebSocketConnection) SendTextBuffer(msg *misc.Buffer) {
	msg.Tag = websocket.TextMessage
	sc._sendingQueue <- msg
}

func (sc *WebSocketConnection) SendBytes(data []byte) {
	var buf *misc.Buffer
	if sc._cfg.BufferPool != nil {
		buf = sc._cfg.BufferPool.Get()
	} else {
		buf = misc.NewBuffer(uint(len(data)))
	}
	buf.Write(data)
	sc.SendBinaryBuffer(buf)
}

func (sc *WebSocketConnection) SendText(msg string) {
	var buf *misc.Buffer
	if sc._cfg.BufferPool != nil {
		buf = sc._cfg.BufferPool.Get()
	} else {
		buf = misc.NewBuffer(uint(len(msg)))
	}
	buf.Write([]byte(msg))
	sc.SendTextBuffer(buf)
}

func (sc *WebSocketConnection) run(qs chan os.Signal) bool {
	sc._lastBeat = time.Now().UnixMilli()
	sc._conn.SetPingHandler(func(appData string) error {
		sc._lastBeat = time.Now().UnixMilli()
		sc._conn.WriteMessage(websocket.PongMessage, nil)
		return nil
	})
	sc._conn.SetPongHandler(func(appData string) error {
		sc._lastBeat = time.Now().UnixMilli()
		return nil
	})
	sc._waitGroup.Add(2)
	go sc.sendLoop()
	go sc.recvLoop()

	if sc._cfg.BeatInterval > 0 {
		sc._waitGroup.Add(1)
		go sc.beatLoop()
	}
	ret := false
	if qs != nil {
		select {
		case s := <-sc._quitChan:
			sc._quitChan <- s
		case s := <-qs:
			sc.Close()
			qs <- s
			ret = true
		}
	}
	sc._waitGroup.Wait()
	sc.Close()
	for {
		select {
		case wm := <-sc._sendingQueue:
			wm.Release()
		case <-sc._quitChan:
			// DO NOTHING
		default:
			return ret
		}
	}
}

func (sc *WebSocketConnection) beatLoop() {
	defer sc._waitGroup.Done()
	beatInterval := time.Second * time.Duration(sc._cfg.BeatInterval)
	if beatInterval == 0 {
		beatInterval = time.Second * 10
	}
	sc._heartBeatTimout = int64(sc._cfg.BeatTimeout * 1000)
	if sc._heartBeatTimout == 0 {
		sc._heartBeatTimout = int64(sc._cfg.BeatInterval * 3000)
	}
	tick := time.NewTicker(beatInterval)
	for {
		select {
		case s := <-sc._quitChan:
			sc._quitChan <- s
			return
		case <-tick.C:
			ts := time.Now().UnixMilli()
			if ts-sc._lastBeat > sc._heartBeatTimout {
				sc.Close()
				return
			} else if sc._triggerBeat {
				sc._conn.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}
}

func (sc *WebSocketConnection) sendLoop() {
	defer sc._waitGroup.Done()
	for {
		select {
		case s := <-sc._quitChan:
			sc._quitChan <- s
			return
		case buf := <-sc._sendingQueue:
			err := sc._conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
			if err != nil {
				return
			}
			buf.Release()
		}
	}
}

func (sc *WebSocketConnection) recvLoop() {
	defer sc._waitGroup.Done()
	for {
		mt, rd, err := sc._conn.NextReader()
		if err != nil {
			logger.Debug("read: %v", err)
			sc._quitChan <- 1
			break
		}

		//if mt == websocket.BinaryMessage {
		var msg *misc.Buffer
		if sc._cfg.BufferPool != nil {
			msg = sc._cfg.BufferPool.Get()
		} else {
			msg = misc.NewBuffer(sc._cfg.BufferSize)
		}
		n, _ := rd.Read(msg.Buffer())
		if n <= 0 {
			msg.Release()
			break
		}
		msg.SetDataLen(n)
		msg.Seek(0, 0)
		if sc._cfg.OnMessage != nil {
			sc._cfg.OnMessage(sc, mt, msg.AddRef(), sc._cfg.Attachment)
		}
		msg.Release()
		//}
	}
}
