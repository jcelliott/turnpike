// Package turnpike provides a Websocket Application Messaging Protocol (WAMP) server and client
package turnpike

import (
	"code.google.com/p/go.net/websocket"
	"github.com/jcelliott/lumber"
)

var (
	log lumber.Logger = lumber.NewConsoleLogger(lumber.INFO)
)

const (
	turnpikeVersion     = "0.2.0"
	turnpikeServerIdent = "turnpike-" + turnpikeVersion
)

// Handler is a interface to support Go1.0.
type Handler interface {
	HandleWebsocket(*websocket.Conn)
}

// HandleWebsocket is a Go1.0 shim for method values.
func HandleWebsocket(t Handler) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		t.HandleWebsocket(conn)
	}
}

// Set the logger to an externally provided one.
func SetLogger(newLog lumber.Logger) {
	log = newLog
}
