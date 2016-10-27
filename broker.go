package turnpike

import "sync"

// Broker is the interface implemented by an object that handles routing EVENTS
// from Publishers to Subscribers.
type Broker interface {
	// Publishes a message to all Subscribers.
	Publish(*Session, *Publish)
	// Subscribes to messages on a URI.
	Subscribe(*Session, *Subscribe)
	// Unsubscribes from messages on a URI.
	Unsubscribe(*Session, *Unsubscribe)
	// Removes all subscriptions of the subscriber.
	RemoveSession(*Session)
}

// A super simple broker that matches URIs to Subscribers.
type defaultBroker struct {
	routes        map[URI]map[ID]*Session
	subscriptions map[ID]URI
	sessions      map[*Session]map[ID]struct{}
	lock          sync.RWMutex
}

// NewDefaultBroker initializes and returns a simple broker that matches URIs to
// Subscribers.
func NewDefaultBroker() Broker {
	return &defaultBroker{
		routes:        make(map[URI]map[ID]*Session),
		subscriptions: make(map[ID]URI),
		sessions:      make(map[*Session]map[ID]struct{}),
	}
}

// Publish sends a message to all subscribed clients except for the sender.
//
// If msg.Options["acknowledge"] == true, the publisher receives a Published event
// after the message has been sent to all subscribers.
func (br *defaultBroker) Publish(pub *Session, msg *Publish) {
	pubID := NewID()
	evtTemplate := Event{
		Publication: pubID,
		Arguments:   msg.Arguments,
		ArgumentsKw: msg.ArgumentsKw,
		Details:     make(map[string]interface{}),
	}

	excludePublisher := true
	if exclude, ok := msg.Options["exclude_me"].(bool); ok {
		excludePublisher = exclude
	}

	br.lock.RLock()
	for id, sub := range br.routes[msg.Topic] {
		// don't send event to publisher
		if sub == pub && excludePublisher {
			continue
		}

		// shallow-copy the template
		event := evtTemplate
		event.Subscription = id
		sub.Send(&event)
	}
	br.lock.RUnlock()

	// only send published message if acknowledge is present and set to true
	if doPub, _ := msg.Options["acknowledge"].(bool); doPub {
		pub.Send(&Published{Request: msg.Request, Publication: pubID})
	}
}

// Subscribe subscribes the client to the given topic.
func (br *defaultBroker) Subscribe(sub *Session, msg *Subscribe) {
	id := NewID()

	br.lock.Lock()
	r, ok := br.routes[msg.Topic]
	if !ok {
		r = make(map[ID]*Session)
		br.routes[msg.Topic] = r
	}
	r[id] = sub

	s, ok := br.sessions[sub]
	if !ok {
		s = make(map[ID]struct{})
		br.sessions[sub] = s
	}
	s[id] = struct{}{}

	br.subscriptions[id] = msg.Topic
	br.lock.Unlock()

	sub.Send(&Subscribed{Request: msg.Request, Subscription: id})
}

func (br *defaultBroker) Unsubscribe(sub *Session, msg *Unsubscribe) {
	br.lock.Lock()
	topic, ok := br.subscriptions[msg.Subscription]
	if !ok {
		br.lock.Unlock()
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   ErrNoSuchSubscription,
		}
		sub.Send(err)
		log.Printf("Error unsubscribing: no such subscription %v", msg.Subscription)
		return
	}
	delete(br.subscriptions, msg.Subscription)

	// clean up routes
	if r, ok := br.routes[topic]; !ok {
		log.Printf("Error unsubscribing: unable to find routes for %s topic", topic)
	} else if _, ok := r[msg.Subscription]; !ok {
		log.Printf("Error unsubscribing: %s route does not exist for %v subscription", topic, msg.Subscription)
	} else {
		delete(r, msg.Subscription)
		if len(r) == 0 {
			delete(br.routes, topic)
		}
	}

	// clean up sender's subscription
	if s, ok := br.sessions[sub]; !ok {
		log.Println("Error unsubscribing: unable to find sender's subscriptions")
	} else if _, ok := s[msg.Subscription]; !ok {
		log.Printf("Error unsubscribing: sender does not contain %v subscription", msg.Subscription)
	} else {
		delete(s, msg.Subscription)
		if len(s) == 0 {
			delete(br.sessions, sub)
		}
	}
	br.lock.Unlock()

	sub.Send(&Unsubscribed{Request: msg.Request})
}

func (br *defaultBroker) RemoveSession(sub *Session) {
	br.lock.Lock()
	defer br.lock.Unlock()

	for id, _ := range br.sessions[sub] {
		topic, ok := br.subscriptions[id]
		if !ok {
			continue
		}
		delete(br.subscriptions, id)

		// clean up routes
		if r, ok := br.routes[topic]; ok {
			if _, ok := r[id]; ok {
				delete(r, id)
				if len(r) == 0 {
					delete(br.routes, topic)
				}
			}
		}
	}
	delete(br.sessions, sub)
}
