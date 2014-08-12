package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jcelliott/turnpike"
)

func main() {
	// turnpike.Debug()
	c, err := turnpike.NewWebsocketClient(turnpike.JSON, "ws://localhost:8000/", "turnpike.examples", turnpike.ALL)
	if err != nil {
		log.Fatal(err)
	}

	if err := c.Subscribe("chat", func(args []interface{}, kwargs map[string]interface{}) {
		fmt.Println(time.Now().Format(time.Stamp), args[0])
	}); err != nil {
		log.Fatalln("Error subscribing to chat channel:", err)
	}

	sendChatMsg(c, "hello world")
	time.Sleep(10 * time.Second)
	sendChatMsg(c, "goodbyte world")
	time.Sleep(10 * time.Second)
}

func sendChatMsg(c *turnpike.Client, msg string) {
	if err := c.Publish("chat", []interface{}{msg}, nil); err != nil {
		log.Println("Error sending message:", err)
	} else {
		// server shouldn't print our published message
		// log.Println(time.Now(), msg)
	}
}
