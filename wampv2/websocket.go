package wampv2

import (
	"github.com/gorilla/websocket"
)

type websocketEndpoint struct {
	conn *websocket.Conn
	serializer Serializer
	messages chan Message
	payloadType int
}

func NewJSONWebsocketClient(url, origin string) (Endpoint, error) {
	return newWebsocketClient(url, jsonWebsocketProtocol, origin, &JSONSerializer{}, websocket.TextMessage)
}

func newWebsocketClient(url, protocol, origin string, serializer Serializer, payloadType int) (Endpoint, error) {
	dialer := websocket.Dialer{
		Subprotocols: []string{protocol},
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	ep := &websocketEndpoint{
			conn: conn,
			messages: make(chan Message, 10),
			serializer: serializer,
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
	return ep, nil
}

func (ep *websocketEndpoint) Send(msg Message) error {
	b, err := ep.serializer.Serialize(msg)
	if err != nil {
		return err
	}
	return ep.conn.WriteMessage(ep.payloadType, b)
}
func (ep *websocketEndpoint) Receive() <-chan Message {
	return ep.messages
}
func (ep *websocketEndpoint) Close() error {
	return ep.conn.Close()
}
