// Package turnpike provides a Websocket Application Messaging Protocol (WAMP) server and client
package turnpike

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	log "github.com/jcelliott/lumber"
	"github.com/nu7hatch/gouuid"
	"sync"
	"turnpike/wamp"
)

func init() {
	log.Level(log.TRACE)
}

var serverBacklog = 10

type Server struct {
	clients map[string]chan<- string
	// this is a map because it cheaply prevents a client from subscribing multiple times
	// the nice side-effect here is it's easy to unsubscribe
	//               topicID     client
	subscriptions map[string]map[string]bool
	//                client      prefixes
	prefixes map[string]wamp.PrefixMap
	subLock  *sync.Mutex
}

func NewServer() *Server {
	return &Server{
		clients:       make(map[string]chan<- string),
		subscriptions: make(map[string]map[string]bool),
		prefixes:      make(map[string]wamp.PrefixMap),
		subLock:       new(sync.Mutex)}
}

func (t *Server) handlePrefix(id string, msg wamp.PrefixMsg) {
	log.Debug("Handling prefix message")
	if _, ok := t.prefixes[id]; !ok {
		t.prefixes[id] = make(wamp.PrefixMap)
	}
	err := t.prefixes[id].RegisterPrefix(msg.Prefix, msg.URI)
	if err != nil {
		log.Error("Error registering prefix: %s", err)
	}
}

func (t *Server) handleCall(id string, msg wamp.CallMsg) {
	log.Warn("Handling call message - not implemented")
	out, err := wamp.CallError(msg.CallID, "error:notimplemented", "RPC not implemented")
	if err != nil {
		// whatever, let the client hang...
		log.Fatal("Error creating callError message: %s", err)
		return
	}
	if client, ok := t.clients[id]; ok {
		client <- string(out)
	}
}

func (t *Server) handleSubscribe(id string, msg wamp.SubscribeMsg) {
	log.Debug("Handling subscribe message")
	t.subLock.Lock()
	topic := wamp.CheckCurie(t.prefixes[id], msg.TopicURI)
	if _, ok := t.subscriptions[topic]; !ok {
		t.subscriptions[topic] = make(map[string]bool)
	}
	t.subscriptions[topic][id] = true
	t.subLock.Unlock()
}

func (t *Server) handleUnsubscribe(id string, msg wamp.UnsubscribeMsg) {
	log.Debug("Handling unsubscribe message")
	t.subLock.Lock()
	topic := wamp.CheckCurie(t.prefixes[id], msg.TopicURI)
	if tmap, ok := t.subscriptions[topic]; ok {
		delete(tmap, id)
	}
	t.subLock.Unlock()
}

func (t *Server) handlePublish(id string, msg wamp.PublishMsg) {
	log.Debug("Handling publish message")
	topic := wamp.CheckCurie(t.prefixes[id], msg.TopicURI)
	tmap, ok := t.subscriptions[topic]
	if !ok {
		return
	}

	out, err := wamp.Event(topic, msg.Event)
	if err != nil {
		log.Error("Error creating event message: %s", err)
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

	log.Debug("Sending event messages")
	for _, tid := range sendTo {
		// we're not locking anything, so we need
		// to make sure the client didn't disconnecct in the
		// last few nanoseconds...
		if client, ok := t.clients[tid]; ok {
			client <- string(out)
		}
	}
}

func (t *Server) HandleWebsocket(conn *websocket.Conn) {
	defer conn.Close()

	log.Debug("Received websocket connection")

	tid, err := uuid.NewV4()
	if err != nil {
		log.Error("Could not create unique id, refusing client connection")
		return
	}
	id := tid.String()

	arr, err := wamp.Welcome(id)
	if err != nil {
		log.Error("Error encoding welcome message")
		return
	}
	log.Debug("Sending welcome message: %s", arr)
	err = websocket.Message.Send(conn, string(arr))
	if err != nil {
		log.Error("Error sending welcome message, aborting connection: %s", err)
		return
	}

	c := make(chan string, serverBacklog)
	t.clients[id] = c

	go func() {
		for msg := range c {
			log.Trace("Sending message: %s", msg)
			err := websocket.Message.Send(conn, string(msg))
			if err != nil {
				log.Error("Error sending message: %s", err)
			}
		}
	}()

	for {
		var rec string
		err := websocket.Message.Receive(conn, &rec)
		if err != nil {
			log.Error("Error receiving message, aborting connection: %s", err)
			break
		}
		log.Trace("Message received: %s", rec)

		data := []byte(rec)

		switch typ := wamp.ParseType(rec); typ {
		case wamp.TYPE_ID_PREFIX:
			var msg wamp.PrefixMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling prefix message: %s", err)
			}
			t.handlePrefix(id, msg)
		case wamp.TYPE_ID_CALL:
			var msg wamp.CallMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling call message: %s", err)
			}
			t.handleCall(id, msg)
		case wamp.TYPE_ID_SUBSCRIBE:
			var msg wamp.SubscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling subscribe message: %s", err)
			}
			t.handleSubscribe(id, msg)
		case wamp.TYPE_ID_UNSUBSCRIBE:
			var msg wamp.UnsubscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling unsubscribe message: %s", err)
			}
			t.handleUnsubscribe(id, msg)
		case wamp.TYPE_ID_PUBLISH:
			var msg wamp.PublishMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("Error unmarshalling publish message: %s", err)
			}
			t.handlePublish(id, msg)
		case wamp.TYPE_ID_WELCOME, wamp.TYPE_ID_CALLRESULT, wamp.TYPE_ID_CALLERROR, wamp.TYPE_ID_EVENT:
			log.Error("Server -> client message received, ignored: %s", wamp.TypeString(typ))
		default:
			log.Error("Invalid message format, message dropped: %s", data)
		}
	}

	delete(t.clients, id)
	close(c)
}
