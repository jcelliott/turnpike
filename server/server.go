package main

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"log"
	"net/http"
	"turnpike"
)

func main() {
	s := turnpike.NewServer()
	// for Go1.1, this will be just websocket.Handler(s.HandleWebsocket)
	http.Handle("/", websocket.Handler(turnpike.HandleWebsocket(s)))
	fmt.Println("Listening on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
