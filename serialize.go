package turnpike

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/ugorji/go/codec"
)

// Serialization indicates the data serialization format used in a WAMP session
type Serialization int

const (
	// Use JSON-encoded strings as a payload.
	JSON Serialization = iota
	// Use msgpack-encoded strings as a payload.
	MSGPACK
)

// applies a list of values from a WAMP message to a message type
func apply(msgType MessageType, arr []interface{}) (Message, error) {
	msg := msgType.New()
	if msg == nil {
		return nil, fmt.Errorf("Unsupported message type")
	}
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for i := 0; i < val.NumField() && i < len(arr)-1; i++ {
		f := val.Field(i)
		if arr[i+1] == nil {
			continue
		}
		arg := reflect.ValueOf(arr[i+1])
		if arg.Kind() == reflect.Ptr {
			arg = arg.Elem()
		}
		if arg.Type().AssignableTo(f.Type()) {
			f.Set(arg)
		} else if arg.Type().ConvertibleTo(f.Type()) {
			f.Set(arg.Convert(f.Type()))
		} else if f.Type().Kind() != arg.Type().Kind() {
			return nil, fmt.Errorf("Message format error: %dth field not recognizable, got %s, expected %s", i+1, arg.Type(), f.Type())
		} else if f.Type().Kind() == reflect.Map {
			if err := applyMap(f, arg); err != nil {
				return nil, err
			}
		} else if f.Type().Kind() == reflect.Slice {
			if err := applySlice(f, arg); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("Message format error: %dth field not recognizable", i+1)
		}
	}
	return msg, nil
}

// attempts to convert a value to another; is a no-op if it's already assignable to the type
func convert(val reflect.Value, typ reflect.Type) (reflect.Value, error) {
	valType := val.Type()
	if !valType.AssignableTo(typ) {
		if valType.ConvertibleTo(typ) {
			return val.Convert(typ), nil
		} else {
			return val, fmt.Errorf("type %s not convertible to %s", valType.Kind(), typ.Kind())
		}
	}
	return val, nil
}

// re-initializes dst and moves all key/value pairs into dst, converting types as necessary
func applyMap(dst reflect.Value, src reflect.Value) error {
	dstKeyType := dst.Type().Key()
	dstValType := dst.Type().Elem()

	dst.Set(reflect.MakeMap(dst.Type()))
	for _, k := range src.MapKeys() {
		if k.Type().Kind() == reflect.Interface {
			k = k.Elem()
		}
		var err error
		if k, err = convert(k, dstKeyType); err != nil {
			return fmt.Errorf("key '%v' invalid type: %s", k.Interface(), err)
		}

		v := src.MapIndex(k)
		if v, err = convert(v, dstValType); err != nil {
			return fmt.Errorf("value for key '%v' invalid type: %s", k.Interface(), err)
		}
		dst.SetMapIndex(k, v)
	}
	return nil
}

// re-initializes dst and moves all values from src to dst, converting types as necessary
func applySlice(dst reflect.Value, src reflect.Value) error {
	dst.Set(reflect.MakeSlice(dst.Type(), src.Len(), src.Len()))
	dstElemType := dst.Type().Elem()
	for i := 0; i < src.Len(); i++ {
		v, err := convert(src.Index(i), dstElemType)
		if err != nil {
			return fmt.Errorf("Invalid %dth value: %s", i, err)
		}
		dst.Index(i).Set(v)
	}
	return nil
}

// Serializer is the interface implemented by an object that can serialize and
// deserialize WAMP messages
type Serializer interface {
	Serialize(Message) ([]byte, error)
	Deserialize([]byte) (Message, error)
}

// convert the message into a list of values, omitting trailing empty values
func toList(msg Message) []interface{} {
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// iterate backwards until a non-empty or non-"omitempty" field is found
	last := val.Type().NumField() - 1
	for ; last > 0; last-- {
		tag := val.Type().Field(last).Tag.Get("wamp")
		if !strings.Contains(tag, "omitempty") || val.Field(last).Len() > 0 {
			break
		}
	}

	ret := []interface{}{int(msg.MessageType())}
	for i := 0; i <= last; i++ {
		ret = append(ret, val.Field(i).Interface())
	}
	return ret
}

// MessagePackSerializer is an implementation of Serializer that handles
// serializing and deserializing msgpack encoded payloads.
type MessagePackSerializer struct {
}

// Serialize encodes a Message into a msgpack payload.
func (s *MessagePackSerializer) Serialize(msg Message) ([]byte, error) {
	var b []byte
	return b, codec.NewEncoderBytes(&b, new(codec.MsgpackHandle)).Encode(toList(msg))
}

// Deserialize decodes a msgpack payload into a Message.
func (s *MessagePackSerializer) Deserialize(data []byte) (Message, error) {
	var arr []interface{}
	if err := codec.NewDecoderBytes(data, new(codec.MsgpackHandle)).Decode(&arr); err != nil {
		return nil, err
	} else if len(arr) == 0 {
		return nil, fmt.Errorf("Invalid message")
	}

	var msgType MessageType
	if typ, ok := arr[0].(int64); ok {
		msgType = MessageType(typ)
	} else {
		return nil, fmt.Errorf("Unsupported message format")
	}

	return apply(msgType, arr)
}

// JSONSerializer is an implementation of Serializer that handles serializing
// and deserializing JSON encoded payloads.
type JSONSerializer struct {
}

// Serialize marshals the payload into a message.
//
// This method does not handle binary data according to WAMP specifications automatically,
// but instead uses the default implementation in encoding/json.
// Use the BinaryData type in your structures if using binary data.
func (s *JSONSerializer) Serialize(msg Message) ([]byte, error) {
	return json.Marshal(toList(msg))
}

// Deserialize unmarshals the payload into a message.
//
// This method does not handle binary data according to WAMP specifications automatically,
// but instead uses the default implementation in encoding/json.
// Use the BinaryData type in your structures if using binary data.
func (s *JSONSerializer) Deserialize(data []byte) (Message, error) {
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	} else if len(arr) == 0 {
		return nil, fmt.Errorf("Invalid message")
	}

	var msgType MessageType
	if typ, ok := arr[0].(float64); ok {
		msgType = MessageType(typ)
	} else {
		return nil, fmt.Errorf("Unsupported message format")
	}
	return apply(msgType, arr)
}

// BindaryData is a byte array that can be marshalled and unmarshalled according
// to WAMP specifications:
// https://github.com/tavendo/WAMP/blob/master/spec/basic.md#binary-conversion-of-json-strings
//
// This type *should* be used in types that will be marshalled as JSON.
type BinaryData []byte

func (b BinaryData) MarshalJSON() ([]byte, error) {
	s := base64.StdEncoding.EncodeToString([]byte(b))
	return json.Marshal("\x00" + s)
}

func (b *BinaryData) UnmarshalJSON(arr []byte) error {
	var s string
	err := json.Unmarshal(arr, &s)
	if err != nil {
		return nil
	}
	if s[0] != '\x00' {
		return fmt.Errorf("Not a binary string, doesn't start with a NUL: %v", arr)
	}
	*b, err = base64.StdEncoding.DecodeString(s[1:])
	return err
}
