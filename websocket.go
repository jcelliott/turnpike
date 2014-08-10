package turnpike

import (
	"github.com/gorilla/websocket"
)

type websocketClient struct {
	conn        *websocket.Conn
	serializer  Serializer
	messages    chan Message
	payloadType int
}

func NewJSONWebsocketClient(url, origin string) (Client, error) {
	return newWebsocketClient(url, jsonWebsocketProtocol, origin, new(JSONSerializer), websocket.TextMessage)
}

func NewMessagePackWebsocketClient(url, origin string) (Client, error) {
	return newWebsocketClient(url, msgpackWebsocketProtocol, origin, new(MessagePackSerializer), websocket.BinaryMessage)
}

func newWebsocketClient(url, protocol, origin string, serializer Serializer, payloadType int) (Client, error) {
	dialer := websocket.Dialer{
		Subprotocols: []string{protocol},
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	ep := &websocketClient{
		conn:        conn,
		messages:    make(chan Message, 10),
		serializer:  serializer,
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

func (ep *websocketClient) Send(msg Message) error {
	b, err := ep.serializer.Serialize(msg)
	if err != nil {
		return err
	}
	return ep.conn.WriteMessage(ep.payloadType, b)
}
func (ep *websocketClient) Receive() <-chan Message {
	return ep.messages
}
func (ep *websocketClient) Close() error {
	return ep.conn.Close()
}
