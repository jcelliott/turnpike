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
