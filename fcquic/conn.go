package fcquic

import (
	"net"

	"github.com/lucas-clemente/quic-go"
)

type QuicConn struct {
	net.Conn
	session quic.Session
	stream  quic.Stream
}

func (c QuicConn) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
}

func (c QuicConn) Write(b []byte) (n int, err error) {
	return c.stream.Write(b)
}

func (c QuicConn) Read(b []byte) (n int, err error) {
	return c.stream.Read(b)
}

func (c QuicConn) Close() error {
	return c.stream.Close()
}
