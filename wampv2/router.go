package wampv2

import "math/rand"
import "fmt"

type realmExists string
func (e realmExists) Error() string {
	return "Realm exists: " + string(e)
}

type unexpectedMessage struct {
	rec MessageType
	exp MessageType
}
func (e unexpectedMessage) Error() string {
	return fmt.Sprintf("Unexpected message: %s; expected %s" , e.rec, e.exp)
}

type Router struct {
	realms map[URI]Realm
}

func NewRouter() *Router {
	return &Router{
		realms: make(map[URI]Realm),
	}
}

func (r *Router) RegisterRealm(uri URI, realm Realm) error {
	if _, ok := r.realms[uri]; ok {
		return realmExists(uri)
	}
	r.realms[uri] = realm
	return nil
}

func (r *Router) Accept(ep Endpoint) (err error) {
	if msg, err := ep.Receive(); err != nil {
		ep.Close()
		return fmt.Errorf("Error receiving message: %s", err)
	} else if msg.MessageType() != HELLO {
		//unexpectedMessage(msg.MessageType())
		ep.Close()
		return unexpectedMessage{rec: msg.MessageType(), exp: HELLO}
	} else if hello, ok := msg.(*Hello); !ok {
		// TODO: log errors somewhere
		ep.Close()
		return fmt.Errorf("HELLO message not an instance of Hello{}")
	} else if _, ok := r.realms[hello.Realm]; !ok {
		// TODO: handle invalid realm more gracefully
		ep.Send(&Abort{Reason: "realm does not exist"})
		ep.Close()
		return fmt.Errorf("Requested realm does not exist")
	} else {
		// TODO: challenge
		err = ep.Send(&Welcome{Id: ID(rand.Intn(2 << 53))})
		if err == nil {
			err = ep.Send(&Goodbye{Reason: "end of the line"})
		}
	}
	return
}
