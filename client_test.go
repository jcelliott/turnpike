// Copyright (c) 2013 Joshua Elliott
// Released under the MIT License
// http://opensource.org/licenses/MIT

package turnpike

import (
	"errors"
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestClient_CallResult(t *testing.T) {
	s := NewServer()
	s.RegisterRPC("rpc:test_result",
		func(client, uri string, args ...interface{}) (interface{}, error) {
			return "ok", nil
		})
	http.Handle("/ws1", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8001", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8001/ws1", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	resultCh := c.Call("rpc:test_result")
	r := <-resultCh
	if r.Error != nil {
		t.Error(r.Error)
	}
	if r.Result != "ok" {
		t.Fail()
	}
}

func TestClient_CallRestultGenericError(t *testing.T) {
	s := NewServer()
	s.RegisterRPC("rpc:test_generic_error",
		func(client, uri string, args ...interface{}) (interface{}, error) {
			return nil, errors.New("error")
		})
	http.Handle("/ws2", s.Handler)
	go func() {
		err := http.ListenAndServe(":8002", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8002/ws2", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	resultCh := c.Call("rpc:test_generic_error")
	r := <-resultCh
	if r.Result != nil {
		t.Fail()
	}
	if r.Error.Error() != "turnpike: RPC error with URI rpc:test_generic_error#generic-error: error" {
		t.Fail()
	}
}

func TestClient_CallRestultCustomError(t *testing.T) {
	s := NewServer()
	s.RegisterRPC("rpc:test_custom_error",
		func(client, uri string, args ...interface{}) (interface{}, error) {
			return nil, RPCError{uri, "custom error", nil}
		})
	http.Handle("/ws3", s.Handler)
	go func() {
		err := http.ListenAndServe(":8003", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8003/ws3", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	resultCh := c.Call("rpc:test_custom_error")
	r := <-resultCh
	if r.Result != nil {
		t.Fail()
	}
	if r.Error.Error() != "turnpike: RPC error with URI rpc:test_custom_error: custom error" {
		t.Fail()
	}
}

func TestClient_Event(t *testing.T) {
	s := NewServer()
	http.Handle("/ws4", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8004", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8004/ws4", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	eventCh := make(chan bool)
	c.Subscribe("event:test", func(uri string, event interface{}) {
		eventCh <- true
	})

	c.Publish("event:test", "test")

	select {
	case <-eventCh:
		return
	case <-time.After(time.Second):
		t.Fail()
	}
}
