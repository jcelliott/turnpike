package turnpike

import (
	"reflect"
	"testing"
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
