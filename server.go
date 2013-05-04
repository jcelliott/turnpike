// Package turnpike provides a Websocket Application Messaging Protocol (WAMP) server and client
package turnpike

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"github.com/nu7hatch/gouuid"
	"log"
	"sync"
	"turnpike/wamp"
)

var serverBacklog = 10

type Server struct {
	clients map[string]chan<- []byte
	// this is a map because it cheaply prevents a client from subscribing multiple times
	// the nice side-effect here is it's easy to unsubscribe
	subscriptions map[string]map[string]bool
	subLock       *sync.Mutex
}

func NewServer() *Server {
	return &Server{
		clients:       make(map[string]chan<- []byte),
		subscriptions: make(map[string]map[string]bool),
		subLock:       new(sync.Mutex)}
}

func (t *Server) handlePrefix(id string, msg wamp.PrefixMsg) {
	panic("not implemented")
}

func (t *Server) handleCall(id string, msg wamp.CallMsg) {
	panic("not implemented")
}

func (t *Server) handleSubscribe(id string, msg wamp.SubscribeMsg) {
	t.subLock.Lock()
	evt := msg.TopicURI
	if _, ok := t.subscriptions[evt]; !ok {
		t.subscriptions[evt] = make(map[string]bool)
	}
	t.subscriptions[evt][id] = true
	t.subLock.Unlock()
}

func (t *Server) handleUnsubscribe(id string, msg wamp.UnsubscribeMsg) {
	t.subLock.Lock()
	if tmap, ok := t.subscriptions[msg.TopicURI]; ok {
		delete(tmap, id)
	}
	t.subLock.Unlock()
}

func (t *Server) handlePublish(id string, msg wamp.PublishMsg) {
	tmap, ok := t.subscriptions[msg.TopicURI]
	if !ok {
		return
	}

	out, err := wamp.Event(msg.TopicURI, msg.Event)
	if err != nil {
		log.Println("Error creating event message:", err)
		return
	}

	var sendTo []string
	if len(msg.ExcludeList) > 0 || len(msg.EligibleList) > 0 {
		// this is super ugly, but I couldn't think of a better way...
		for tid := range tmap {
			include := true
			for _, _tid := range msg.ExcludeList {
				if tid == _tid {
					include = false
					break
				}
			}
			if include {
				sendTo = append(sendTo, tid)
			}
		}

		for _, tid := range msg.EligibleList {
			include := true
			for _, _tid := range sendTo {
				if _tid == tid {
					include = false
					break
				}
			}
			if include {
				sendTo = append(sendTo, tid)
			}
		}
	} else {
		for tid := range tmap {
			if tid == id && msg.ExcludeMe {
				continue
			}
			sendTo = append(sendTo, tid)
		}
	}

	for _, tid := range sendTo {
		// we're not locking anything, so we need
		// to make sure the client didn't disconnecct in the
		// last few nanoseconds...
		if client, ok := t.clients[tid]; ok {
			client <- out
		}
	}
}

func (t *Server) HandleWebsocket(conn *websocket.Conn) {
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

	c := make(chan []byte, serverBacklog)
	t.clients[id] = c

	go func() {
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
