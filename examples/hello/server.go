package main

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/jcelliott/turnpike"
	"log"
	"net/http"
)

func main() {
	s := turnpike.NewServer()
	// for Go1.1, this will be just websocket.Handler(s.HandleWebsocket).ServeHTTP
	ws := websocket.Handler(turnpike.HandleWebsocket(s))
	http.Handle("/ws", ws)
	http.Handle("/", http.FileServer(http.Dir("web")))

	fmt.Println("Listening on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
