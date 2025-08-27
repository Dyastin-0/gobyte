package tofu

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func (t *Tofu) trust(peerID string, cert []byte) error {
	fingerprint := sha256.Sum256(cert)
	prefixedFingerprint := t.format("sha256", fingerprint[:])

	return os.WriteFile(filepath.Join(t.TrustPath, peerID), []byte(prefixedFingerprint), 0600)
}

func (t *Tofu) known(peerID string, cert []byte) (bool, error) {
	storedFingerprint, err := os.ReadFile(filepath.Join(t.TrustPath, peerID))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	fingerprint := sha256.Sum256(cert)
	prefixedFingerprint := t.format("sha256", fingerprint[:])

	return bytes.Equal([]byte(prefixedFingerprint), storedFingerprint), nil
}

func (t *Tofu) format(a string, fingerprint []byte) string {
	hexFingerprint := hex.EncodeToString(fingerprint)
	prefixedFingerprint := fmt.Sprintf("%s:%s", a, hexFingerprint)

	return prefixedFingerprint
}
