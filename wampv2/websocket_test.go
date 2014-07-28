package wampv2

import (
	"testing"
	"net"
	"net/http"
	"io"
	"fmt"

	"github.com/gorilla/websocket"
)

func newWebsocketServer(t *testing.T) (int, Router, io.Closer) {
	r := NewBasicRouter()
	r.RegisterRealm(test_realm, NewBasicRealm())
	s := NewWebsocketServer(r)
	s.RegisterProtocol(jsonWebsocketProtocol, websocket.TextMessage, new(JSONSerializer))
	s.RegisterProtocol(msgpackWebsocketProtocol, websocket.BinaryMessage, new(MessagePackSerializer))
	server := &http.Server{
		Handler: s,
	}

	var addr net.TCPAddr
	l, err := net.ListenTCP("tcp", &addr)
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(l)
	return l.Addr().(*net.TCPAddr).Port, r, l
}

func TestWSHandshakeJSON(t *testing.T) {
	port, r, closer := newWebsocketServer(t)
	defer closer.Close()

	ep, err := NewJSONWebsocketClient(fmt.Sprintf("ws://localhost:%d/", port), "http://localhost")
	if err != nil {
		t.Fatal(err)
	}

	ep.Send(&Hello{Realm: test_realm})
	go r.Accept(ep)

	if msg, ok := <-ep.Receive(); !ok {
		t.Fatal("Receive buffer closed")
	} else if _, ok := msg.(*Welcome); !ok {
		t.Errorf("Message not Welcome message: %T, %+v", msg, msg)
	}
}

func TestWSHandshakeMsgpack(t *testing.T) {
	port, r, closer := newWebsocketServer(t)
	defer closer.Close()

	ep, err := NewMessagePackWebsocketClient(fmt.Sprintf("ws://localhost:%d/", port), "http://localhost")
	if err != nil {
		t.Fatal(err)
	}

	ep.Send(&Hello{Realm: test_realm})
	go r.Accept(ep)

	if msg, ok := <-ep.Receive(); !ok {
		t.Fatal("Receive buffer closed")
	} else if _, ok := msg.(*Welcome); !ok {
		t.Errorf("Message not Welcome message: %T, %+v", msg, msg)
	}
}
