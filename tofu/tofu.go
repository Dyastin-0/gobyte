package tofu

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Tofu struct {
	PeerID       string
	CertPath     string
	TrustPath    string
	Certificate  tls.Certificate
	ServerConfig *tls.Config
	ClientConfig *tls.Config
	OnNewPeer    func(peerID string) bool
}

type ConnectionHandler func(net.Conn, net.Listener, string)

func New(peerID, certPath, trustPath string) (*Tofu, error) {
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
		PeerID:    peerID,
		CertPath:  certPath,
		TrustPath: trustPath,
		OnNewPeer: func(peerID string) bool {
			return true
		},
	}

	cert, err := tofu.loadOrGenerateCert()
	if err != nil {
		return nil, err
	}
	tofu.Certificate = cert

	if err := tofu.setupTLSConfigs(); err != nil {
		return nil, err
	}

	return tofu, nil
}

func (m *Tofu) loadOrGenerateCert() (tls.Certificate, error) {
	certFile := filepath.Join(m.CertPath, "gobyte.crt")
	keyFile := filepath.Join(m.CertPath, "gobyte.key")

	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return tls.Certificate{}, err
			}
			return cert, nil
		}
	}

	return m.generateSelfSignedCert()
}

func (m *Tofu) generateSelfSignedCert() (tls.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: m.PeerID,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	certFile := filepath.Join(m.CertPath, m.PeerID+".crt")
	keyFile := filepath.Join(m.CertPath, m.PeerID+".key")

	if err = os.WriteFile(certFile, certPEM, 0600); err != nil {
		return tls.Certificate{}, err
	}

	if err = os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		return tls.Certificate{}, err
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}

	return cert, nil
}

func (m *Tofu) savePeerFingerprint(peerID string, cert []byte) error {
	fingerprint := sha256.Sum256(cert)
	return os.WriteFile(filepath.Join(m.TrustPath, peerID), fingerprint[:], 0600)
}

func (m *Tofu) checkPeerFingerprint(peerID string, cert []byte) (bool, error) {
	storedFingerprint, err := os.ReadFile(filepath.Join(m.TrustPath, peerID))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	fingerprint := sha256.Sum256(cert)

	fmt.Printf("Stored: %x\n", storedFingerprint)
	fmt.Printf("Calculated: %x\n", fingerprint)

	return bytes.Equal(fingerprint[:], storedFingerprint), nil
}

func (m *Tofu) setupTLSConfigs() error {
	m.ClientConfig = &tls.Config{
		Certificates:       []tls.Certificate{m.Certificate},
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			return m.verifyPeer(cs)
		},
		MinVersion: tls.VersionTLS12,
	}

	m.ServerConfig = &tls.Config{
		Certificates: []tls.Certificate{m.Certificate},
		ClientAuth:   tls.RequireAnyClientCert,
		VerifyConnection: func(cs tls.ConnectionState) error {
			return m.verifyPeer(cs)
		},
		MinVersion: tls.VersionTLS12,
	}

	return nil
}

func (m *Tofu) verifyPeer(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return ErrorNoCertificateProvided
	}

	cert := cs.PeerCertificates[0]
	peerID := cert.Subject.CommonName
	fingerprint, _ := x509.MarshalPKIXPublicKey(cert.Raw)

	known, err := m.checkPeerFingerprint(peerID, fingerprint)
	if err != nil {
		return err
	}

	if !known {
		if !m.OnNewPeer(peerID) {
			return ErrorConnectionDenied
		}
		err = m.savePeerFingerprint(peerID, fingerprint)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Tofu) Start(address string, handler func(net.Listener)) error {
	listener, err := tls.Listen("tcp", address, m.ServerConfig)
	if err != nil {
		return err
	}

	handler(listener)

	return nil
}

func (m *Tofu) Connect(address string) (*tls.Conn, error) {
	conn, err := tls.Dial("tcp", address, m.ClientConfig)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (m *Tofu) GetPeerIDFromConn(conn *tls.Conn) (string, error) {
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", ErrorNoCertificateFound
	}
	return certs[0].Subject.CommonName, nil
}
