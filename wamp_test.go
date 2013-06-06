package turnpike

import (
	"encoding/json"
	"github.com/stretchrcom/testify/assert"
	"testing"
)

type testObj struct {
	Name  string  `json:"name"`
	Value float32 `json:"value"`
	List  []int   `json:"list"`
}

func TestWelcome(t *testing.T) {
	exp := `[0,"12345678",1,"turnpike-0.1.0"]`
	msg, err := CreateWelcome("12345678", "turnpike-0.1.0")
	if err != nil {
		t.Errorf("error creating welcome message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	exp = `[0,"87654321",1,"a different server"]`
	msg, err = CreateWelcome("87654321", "a different server")
	if err != nil {
		t.Errorf("error creating welcome message: %s", err)
	}
	assert.Equal(t, exp, string(msg))
}

func TestParseWelcome(t *testing.T) {
	data := []byte(`[0,"12345678",1,"turnpike-0.1.0"]`)
	var msg WelcomeMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(WelcomeMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "12345678", msg.SessionId)
	assert.Equal(t, 1, msg.ProtocolVersion)
	assert.Equal(t, "turnpike-0.1.0", msg.ServerIdent)
}

func TestPrefix(t *testing.T) {
	exp := `[1,"prefix","http://www.example.com/api/start"]`
	msg, err := CreatePrefix("prefix", "http://www.example.com/api/start")
	if err != nil {
		t.Errorf("error creating prefix message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	// test bad uri
	_, err = CreatePrefix("prefix", "httpppppppp")
	assert.Error(t, err)
}

func TestParsePrefix(t *testing.T) {
	data := []byte(`[1,"prefix","http://www.example.com/api/start"]`)
	var msg PrefixMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(PrefixMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "prefix", msg.Prefix)
	assert.Equal(t, "http://www.example.com/api/start", msg.URI)
}

func TestCall(t *testing.T) {
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
	_, err := CreateCall("abcd", "httpnopenopenope")
	assert.Error(t, err)
}

func compareCall(t *testing.T, expected, callID, procURI string, args ...interface{}) {
	msg, err := CreateCall(callID, procURI, args...)
	if err != nil {
		t.Errorf("error creating call message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestParseCall(t *testing.T) {
	// no call args
	data := []byte(`[2,"123456","http://example.com/testRPC"]`)
	var msg CallMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(CallMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "123456", msg.CallID)
	assert.Equal(t, "http://example.com/testRPC", msg.ProcURI)
	assert.Nil(t, msg.CallArgs)

	// simple call args
	data = []byte(`[2,"a1b2c3d4","http://example.com/dosomething/rpc","call arg"]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "a1b2c3d4", msg.CallID)
	assert.Equal(t, "http://example.com/dosomething/rpc", msg.ProcURI)
	assert.Equal(t, "call arg", msg.CallArgs[0].(string))

	// complex call args
	data = []byte(`[2,"1234","http://example.com/rpc",{"name":"george","value":14.98,"list":[1,3,5]},"astring"]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "1234", msg.CallID)
	assert.Equal(t, "http://example.com/rpc", msg.ProcURI)
	assert.Equal(t, "george", msg.CallArgs[0].(map[string]interface{})["name"])
	assert.Equal(t, 14.98, msg.CallArgs[0].(map[string]interface{})["value"])
	assert.Equal(t, 5, msg.CallArgs[0].(map[string]interface{})["list"].([]interface{})[2])
	assert.Equal(t, "astring", msg.CallArgs[1].(string))
}

func TestCallResult(t *testing.T) {
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
	msg, err := CreateCallResult(callID, result)
	if err != nil {
		t.Errorf("error creating callresult message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestParseCallResult(t *testing.T) {
	// null result
	data := []byte(`[3,"123456",null]`)
	var msg CallResultMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(CallResultMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "123456", msg.CallID)
	assert.Nil(t, msg.Result)

	// simple result
	data = []byte(`[3,"abcdefg","a cool result"]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "abcdefg", msg.CallID)
	assert.Equal(t, "a cool result", msg.Result)

	// complex result
	data = []byte(`[3,"asdf",{"name":"sally","value":43.1,"list":[2,4,6]}]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "asdf", msg.CallID)
	assert.Equal(t, "sally", msg.Result.(map[string]interface{})["name"])
	assert.Equal(t, 43.1, msg.Result.(map[string]interface{})["value"])
	assert.Equal(t, 6, msg.Result.(map[string]interface{})["list"].([]interface{})[2])
}

func TestCallError(t *testing.T) {
	// generic error
	exp := `[4,"1234","http://example.com/app/error#generic","there was an error"]`
	compareCallError(t, exp, "1234", "http://example.com/app/error#generic", "there was an error")

	// integer error details
	exp = `[4,"asdf","http://example.com/error","integer error",4567]`
	compareCallError(t, exp, "asdf", "http://example.com/error", "integer error", 4567)

	// complex error details
	exp = `[4,"asd123","http://example.com/error","big error",{"name":"huge","value":9000,"list":[10,60]}]`
	obj := testObj{Name: "huge", Value: 9000, List: []int{10, 60}}
	compareCallError(t, exp, "asd123", "http://example.com/error", "big error", obj)
}

func compareCallError(t *testing.T, expected, callID, errorURI, errorDesc string, errorDetails ...interface{}) {
	msg, err := CreateCallError(callID, errorURI, errorDesc, errorDetails...)
	if err != nil {
		t.Errorf("error creating callerror message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestParseCallError(t *testing.T) {
	// generic error
	data := []byte(`[4,"1234","http://example.com/app/error#generic","there was an error"]`)
	var msg CallErrorMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(CallErrorMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "1234", msg.CallID)
	assert.Equal(t, "http://example.com/app/error#generic", msg.ErrorURI)
	assert.Equal(t, "there was an error", msg.ErrorDesc)
	assert.Nil(t, msg.ErrorDetails)

	// integer error details
	data = []byte(`[4,"asdf","http://example.com/error","integer error",4567]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "asdf", msg.CallID)
	assert.Equal(t, "http://example.com/error", msg.ErrorURI)
	assert.Equal(t, "integer error", msg.ErrorDesc)
	assert.Equal(t, 4567, msg.ErrorDetails)

	// complex error details
	data = []byte(`[4,"asd123","http://example.com/error","big error",{"name":"huge","value":9000,"list":[10,60]}]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "asd123", msg.CallID)
	assert.Equal(t, "http://example.com/error", msg.ErrorURI)
	assert.Equal(t, "big error", msg.ErrorDesc)
	assert.Equal(t, "huge", msg.ErrorDetails.(map[string]interface{})["name"])
	assert.Equal(t, 9000, msg.ErrorDetails.(map[string]interface{})["value"])
	assert.Equal(t, 60, msg.ErrorDetails.(map[string]interface{})["list"].([]interface{})[1])
}

func TestSubscribe(t *testing.T) {
	exp := `[5,"http://example.com/simple"]`
	msg, err := CreateSubscribe("http://example.com/simple")
	if err != nil {
		t.Errorf("error creating subscribe message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	// test bad uri
	_, err = CreateSubscribe("qwerty")
	assert.Error(t, err)
}

func TestParseSubscribe(t *testing.T) {
	data := []byte(`[5,"http://example.com/simple"]`)
	var msg SubscribeMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(SubscribeMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/simple", msg.TopicURI)
}

func TestUnsubscribe(t *testing.T) {
	exp := `[6,"http://example.com/something"]`
	msg, err := CreateUnsubscribe("http://example.com/something")
	if err != nil {
		t.Errorf("error creating unsubscribe message: %s", err)
	}
	assert.Equal(t, exp, string(msg))

	// test bad uri
	_, err = CreateUnsubscribe("qwerty")
	assert.Error(t, err)
}

func TestParseUnsubscribe(t *testing.T) {
	data := []byte(`[6,"http://example.com/something"]`)
	var msg UnsubscribeMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(UnsubscribeMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/something", msg.TopicURI)
}

func TestPublish(t *testing.T) {
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
	exp = `[7,"http://example.com/api/testing:thing","this is an event",["bob","john"]]`
	comparePublish(t, exp, "http://example.com/api/testing:thing", "this is an event", []string{"bob", "john"})

	// test with eligible list
	exp = `[7,"http://example.com/api/testing:thing","this is an event",[],["sam","fred"]]`
	comparePublish(t, exp, "http://example.com/api/testing:thing", "this is an event", []string{}, []string{"sam", "fred"})

	// test bad uri
	_, err := CreatePublish("asdfasdf", "bad uri")
	assert.Error(t, err)
}

func comparePublish(t *testing.T, expected, topic string, event interface{}, args ...interface{}) {
	msg, err := CreatePublish(topic, event, args...)
	if err != nil {
		t.Errorf("error creating message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestParsePublish(t *testing.T) {
	// nill event
	data := []byte(`[7,"http://example.com/api/test",null]`)
	var msg PublishMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(PublishMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/api/test", msg.TopicURI)
	assert.Nil(t, msg.Event)
	assert.False(t, msg.ExcludeMe)
	assert.Nil(t, msg.ExcludeList)
	assert.Nil(t, msg.EligibleList)

	// simple event
	data = []byte(`[7,"http://example.com/api/testing:thing","this is an event"]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/api/testing:thing", msg.TopicURI)
	assert.Equal(t, "this is an event", msg.Event)
	assert.False(t, msg.ExcludeMe)
	assert.Nil(t, msg.ExcludeList)
	assert.Nil(t, msg.EligibleList)

	// complex event
	data = []byte(`[7,"http://www.example.com/doc#thing",{"name":"the test","value":17.3,"list":[1,2,3]}]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://www.example.com/doc#thing", msg.TopicURI)
	assert.Equal(t, "the test", msg.Event.(map[string]interface{})["name"])
	assert.Equal(t, 17.3, msg.Event.(map[string]interface{})["value"])
	assert.Equal(t, 3, msg.Event.(map[string]interface{})["list"].([]interface{})[2])

	// with excludeMe
	data = []byte(`[7,"http://example.com/api/testing:thing","this is an event",true]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/api/testing:thing", msg.TopicURI)
	assert.Equal(t, "this is an event", msg.Event)
	assert.True(t, msg.ExcludeMe)

	// with exclude list
	data = []byte(`[7,"http://example.com/api/testing:thing","this is an event",["bob","john"]]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/api/testing:thing", msg.TopicURI)
	assert.Equal(t, "this is an event", msg.Event)
	assert.Equal(t, "bob", msg.ExcludeList[0])
	assert.Equal(t, "john", msg.ExcludeList[1])
	assert.Nil(t, msg.EligibleList)

	// with eligible list
	data = []byte(`[7,"http://example.com/api/testing:thing","this is an event",[],["sam","fred"]]`)
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/api/testing:thing", msg.TopicURI)
	assert.Equal(t, "sam", msg.EligibleList[0])
	assert.Equal(t, "fred", msg.EligibleList[1])
}

func TestEvent(t *testing.T) {
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
	_, err := CreateEvent("asdfasdf", "bad uri")
	assert.Error(t, err)
}

func compareEvent(t *testing.T, expected, topic string, event interface{}) {
	msg, err := CreateEvent(topic, event)
	if err != nil {
		t.Errorf("error creating message: %s", err)
	}
	assert.Equal(t, expected, string(msg))
}

func TestParseEvent(t *testing.T) {
	// nil event
	data := []byte(`[8,"http://example.com/api/test",null]`)
	var msg EventMsg
	assert.Implements(t, (*json.Unmarshaler)(nil), new(EventMsg))
	err := json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/api/test", msg.TopicURI)
	assert.Nil(t, msg.Event)

	// simple event
	data = []byte(`[8,"http://example.com/api/testing:thing","this is an event"]`)
	assert.Implements(t, (*json.Unmarshaler)(nil), new(EventMsg))
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://example.com/api/testing:thing", msg.TopicURI)
	assert.Equal(t, "this is an event", msg.Event)

	// complex event
	data = []byte(`[8,"http://www.example.com/doc#thing",{"name":"the test","value":17.3,"list":[1,2,3]}]`)
	assert.Implements(t, (*json.Unmarshaler)(nil), new(EventMsg))
	err = json.Unmarshal(data, &msg)
	if err != nil {
		t.Errorf("error unmarshalling json: %s", err)
	}
	assert.Equal(t, "http://www.example.com/doc#thing", msg.TopicURI)
	assert.Equal(t, "the test", msg.Event.(map[string]interface{})["name"])
	assert.Equal(t, 17.3, msg.Event.(map[string]interface{})["value"])
	assert.Equal(t, 3, msg.Event.(map[string]interface{})["list"].([]interface{})[2])
}

func TestParseType(t *testing.T) {
	data := `[8,"http://example.com/api/test",null]`
	i := ParseType(data)
	assert.Equal(t, EVENT, i)

	data = `[true,"blah"]`
	i = ParseType(data)
	assert.Equal(t, -1, i)
}

func TestTypeString(t *testing.T) {
	assert.Equal(t, "WELCOME", TypeString(0))
	assert.Equal(t, "SUBSCRIBE", TypeString(5))
	assert.Equal(t, "", TypeString(9))
}
