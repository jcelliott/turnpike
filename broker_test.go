package turnpike

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type TestPeer struct {
	received Message
}

func (s *TestPeer) Send(msg Message) error  { s.received = msg; return nil }
func (s *TestPeer) Receive() <-chan Message { return nil }
func (s *TestPeer) Close() error            { return nil }

func TestSubscribe(t *testing.T) {
	Convey("Subscribing to a topic", t, func() {
		broker := NewDefaultBroker().(*defaultBroker)
		subscriber := &TestPeer{}
		sess := &Session{Peer: subscriber}
		testTopic := URI("turnpike.test.topic")
		msg := &Subscribe{Request: 123, Topic: testTopic}
		broker.Subscribe(sess, msg)

		Convey("The subscriber should have received a SUBSCRIBED message", func() {
			sub := subscriber.received.(*Subscribed).Subscription
			So(sub, ShouldNotEqual, 0)
		})

		Convey("The broker should have created the subscription", func() {
			sub := subscriber.received.(*Subscribed).Subscription
			topic, ok := broker.subscriptions[sub]
			So(ok, ShouldBeTrue)
			So(topic, ShouldEqual, testTopic)
			_, ok = broker.routes[testTopic]
			So(ok, ShouldBeTrue)
			_, ok = broker.sessions[sess]
			So(ok, ShouldBeTrue)
		})

		// TODO: multiple subscribe requests?
	})
}

func TestUnsubscribe(t *testing.T) {
	broker := NewDefaultBroker().(*defaultBroker)
	subscriber := &TestPeer{}
	testTopic := URI("turnpike.test.topic")
	msg := &Subscribe{Request: 123, Topic: testTopic}
	sess := &Session{Peer: subscriber}
	broker.Subscribe(sess, msg)
	sub := subscriber.received.(*Subscribed).Subscription

	Convey("Unsubscribing from a topic", t, func() {
		msg := &Unsubscribe{Request: 124, Subscription: sub}
		broker.Unsubscribe(sess, msg)

		Convey("The peer should have received an UNSUBSCRIBED message", func() {
			unsub := subscriber.received.(*Unsubscribed).Request
			So(unsub, ShouldNotEqual, 0)
		})

		Convey("The broker should have removed the subscription", func() {
			_, ok := broker.subscriptions[sub]
			So(ok, ShouldBeFalse)
			_, ok = broker.routes[testTopic]
			So(ok, ShouldBeFalse)
			_, ok = broker.sessions[sess]
			So(ok, ShouldBeFalse)
		})
	})
}

func TestRemove(t *testing.T) {
	broker := NewDefaultBroker().(*defaultBroker)
	subscriber := &TestPeer{}
	sess := &Session{Peer: subscriber}
	testTopic := URI("turnpike.test.topic")
	msg := &Subscribe{Request: 123, Topic: testTopic}
	broker.Subscribe(sess, msg)
	sub := subscriber.received.(*Subscribed).Subscription

	testTopic2 := URI("turnpike.test.topic2")
	msg2 := &Subscribe{Request: 456, Topic: testTopic2}
	broker.Subscribe(sess, msg2)
	sub2 := subscriber.received.(*Subscribed).Subscription

	Convey("Removing subscriber", t, func() {
		broker.RemoveSession(sess)

		Convey("The broker should have removed the subscription", func() {
			_, ok := broker.subscriptions[sub]
			So(ok, ShouldBeFalse)
			_, ok = broker.routes[testTopic]
			So(ok, ShouldBeFalse)

			_, ok = broker.subscriptions[sub2]
			So(ok, ShouldBeFalse)
			_, ok = broker.routes[testTopic2]
			So(ok, ShouldBeFalse)

			_, ok = broker.sessions[sess]
			So(ok, ShouldBeFalse)
		})
	})
}
