package websvc

import (
	"context"
	"testing"

	"github.com/acsl-go/misc"
	"github.com/acsl-go/service"
	"github.com/gin-gonic/gin"
)

func TestWSServer(t *testing.T) {
	service.Run(HttpTask("test", ":11771", func(ctx context.Context, router *gin.Engine) {
		router.GET("/ws", WebSocketHandler(ctx, &WebSocketConfig{
			OnConnected: func(conn *WebSocketConnection, attachment interface{}) {
				println("Connected")
			},
			OnDisconnected: func(conn *WebSocketConnection, attachment interface{}) {
				println("Disconnected")
			},
			OnMessage: func(conn *WebSocketConnection, mt int, msg *misc.Buffer, attachment interface{}) {
				println("Message " + string(msg.Bytes()))
			},
			BeatInterval: 1000,
		}))
	}))
	service.Start()
}

func TestWSClient(t *testing.T) {
	service.Run(WebSocketTask(&WebSocketConfig{
		RemoteURL: "ws://localhost:11771/ws",
		OnConnected: func(conn *WebSocketConnection, attachment interface{}) {
			println("Connected")
			buf := misc.NewBuffer(1024)
			buf.Write([]byte("Hello"))
			conn.Send(buf)
		},
		OnDisconnected: func(conn *WebSocketConnection, attachment interface{}) {
			println("Disconnected")
		},
		OnMessage: func(conn *WebSocketConnection, mt int, msg *misc.Buffer, attachment interface{}) {
			println("Message")
		},
		BeatInterval: 1000,
	}))
	service.Start()
}
