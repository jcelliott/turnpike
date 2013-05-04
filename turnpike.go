// Package turnpike provides a Websocket Application Messaging Protocol (WAMP) server and client
package turnpike

import (
	"turnpike/wamp"
	"code.google.com/p/go.net/websocket"
	"github.com/nu7hatch/gouuid"
	"log"
	"encoding/json"
)

var backlog = 10

type WAMPClient struct {
	Out chan<- []byte
	conn *websocket.Conn
}

type Turnpike struct {
	clients map[string]WAMPClient
}

// HandleWebsocket is a Go1.0 shim to imitate Turnpike#HandleWebsocket
func HandleWebsocket(t *Turnpike) func(*websocket.Conn) {
	return func (conn *websocket.Conn) {
		t.HandleWebsocket(conn)
	}
}

func (t *Turnpike) handlePrefix(id string, msg wamp.PrefixMsg) {
	panic("not implemented")
}

func (t *Turnpike) handleCall(id string, msg wamp.CallMsg) {
	panic("not implemented")
}

func (t *Turnpike) handleSubscribe(id string, msg wamp.SubscribeMsg) {
	panic("not implemented")
}

func (t *Turnpike) handleUnsubscribe(id string, msg wamp.UnsubscribeMsg) {
	panic("not implemented")
}

func (t *Turnpike) handlePublish(id string, msg wamp.PublishMsg) {
	panic("not implemented")
}

func (t *Turnpike) HandleWebsocket(conn *websocket.Conn) {
	defer conn.Close()

	tid, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	id := tid.String()

	arr, err := wamp.Welcome(id)
	if err != nil {
		log.Println("Error encoding welcome message")
		return
	}
	err = websocket.Message.Send(conn, arr)
	if err != nil {
		log.Println("Error sending welcome message, aborting connection")
		return
	}

	c := make(chan []byte, backlog)
	t.clients[id] = WAMPClient{c, conn}

	go func () {
		for msg := range c {
			err := websocket.Message.Send(conn, msg)
			if err != nil {
				log.Println("Error sending message:", err)
			}
		}
	}()

	for {
		var data []byte
		err := websocket.Message.Receive(conn, &data)
		if err != nil {
			log.Println("Error receiving message, aborting connection")
			break
		}

		switch typ := wamp.ParseType(data); typ {
		case wamp.TYPE_ID_PREFIX:
			var msg wamp.PrefixMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Println("Error unmarshalling prefix message:", err)
			}
			t.handlePrefix(id, msg)
		case wamp.TYPE_ID_CALL:
			var msg wamp.CallMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Println("Error unmarshalling prefix message:", err)
			}
			t.handleCall(id, msg)
		case wamp.TYPE_ID_SUBSCRIBE:
			var msg wamp.SubscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Println("Error unmarshalling prefix message:", err)
			}
			t.handleSubscribe(id, msg)
		case wamp.TYPE_ID_UNSUBSCRIBE:
			var msg wamp.UnsubscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Println("Error unmarshalling prefix message:", err)
			}
			t.handleUnsubscribe(id, msg)
		case wamp.TYPE_ID_PUBLISH:
			var msg wamp.PublishMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Println("Error unmarshalling prefix message:", err)
			}
			t.handlePublish(id, msg)
		case wamp.TYPE_ID_WELCOME, wamp.TYPE_ID_CALLRESULT, wamp.TYPE_ID_CALLERROR, wamp.TYPE_ID_EVENT:
			log.Println("Server -> client message received, ignored:", typ)
		default:
			log.Println("Invalid message format, message dropped:", string(data))
		}
	}

	delete(t.clients, id)
	close(c)
}
