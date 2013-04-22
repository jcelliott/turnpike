package wamp

import (
	// "encoding/json"
	"github.com/stretchrcom/testify/assert"
	"testing"
)

type testObj struct {
	Name  string  `json:"name"`
	Value float32 `json:"value"`
	List  []int   `json:"list"`
}

func TestWelcomeMsg(t *testing.T) {
	exp := `[0,"12345678",1,"turnpike-0.1.0"]`
	msg, err := WelcomeMsg("12345678")
	if err != nil {
		t.Errorf("error creating welcome message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	ServerIdent = "a different server"
	exp = `[0,"87654321",1,"a different server"]`
	msg, err = WelcomeMsg("87654321")
	if err != nil {
		t.Errorf("error creating welcome message: %s", err)
	}
	assert.Equal(t, exp, string(msg))
}

func TestPrefixMsg(t *testing.T) {
	exp := `[1,"prefix","http://www.example.com/api/start"]`
	msg, err := PrefixMsg("prefix", "http://www.example.com/api/start")
	if err != nil {
		t.Errorf("error creating prefix message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	// test bad uri
	_, err = PrefixMsg("prefix", "httpppppppp")
	assert.Error(t, err)
}

func TestCallMsg(t *testing.T) {
	// zero call arguments
	exp := `[2,"123456","http://example.com/testRPC"]`
	compareCall(t, exp, "123456", "http://example.com/testRPC")

	// nil call argument
	exp = `[2,"654321","http://example.com/otherRPC",null]`
	compareCall(t, exp, "654321", "http://example.com/otherRPC", nil)

	// one call argument
	exp = `[2,"a1b2c3d4","http://example.com/dosomething/rpc","call arg"]`
	compareCall(t, exp, "a1b2c3d4", "http://example.com/dosomething/rpc", "call arg")

	// more call arguments
	exp = `[2,"abcdefg","http://example.com/rpc","arg1","arg2"]`
	compareCall(t, exp, "abcdefg", "http://example.com/rpc", "arg1", "arg2")

	// complex call argument
	exp = `[2,"1234","http://example.com/rpc",{"name":"george","value":14.98,"list":[1,3,5]},"astring"]`
	obj := testObj{Name: "george", Value: 14.98, List: []int{1, 3, 5}}
	compareCall(t, exp, "1234", "http://example.com/rpc", obj, "astring")

	// test bad uri
	_, err := CallMsg("abcd", "httpnopenopenope")
	assert.Error(t, err)
}

func compareCall(t *testing.T, expected, callID, procURI string, args ...interface{}) {
	msg, err := CallMsg(callID, procURI, args...)
	if err != nil {
		t.Errorf("error creating call message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestCallResultMsg(t *testing.T) {
	// null result
	exp := `[3,"123456",null]`
	compareCallResult(t, exp, "123456", nil)

	// simple result
	exp = `[3,"abcdefg","a cool result"]`
	compareCallResult(t, exp, "abcdefg", "a cool result")

	// complex result
	exp = `[3,"asdf",{"name":"sally","value":43.1,"list":[2,4,6]}]`
	obj := testObj{Name: "sally", Value: 43.1, List: []int{2, 4, 6}}
	compareCallResult(t, exp, "asdf", obj)
}

func compareCallResult(t *testing.T, expected, callID string, result interface{}) {
	msg, err := CallResultMsg(callID, result)
	if err != nil {
		t.Errorf("error creating callresult message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestCallErrorMsg(t *testing.T) {
	// generic error
	exp := `[4,"1234","http://example.com/app/error#generic","there was an error"]`
	compareCallError(t, exp, "1234", "http://example.com/app/error#generic", "there was an error")

	// integer error details
	exp = `[4,"asdf","http://example.com/error","integer error",4567]`
	compareCallError(t, exp, "asdf", "http://example.com/error", "integer error", 4567)

	// complex error details
	exp = `[4,"asd123","http://example.com/error","big error",{"name":"huge","value":9000.1,"list":[10,60]}]`
	obj := testObj{Name: "huge", Value: 9000.1, List: []int{10, 60}}
	compareCallError(t, exp, "asd123", "http://example.com/error", "big error", obj)
}

func compareCallError(t *testing.T, expected, callID, errorURI, errorDesc string, errorDetails ...interface{}) {
	msg, err := CallErrorMsg(callID, errorURI, errorDesc, errorDetails...)
	if err != nil {
		t.Errorf("error creating callerror message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestSubscribeMsg(t *testing.T) {
	exp := `[5,"http://example.com/simple"]`
	msg, err := SubscribeMsg("http://example.com/simple")
	if err != nil {
		t.Errorf("error creating subscribe message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	// test bad uri
	_, err = SubscribeMsg("qwerty")
	assert.Error(t, err)
}

func TestUnsubscribeMsg(t *testing.T) {
	exp := `[6,"http://example.com/something"]`
	msg, err := UnsubscribeMsg("http://example.com/something")
	if err != nil {
		t.Errorf("error creating unsubscribe message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	// test bad uri
	_, err = UnsubscribeMsg("qwerty")
	assert.Error(t, err)
}

func TestPublishMsg(t *testing.T) {
	// test nil event
	exp := `[7,"http://example.com/api/test",null]`
	comparePublish(t, exp, "http://example.com/api/test", nil)

	// test simple event
	exp = `[7,"http://example.com/api/testing:thing","this is an event"]`
	comparePublish(t, exp, "http://example.com/api/testing:thing", "this is an event")

	// test complex event
	obj := testObj{"the test", 17.3, []int{1, 2, 3}}
	exp = `[7,"http://www.example.com/doc#thing",{"name":"the test","value":17.3,"list":[1,2,3]}]`
	comparePublish(t, exp, "http://www.example.com/doc#thing", obj)

	// test with excludeMe
	exp = `[7,"http://example.com/api/testing:thing","this is an event",true]`
	comparePublish(t, exp, "http://example.com/api/testing:thing", "this is an event", true)

	// test with exclude list
	exp = `[7,"http://example.com/api/testing:thing","this is an event",["bob","john"],[]]`
	comparePublish(t, exp, "http://example.com/api/testing:thing", "this is an event", []string{"bob", "john"}, []string{})

	// test with eligible list
	exp = `[7,"http://example.com/api/testing:thing","this is an event",[],["sam","fred"]]`
	comparePublish(t, exp, "http://example.com/api/testing:thing", "this is an event", []string{}, []string{"sam", "fred"})

	// test bad uri
	_, err := PublishMsg("asdfasdf", "bad uri")
	assert.Error(t, err)
}

func comparePublish(t *testing.T, expected, topic string, event interface{}, args ...interface{}) {
	msg, err := PublishMsg(topic, event, args...)
	if err != nil {
		t.Errorf("error creating message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestEventMsg(t *testing.T) {
	// test nil event
	exp := `[8,"http://example.com/api/test",null]`
	compareEvent(t, exp, "http://example.com/api/test", nil)

	// test simple event
	exp = `[8,"http://example.com/api/testing:thing","this is an event"]`
	compareEvent(t, exp, "http://example.com/api/testing:thing", "this is an event")

	// test complex event
	obj := testObj{"the test", 17.3, []int{1, 2, 3}}
	exp = `[8,"http://www.example.com/doc#thing",{"name":"the test","value":17.3,"list":[1,2,3]}]`
	compareEvent(t, exp, "http://www.example.com/doc#thing", obj)

	// test bad uri
	_, err := EventMsg("asdfasdf", "bad uri")
	assert.Error(t, err)
}

func compareEvent(t *testing.T, expected, topic string, event interface{}) {
	msg, err := EventMsg(topic, event)
	if err != nil {
		t.Errorf("error creating message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}
