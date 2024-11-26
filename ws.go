package websvc

import (
	"net/http"
	"os"
	"sync"

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

func WebSocketTask(url string, cfg *WebSocketHandlerConfig) service.ServiceTask {
	return func(wg *sync.WaitGroup, qs chan os.Signal) {
		defer wg.Done()
		cli := NewWebSocketConnection(cfg)
		for {
			if cli.Connect(url, qs) {
				break
			}
		}
	}
}

func WebSocketHandler(cfg *WebSocketHandlerConfig) gin.HandlerFunc {
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
		if cfg.BeforeUpgrade != nil {
			code, data, err := cfg.BeforeUpgrade(c, cfg.Attachment)
			if err == nil && code != 0 {
				processResp(c, code, data)
			} else {
				logger.Error("Error: %+v", err)
				c.AbortWithStatus(500)
				return
			}
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

		cli._conn = conn
		cli._pool = cfg.ConnectionPool
		cli._refCount = 1
		cli._cfg = cfg
		cli._triggerBeat = false // Disable heartbeat sender for server side

		if cfg.OnConnected != nil {
			cfg.OnConnected(cli, cfg.Attachment)
		}
		cli.run(nil)
		if cfg.OnDisconnected != nil {
			cfg.OnDisconnected(cli, cfg.Attachment)
		}
		cli.Release()

	}
}