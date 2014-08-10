package turnpike

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ugorji/go/codec"
)

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
		if !arg.Type().ConvertibleTo(f.Type()) {
			// special-case map maps
			if arg.Type().Kind() == reflect.Map && f.Type().Kind() == reflect.Map {
				keyType := f.Type().Key()
				valType := f.Type().Elem()
				m := reflect.MakeMap(f.Type())
				for _, k := range arg.MapKeys() {
					if !k.Type().ConvertibleTo(keyType) {
						return nil, fmt.Errorf("Message format error: %dth field not recognizable")
					}
					v := arg.MapIndex(k)
					if !v.Type().ConvertibleTo(valType) {
						return nil, fmt.Errorf("Message format error: %dth field not recognizable")
					}
					m.SetMapIndex(k.Convert(keyType), v.Convert(valType))
				}
				f.Set(m)
			} else {
				return nil, fmt.Errorf("Message format error: %dth field not recognizable", i+1)
			}
		} else {
			f.Set(arg.Convert(f.Type()))
		}
	}
	return msg, nil
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
		if !arg.Type().ConvertibleTo(f.Type()) {
			return nil, fmt.Errorf("Message format error: %dth field not recognizable", i+1)
		} else {
			f.Set(arg.Convert(f.Type()))
		}
	}
	return msg, nil
}
