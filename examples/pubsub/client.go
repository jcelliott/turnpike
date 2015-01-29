package main

import (
	"fmt"
	"time"

	"gopkg.in/jcelliott/turnpike.v1"
)

func testHandler(uri string, event interface{}) {
	fmt.Printf("Received event: %s\n", event)
}

func main() {
	c := turnpike.NewClient()
	err := c.Connect("ws://127.0.0.1:8080/ws", "http://localhost/")
	if err != nil {
		panic("Error connecting:" + err.Error())
	}

	c.Subscribe("event:test", testHandler)

	for {
		c.Publish("event:test", "test")
		<-time.After(time.Second)
	}
}
