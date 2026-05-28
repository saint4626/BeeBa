package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

type Box struct {
	aead cipher.AEAD
}

func New(base64Key string) (Box, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Key))
	if err != nil {
		return Box{}, fmt.Errorf("decode secret box key: %w", err)
	}
	if len(key) != 32 {
		return Box{}, fmt.Errorf("secret box key must decode to 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Box{}, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Box{}, fmt.Errorf("create aes-gcm: %w", err)
	}
	return Box{aead: aead}, nil
}

func (b Box) EncryptString(value string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := b.aead.Seal(nil, nonce, []byte(value), nil)
	out := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (b Box) DecryptString(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceSize := b.aead.NonceSize()
	if len(raw) <= nonceSize {
		return "", fmt.Errorf("ciphertext is too short")
	}
	plaintext, err := b.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return string(plaintext), nil
}
