package turnpike_test

import (
	"github.com/jcelliott/turnpike"
)

func ExampleClient_NewClient() {
	c := turnpike.NewClient()
	err := c.Connect("ws://127.0.0.1:8080/ws", "http://localhost/")
	if err != nil {
		panic("Error connecting:" + err.Error())
	}

	c.Call("rpc:test")
}
