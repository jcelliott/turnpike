turnpike
========

Go implementation of WAMP - The Websocket Application Messaging Protocol

turnpike/wamp is the module that implements all the details of the protocol and may be used on its
own. Turnpike itself will be an implementation of a WAMP server and client.

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
