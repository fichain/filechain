package fcquic

import (
	"context"
	"net"

	"github.com/lucas-clemente/quic-go"
)

type QuicListener struct {
	net.Listener
	listener quic.Listener
}

func (l QuicListener) Addr() net.Addr {
	return l.listener.Addr()
}

func (l QuicListener) Accept() (net.Conn, error) {
	sess, err := l.listener.Accept(context.Background())
	if err != nil {
		return nil, err
	}
	stream, err := sess.AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}
	return QuicConn{
		session: sess,
		stream:  stream,
	}, nil
}

func (l QuicListener) Close() error {
	return l.listener.Close()
}
