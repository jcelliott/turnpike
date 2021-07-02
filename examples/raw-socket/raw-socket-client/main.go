package main

import (
	"flag"
	"log"
	"time"

	"gopkg.in/jcelliott/turnpike.v2"
)

var (
	addr string
)

func init() {
	flag.StringVar(&addr, "addr", ":9000", "address to connect to")
	flag.Parse()
}

func gotMessage(args []interface{}, kwargs map[string]interface{}) {
	log.Printf("Got message: %s %s\n", args[0], args[1])
}

func main() {
	// turnpike.Debug()

	log.Println("New client")
	c, err := turnpike.NewRawSocketClient(addr)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = c.JoinRealm("turnpike.example", turnpike.ALLROLES, nil)
	if err != nil {
		log.Fatalln("Error joining realm:", err)
	}

	err = c.Subscribe("messages", gotMessage)
	if err != nil {
		log.Fatalln("Error subscribing to message channel:", err)
	}

	for now := range time.Tick(time.Second * 5) {
		c.Publish("messages", []interface{}{now.String(), flag.Args()[0]}, nil)
	}
}
