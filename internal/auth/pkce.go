package auth

import (
	"crypto/sha256"
	"encoding/base64"
)

func pkceChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
