package tofu

import (
	"crypto/tls"
	"net"
	"os"
)

type (
	ConnectionHandler func(net.Listener)
	NewPeerHandler    func(string, []byte) bool
)

type Tofu struct {
	ID           string
	CertPath     string
	TrustPath    string
	Certificate  tls.Certificate
	ServerConfig *tls.Config
	ClientConfig *tls.Config
	OnNewPeer    NewPeerHandler
}

func New(id, certPath, trustPath string) (*Tofu, error) {
	if certPath == "" || trustPath == "" {
		return nil, ErrorMustSpecifyCertPaths
	}

	if err := os.MkdirAll(certPath, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(trustPath, 0700); err != nil {
		return nil, err
	}

	tofu := &Tofu{
		ID:        id,
		CertPath:  certPath,
		TrustPath: trustPath,
		OnNewPeer: func(peerID string, fingerprint []byte) bool {
			return true
		},
	}

	cert, err := tofu.loadOrGenerateCert()
	if err != nil {
		return nil, err
	}
	tofu.Certificate = cert

	return tofu, nil
}

func (m *Tofu) Start(address string, handler ConnectionHandler) error {
	m.ServerConfig = m.newServerConfig()

	listener, err := tls.Listen("tcp", address, m.ServerConfig)
	if err != nil {
		return err
	}

	handler(listener)

	return nil
}

func (m *Tofu) Connect(address string) (*tls.Conn, error) {
	m.ClientConfig = m.newClientConfig()

	conn, err := tls.Dial("tcp", address, m.ClientConfig)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
