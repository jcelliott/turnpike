package turnpike

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

const (
	jsonWebsocketProtocol    = "wamp.2.json"
	msgpackWebsocketProtocol = "wamp.2.msgpack"
)

type invalidPayload byte

func (e invalidPayload) Error() string {
	return fmt.Sprintf("Invalid payloadType: %d", e)
}

type protocolExists string

func (e protocolExists) Error() string {
	return "This protocol has already been registered: " + string(e)
}

type protocol struct {
	payloadType int
	serializer  Serializer
}

// WebsocketServer handles websocket connections.
type WebsocketServer struct {
	upgrader *websocket.Upgrader
	router   Router

	protocols map[string]protocol

	// The serializer to use for text frames. Defaults to JSONSerializer.
	TextSerializer Serializer
	// The serializer to use for binary frames. Defaults to JSONSerializer.
	BinarySerializer Serializer
}

func NewWebsocketServer(r Router) *WebsocketServer {
	s := &WebsocketServer{
		router:    r,
		protocols: make(map[string]protocol),
	}

	s.upgrader = &websocket.Upgrader{}
	return s
}

func (s *WebsocketServer) RegisterProtocol(proto string, payloadType int, serializer Serializer) error {
	if payloadType != websocket.TextMessage && payloadType != websocket.BinaryMessage {
		return invalidPayload(payloadType)
	}
	if _, ok := s.protocols[proto]; ok {
		return protocolExists(proto)
	}
	s.protocols[proto] = protocol{payloadType, serializer}
	s.upgrader.Subprotocols = append(s.upgrader.Subprotocols, proto)
	return nil
}

func (s *WebsocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: subprotocol?
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error:", err)
	}
	s.handleWebsocket(conn)
}

func (s *WebsocketServer) handleWebsocket(conn *websocket.Conn) {
	var serializer Serializer
	var payloadType int
	if proto, ok := s.protocols[conn.Subprotocol()]; ok {
		serializer = proto.serializer
		payloadType = proto.payloadType
	} else {
		// TODO: this will not currently ever be hit because
		//       gorilla/websocket will reject the conncetion
		//       if the subprotocol isn't registered
		switch conn.Subprotocol() {
		case jsonWebsocketProtocol:
			serializer = new(JSONSerializer)
			payloadType = websocket.TextMessage
		case msgpackWebsocketProtocol:
			serializer = new(MessagePackSerializer)
			payloadType = websocket.BinaryMessage
		default:
			conn.Close()
			return
		}
	}

	ep := websocketEndpoint{
		conn:        conn,
		serializer:  serializer,
		messages:    make(chan Message, 10),
		payloadType: payloadType,
	}
	go func() {
		for {
			// TODO: use conn.NextMessage() and stream
			// TODO: do something different based on binary/text frames
			if _, b, err := conn.ReadMessage(); err != nil {
				conn.Close()
				break
			} else {
				msg, err := serializer.Deserialize(b)
				if err != nil {
					// TODO: handle error
				} else {
					ep.messages <- msg
				}
			}
		}
	}()
	s.router.Accept(&ep)
}
