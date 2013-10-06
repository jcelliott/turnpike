package turnpike

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"github.com/nu7hatch/gouuid"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	serverBacklog = 20
)

const (
	clientConnTimeout = 6
	clientMaxFailures = 3
)

// RPCError represents a call error and is the recommended way to return an
// error from a RPC handler.
type RPCError interface {
	error
	URI() string
	Description() string
	Details() interface{}
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

// RPCHandler is an interface for that handlers to RPC calls should implement.
// The first parameter is the call ID, the second is the proc URI. Last comes
// all optional arguments to the RPC call. The return can be of any type that
// can be marshaled to JSON, or a error (preferably RPCError but any error works.)
// NOTE: this may be broken in v2 if multiple-return is implemented
type RPCHandler func(string, string, ...interface{}) (interface{}, error)

// Server represents a WAMP server that handles RPC and pub/sub.
type Server struct {
	clients map[string]chan string
	// this is a map because it cheaply prevents a client from subscribing multiple times
	// the nice side-effect here is it's easy to unsubscribe
	//               topicID  clients
	subscriptions map[string]listenerMap
	//           client    prefixes
	prefixes            map[string]prefixMap
	rpcHooks            map[string]RPCHandler
	sessionOpenCallback func(string)
	subLock             *sync.Mutex
	websocket.Server
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

// NewServer creates a new WAMP server.
func NewServer() *Server {
	s := &Server{
		clients:       make(map[string]chan string),
		subscriptions: make(map[string]listenerMap),
		prefixes:      make(map[string]prefixMap),
		rpcHooks:      make(map[string]RPCHandler),
		subLock:       new(sync.Mutex),
	}
	s.Server = websocket.Server{
		Handshake: checkWAMPHandshake,
		Handler:   websocket.Handler(s.HandleWebsocket),
	}
	return s
}

func (t *Server) handlePrefix(id string, msg prefixMsg) {
	log.Trace("turnpike: handling prefix message")
	if _, ok := t.prefixes[id]; !ok {
		t.prefixes[id] = make(prefixMap)
	}
	if err := t.prefixes[id].registerPrefix(msg.Prefix, msg.URI); err != nil {
		log.Error("turnpike: error registering prefix: %s", err)
	}
	log.Debug("turnpike: client %s registered prefix '%s' for URI: %s", id, msg.Prefix, msg.URI)
}

func (t *Server) handleCall(id string, msg callMsg) {
	log.Trace("turnpike: handling call message")
	var out string
	var err error

	if f, ok := t.rpcHooks[msg.ProcURI]; ok && f != nil {
		var res interface{}
		res, err = f(id, msg.ProcURI, msg.CallArgs...)
		if err != nil {
			var errorURI, desc string
			var details interface{}
			if er, ok := err.(RPCError); ok {
				errorURI = er.URI()
				desc = er.Description()
				details = er.Details()
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
		log.Warn("turnpike: RPC call not registered: %s", msg.ProcURI)
		out, err = createCallError(msg.CallID, "error:notimplemented", "RPC call '%s' not implemented", msg.ProcURI)
	}

	if err != nil {
		// whatever, let the client hang...
		log.Fatal("turnpike: error creating callError message: %s", err)
		return
	}
	if client, ok := t.clients[id]; ok {
		client <- out
	}
}

func (t *Server) handleSubscribe(id string, msg subscribeMsg) {
	log.Trace("turnpike: handling subscribe message")
	t.subLock.Lock()
	topic := checkCurie(t.prefixes[id], msg.TopicURI)
	if _, ok := t.subscriptions[topic]; !ok {
		t.subscriptions[topic] = make(map[string]bool)
	}
	t.subscriptions[topic].add(id)
	t.subLock.Unlock()
	log.Debug("turnpike: client %s subscribed to topic: %s", id, topic)
}

func (t *Server) handleUnsubscribe(id string, msg unsubscribeMsg) {
	log.Trace("turnpike: handling unsubscribe message")
	t.subLock.Lock()
	topic := checkCurie(t.prefixes[id], msg.TopicURI)
	if lm, ok := t.subscriptions[topic]; ok {
		lm.remove(id)
	}
	t.subLock.Unlock()
	log.Debug("turnpike: client %s unsubscribed from topic: %s", id, topic)
}

func (t *Server) handlePublish(id string, msg publishMsg) {
	log.Trace("turnpike: handling publish message")
	topic := checkCurie(t.prefixes[id], msg.TopicURI)
	lm, ok := t.subscriptions[topic]
	if !ok {
		return
	}

	out, err := createEvent(topic, msg.Event)
	if err != nil {
		log.Error("turnpike: error creating event message: %s", err)
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

// HandleWebsocket implements the go.net/websocket.Handler interface.
func (t *Server) HandleWebsocket(conn *websocket.Conn) {
	defer conn.Close()

	log.Debug("turnpike: received websocket connection")

	tid, err := uuid.NewV4()
	if err != nil {
		log.Error("turnpike: could not create unique id, refusing client connection")
		return
	}
	id := tid.String()
	log.Info("turnpike: client connected: %s", id)

	arr, err := createWelcome(id, turnpikeServerIdent)
	if err != nil {
		log.Error("turnpike: error encoding welcome message")
		return
	}
	log.Debug("turnpike: sending welcome message: %s", arr)
	err = websocket.Message.Send(conn, string(arr))
	if err != nil {
		log.Error("turnpike: error sending welcome message, aborting connection: %s", err)
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
			log.Trace("turnpike: sending message: %s", msg)
			conn.SetWriteDeadline(time.Now().Add(clientConnTimeout * time.Second))
			err := websocket.Message.Send(conn, msg)
			if err != nil {
				if nErr, ok := err.(net.Error); ok && (nErr.Timeout() || nErr.Temporary()) {
					log.Warn("Network error: %s", nErr)
					failures++
					if failures > clientMaxFailures {
						break
					}
				} else {
					log.Error("turnpike: error sending message: %s", err)
					break
				}
			}
		}
		log.Info("Client %s disconnected", id)
		conn.Close()
	}()

	for {
		var rec string
		err := websocket.Message.Receive(conn, &rec)
		if err != nil {
			if err != io.EOF {
				log.Error("turnpike: error receiving message, aborting connection: %s", err)
			}
			break
		}
		log.Trace("turnpike: message received: %s", rec)

		data := []byte(rec)

		switch typ := parseMessageType(rec); typ {
		case msgPrefix:
			var msg prefixMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("turnpike: error unmarshalling prefix message: %s", err)
				continue
			}
			t.handlePrefix(id, msg)
		case msgCall:
			var msg callMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("turnpike: error unmarshalling call message: %s", err)
				continue
			}
			t.handleCall(id, msg)
		case msgSubscribe:
			var msg subscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("turnpike: error unmarshalling subscribe message: %s", err)
				continue
			}
			t.handleSubscribe(id, msg)
		case msgUnsubscribe:
			var msg unsubscribeMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("turnpike: error unmarshalling unsubscribe message: %s", err)
				continue
			}
			t.handleUnsubscribe(id, msg)
		case msgPublish:
			var msg publishMsg
			err := json.Unmarshal(data, &msg)
			if err != nil {
				log.Error("turnpike: error unmarshalling publish message: %s", err)
				continue
			}
			t.handlePublish(id, msg)
		case msgWelcome, msgCallResult, msgCallError, msgEvent:
			log.Error("turnpike: server -> client message received, ignored: %s", messageTypeString(typ))
		default:
			log.Error("turnpike: invalid message format, message dropped: %s", data)
		}
	}

	delete(t.clients, id)
	close(c)
}

// SetSessionOpenCallback adds a callback function that is run when a new session begins.
// The callback function must accept a string argument that is the session ID.
func (t *Server) SetSessionOpenCallback(f func(string)) {
	t.sessionOpenCallback = f
}

// RegisterRPC adds a handler for the RPC named uri.
func (t *Server) RegisterRPC(uri string, f RPCHandler) {
	if f != nil {
		t.rpcHooks[uri] = f
	}
}

// UnregisterRPC removes a handler for the RPC named uri.
func (t *Server) UnregisterRPC(uri string) {
	delete(t.rpcHooks, uri)
}

// SendEvent sends an event with topic directly (not via Client.Publish())
func (t *Server) SendEvent(topic string, event interface{}) {
	t.handlePublish(topic, publishMsg{
		TopicURI: topic,
		Event:    event,
	})
}
