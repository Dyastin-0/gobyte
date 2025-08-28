package tofu

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
)

type (
	ConnectionHandler func(net.Listener) error
	NewPeerHandler    func(string, string) bool
)

var UnsafeNewPeerHandler = func(peerID, fingerprint string) bool {
	return true
}

type Tofu struct {
	ID           string
	CertPath     string
	TrustPath    string
	Certificate  *tls.Certificate
	ServerConfig *tls.Config
	ClientConfig *tls.Config
	OnNewPeer    NewPeerHandler
}

func New(id string) *Tofu {
	return &Tofu{ID: id}
}

func (t *Tofu) Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	certPath := filepath.Join(homeDir, "gobyte", "cert")
	if err := os.MkdirAll(certPath, 0700); err != nil {
		return err
	}

	t.CertPath = certPath

	trustPath := filepath.Join(homeDir, "gobyte", "trust")
	if err := os.MkdirAll(trustPath, 0700); err != nil {
		return err
	}

	t.TrustPath = trustPath

	cert, err := t.cert()
	if err != nil {
		return err
	}
	t.Certificate = cert

	return nil
}

func (t *Tofu) Listen(address string) (net.Listener, error) {
	// Start will use the UnsafeNewPeerHandler if not set
	if t.OnNewPeer == nil {
		t.OnNewPeer = UnsafeNewPeerHandler
	}

	if t.ServerConfig == nil {
		t.ServerConfig = t.DefaultServerConfig()
	}

	listener, err := tls.Listen("tcp", address, t.ServerConfig)
	if err != nil {
		return nil, err
	}

	return listener, err
}

func (t *Tofu) Dial(address string) (*tls.Conn, error) {
	if t.ClientConfig == nil {
		t.ClientConfig = t.DefaultClientConfig()
	}

	conn, err := tls.Dial("tcp", address, t.ClientConfig)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
