// Copyright (c) 2013 Joshua Elliott
// Released under the MIT License
// http://opensource.org/licenses/MIT

package turnpike

import (
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestServer_SubNoHandler(t *testing.T) {
	s := NewServer()

	http.Handle("/ws_s1", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8101", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8101/ws_s1", "http://localhost/")
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

func TestServer_RegisterSubHandler(t *testing.T) {
	s := NewServer()
	subCh := make(chan bool)
	s.RegisterSubHandler("event:test", func(clientID, topicURI string) bool {
		subCh <- true
		return true
	})

	http.Handle("/ws_s2", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8102", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8102/ws_s2", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	c.Subscribe("event:test", func(uri string, event interface{}) {})

	select {
	case <-subCh:
		return
	case <-time.After(time.Second):
		t.Fail()
	}
}

func TestServer_SubHandlerAccept(t *testing.T) {
	s := NewServer()
	s.RegisterSubHandler("event:test", func(clientID, topicURI string) bool {
		return true
	})

	http.Handle("/ws_s3", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8103", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8103/ws_s3", "http://localhost/")
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

func TestServer_SubHandlerDeny(t *testing.T) {
	s := NewServer()
	s.RegisterSubHandler("event:test", func(clientID, topicURI string) bool {
		return false
	})

	http.Handle("/ws_s4", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8104", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8104/ws_s4", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	c.Subscribe("event:test", func(uri string, event interface{}) {
		t.Fail()
	})

	c.Publish("event:test", "test")
	<-time.After(time.Second)
}

func TestServer_RegisterPubHandler(t *testing.T) {
	s := NewServer()
	pubCh := make(chan bool)
	s.RegisterPubHandler("event:test", func(topicURI string, event interface{}) interface{} {
		pubCh <- true
		return event
	})

	http.Handle("/ws_s5", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8105", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8105/ws_s5", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	// c.Subscribe("event:test", func(uri string, event interface{}) {})
	c.Publish("event:test", "test")

	select {
	case <-pubCh:
		return
	case <-time.After(time.Second):
		t.Fail()
	}
}

func TestServer_PubHandlerChange(t *testing.T) {
	s := NewServer()
	s.RegisterPubHandler("event:test", func(topicURI string, event interface{}) interface{} {
		return event.(string) + "2"
	})

	http.Handle("/ws_s6", s.Handler)
	// TODO: needs better way of running multiple listen and serve.
	// Currently there is no way of closing the listener. A cusom server and
	// handler will work but requires more work. TBD.
	go func() {
		err := http.ListenAndServe(":8106", nil)
		if err != nil {
			t.Fatal("ListenAndServe: " + err.Error())
		}
	}()

	// Let the server goroutine start.
	runtime.Gosched()

	c := NewClient()
	err := c.Connect("ws://127.0.0.1:8106/ws_s6", "http://localhost/")
	if err != nil {
		t.Fatal("error connecting: " + err.Error())
	}

	eventCh := make(chan bool)
	c.Subscribe("event:test", func(uri string, event interface{}) {
		if event != "test2" {
			t.Fail()
		}
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
