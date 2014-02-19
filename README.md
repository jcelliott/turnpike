turnpike
========

Go implementation of [WAMP](http://www.wamp.ws/) - The Websocket Application Messaging Protocol

Turnpike provides a WAMP server and client.

[![Build Status](https://drone.io/github.com/jcelliott/turnpike/status.png)](https://drone.io/github.com/jcelliott/turnpike/latest)

Examples
========

chat
----

Very simple chat server and client written in Dart. To run, first install [Dart](http://www.dartlang.org/tools/sdk/) (Dart editor should work as well), then:

    cd examples/dart
    pub install
    ./build.sh
    go run server.go

Open a browser (or more) to localhost:8080. Type in the textbox and hit enter.

hello
-----

Connect to a Turnpike server with autobahn.js:

    cd examples/hello
    go run server.go

Open a browser to localhost:8080. You should see a message when autobahn.js connects to Turnpike.

