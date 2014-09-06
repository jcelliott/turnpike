package main

import (
	"log"
	"net/http"

	"gopkg.in/jcelliott/turnpike.v2"
)

func main() {
	turnpike.Debug()
	s := turnpike.NewBasicWebsocketServer("turnpike.chat.realm")
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.Handle("/ws", s)
	log.Println("turnpike server starting on port 8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
