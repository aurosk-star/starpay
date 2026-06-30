package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

func EncryptSecret(key string, plaintext string) (string, error) {
	aead, err := newSecretAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecryptSecret(key string, ciphertext string) (string, error) {
	aead, err := newSecretAEAD(key)
	if err != nil {
		return "", err
	}
	payload, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	if len(payload) <= aead.NonceSize() {
		return "", errors.New("invalid secret ciphertext")
	}
	nonce := payload[:aead.NonceSize()]
	encrypted := payload[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func newSecretAEAD(key string) (cipher.AEAD, error) {
	keyBytes := []byte(key)
	switch len(keyBytes) {
	case 16, 24, 32:
	default:
		return nil, fmt.Errorf("secret encryption key must be 16, 24, or 32 bytes")
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
