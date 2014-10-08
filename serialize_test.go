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
			t.Error("Error parsing good packet: %s, %s", err, tst.packet)
		} else if msg.MessageType() != tst.exp.MessageType() {
			t.Error("Incorrect message type: %d != %d", msg.MessageType(), tst.exp.MessageType())
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
		t.Errorf("Error unmarshalling marshalled BinaryData:", err)
	} else if !bytes.Equal([]byte(b), from) {
		t.Errorf("%s != %s", string(b), string(from))
	}
}
