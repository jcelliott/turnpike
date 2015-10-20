Turnpike [![Build Status](https://drone.io/github.com/jcelliott/turnpike/status.png)](https://drone.io/github.com/jcelliott/turnpike/latest) [![Coverage Status](https://coveralls.io/repos/jcelliott/turnpike/badge.svg?branch=v2)](https://coveralls.io/r/jcelliott/turnpike?branch=v2) [![GoDoc](https://godoc.org/gopkg.in/jcelliott/turnpike?status.svg)](http://godoc.org/gopkg.in/jcelliott/turnpike.v2)
===

Go implementation of [WAMP](http://wamp.ws/) - The Web Application Messaging Protocol

> WAMP ("The Web Application Messaging Protocol") is a communication protocol
> that enables distributed application architectures, with application
> functionality spread across nodes and all application communication decoupled
> by messages routed via dedicated WAMP routers.

> At its core, WAMP provides applications with two asynchronous messaging
> patterns within one unified protocol:
> * Publish & Subscribe
> * Remote Procedure Calls

This package provides router and client library implementations as well as a
basic stand-alone router. The router library can be used to embed a WAMP router
in another application, or to build a custom router implementation. The client
library can be used to communicate with any WAMP router.

This version of Turnpike supports WAMP v2. For WAMP v1 support see the [v1 branch](https://github.com/jcelliott/turnpike/tree/v1).

Status
---

Turnpike v2 is still under development, but is getting close to a stable
release. If you have any feedback or suggestions, please
[open an issue](https://github.com/jcelliott/turnpike/issues/new).

Installation
---

Library:

    go get -u gopkg.in/jcelliott/turnpike.v2

Stand-alone router:

    go get -u gopkg.in/jcelliott/turnpike.v2/turnpike

Client library usage
---

```go
// TODO
```

Server library usage
---

main.go:
```go
package main

import (
	"log"
	"net/http"

	"gopkg.in/jcelliott/turnpike.v2"
)

func main() {
	turnpike.Debug()
	s := turnpike.NewBasicWebsocketServer("example.realm")
	server := &http.Server{
		Handler: s,
		Addr:    ":8000",
	}
	log.Println("turnpike server starting on port 8000")
	log.Fatal(server.ListenAndServe())
}
```

This creates a simple WAMP router listening for websocket connections on port
8000 with a single realm configured.

You can build it like this:

    go build -o router main.go

Which will create an executable in your working directory that can be run like
this:

    ./router

Stand-alone router usage
---

Run the router with default settings:

    $GOPATH/bin/turnpike

Router options:

```
Usage of turnpike:
  -port int
        port to run on (default 8000)
  -realm string
        realm name (default "realm1")
```
