package main

import (
	"net/http"

	"gopkg.in/jcelliott/turnpike.v1"
)

func main() {
	s := turnpike.NewServer()

	http.Handle("/ws", s.Handler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
