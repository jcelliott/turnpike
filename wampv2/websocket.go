package wampv2

import "code.google.com/p/go.net/websocket"

type websocketEndpoint struct {
	conn *websocket.Conn
	serializer Serializer
	messages chan Message
	payloadType byte
}

func NewJSONWebsocketClient(url, origin string) (Endpoint, error) {
	return newWebsocketClient(url, jsonWebsocketProtocol, origin, &JSONSerializer{}, websocket.TextFrame)
}

func newWebsocketClient(url, protocol, origin string, serializer Serializer, payloadType byte) (Endpoint, error) {
	conn, err := websocket.Dial(url, protocol, origin)
	if err != nil {
		return nil, err
	}
	ep := &websocketEndpoint{
			conn: conn,
			messages: make(chan Message, 10),
			serializer: serializer,
			payloadType: payloadType,
	}
	switch payloadType {
	case websocket.TextFrame:
		go ep.receiveTextFrames()
	case websocket.BinaryFrame:
		go ep.receiveBinaryFrames()
	}
	return ep, nil
}

func (ep *websocketEndpoint) receiveTextFrames() error {
	for {
		var str string
		if err := websocket.Message.Receive(ep.conn, &str); err != nil {
			// TODO: check if it's a closing err
			return err
		}
		if msg, err := ep.serializer.Deserialize([]byte(str)); err != nil {
			// TODO: handle error
		} else {
			ep.messages <- msg
		}
	}
	return nil
}

func (ep *websocketEndpoint) receiveBinaryFrames() (err error) {
	for {
		var b []byte
		if err := websocket.Message.Receive(ep.conn, &b); err != nil {
			// TODO: check if it's a closing err
			return err
		}
		if msg, err := ep.serializer.Deserialize(b); err != nil {
			// TODO: handle error
		} else {
			ep.messages <- msg
		}
	}
	return nil
}

func (ep *websocketEndpoint) Send(msg Message) error {
	b, err := ep.serializer.Serialize(msg)
	if err != nil {
		return err
	}
	switch ep.payloadType {
	case websocket.TextFrame:
		return websocket.Message.Send(ep.conn, string(b))
	case websocket.BinaryFrame:
		return websocket.Message.Send(ep.conn, b)
	}
	return nil
}
func (ep *websocketEndpoint) Receive() <-chan Message {
	return ep.messages
}
func (ep *websocketEndpoint) Close() error {
	return ep.conn.Close()
}
