package turnpike

import (
	"fmt"
	"time"
)

var defaultWelcomeDetails = map[string]interface{}{
	"roles": map[string]struct{}{
		"broker": {},
		"dealer": {},
	},
}

type realmExists string

func (e realmExists) Error() string {
	return "Realm exists: " + string(e)
}

type unexpectedMessage struct {
	rec MessageType
	exp MessageType
}

func (e unexpectedMessage) Error() string {
	return fmt.Sprintf("Unexpected message: %s; expected %s", e.rec, e.exp)
}

// A Router handles new Peers and routes requests to the requested Realm.
type Router interface {
	Accept(Peer) error
	Close() error
	RegisterRealm(URI, Realm) error
	GetLocalPeer(URI) (Peer, error)
	AddSessionOpenCallback(func(uint, string))
	AddSessionCloseCallback(func(uint, string))
}

// DefaultRouter is a very basic WAMP router.
type defaultRouter struct {
	Broker
	Dealer

	realms                map[URI]Realm
	clients               map[URI][]Session
	closing               bool
	lastId                int
	sessionOpenCallbacks  []func(uint, string)
	sessionCloseCallbacks []func(uint, string)
}

// NewDefaultRouter creates a very basic WAMP router.
func NewDefaultRouter() Router {
	return &defaultRouter{
		Broker:                NewDefaultBroker(),
		Dealer:                NewDefaultDealer(),
		realms:                make(map[URI]Realm),
		clients:               make(map[URI][]Session),
		sessionOpenCallbacks:  []func(uint, string){},
		sessionCloseCallbacks: []func(uint, string){},
	}
}

func (r *defaultRouter) AddSessionOpenCallback(fn func(uint, string)) {
	r.sessionOpenCallbacks = append(r.sessionOpenCallbacks, fn)
}

func (r *defaultRouter) AddSessionCloseCallback(fn func(uint, string)) {
	r.sessionCloseCallbacks = append(r.sessionCloseCallbacks, fn)
}

func (r *defaultRouter) Close() error {
	r.closing = true
	for _, clients := range r.clients {
		for _, client := range clients {
			client.kill <- WAMP_ERROR_SYSTEM_SHUTDOWN
		}
	}
	return nil
}

func (r *defaultRouter) RegisterRealm(uri URI, realm Realm) error {
	if _, ok := r.realms[uri]; ok {
		return realmExists(uri)
	}
	if realm.Broker == nil {
		realm.Broker = NewDefaultBroker()
	}
	if realm.Dealer == nil {
		realm.Dealer = NewDefaultDealer()
	}
	r.realms[uri] = realm
	return nil
}

func (r *defaultRouter) handleSession(sess Session, realmURI URI) {
	defer func() {
		for _, callback := range r.sessionCloseCallbacks {
			go callback(uint(sess.Id), string(realmURI))
		}
		sess.Close()
	}()

	c := sess.Receive()
	// TODO: what happens if the realm is closed?
	realm := r.realms[realmURI]

	for {
		var msg Message
		var open bool
		select {
		case msg, open = <-c:
			if !open {
				log.Println("lost session:", sess.Id)
				return
			}
		case reason := <-sess.kill:
			sess.Send(&Goodbye{Reason: reason, Details: make(map[string]interface{})})
			log.Println("kill session:", sess.Id)
			// TODO: wait for client Goodbye?
			return
		}

		log.Printf("[%v] %s: %+v", sess.Id, msg.MessageType(), msg)

		switch msg := msg.(type) {
		case *Goodbye:
			sess.Send(&Goodbye{Reason: WAMP_ERROR_GOODBYE_AND_OUT, Details: make(map[string]interface{})})
			return

		// Broker messages
		case *Publish:
			realm.Broker.Publish(sess.Peer, msg)
		case *Subscribe:
			realm.Broker.Subscribe(sess.Peer, msg)
		case *Unsubscribe:
			realm.Broker.Unsubscribe(sess.Peer, msg)

		// Dealer messages
		case *Register:
			realm.Dealer.Register(sess.Peer, msg)
		case *Unregister:
			realm.Dealer.Unregister(sess.Peer, msg)
		case *Call:
			realm.Dealer.Call(sess.Peer, msg)
		case *Yield:
			realm.Dealer.Yield(sess.Peer, msg)

		// Error messages
		case *Error:
			// TODO:
			log.Println("TODO: handle error messages")

		default:
			log.Println("Unhandled message:", msg.MessageType())
		}
	}
}

