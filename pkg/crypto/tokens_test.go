package crypto

import (
	"crypto/rand"
	"errors"
	"testing"
)

func newTestCipher(t *testing.T) *TokenCipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c, err := NewTokenCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTokenCipher_RoundTrip(t *testing.T) {
	c := newTestCipher(t)

	cases := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"short", "hello"},
		{"strava-token-shape", "abcd1234efgh5678ijkl9012mnop3456"},
		{"unicode", "🔐 secrets ✨"},
		{"long", string(make([]byte, 4096))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, err := c.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			pt, err := c.Decrypt(ct)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if pt != tc.plaintext {
				t.Fatalf("got %q, want %q", pt, tc.plaintext)
			}
		})
	}
}

func TestNewTokenCipher_BadKeyLength(t *testing.T) {
	cases := []struct {
		name   string
		keyLen int
	}{
		{"too-short", 16},
		{"too-long", 64},
		{"zero", 0},
		{"odd-31", 31},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewTokenCipher(make([]byte, tc.keyLen))
			if !errors.Is(err, ErrKeyLength) {
				t.Fatalf("expected ErrKeyLength, got %v", err)
			}
		})
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	c := newTestCipher(t)
	_, err := c.Decrypt([]byte{0x01, 0x02})
	if !errors.Is(err, ErrCiphertextTooShort) {
		t.Fatalf("expected ErrCiphertextTooShort, got %v", err)
	}
}

func TestDecrypt_TamperedCiphertextFails(t *testing.T) {
	c := newTestCipher(t)
	ct, err := c.Encrypt("hello")
	if err != nil {
		t.Fatal(err)
	}
	// Flip the last byte (the GCM tag).
	ct[len(ct)-1] ^= 0xff
	if _, err := c.Decrypt(ct); err == nil {
		t.Fatal("expected error decrypting tampered ciphertext")
	}
}

func TestEncrypt_UniqueNoncesProduceDifferentCiphertexts(t *testing.T) {
	c := newTestCipher(t)
	a, err := c.Encrypt("hello")
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Encrypt("hello")
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		t.Fatal("expected different ciphertexts (different nonces) for same plaintext")
	}
}
