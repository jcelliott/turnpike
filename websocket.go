package turnpike

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type websocketPeer struct {
	conn        *websocket.Conn
	serializer  Serializer
	messages    chan Message
	payloadType int
	closed      bool
	sendMutex   sync.Mutex
}

// NewWebsocketPeer connects to the websocket server at the specified url.
func NewWebsocketPeer(serialization Serialization, url, origin string, tlscfg *tls.Config) (Peer, error) {
	switch serialization {
	case JSON:
		return newWebsocketPeer(url, jsonWebsocketProtocol, origin,
			new(JSONSerializer), websocket.TextMessage, tlscfg,
		)
	case MSGPACK:
		return newWebsocketPeer(url, msgpackWebsocketProtocol, origin,
			new(MessagePackSerializer), websocket.BinaryMessage, tlscfg,
		)
	default:
		return nil, fmt.Errorf("Unsupported serialization: %v", serialization)
	}
}

func newWebsocketPeer(url, protocol, origin string, serializer Serializer, payloadType int, tlscfg *tls.Config) (Peer, error) {
	dialer := websocket.Dialer{
		Subprotocols:    []string{protocol},
		TLSClientConfig: tlscfg,
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	ep := &websocketPeer{
		conn:        conn,
		messages:    make(chan Message, 10),
		serializer:  serializer,
		payloadType: payloadType,
	}
	go ep.run()

	return ep, nil
}

// TODO: make this just add the message to a channel so we don't block
func (ep *websocketPeer) Send(msg Message) error {
	b, err := ep.serializer.Serialize(msg)
	if err != nil {
		return err
	}
	ep.sendMutex.Lock()
	defer ep.sendMutex.Unlock()
	return ep.conn.WriteMessage(ep.payloadType, b)
}
func (ep *websocketPeer) Receive() <-chan Message {
	return ep.messages
}
func (ep *websocketPeer) Close() error {
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")
	err := ep.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(5*time.Second))
	if err != nil {
		log.Println("error sending close message:", err)
	}
	ep.closed = true
	return ep.conn.Close()
}

func (ep *websocketPeer) run() {
	for {
		// TODO: use conn.NextMessage() and stream
		// TODO: do something different based on binary/text frames
		if msgType, b, err := ep.conn.ReadMessage(); err != nil {
			if ep.closed {
				log.Println("peer connection closed")
			} else {
				log.Println("error reading from peer:", err)
				ep.conn.Close()
			}
			close(ep.messages)
			break
		} else if msgType == websocket.CloseMessage {
			ep.conn.Close()
			close(ep.messages)
			break
		} else {
			msg, err := ep.serializer.Deserialize(b)
			if err != nil {
				log.Println("error deserializing peer message:", err)
				// TODO: handle error
			} else {
				ep.messages <- msg
			}
		}
	}
}
