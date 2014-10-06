package turnpike

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ugorji/go/codec"
)

const (
	JSON = iota
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

type Serializer interface {
	Serialize(Message) ([]byte, error)
	Deserialize([]byte) (Message, error)
}

type MessagePackSerializer struct {
}

func (s *MessagePackSerializer) Serialize(msg Message) ([]byte, error) {
	arr := []interface{}{int(msg.MessageType())}
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for i := 0; i < val.NumField(); i++ {
		arr = append(arr, val.Field(i).Interface())
	}

	var b []byte
	return b, codec.NewEncoderBytes(&b, new(codec.MsgpackHandle)).Encode(arr)
}

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

type JSONSerializer struct {
}

func (s *JSONSerializer) Serialize(msg Message) ([]byte, error) {
	arr := []interface{}{int(msg.MessageType())}
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for i := 0; i < val.NumField(); i++ {
		arr = append(arr, val.Field(i).Interface())
	}
	return json.Marshal(arr)
}

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
