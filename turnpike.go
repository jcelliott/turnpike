// Package turnpike provides a Websocket Application Messaging Protocol (WAMP) server and client
package turnpike

import (
	"code.google.com/p/go.net/websocket"
)

type Handler interface {
	HandleWebsocket(*websocket.Conn)
}

// HandleWebsocket is a Go1.0 shim for method values
func HandleWebsocket(t Handler) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		t.HandleWebsocket(conn)
	}
}
