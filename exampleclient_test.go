package turnpike_test

import (
	"code.google.com/p/go.net/websocket"
	"github.com/jcelliott/turnpike"
	"net/http"
)

func ExampleClient_NewClient() {
	c := turnpike.NewClient()
	err := c.Connect("ws://127.0.0.1:8080/ws", "http://localhost/")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}

	c.Call("rpc:test")
}
