// The MIT License (MIT)

// Copyright (c) 2013 Joshua Elliott

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package turnpike

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	"github.com/nu7hatch/gouuid"
	"io"
	"log"
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
type RPCError struct {
	URI         string
	Description string
	Details     interface{}
}

// Error returns an error description.
func (e RPCError) Error() string {
	return fmt.Sprintf("turnpike: RPC error with URI %s: %s", e.URI, e.Description)
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

	if f, ok := t.rpcHooks[msg.ProcURI]; ok && f != nil {
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
	t.subLock.Lock()
	topic := checkCurie(t.prefixes[id], msg.TopicURI)
	if _, ok := t.subscriptions[topic]; !ok {
		t.subscriptions[topic] = make(map[string]bool)
	}
	t.subscriptions[topic].add(id)
	t.subLock.Unlock()
	if debug {
		log.Printf("turnpike: client %s subscribed to topic: %s", id, topic)
	}
}

func (t *Server) handleUnsubscribe(id string, msg unsubscribeMsg) {
	if debug {
		log.Print("turnpike: handling unsubscribe message")
	}
	t.subLock.Lock()
	topic := checkCurie(t.prefixes[id], msg.TopicURI)
	if lm, ok := t.subscriptions[topic]; ok {
		lm.remove(id)
	}
	t.subLock.Unlock()
	if debug {
		log.Printf("turnpike: client %s unsubscribed from topic: %s", id, topic)
	}
}

func (t *Server) handlePublish(id string, msg publishMsg) {
	if debug {
		log.Print("turnpike: handling publish message")
	}
	topic := checkCurie(t.prefixes[id], msg.TopicURI)
	lm, ok := t.subscriptions[topic]
	if !ok {
		return
	}

	out, err := createEvent(topic, msg.Event)
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
