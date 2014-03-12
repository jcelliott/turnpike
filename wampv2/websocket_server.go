package wampv2

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"net/http"
)

const (
	jsonWebsocketProtocol = "wamp.2.json"
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
	payloadType byte
	serializer Serializer
}

// WebsocketServer handles websocket connections.
type WebsocketServer struct {
	server websocket.Server
	router Router

	protocols map[string]protocol

	// The serializer to use for text frames. Defaults to JSONSerializer.
	TextSerializer Serializer
	// The serializer to use for binary frames. Defaults to JSONSerializer.
	BinarySerializer Serializer
}

func NewWebsocketServer(r Router) *WebsocketServer {
	s := &WebsocketServer{
		router: r,
		protocols: make(map[string]protocol),
	}

	s.server = websocket.Server{
		Handshake: s.handshake,
		Handler:   websocket.Handler(s.handleWebsocket),
	}
	return s
}

func (s *WebsocketServer) RegisterProtocol(proto string, payloadType byte, serializer Serializer) error {
	if payloadType != websocket.TextFrame && payloadType != websocket.BinaryFrame {
		return invalidPayload(payloadType)
	}
	if _, ok := s.protocols[proto]; ok {
		return protocolExists(proto)
	}
	s.protocols[proto] = protocol{payloadType, serializer}
	return nil
}

func (s *WebsocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.server.ServeHTTP(w, r)
}

func (s *WebsocketServer) handshake(config *websocket.Config, req *http.Request) error {
	for _, protocol := range config.Protocol {
		if _, ok := s.protocols[protocol]; ok {
			config.Protocol = []string{protocol}
			return nil
		}
		if protocol == jsonWebsocketProtocol || protocol == msgpackWebsocketProtocol {
			config.Protocol = []string{protocol}
			return nil
		}
	}
	return websocket.ErrBadWebSocketProtocol
}

func (s *WebsocketServer) handleWebsocket(conn *websocket.Conn) {
	var serializer Serializer
	var payloadType byte
	for _, proto := range conn.Config().Protocol {
		if protocol, ok := s.protocols[proto]; ok {
			serializer = protocol.serializer
			payloadType = protocol.payloadType
			break
		} else if proto == "wamp.2.json" {
			serializer = new(JSONSerializer)
			payloadType = websocket.TextFrame
			break
		} else if proto == "wamp.2.msgpack" {
			// TODO: implement msgpack
		}
	}
	if serializer == nil {
		conn.Close()
		return
	}

	ep := websocketEndpoint{
		conn: conn,
		serializer: serializer,
		messages: make(chan Message, 10),
		payloadType: payloadType,
	}
	if payloadType == websocket.TextFrame {
		go ep.receiveTextFrames()
	} else if payloadType == websocket.BinaryFrame {
		go ep.receiveBinaryFrames()
	}
	s.router.Accept(&ep)
}
