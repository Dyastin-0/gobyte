package tofu

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func (t *Tofu) loadOrGenerateCert() (tls.Certificate, error) {
	certFile := filepath.Join(t.CertPath, "gobyte.crt")
	keyFile := filepath.Join(t.CertPath, "gobyte.key")

	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return tls.Certificate{}, err
			}
			return cert, nil
		}
	}

	return t.generateSelfSignedCert()
}

func (t *Tofu) generateSelfSignedCert() (tls.Certificate, error) {
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
			CommonName: t.ID,
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

	certFile := filepath.Join(t.CertPath, t.ID+".crt")
	keyFile := filepath.Join(t.CertPath, t.ID+".key")

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
