package websvc

import (
	"sync"
	"sync/atomic"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/misc"
	"github.com/gorilla/websocket"
)

type WebSocketConnection struct {
	_waitGroup    sync.WaitGroup
	_quitChan     chan int
	_sendingQueue chan *misc.Buffer

	_conn     *websocket.Conn
	_pool     *sync.Pool
	_cfg      *WebSocketHandlerConfig
	_refCount int32
}

func NewWebSocketConnection() *WebSocketConnection {
	return &WebSocketConnection{
		_quitChan:     make(chan int),
		_sendingQueue: make(chan *misc.Buffer, 100),
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

func (sc *WebSocketConnection) Close() {
	if sc._conn != nil {
		sc._conn.Close()
		sc._conn = nil
	}
}

func (sc *WebSocketConnection) run() {
	sc._waitGroup.Add(1)
	go sc.sendLoop()
	sc.recvLoop()
	sc._quitChan <- 1
	sc._waitGroup.Wait()
	sc.Close()
	for {
		select {
		case wm := <-sc._sendingQueue:
			wm.Release()
		case <-sc._quitChan:
			// DO NOTHING
		default:
			return
		}
	}
}

func (sc *WebSocketConnection) sendLoop() {
	defer sc._waitGroup.Done()
	for {
		select {
		case <-sc._quitChan:
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
			break
		}

		if mt == websocket.PingMessage {
			sc._conn.WriteMessage(websocket.PongMessage, nil)
		} else if mt == websocket.BinaryMessage {
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
				sc._cfg.OnMessage(sc, msg.AddRef(), sc._cfg.Attachment)
			}
			msg.Release()
		}
	}
}
