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

func TestEventMessage(t *testing.T) {
	// test 1
	exp := `[8,"http://example.com/api/test",null]`
	compareEvent(t, exp, "http://example.com/api/test", nil)

	// test 2
	exp = `[8,"http://example.com/api/testing:thing","this is an event"]`
	compareEvent(t, exp, "http://example.com/api/testing:thing", "this is an event")

	// test 3
	obj := testObj{"the test", 17.3, []int{1, 2, 3}}
	exp = `[8,"http://www.example.com/doc#thing",{"name":"the test","value":17.3,"list":[1,2,3]}]`
	compareEvent(t, exp, "http://www.example.com/doc#thing", obj)

	// test 4
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
