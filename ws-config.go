package websvc

import (
	"sync"

	"github.com/acsl-go/misc"
	"github.com/gin-gonic/gin"
)

type WebSocketConfig struct {
	// [Optional] Used to do before upgrade operations, such as authentication
	// If the upgrade is allowed, return 0, <attachment>, nil, the <attachment> will be set to the Attachment of the connection object
	// If the upgrade is not allowed, the return data will be processed as response.
	BeforeUpgrade func(ctx *gin.Context, attachment interface{}) (int, interface{}, error)

	// [Optional] Connected event processor
	// Initialize logic data for the connection
	OnConnected func(conn *WebSocketConnection, attachment interface{})

	// [Optional] Disconnected event processor
	OnDisconnected func(conn *WebSocketConnection, attachment interface{})

	OnMessage func(conn *WebSocketConnection, msgType int, msg *misc.Buffer, attachment interface{})

	// [Optional] The connection pool for websocket connections
	// A connection poll could be created by NewConnectionPool() function
	// If specified, the connections will be pooled and reused, this will improve performance but may need more memory
	// If set to nil, the connections will not be pooled, this will reduce memory usage but may decrease performance
	ConnectionPool *sync.Pool

	// [Optional] The buffer pool for websocket connections
	// A buffer pool could be created by misc.NewBufferPool() function
	// If specified, the buffers will be pooled and reused, this will improve performance but may need more memory
	// If set to nil, the buffers will not be pooled, this will reduce memory usage but may decrease performance
	BufferPool *misc.BufferPool

	// [Optional] The buffer size for websocket connections
	// If BufferPool is nil, this value must be specified and will be used to create a buffer pool
	BufferSize uint

	// [Optional] The heartbeat interval in seconds
	// If set to 0, the default value 10s will be used
	// Heartbeat will be triggered by the client side
	BeatInterval int

	// [Optional] The heartbeat timeout in seconds
	// If set to 0, the heartbeat timeout will be set to BeatInterval * 3
	BeatTimeout int

	// [Optional] The reconnect interval in seconds, client side only
	// If set to 0, the default value 5s will be used
	ReconnectInterval int

	// User-defined attachment
	// The attachment will be passed to all handle functions in this config
	Attachment interface{}
}
