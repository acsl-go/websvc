package websvc

import (
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/misc"
	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

type WebSocketConnection struct {
	Attachment    interface{}
	_waitGroup    sync.WaitGroup
	_quitChan     chan int
	_sendingQueue chan *misc.Buffer

	_conn     *websocket.Conn
	_pool     *sync.Pool
	_cfg      *WebSocketConfig
	_lastBeat int64
	_refCount int32
}

func NewWebSocketConnection(cfg *WebSocketConfig) *WebSocketConnection {
	return &WebSocketConnection{
		_quitChan:     make(chan int, 5),
		_sendingQueue: make(chan *misc.Buffer, 100),
		_conn:         nil,
		_pool:         nil,
		_cfg:          cfg,
		_refCount:     1,
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

	dialer := websocket.DefaultDialer

	if sc._cfg.Socks5Proxy != "" {

		socks5Dialer, err := proxy.SOCKS5("tcp", sc._cfg.Socks5Proxy, nil, proxy.Direct)
		if err != nil {
			logger.Error("websvc:ws:dial %s failed: %s", url, err.Error())
			if sc._cfg.OnDisconnected != nil {
				sc._cfg.OnDisconnected(sc, sc._cfg.Attachment)
			}
			return false
		}

		dialer = &websocket.Dialer{
			NetDial: func(network, addr string) (net.Conn, error) {
				return socks5Dialer.Dial(network, addr)
			},
		}

	}

	conn, _, e := dialer.Dial(url, headers)
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
	buf := sc._alloc_buffer()
	buf.Write(data)
	sc.SendBinaryBuffer(buf)
}

func (sc *WebSocketConnection) SendText(msg string) {
	buf := sc._alloc_buffer()
	buf.Write([]byte(msg))
	sc.SendTextBuffer(buf)
}

func (sc *WebSocketConnection) SendJson(data interface{}) {
	buf := sc._alloc_buffer()
	buf.WriteJson(data)
	sc.SendTextBuffer(buf)
}

func (sc *WebSocketConnection) run(qs chan os.Signal) bool {
	sc._lastBeat = time.Now().UnixMilli()
	sc._conn.SetPingHandler(func(appData string) error {
		sc._lastBeat = time.Now().UnixMilli()
		buf := sc._alloc_buffer()
		buf.Tag = websocket.PongMessage
		sc._sendingQueue <- buf
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
	beatInterval := time.Millisecond * time.Duration(sc._cfg.BeatInterval)
	if beatInterval == 0 {
		beatInterval = time.Second * 10
	}

	nextPing := int64(0x7FFFFFFFFFFFFFFF)
	if sc._cfg.PingInterval == 0 {
		nextPing = time.Now().UnixMilli() + int64(sc._cfg.PingInterval)
	}

	beatTimout := int64(0x7FFFFFFFFFFFFFFF)
	if sc._cfg.BeatTimeout > 0 {
		beatTimout = int64(sc._cfg.BeatTimeout)
	}

	tick := time.NewTicker(beatInterval)
	for {
		select {
		case s := <-sc._quitChan:
			sc._quitChan <- s
			return
		case <-tick.C:
			ts := time.Now().UnixMilli()
			if sc._cfg.OnBeat != nil {
				sc._cfg.OnBeat(sc, sc._cfg.Attachment)
			}

			if ts >= nextPing {
				buf := sc._alloc_buffer()
				buf.Tag = websocket.PingMessage
				sc._sendingQueue <- buf
				if sc._cfg.OnSendPing != nil {
					sc._cfg.OnSendPing(sc, sc._cfg.Attachment)
				}
				nextPing = ts + int64(sc._cfg.PingInterval)
			}

			if ts-sc._lastBeat > beatTimout {
				sc.Close()
				return
			}
		}
	}
}

func (sc *WebSocketConnection) sendLoop() {
	defer sc._waitGroup.Done()
	conn := sc._conn
	for {
		select {
		case s := <-sc._quitChan:
			sc._quitChan <- s
			return
		case buf := <-sc._sendingQueue:
			if buf.Tag == websocket.PingMessage {
				conn.WriteMessage(websocket.PingMessage, nil)
			} else if buf.Tag == websocket.PongMessage {
				conn.WriteMessage(websocket.PongMessage, nil)
			} else {
				if buf.Tag == 0 {
					buf.Tag = websocket.TextMessage
				}
				err := conn.WriteMessage(buf.Tag, buf.Bytes())
				if err != nil {
					buf.Release()
					return
				}
			}
			buf.Release()
		}
	}
}

func (sc *WebSocketConnection) recvLoop() {
	defer sc._waitGroup.Done()
	conn := sc._conn
	for {
		mt, rd, err := conn.NextReader()
		if err != nil {
			sc._quitChan <- 1
			break
		}

		msg := sc._alloc_buffer()
		p := 0
		buf := msg.Buffer()
		for {
			n, _ := rd.Read(buf[p:])
			if n <= 0 {
				break
			}
			p += n
			if p >= len(buf) {
				break
			}
		}
		msg.SetDataLen(p)
		msg.Seek(0, 0)
		if sc._cfg.OnMessage != nil {
			sc._cfg.OnMessage(sc, mt, msg.AddRef(), sc._cfg.Attachment)
		}
		msg.Release()
	}
}

func (sc *WebSocketConnection) _alloc_buffer() *misc.Buffer {
	var buf *misc.Buffer
	if sc._cfg.BufferPool != nil {
		buf = sc._cfg.BufferPool.Get()
	} else {
		buf = misc.NewBuffer(0)
	}
	return buf
}
