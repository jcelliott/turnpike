package main

import (
	"log"
	"os"

	"gopkg.in/jcelliott/turnpike.v2"
)

const (
	MAX_NAME_LENGTH    = 32
	MAX_MESSAGE_LENGTH = 256
	MESSAGE_WIN_SIZE   = 3
	PROMPT             = "> "
)

type message struct {
	From, Message string
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <username>", os.Args[0])
	}

	if fi, err := os.Stderr.Stat(); err != nil {
		log.Fatal("Error checking stderr:", err)
	} else if fi.Mode()&os.ModeDevice != 0 {
		stderr, err := os.Open(os.DevNull)
		if err != nil {
			log.Fatal("Error redirecting stderr")
		}
		log.SetOutput(stderr)
	}

	username := os.Args[1]

	// turnpike.Debug()
	c, err := turnpike.NewWebsocketClient(turnpike.JSON, "ws://localhost:8000/")
	if err != nil {
		log.Fatal(err)
	}
	_, err = c.JoinRealm("turnpike.examples", nil)
	if err != nil {
		log.Fatal(err)
	}

	messages := make(chan message)
	if err := c.Subscribe("chat", nil, func(args []interface{}, kwargs map[string]interface{}) {
		if len(args) == 2 {
			if from, ok := args[0].(string); !ok {
				log.Println("First argument not a string:", args[0])
			} else if msg, ok := args[1].(string); !ok {
				log.Println("Second argument not a string:", args[1])
			} else {
				log.Printf("%s: %s", from, msg)
				messages <- message{From: from, Message: msg}
			}
		}
	}); err != nil {
		log.Fatalln("Error subscribing to chat channel:", err)
	}

	outgoing := make(chan message, 10)
	go sendMessages(c, outgoing)
	cw := newChatWin(username, outgoing)
	cw.dialog(messages)
}

func sendMessages(c *turnpike.Client, messages chan message) {
	for msg := range messages {
		if err := c.Publish("chat", nil, []interface{}{msg.From, msg.Message}, nil); err != nil {
			log.Println("Error sending message:", err)
		}
	}
}
