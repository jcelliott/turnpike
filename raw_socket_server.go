package turnpike

import (
	"net"
)

type RawSocketServer struct {
	Router
}

func (s *RawSocketServer) handle(conn net.Conn) {
	var header [4]byte
	_, err := conn.Read(header[:])
	if err != nil {
		conn.Close()
		return
	}
	if header[0] != magic {
		log.Println("unknown protocol: first byte received not the WAMP magic value")
		conn.Close()
		return
	}
	serializer := header[1] & 0x0f
	peer := &rawSocketPeer{
		conn:      conn,
		messages:  make(chan Message),
		maxLength: toLength(header[1] >> 4),
	}
	switch serializer {
	case rawSocketMsgpack:
		peer.serializer = new(MessagePackSerializer)
	case rawSocketJSON:
		peer.serializer = new(JSONSerializer)
	}

	_, err = conn.Write([]byte{magic, header[1], 0, 0})
	if err != nil {
		conn.Close()
		return
	}

	go peer.handleMessages()

	s.Accept(peer)
}

func (s *RawSocketServer) HandleListener(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			l.Close()
			return
		}
		go s.handle(conn)
	}
}

func NewBasicRawSocketServer(realms ...string) *RawSocketServer {
	s := &RawSocketServer{Router: NewDefaultRouter()}
	for _, realm := range realms {
		s.RegisterRealm(URI(realm), Realm{})
	}
	return s
}
