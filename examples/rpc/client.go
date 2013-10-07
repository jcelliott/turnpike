package main

import (
	"fmt"
	"github.com/jcelliott/turnpike"
)

func main() {
	c := turnpike.NewClient()
	err := c.Connect("ws://127.0.0.1:8080/ws", "http://localhost/")
	if err != nil {
		panic("Error connecting:" + err.Error())
	}

	resultCh, err := c.Call("rpc:test")
	if err != nil {
		panic("Error calling: " + err.Error())
	}

	result := <-resultCh
	fmt.Printf("Call result is: %s\n", result.Result)
}
