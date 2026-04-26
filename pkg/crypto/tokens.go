// Package crypto provides AES-256-GCM token encryption used for
// at-rest protection of OAuth access/refresh tokens (Strava, etc.).
//
// Format: nonce ‖ ciphertext ‖ tag, all in one bytea blob. The nonce is
// generated per encryption from crypto/rand.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// ErrKeyLength is returned when NewTokenCipher receives a key of the
// wrong length.
var ErrKeyLength = errors.New("crypto: key must be 32 bytes for AES-256-GCM")

// ErrCiphertextTooShort is returned when Decrypt receives a buffer that
// can't possibly contain a nonce + tag.
var ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")

// TokenCipher encrypts and decrypts short token strings using
// AES-256-GCM. Safe for concurrent use after construction.
type TokenCipher struct {
	aead cipher.AEAD
}

// NewTokenCipher constructs a TokenCipher from a 32-byte key.
// In production, load the key from a securely-stored env var
// (ENCRYPTION_KEY). Generate one with `openssl rand -base64 32`.
func NewTokenCipher(key []byte) (*TokenCipher, error) {
	if len(key) != 32 {
		return nil, ErrKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &TokenCipher{aead: aead}, nil
}

// Encrypt encrypts plaintext and returns nonce-prefixed ciphertext.
func (c *TokenCipher) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt decrypts a nonce-prefixed ciphertext produced by Encrypt.
func (c *TokenCipher) Decrypt(ciphertext []byte) (string, error) {
	ns := c.aead.NonceSize()
	if len(ciphertext) < ns {
		return "", ErrCiphertextTooShort
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