func (r *defaultRouter) Accept(client Peer) error {
	if r.closing {
		client.Send(&Abort{Reason: WAMP_ERROR_SYSTEM_SHUTDOWN})
		client.Close()
		return fmt.Errorf("Router is closing, no new connections are allowed")
	}

	c := client.Receive()

	select {
	case <-time.After(5 * time.Second):
		client.Close()
		return fmt.Errorf("Timeout on receiving messages")
	case msg, open := <-c:
		if !open {
			client.Close()
			return fmt.Errorf("No messages received")
		}
		log.Printf("%s: %+v", msg.MessageType(), msg)
		if hello, ok := msg.(*Hello); !ok {
			if err := client.Send(&Abort{Reason: WAMP_ERROR_NOT_AUTHORIZED}); err != nil {
				return err
			}
			return client.Close()
		} else if realm, ok := r.realms[hello.Realm]; !ok {
			// TODO: handle invalid realm more gracefully
			if err := client.Send(&Abort{Reason: WAMP_ERROR_NO_SUCH_REALM}); err != nil {
				return err
			}
			return client.Close()
		} else if welcome, err := r.authenticate(client, realm, hello.Details); err != nil {
			abort := &Abort{
				Reason:  WAMP_ERROR_AUTHORIZATION_FAILED,
				Details: map[string]interface{}{"error": err.Error()},
			}
			if err := client.Send(abort); err != nil {
				return err
			}
			return client.Close()
		} else {
			id := NewID()
			welcome.Id = id

			if welcome.Details == nil {
				welcome.Details = make(map[string]interface{})
			}
			// add default details to welcome message
			for k, v := range defaultWelcomeDetails {
				// TODO: check if key already set
				welcome.Details[k] = v
			}
			if err := client.Send(welcome); err != nil {
				return err
			}
			log.Println("Established session:", id)

			sess := Session{Peer: client, Id: id, kill: make(chan URI, 1)}
			r.clients[hello.Realm] = append(r.clients[hello.Realm], sess)
			for _, callback := range r.sessionOpenCallbacks {
				go callback(uint(sess.Id), string(hello.Realm))
			}
			go r.handleSession(sess, hello.Realm)
		}
	}

	return nil
}

// GetLocalPeer returns an internal peer connected to the specified realm.
// This is experimental, and allows creating WAMP routers with embedded clients.
// Currently, the error will always be nil; but this is subject to change.
func (r *defaultRouter) GetLocalPeer(realm URI) (Peer, error) {
	peerA, peerB := localPipe()
	sess := Session{Peer: peerA, Id: NewID(), kill: make(chan URI, 1)}
	log.Println("Established internal session:", sess.Id)
	r.clients[realm] = append(r.clients[realm], sess)
	go r.handleSession(sess, realm)
	return peerB, nil
}

func (r *defaultRouter) getTestPeer() Peer {
	peerA, peerB := localPipe()
	go r.Accept(peerA)
	return peerB
}

func (r *defaultRouter) authenticate(client Peer, realm Realm, details map[string]interface{}) (*Welcome, error) {
	msg, err := realm.Authenticate(details)
	if err != nil {
		return nil, err
	}
	// we should never get anything besides WELCOME and CHALLENGE
	if msg.MessageType() == WELCOME {
		return msg.(*Welcome), nil
	} else {
		// Challenge response
		challenge := msg.(*Challenge)
		if err := client.Send(challenge); err != nil {
			return nil, err
		}

		c := client.Receive()
		select {
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("Timeout on receiving messages")
		case msg, open := <-c:
			if !open {
				return nil, fmt.Errorf("No messages received")
			}
			log.Printf("%s: %+v", msg.MessageType(), msg)
			if authenticate, ok := msg.(*Authenticate); !ok {
				return nil, fmt.Errorf("unexpected %s message received", msg.MessageType())
			} else {
				return realm.CheckResponse(challenge, authenticate)
			}
		}
	}
}
