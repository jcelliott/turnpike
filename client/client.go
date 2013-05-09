package main

import (
	"fmt"
	"time"
	"turnpike"
)

func main() {
	c := turnpike.NewClient()
	if err := c.Connect("ws://localhost:8080", "http://localhost:8070"); err != nil {
		fmt.Println("Error connecting:", err)
		return
	}

	time.Sleep(time.Second * 3)
	c.Prefix("test", "http://example.com/test#")
	time.Sleep(time.Second * 3)
	c.Subscribe("test:topic")
	c.Publish("test:topic", "something really cool happened!")
	time.Sleep(time.Second * 3)
}
