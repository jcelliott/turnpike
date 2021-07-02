package main

import (
	"flag"
	"log"
	"net"

	"gopkg.in/jcelliott/turnpike.v2"
)

var (
	addr string
)

func init() {
	flag.StringVar(&addr, "addr", ":9000", "address to listen on for raw socket connections")
	flag.Parse()
}

func main() {
	turnpike.Debug()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Raw Socket server listening on", l.Addr())
	s := turnpike.NewBasicRawSocketServer("turnpike.example")
	s.HandleListener(l)
}
