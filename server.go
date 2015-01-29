// Copyright (c) 2013 Joshua Elliott
// Released under the MIT License
// http://opensource.org/licenses/MIT

package turnpike

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"

	"github.com/nu7hatch/gouuid"
)

var (
	// The amount of messages to buffer before sending to client.
	serverBacklog = 20
)

const (
	clientConnTimeout = 6
	clientMaxFailures = 3
)

// Server represents a WAMP server that handles RPC and pub/sub.
type Server struct {
	// Client ID -> send channel
	clients map[string]chan string
	// Client ID -> prefix mapping
	prefixes map[string]prefixMap
	// Proc URI -> handler
	rpcHandlers map[string]RPCHandler
	subHandlers map[string]SubHandler
	pubHandlers map[string]PubHandler
	// Topic URI -> subscribed clients
	subscriptions       map[string]listenerMap
	subLock             *sync.Mutex
	sessionOpenCallback func(string)
	websocket.Server
}

// RPCHandler is an interface that handlers to RPC calls should implement.
// The first parameter is the call ID, the second is the proc URI. Last comes
// all optional arguments to the RPC call. The return can be of any type that
// can be marshaled to JSON, or a error (preferably RPCError but any error works.)
// NOTE: this may be broken in v2 if multiple-return is implemented
type RPCHandler func(clientID string, topicURI string, args ...interface{}) (interface{}, error)

// RPCError represents a call error and is the recommended way to return an
// error from a RPC handler.
type RPCError struct {
	URI         string
	Description string
	Details     interface{}
}

// Error returns an error description.
func (e RPCError) Error() string {
	return fmt.Sprintf("turnpike: RPC error with URI %s: %s", e.URI, e.Description)
}

// SubHandler is an interface that handlers for subscriptions should implement to
// control with subscriptions are valid. A subscription is allowed by returning
// true or denied by returning false.
type SubHandler func(clientID string, topicURI string) bool

// PubHandler is an interface that handlers for publishes should implement to
// get notified on a client publish with the possibility to modify the event.
// The event that will be published should be returned.
type PubHandler func(topicURI string, event interface{}) interface{}

// NewServer creates a new WAMP server.
func NewServer() *Server {
	s := &Server{
		clients:       make(map[string]chan string),
		prefixes:      make(map[string]prefixMap),
		rpcHandlers:   make(map[string]RPCHandler),
		subHandlers:   make(map[string]SubHandler),
		pubHandlers:   make(map[string]PubHandler),
		subscriptions: make(map[string]listenerMap),
		subLock:       new(sync.Mutex),
	}
	s.Server = websocket.Server{
		Handshake: checkWAMPHandshake,
		Handler:   websocket.Handler(s.HandleWebsocket),
	}
	return s
}

// SetSessionOpenCallback adds a callback function that is run when a new session begins.
// The callback function must accept a string argument that is the session ID.
func (t *Server) SetSessionOpenCallback(f func(string)) {
	t.sessionOpenCallback = f
}

// RegisterRPC adds a handler for the RPC named uri.
func (t *Server) RegisterRPC(uri string, f RPCHandler) {
	if f != nil {
		t.rpcHandlers[uri] = f
	}
}

// UnregisterRPC removes a handler for the RPC named uri.
func (t *Server) UnregisterRPC(uri string) {
	delete(t.rpcHandlers, uri)
}

// RegisterSubHandler adds a handler called when a client subscribes to URI.
// The subscription can be canceled in the handler by returning false, or
// approved by returning true.
func (t *Server) RegisterSubHandler(uri string, f SubHandler) {
	if f != nil {
		t.subHandlers[uri] = f
	}
}

// UnregisterSubHandler removes a subscription handler for the URI.
func (t *Server) UnregisterSubHandler(uri string) {
	delete(t.subHandlers, uri)
}

// RegisterPubHandler adds a handler called when a client publishes to URI.
// The event can be modified in the handler and the returned event is what is
// published to the other clients.
func (t *Server) RegisterPubHandler(uri string, f PubHandler) {
	if f != nil {
		t.pubHandlers[uri] = f
	}
}

// UnregisterPubHandler removes a publish handler for the URI.
func (t *Server) UnregisterPubHandler(uri string) {
	delete(t.pubHandlers, uri)
}

// SendEvent sends an event with topic directly (not via Client.Publish())
func (t *Server) SendEvent(topic string, event interface{}) {
	t.handlePublish(topic, publishMsg{
		TopicURI: topic,
		Event:    event,
	})
}

