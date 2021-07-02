package turnpike

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
)

const (
	magic = 0x7f
)

const (
	rawSocketJSON    = 1
	rawSocketMsgpack = 2
)

type rawSocketPeer struct {
	serializer Serializer
	conn       net.Conn
	messages   chan Message
	maxLength  int
}

func intToBytes(i int) [3]byte {
	return [3]byte{
		byte((i >> 16) & 0xff),
		byte((i >> 8) & 0xff),
		byte(i & 0xff),
	}
}

func bytesToInt(arr []byte) (val int) {
	shift := uint(8 * (len(arr) - 1))
	for _, b := range arr {
		val |= int(uint(b) << shift)
		shift -= 8
	}
	return
}

func (ep *rawSocketPeer) Send(msg Message) error {
	b, err := ep.serializer.Serialize(msg)
	if err != nil {
		return err
	}

	if len(b) > ep.maxLength {
		return fmt.Errorf("message too big: %d > %d", len(b), ep.maxLength)
	}

	arr := intToBytes(len(b))
	header := []byte{0x0, arr[0], arr[1], arr[2]}
	_, err = ep.conn.Write(header)
	if err == nil {
		_, err = ep.conn.Write(b)
	}
	return err
}
func (ep *rawSocketPeer) Receive() <-chan Message {
	return ep.messages
}
func (ep *rawSocketPeer) Close() error {
	return ep.conn.Close()
}

func (ep *rawSocketPeer) handleMessages() {
	for {
		var header [4]byte
		if _, err := ep.conn.Read(header[:]); err != nil {
			ep.conn.Close()
			return
		}

		length := bytesToInt(header[1:])
		if length > ep.maxLength {
			// TODO: handle error more nicely?
			ep.conn.Close()
			return
		}
		switch header[0] & 0x7 {
		// WAMP message
		case 0:
			buf := make([]byte, length)
			_, err := ep.conn.Read(buf)
			if err != nil {
				ep.conn.Close()
				return
			}
			msg, err := ep.serializer.Deserialize(buf)
			if err != nil {
				// TODO: handle error
				log.Println("Error deserializing message:", err)
			} else {
				ep.messages <- msg
			}
		// PING
		case 1:
			header[0] = 0x02
			_, err := ep.conn.Write(header[:])
			if err != nil {
				ep.conn.Close()
				return
			}
			_, err = io.CopyN(ep.conn, ep.conn, int64(length))
			if err != nil {
				ep.conn.Close()
				return
			}
		// PONG
		case 2:
			_, err := io.CopyN(ioutil.Discard, ep.conn, int64(length))
			if err != nil {
				ep.conn.Close()
				return
			}
		}
	}
}

// TODO: rename me
func toLength(b byte) int {
	// lengths are specified as a 4-bit number
	// and represent values between 2**9 and 2**24
	return (2 << 8) << b
}

func (ep *rawSocketPeer) handshakeClient() error {
	const serializer = rawSocketMsgpack
	const length = 0xf

	if _, err := ep.conn.Write([]byte{magic, length<<4 | serializer, 0, 0}); err != nil {
		return err
	}
	var buf [4]byte
	if _, err := ep.conn.Read(buf[:]); err != nil {
		return err
	}
	if buf[0] != magic {
		return errors.New("unknown protocol: first byte received not the WAMP magic value")
	}
	if buf[1]&0xf == 0 {
		errCode := buf[1] >> 4
		switch errCode {
		case 0:
			return errors.New("serializer unsupported")
		case 1:
			return errors.New("maximum message length unsupported")
		case 2:
			return errors.New("use of reserved bits (unsupported feature)")
		case 3:
			return errors.New("maximum connection count reached")
		default:
			return fmt.Errorf("unknown error: %d", errCode)
		}
	}
	if buf[1]&0xf != serializer {
		return errors.New("serializer mismatch: server responded with different serializer than requested")
	}
	// TODO: allow server to set this lower?
	if buf[1]>>4 != length {
		return fmt.Errorf("length mismatch: requested: %d, responded: %d", length, buf[1]>>4)
	}
	ep.maxLength = toLength(length)
	return nil
}

func NewRawSocketClient(url string) (*Client, error) {
	conn, err := net.Dial("tcp", url)
	if err != nil {
		return nil, err
	}

	peer := &rawSocketPeer{
		conn:       conn,
		serializer: new(MessagePackSerializer),
		messages:   make(chan Message),
	}

	err = peer.handshakeClient()
	if err != nil {
		return nil, err
	}
	go peer.handleMessages()
	return NewClient(peer), nil
}
