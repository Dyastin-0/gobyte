package tofu

import "crypto/tls"

func (t *Tofu) DefaultServerConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{*t.Certificate},
		ClientAuth:   tls.RequireAnyClientCert,
		VerifyConnection: func(cs tls.ConnectionState) error {
			return t.verify(cs)
		},
		MinVersion: tls.VersionTLS12,
	}
}

func (t *Tofu) DefaultClientConfig() *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{*t.Certificate},
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			return t.verify(cs)
		},
		MinVersion: tls.VersionTLS12,
	}
}
