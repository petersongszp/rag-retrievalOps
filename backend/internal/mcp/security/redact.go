package security

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func AuthorizationFingerprint(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return "missing"
	}
	sum := sha256.Sum256([]byte(header))
	return "sha256:" + hex.EncodeToString(sum[:4])
}
