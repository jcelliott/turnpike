package main

import (
	"log"
	"net/http"
	"time"

	"gopkg.in/jcelliott/turnpike.v2"
)

var client *turnpike.Client

func main() {
	turnpike.Debug()
	s := turnpike.NewBasicWebsocketServer("turnpike.examples")
	server := &http.Server{
		Handler: s,
		Addr:    ":8000",
	}
	client, _ = s.GetLocalClient("turnpike.examples", nil)
	if err := client.BasicRegister("alarm.set", alarmSet); err != nil {
		panic(err)
	}
	log.Println("turnpike server starting on port 8000")
	log.Fatal(server.ListenAndServe())
}

// takes one argument, the (integer) number of seconds to set the alarm for
func alarmSet(args []interface{}, kwargs map[string]interface{}) (result *turnpike.CallResult) {
	duration, ok := args[0].(float64)
	if !ok {
		return &turnpike.CallResult{Err: turnpike.URI("rpc-example.invalid-argument")}
	}
	go func() {
		time.Sleep(time.Duration(duration) * time.Second)
		client.Publish("alarm.ring", nil, nil)
	}()
	return &turnpike.CallResult{Args: []interface{}{"hello"}}
}
