package websvc

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func NewConnectionPool() *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return NewWebSocketConnection(nil)
		},
	}
}

func WebSocketTask(url string, cfg *WebSocketConfig) service.ServiceTask {
	return func(wg *sync.WaitGroup, qs chan os.Signal) {
		defer wg.Done()
		cli := NewWebSocketConnection(cfg)
		interval := cfg.ReconnectInterval
		if interval == 0 {
			interval = 5
		}
		reconnectTicker := time.NewTicker(time.Duration(interval) * time.Second)
		for {
			if !cli.Connect(url, qs) {
				return
			}
			select {
			case <-reconnectTicker.C:
				// DO NOTHING
			case s := <-qs:
				cli.Close()
				qs <- s
				return
			}
		}
	}
}

func WebSocketHandler(cfg *WebSocketConfig) gin.HandlerFunc {
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 16384
	}
	webSocketUpgrader := websocket.Upgrader{
		ReadBufferSize:  int(cfg.BufferSize),
		WriteBufferSize: int(cfg.BufferSize),
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return func(c *gin.Context) {
		var connectionAttachment interface{}
		if cfg.BeforeUpgrade != nil {
			code, data, err := cfg.BeforeUpgrade(c, cfg.Attachment)
			if err != nil {
				logger.Error("Error: %+v", err)
				c.AbortWithStatus(500)
				return
			} else if code != 0 {
				processResp(c, code, data)
				return
			}
			connectionAttachment = data
		}
		conn, err := webSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Debug("Failed to upgrade connection: %v", err)
			c.AbortWithStatus(500)
			return
		}

		var cli *WebSocketConnection
		if cfg.ConnectionPool != nil {
			cli = cfg.ConnectionPool.Get().(*WebSocketConnection)
		} else {
			cli = NewWebSocketConnection(cfg)
		}

		cli.Attachment = connectionAttachment
		cli._conn = conn
		cli._pool = cfg.ConnectionPool
		cli._refCount = 1
		cli._cfg = cfg

		if cfg.OnConnected != nil {
			cfg.OnConnected(cli, cfg.Attachment)
		}
		cli.run(nil)
		cli.Release()

	}
}
