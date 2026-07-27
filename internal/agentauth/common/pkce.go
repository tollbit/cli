package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const PKCEChallengeMethod = "S256"

func GeneratePKCE() (string, string, error) {
	verifier, err := RandomURLToken(48)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func RandomURLToken(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
