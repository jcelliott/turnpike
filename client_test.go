// Copyright (c) 2013 Joshua Elliott
// Released under the MIT License
// http://opensource.org/licenses/MIT

package turnpike

import (
	"errors"
	"net/http"
	"runtime"
	"testing"
)

func handleResult(client, uri string, args ...interface{}) (interface{}, error) {
	return "ok", nil
}

func handleResultGenericError(client, uri string, args ...interface{}) (interface{}, error) {
	return nil, errors.New("error")
}

func handleResultCustomError(client, uri string, args ...interface{}) (interface{}, error) {
	return nil, RPCError{uri, "custom error", nil}
}

func TestClient_CallResult(t *testing.T) {
	s := NewServer()
	s.RegisterRPC("rpc:test_result", handleResult)
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

	resultCh, err := c.Call("rpc:test_result")
	if err != nil {
		t.Error(err)
	}

	r := <-resultCh
	if r.Result != "ok" {
		t.Fail()
	}
}

func TestClient_CallRestultGenericError(t *testing.T) {
	s := NewServer()
	s.RegisterRPC("rpc:test_generic_error", handleResultGenericError)
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

	resultCh, err := c.Call("rpc:test_generic_error")
	if err != nil {
		t.Error(err)
	}

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
	s.RegisterRPC("rpc:test_custom_error", handleResultCustomError)
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

	resultCh, err := c.Call("rpc:test_custom_error")
	if err != nil {
		t.Error(err)
	}

	r := <-resultCh
	if r.Result != nil {
		t.Fail()
	}
	if r.Error.Error() != "turnpike: RPC error with URI rpc:test_custom_error: custom error" {
		t.Fail()
	}
}
