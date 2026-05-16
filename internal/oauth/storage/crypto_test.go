package storage

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	c, err := NewCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain := []byte("organizze-api-key-very-secret")
	ciphertext, nonce, err := c.Seal(plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(ciphertext, plain) {
		t.Error("ciphertext contains plaintext substring")
	}
	got, err := c.Open(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("Open got %q, want %q", got, plain)
	}
}

func TestOpenWithWrongNonceFails(t *testing.T) {
	c, _ := NewCipher(mustKey(t))
	ct, _, _ := c.Seal([]byte("x"))
	badNonce := make([]byte, 12)
	if _, err := c.Open(ct, badNonce); err == nil {
		t.Error("expected error opening with wrong nonce")
	}
}

func TestOpenRejectsTamperedCiphertext(t *testing.T) {
	c, _ := NewCipher(mustKey(t))
	ct, nonce, err := c.Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	ct[0] ^= 1
	if _, err := c.Open(ct, nonce); err == nil {
		t.Error("expected auth failure on tampered ciphertext")
	}
}

func TestNewCipherRejectsBadKeyLength(t *testing.T) {
	if _, err := NewCipher([]byte("short")); err == nil {
		t.Error("expected error for 5-byte key")
	}
}

func TestHashTokenIsDeterministicAnd32Bytes(t *testing.T) {
	a := HashToken("hello")
	b := HashToken("hello")
	c := HashToken("world")
	if !bytes.Equal(a, b) {
		t.Error("HashToken not deterministic")
	}
	if bytes.Equal(a, c) {
		t.Error("HashToken collisions on different inputs")
	}
	if len(a) != 32 {
		t.Errorf("HashToken length = %d, want 32", len(a))
	}
}
