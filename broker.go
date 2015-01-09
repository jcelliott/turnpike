package turnpike

// type Publisher interface {
// 	ErrorHandler
//
// 	// SendPublished sends acknowledgement that the Event has
// 	// been successfully published.
// 	SendPublished(*Published)
// }
//
// // A Subscriber can subscribe to messages on a Topic URI.
// type Subscriber interface {
// 	ErrorHandler
//
// 	// SendEvent sends a Published Event to the client.
// 	SendEvent(*Event)
// 	// SendUnsubscribed sends an acknowledgement that the
// 	// client has been unsubscribed from messages on the Topic.
// 	SendUnsubscribed(*Unsubscribed)
// 	// SendSubscribed sends an acknowledgement that the
// 	// client has been subscribed to messages on the Topic.
// 	SendSubscribed(*Subscribed)
// }
//
type Broker interface {
	// Publishes a message to all Subscribers.
	Publish(Sender, *Publish)
	// Subscribes to messages on a URI.
	Subscribe(Sender, *Subscribe)
	// Unsubscribes from messages on a URI.
	Unsubscribe(Sender, *Unsubscribe)
}

// A super simple broker that matches URIs to Subscribers.
type DefaultBroker struct {
	routes        map[URI]map[ID]Sender
	subscriptions map[ID]URI
}

func NewDefaultBroker() *DefaultBroker {
	return &DefaultBroker{
		routes:        make(map[URI]map[ID]Sender),
		subscriptions: make(map[ID]URI),
	}
}

// Publish sends a message to all subscribed clients except for the sender.
//
// If msg.Options["acknowledge"] == true, the publisher receives a Published event
// after the message has been sent to all subscribers.
func (br *DefaultBroker) Publish(pub Sender, msg *Publish) {
	pubId := NewID()
	evtTemplate := Event{
		Publication: pubId,
		Arguments:   msg.Arguments,
		ArgumentsKw: msg.ArgumentsKw,
		Details:     make(map[string]interface{}),
	}
	for id, sub := range br.routes[msg.Topic] {
		// shallow-copy the template
		event := evtTemplate
		event.Subscription = id
		// don't send event to publisher
		if sub != pub {
			sub.Send(&event)
		}
	}

	// only send published message if acknowledge is present and set to true
	if doPub, _ := msg.Options["acknowledge"].(bool); doPub {
		pub.Send(&Published{Request: msg.Request, Publication: pubId})
	}
}

func (br *DefaultBroker) Subscribe(sub Sender, msg *Subscribe) {
	if _, ok := br.routes[msg.Topic]; !ok {
		br.routes[msg.Topic] = make(map[ID]Sender)
	}
	id := NewID()
	br.routes[msg.Topic][id] = sub

	br.subscriptions[id] = msg.Topic

	sub.Send(&Subscribed{Request: msg.Request, Subscription: id})
}

func (br *DefaultBroker) Unsubscribe(sub Sender, msg *Unsubscribe) {
	topic, ok := br.subscriptions[msg.Subscription]
	if !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   WAMP_ERROR_NO_SUCH_SUBSCRIPTION,
		}
		sub.Send(err)
		return
	}

	if r, ok := br.routes[topic]; !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   URI("wamp.error.internal_error"),
		}
		sub.Send(err)
	} else if _, ok := r[msg.Subscription]; !ok {
		err := &Error{
			Type:    msg.MessageType(),
			Request: msg.Request,
			Error:   URI("wamp.error.internal_error"),
		}
		sub.Send(err)
	} else {
		delete(r, msg.Subscription)
		if len(r) == 0 {
			delete(br.routes, topic)
		}
		sub.Send(&Unsubscribed{Request: msg.Request})
	}
}

func spliceSubscribers(subs []Sender, i int) []Sender {
	if i == len(subs)-1 {
		return subs[:i]
	}
	return append(subs[:i], subs[i+1:]...)
}
