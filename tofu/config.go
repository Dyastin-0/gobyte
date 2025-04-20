package tofu

import "crypto/tls"

func (t *Tofu) newServerConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{t.Certificate},
		ClientAuth:   tls.RequireAnyClientCert,
		VerifyConnection: func(cs tls.ConnectionState) error {
			return t.verifyPeer(cs)
		},
		MinVersion: tls.VersionTLS12,
	}
}

func (t *Tofu) newClientConfig() *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{t.Certificate},
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			return t.verifyPeer(cs)
		},
		MinVersion: tls.VersionTLS12,
	}
}
