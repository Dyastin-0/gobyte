package tofu

import (
	"crypto/tls"
	"net"
	"os"
)

type (
	ConnectionHandler func(net.Listener) error
	NewPeerHandler    func(string, string) bool
)

var UnsafeNewPeerHandler = func(peerID string, fingerprint string) bool {
	return true
}

type Tofu struct {
	ID           string
	CertPath     string
	TrustPath    string
	Certificate  tls.Certificate
	ServerConfig *tls.Config
	ClientConfig *tls.Config
	OnNewPeer    NewPeerHandler
}

// New uses the UnsafeNewPeerHandler by default
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
		OnNewPeer: UnsafeNewPeerHandler,
	}

	cert, err := tofu.loadOrGenerateCert()
	if err != nil {
		return nil, err
	}
	tofu.Certificate = cert

	return tofu, nil
}

func (t *Tofu) Start(address string, handler ConnectionHandler) error {
	t.ServerConfig = t.newServerConfig()

	listener, err := tls.Listen("tcp", address, t.ServerConfig)
	if err != nil {
		return err
	}

	return handler(listener)
}

func (m *Tofu) Connect(address string) (*tls.Conn, error) {
	m.ClientConfig = m.newClientConfig()

	conn, err := tls.Dial("tcp", address, m.ClientConfig)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
