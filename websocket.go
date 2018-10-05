package turnpike

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type websocketPeer struct {
	incomeMiddleware func(Message) (Message, error)
	conn             *websocket.Conn
	serializer       Serializer
	messages         chan Message
	payloadType      int
	closed           bool
	sendMutex        sync.Mutex
	ctx              context.Context
}

func NewWebsocketPeer(serialization Serialization, url string, requestHeader http.Header, tlscfg *tls.Config, dial DialFunc) (Peer, error) {
	switch serialization {
	case JSON:
		return newWebsocketPeer(url, requestHeader, jsonWebsocketProtocol,
			new(JSONSerializer), websocket.TextMessage, tlscfg, dial,
		)
	case MSGPACK:
		return newWebsocketPeer(url, requestHeader, msgpackWebsocketProtocol,
			new(MessagePackSerializer), websocket.BinaryMessage, tlscfg, dial,
		)
	default:
		return nil, fmt.Errorf("Unsupported serialization: %v", serialization)
	}
}

func newWebsocketPeer(url string, reqHeader http.Header, protocol string, serializer Serializer, payloadType int, tlscfg *tls.Config, dial DialFunc) (Peer, error) {
	dialer := websocket.Dialer{
		Subprotocols:    []string{protocol},
		TLSClientConfig: tlscfg,
		Proxy:           http.ProxyFromEnvironment,
		NetDial:         dial,
	}
	conn, _, err := dialer.Dial(url, reqHeader)
	if err != nil {
		return nil, err
	}
	ep := &websocketPeer{
		conn:             conn,
		messages:         make(chan Message, 10),
		serializer:       serializer,
		payloadType:      payloadType,
		incomeMiddleware: nil,
		ctx:              context.TODO(),
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

//AddIncomeMiddleware implements preprocess income messages
func (ep *websocketPeer) AddIncomeMiddleware(f func(Message) (Message, error)) {
	ep.sendMutex.Lock()
	ep.incomeMiddleware = f
	ep.sendMutex.Unlock()
}

//GetContext returns context
func (ep *websocketPeer) GetContext() context.Context {
	return ep.ctx
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
				if ep.incomeMiddleware == nil {
					ep.messages <- msg
				} else {
					m, err := ep.incomeMiddleware(msg)
					if err != nil {
						log.Println(err)
					} else {
						ep.messages <- m
					}
				}

			}
		}
	}
}
