package turnpike

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestJSONDeserialize(t *testing.T) {
	type test struct {
		packet string
		exp    Message
		// number of args past the message type
		args int
	}

	tests := []test{
		{
			`[1,"some.realm",{}]`,
			&Hello{"some.realm", make(map[string]interface{})},
			2,
		},
	}

	s := new(JSONSerializer)
	for _, tst := range tests {
		if msg, err := s.Deserialize([]byte(tst.packet)); err != nil {
			t.Errorf("Error parsing good packet: %s, %s", err, tst.packet)
		} else if msg.MessageType() != tst.exp.MessageType() {
			t.Errorf("Incorrect message type: %d != %d", msg.MessageType(), tst.exp.MessageType())
		} else if !reflect.DeepEqual(msg, tst.exp) {
			t.Errorf("%+v != %+v", msg, tst.exp)
		}
	}
}

func TestApplySlice(t *testing.T) {
	const msgType = PUBLISH

	pubArgs := []string{"hello", "world"}
	Convey("Deserializing into a message with a slice", t, func() {
		args := []interface{}{msgType, 123, make(map[string]interface{}), "some.valid.topic", pubArgs}
		msg, err := apply(msgType, args)
		Convey("Should not error", func() {
			So(err, ShouldBeNil)
		})

		pubMsg, ok := msg.(*Publish)
		Convey("The message returned should be a publish message", func() {
			So(ok, ShouldBeTrue)
		})

		Convey("The message received should have the correct arguments", func() {
			So(len(pubMsg.Arguments), ShouldEqual, 2)
			So(pubMsg.Arguments[0], ShouldEqual, pubArgs[0])
			So(pubMsg.Arguments[1], ShouldEqual, pubArgs[1])
		})
	})
}

func TestBinaryData(t *testing.T) {
	from := []byte("hello")

	arr, err := json.Marshal(BinaryData(from))
	if err != nil {
		t.Error("Error marshalling BinaryData:", err.Error())
	}

	exp := fmt.Sprintf(`"\u0000%s"`, base64.StdEncoding.EncodeToString(from))
	if !bytes.Equal([]byte(exp), arr) {
		t.Errorf("%s != %s", string(arr), exp)
	}

	var b BinaryData
	err = json.Unmarshal(arr, &b)
	if err != nil {
		t.Errorf("Error unmarshalling marshalled BinaryData: %v", err)
	} else if !bytes.Equal([]byte(b), from) {
		t.Errorf("%s != %s", string(b), string(from))
	}
}

func TestToList(t *testing.T) {
	type test struct {
		args   []interface{}
		kwArgs map[string]interface{}
		omit   int
		msg    string
	}
	tests := []test{
		{[]interface{}{1}, map[string]interface{}{"a": nil}, 0, "default case"},
		{nil, map[string]interface{}{"a": nil}, 0, "nil args, non-empty kwArgs"},
		{[]interface{}{}, map[string]interface{}{"a": nil}, 0, "empty args, non-empty kwArgs"},
		{[]interface{}{1}, nil, 1, "non-empty args, nil kwArgs"},
		{[]interface{}{1}, make(map[string]interface{}), 1, "non-empty args, empty kwArgs"},
		{[]interface{}{}, make(map[string]interface{}), 2, "empty args, empty kwArgs"},
		{nil, nil, 2, "nil args, nil kwArgs"},
	}

	for _, tst := range tests {
		msg := &Event{0, 0, nil, tst.args, tst.kwArgs}
		// +1 to account for the message type
		numField := reflect.ValueOf(msg).Elem().NumField() + 1
		exp := numField - tst.omit
		if l := len(toList(msg)); l != exp {
			t.Errorf("Incorrect number of fields: %d != %d: %s", l, exp, tst.msg)
		}
	}
}
