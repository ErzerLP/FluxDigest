package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const EncryptedValuePrefix = "enc:"
const secretCipherKeySize = 32

type SecretCipher struct {
	aead cipher.AEAD
}

func NewSecretCipher(key string) (*SecretCipher, error) {
	if len([]byte(key)) != secretCipherKeySize {
		return nil, fmt.Errorf("secret cipher key must be %d bytes", secretCipherKeySize)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	return &SecretCipher{aead: gcm}, nil
}

func (c *SecretCipher) EncryptString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if c == nil || c.aead == nil {
		return "", errors.New("secret cipher is required")
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	sealed := c.aead.Seal(nonce, nonce, []byte(value), nil)
	return EncryptedValuePrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *SecretCipher) DecryptString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !HasEncryptedPrefix(value) {
		return value, nil
	}
	if c == nil || c.aead == nil {
		return "", errors.New("secret cipher is required")
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, EncryptedValuePrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted value: %w", err)
	}
	if len(raw) < c.aead.NonceSize() {
		return "", errors.New("encrypted value is too short")
	}

	nonce := raw[:c.aead.NonceSize()]
	ciphertext := raw[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt value: %w", err)
	}
	return string(plaintext), nil
}

func HasEncryptedPrefix(value string) bool {
	return strings.HasPrefix(value, EncryptedValuePrefix)
}
