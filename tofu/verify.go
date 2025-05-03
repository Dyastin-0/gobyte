package tofu

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
)

func (t *Tofu) verifyPeer(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return ErrorNoCertificateProvided
	}

	cert := cs.PeerCertificates[0]
	peerID := cert.Subject.CommonName
	fingerprint, _ := x509.MarshalPKIXPublicKey(cert.Raw)

	known, err := t.checkPeerFingerprint(peerID, fingerprint)
	if err != nil {
		return err
	}

	sha256Fingerprint := sha256.Sum256(fingerprint)
	prefixedFingerprint := t.prefixFingerprint(sha256Fingerprint[:])

	if !known {
		if !t.OnNewPeer(peerID, prefixedFingerprint) {
			return ErrorConnectionDenied
		}
		err = t.savePeerFingerprint(peerID, fingerprint)
		if err != nil {
			return err
		}
	}

	return nil
}
