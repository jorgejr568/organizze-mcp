package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// Cipher seals and opens secrets with AES-256-GCM. The 32-byte key is the
// OAUTH_ENCRYPTION_KEY env value (hex-decoded). Losing the key means every
// stored Organizze api_key becomes unreadable — back it up.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher returns a Cipher bound to key (must be exactly 32 bytes).
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("oauth/storage: encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("oauth/storage: aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("oauth/storage: cipher.NewGCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext, returning (ciphertext, nonce). Nonce is freshly
// random per call — DO NOT reuse a nonce with the same key.
func (c *Cipher) Seal(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("oauth/storage: read nonce: %w", err)
	}
	return c.aead.Seal(nil, nonce, plaintext, nil), nonce, nil
}

// Open decrypts ciphertext using the given nonce.
func (c *Cipher) Open(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != c.aead.NonceSize() {
		return nil, errors.New("oauth/storage: nonce length mismatch")
	}
	out, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth/storage: gcm.Open: %w", err)
	}
	return out, nil
}

// HashToken returns the SHA-256 hash of token, used as the primary-key
// column for oauth_codes and oauth_tokens. The raw token never lands in
// the DB, so a DB leak cannot replay sessions.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