// ConnectedClients returns a slice of the ids of all connected clients
func (t *Server) ConnectedClients() []string {
	clientIDs := []string{}
	for id, _ := range t.clients {
		clientIDs = append(clientIDs, id)
	}
	return clientIDs
}

// HandleWebsocket implements the go.net/websocket.Handler interface.
func (t *Server) HandleWebsocket(conn *websocket.Conn) {
	defer conn.Close()

	if debug {
		log.Print("turnpike: received websocket connection")
	}

	tid, err := uuid.NewV4()
	if err != nil {
		if debug {
			log.Print("turnpike: could not create unique id, refusing client connection")
		}
		return
	}
	id := tid.String()
	if debug {
		log.Printf("turnpike: client connected: %s", id)
	}

	arr, err := createWelcome(id, turnpikeServerIdent)
	if err != nil {
		if debug {
			log.Print("turnpike: error encoding welcome message")
		}
		return
	}
	if debug {
		log.Printf("turnpike: sending welcome message: %s", arr)
	}
	err = websocket.Message.Send(conn, string(arr))
	if err != nil {
		if debug {
			log.Printf("turnpike: error sending welcome message, aborting connection: %s", err)
		}
		return
	}

	c := make(chan string, serverBacklog)
	t.clients[id] = c

	if t.sessionOpenCallback != nil {
		t.sessionOpenCallback(id)
	}

	failures := 0
	go func() {
		for msg := range c {
			if debug {
				log.Printf("turnpike: sending message: %s", msg)
			}
			conn.SetWriteDeadline(time.Now().Add(clientConnTimeout * time.Second))
			err := websocket.Message.Send(conn, msg)
			if err != nil {
				if nErr, ok := err.(net.Error); ok && (nErr.Timeout() || nErr.Temporary()) {
					log.Printf("Network error: %s", nErr)
					failures++
					if failures > clientMaxFailures {
						break
					}
				} else {
					if debug {
						log.Printf("turnpike: error sending message: %s", err)
					}
					break
				}
			}
		}
		if debug {
			log.Printf("Client %s disconnected", id)
		}
		conn.Close()
	}()

	for {
		var rec string
		err := websocket.Message.Receive(conn, &rec)
		if err != nil {
			if err != io.EOF {
				if debug {
					log.Printf("turnpike: error receiving message, aborting connection: %s", err)
				}
			}
			break
		}
		if debug {
			log.Printf("turnpike: message received: %s", rec)
		}

		data := []byte(rec)

		switch typ := parseMessageType(rec); typ {
		case msgPrefix:
			var msg prefixMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				if debug {
					log.Printf("turnpike: error unmarshalling prefix message: %s", err)
				}
				continue
			}
			t.handlePrefix(id, msg)
		case msgCall:
			var msg callMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				if debug {
					log.Printf("turnpike: error unmarshalling call message: %s", err)
				}
				continue
			}
			t.handleCall(id, msg)
		case msgSubscribe:
			var msg subscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				if debug {
					log.Printf("turnpike: error unmarshalling subscribe message: %s", err)
				}
				continue
			}
			t.handleSubscribe(id, msg)
		case msgUnsubscribe:
			var msg unsubscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				if debug {
					log.Printf("turnpike: error unmarshalling unsubscribe message: %s", err)
				}
				continue
			}
			t.handleUnsubscribe(id, msg)
		case msgPublish:
			var msg publishMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				if debug {
					log.Printf("turnpike: error unmarshalling publish message: %s", err)
				}
				continue
			}
			t.handlePublish(id, msg)
		case msgWelcome, msgCallResult, msgCallError, msgEvent:
			if debug {
				log.Printf("turnpike: server -> client message received, ignored: %s", messageTypeString(typ))
			}
		default:
			if debug {
				log.Printf("turnpike: invalid message format, message dropped: %s", data)
			}
		}
	}

	delete(t.clients, id)
	close(c)
}

func (t *Server) handlePrefix(id string, msg prefixMsg) {
	if debug {
		log.Print("turnpike: handling prefix message")
	}
	if _, ok := t.prefixes[id]; !ok {
		t.prefixes[id] = make(prefixMap)
	}
	if err := t.prefixes[id].registerPrefix(msg.Prefix, msg.URI); err != nil {
		if debug {
			log.Printf("turnpike: error registering prefix: %s", err)
		}
	}
	if debug {
		log.Printf("turnpike: client %s registered prefix '%s' for URI: %s", id, msg.Prefix, msg.URI)
	}
}

