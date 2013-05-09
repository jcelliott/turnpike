package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"turnpike"
	// "turnpike/wamp"
)

func main() {
	c := turnpike.NewClient()
	fmt.Print("Server address (default: localhost:8080)\n> ")
	server := ""
	read := bufio.NewReader(os.Stdin)
	// if _, err := fmt.Scanln(&server); err != nil {
	server, err := read.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading from stdin:", err)
		return
	}
	server = strings.TrimSpace(server)
	if server == "" {
		server = "localhost:8080"
	}
	if err := c.Connect("ws://"+server, "http://localhost:8070"); err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	fmt.Println("Connected to server at:", server)
	fmt.Print("Enter WAMP message, parameters separated by spaces\n",
		"PREFIX=1, CALL=2, SUBSCRIBE=5, UNSUBSCRIBE=6, PUBLISH=7\n")

	for {
		// msgType := -1
		// args := []*string{}
		fmt.Print("> ")
		// if _, err := fmt.Scanln(&msgType, &args); err != nil {
		line, err := read.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin:", err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fmt.Println(line)

		// switch msgType {
		// case wamp.PREFIX:

		// }

	}

	time.Sleep(time.Second * 3)
	c.Prefix("test", "http://example.com/test#")
	time.Sleep(time.Second * 3)
	c.Subscribe("test:topic")
	c.Publish("test:topic", "something really cool happened!")
	time.Sleep(time.Second * 3)
}
