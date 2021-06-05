package fcquic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"

	quic "github.com/lucas-clemente/quic-go"
)

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"filechain-blockchain"},
	}
}

var udpConn *net.UDPConn

func Listen(addr string) (net.Listener, error) {
	if udpConn != nil {
		return nil, fmt.Errorf("修改为 QUIC 以后，只允许一个监听者")
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	listener, err := quic.Listen(udpConn, generateTLSConfig(), nil)
	if err != nil {
		return nil, err
	}
	return QuicListener{
		listener: listener,
	}, nil
}

func Dial(addr net.Addr) (net.Conn, error) {
	if udpConn == nil {
		return nil, fmt.Errorf("监听者为空，无法建立远端连接")
	}
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"filechain-blockchain"},
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return nil, err
	}

	session, err := quic.Dial(udpConn, udpAddr, addr.String(), tlsConf, nil)
	if err != nil {
		return nil, err
	}

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return nil, err
	}
	return QuicConn{
		session: session,
		stream:  stream,
	}, nil
}
