package turnpike_test

import (
	"code.google.com/p/go.net/websocket"
	"github.com/jcelliott/turnpike"
	"net/http"
)

func ExampleNewServer() {
	s := turnpike.NewServer()
	ws := websocket.Handler(s.HandleWebsocket)

	http.Handle("/ws", ws)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