func (t *Server) handleCall(id string, msg callMsg) {
	if debug {
		log.Print("turnpike: handling call message")
	}

	var out string
	var err error

	if f, ok := t.rpcHandlers[msg.ProcURI]; ok && f != nil {
		var res interface{}
		res, err = f(id, msg.ProcURI, msg.CallArgs...)
		if err != nil {
			var errorURI, desc string
			var details interface{}
			if er, ok := err.(RPCError); ok {
				errorURI = er.URI
				desc = er.Description
				details = er.Details
			} else {
				errorURI = msg.ProcURI + "#generic-error"
				desc = err.Error()
			}

			if details != nil {
				out, err = createCallError(msg.CallID, errorURI, desc, details)
			} else {
				out, err = createCallError(msg.CallID, errorURI, desc)
			}
		} else {
			out, err = createCallResult(msg.CallID, res)
		}
	} else {
		if debug {
			log.Printf("turnpike: RPC call not registered: %s", msg.ProcURI)
		}
		out, err = createCallError(msg.CallID, "error:notimplemented", "RPC call '%s' not implemented", msg.ProcURI)
	}

	if err != nil {
		// whatever, let the client hang...
		if debug {
			log.Printf("turnpike: error creating callError message: %s", err)
		}
		return
	}
	if client, ok := t.clients[id]; ok {
		client <- out
	}
}

func (t *Server) handleSubscribe(id string, msg subscribeMsg) {
	if debug {
		log.Print("turnpike: handling subscribe message")
	}

	uri := checkCurie(t.prefixes[id], msg.TopicURI)
	h := t.getSubHandler(uri)
	if h != nil && !h(id, uri) {
		if debug {
			log.Printf("turnpike: client %s denied subscription of topic: %s", id, uri)
		}
		return
	}

	t.subLock.Lock()
	defer t.subLock.Unlock()
	if _, ok := t.subscriptions[uri]; !ok {
		t.subscriptions[uri] = make(map[string]bool)
	}
	t.subscriptions[uri].add(id)
	if debug {
		log.Printf("turnpike: client %s subscribed to topic: %s", id, uri)
	}
}

func (t *Server) handleUnsubscribe(id string, msg unsubscribeMsg) {
	if debug {
		log.Print("turnpike: handling unsubscribe message")
	}
	t.subLock.Lock()
	uri := checkCurie(t.prefixes[id], msg.TopicURI)
	if lm, ok := t.subscriptions[uri]; ok {
		lm.remove(id)
	}
	t.subLock.Unlock()
	if debug {
		log.Printf("turnpike: client %s unsubscribed from topic: %s", id, uri)
	}
}

func (t *Server) handlePublish(id string, msg publishMsg) {
	if debug {
		log.Print("turnpike: handling publish message")
	}
	uri := checkCurie(t.prefixes[id], msg.TopicURI)

	h := t.getPubHandler(uri)
	event := msg.Event
	if h != nil {
		event = h(uri, event)
	}

	lm, ok := t.subscriptions[uri]
	if !ok {
		return
	}

	out, err := createEvent(uri, event)
	if err != nil {
		if debug {
			log.Printf("turnpike: error creating event message: %s", err)
		}
		return
	}

	var sendTo []string
	if len(msg.ExcludeList) > 0 || len(msg.EligibleList) > 0 {
		// this is super ugly, but I couldn't think of a better way...
		for tid := range lm {
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
		for tid := range lm {
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
			if len(client) == cap(client) {
				<-client
			}
			client <- string(out)
		}
	}
}

func (t *Server) getSubHandler(uri string) SubHandler {
	for i := len(uri); i >= 0; i-- {
		u := uri[:i]
		if h, ok := t.subHandlers[u]; ok {
			return h
		}
	}
	return nil
}

func (t *Server) getPubHandler(uri string) PubHandler {
	for i := len(uri); i >= 0; i-- {
		u := uri[:i]
		if h, ok := t.pubHandlers[u]; ok {
			return h
		}
	}
	return nil
}

type listenerMap map[string]bool

func (lm listenerMap) add(id string) {
	lm[id] = true
}
func (lm listenerMap) contains(id string) bool {
	return lm[id]
}
func (lm listenerMap) remove(id string) {
	delete(lm, id)
}

func checkWAMPHandshake(config *websocket.Config, req *http.Request) error {
	for _, protocol := range config.Protocol {
		if protocol == "wamp" {
			config.Protocol = []string{protocol}
			return nil
		}
	}
	return websocket.ErrBadWebSocketProtocol
}
