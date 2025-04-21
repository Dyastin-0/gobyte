package tofu

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	tmpDir := t.TempDir()

	tofu := Tofu{
		CertPath: tmpDir,
		ID:       "testnode",
	}

	cert, err := tofu.generateSelfSignedCert()
	if err != nil {
		t.Fatalf("failed to generate self-signed cert: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse generated certificate: %v", err)
	}

	if x509Cert.Subject.CommonName != "testnode" {
		t.Errorf("unexpected CN: got %s, want %s", x509Cert.Subject.CommonName, "testnode")
	}

	certPath := filepath.Join(tmpDir, "testnode.crt")
	keyPath := filepath.Join(tmpDir, "testnode.key")

	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("expected cert file to exist: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("expected key file to exist: %v", err)
	}
}

func TestLoadOrGenerateCert(t *testing.T) {
	tmpDir := t.TempDir()

	tofu := &Tofu{
		CertPath: tmpDir,
		ID:       "node123",
	}

	cert, err := tofu.loadOrGenerateCert()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cert.Certificate) == 0 {
		t.Error("expected a certificate to be generated")
	}

	loadedCert, err := tofu.loadOrGenerateCert()
	if err != nil {
		t.Fatalf("failed to load existing cert: %v", err)
	}

	if len(loadedCert.Certificate) == 0 {
		t.Error("expected loaded cert to have certificates")
	}

	if string(cert.Certificate[0]) != string(loadedCert.Certificate[0]) {
		t.Error("expected the loaded cert to match the originally generated one")
	}
}

func TestSaveAndCheckPeerFingerprint(t *testing.T) {
	tmpDir := t.TempDir()
	tofu := &Tofu{
		TrustPath: tmpDir,
	}

	peerID := "peer123"
	cert := []byte("dummy certificate data")
	otherCert := []byte("different cert data")

	ok, err := tofu.checkPeerFingerprint(peerID, cert)
	if err != nil {
		t.Fatalf("unexpected error checking nonexistent fingerprint: %v", err)
	}
	if ok {
		t.Error("expected fingerprint check to fail for non-existent fingerprint")
	}

	if err = tofu.savePeerFingerprint(peerID, cert); err != nil {
		t.Fatalf("failed to save fingerprint: %v", err)
	}

	ok, err = tofu.checkPeerFingerprint(peerID, cert)
	if err != nil {
		t.Fatalf("unexpected error during fingerprint check: %v", err)
	}
	if !ok {
		t.Error("expected fingerprint check to pass for matching fingerprint")
	}

	ok, err = tofu.checkPeerFingerprint(peerID, otherCert)
	if err != nil {
		t.Fatalf("unexpected error during fingerprint check: %v", err)
	}
	if ok {
		t.Error("expected fingerprint check to fail for mismatched fingerprint")
	}

	expected := sha256.Sum256(cert)
	path := filepath.Join(tmpDir, peerID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fingerprint file: %v", err)
	}
	if string(data) != string(expected[:]) {
		t.Error("stored fingerprint does not match expected value")
	}
}

func createTestCert(t *testing.T, commonName string) *x509.Certificate {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}

	return cert
}

func TestVerifyPeer(t *testing.T) {
	tmp := t.TempDir()

	peerID := "peerABC"
	tofu := &Tofu{
		TrustPath: tmp,
		OnNewPeer: func(id string, fingerprint []byte) bool {
			return id == peerID
		},
	}

	t.Run("no cert provided", func(t *testing.T) {
		err := tofu.verifyPeer(tls.ConnectionState{})
		if err != ErrorNoCertificateProvided {
			t.Errorf("expected ErrorNoCertificateProvided, got: %v", err)
		}
	})

	t.Run("unknown cert, trusted by OnNewPeer", func(t *testing.T) {
		cert := createTestCert(t, peerID)

		state := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		}

		err := tofu.verifyPeer(state)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		err = tofu.verifyPeer(state)
		if err != nil {
			t.Fatalf("expected known cert to be accepted, got: %v", err)
		}
	})

	t.Run("unknown cert, rejected by OnNewPeer", func(t *testing.T) {
		tofu.OnNewPeer = func(id string, fingerprint []byte) bool {
			return false
		}
		cert := createTestCert(t, "rejected-peer")
		state := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		}

		err := tofu.verifyPeer(state)
		if err != ErrorConnectionDenied {
			t.Errorf("expected ErrorConnectionDenied, got: %v", err)
		}
	})
}
