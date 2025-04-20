package tofu

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
)

func (t *Tofu) savePeerFingerprint(peerID string, cert []byte) error {
	fingerprint := sha256.Sum256(cert)
	return os.WriteFile(filepath.Join(t.TrustPath, peerID), fingerprint[:], 0600)
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

	return bytes.Equal(fingerprint[:], storedFingerprint), nil
}
