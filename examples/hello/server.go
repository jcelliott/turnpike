package main

import (
	"fmt"
	"log"
	"net/http"

	"gopkg.in/jcelliott/turnpike.v1"
)

func main() {
	s := turnpike.NewServer()
	http.Handle("/ws", s.Handler)
	http.Handle("/", http.FileServer(http.Dir("web")))

	fmt.Println("Listening on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
