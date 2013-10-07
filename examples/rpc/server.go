package main

import (
	"github.com/jcelliott/turnpike"
	"net/http"
)

func handleTest(client, uri string, args ...interface{}) (interface{}, error) {
	return "hello world", nil
}

func main() {
	s := turnpike.NewServer()
	s.RegisterRPC("rpc:test", handleTest)

	http.Handle("/ws", s.Handler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
