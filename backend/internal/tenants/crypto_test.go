package tenants

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// testKey32 devuelve 32 B aleatorios (fixture, NUNCA secreto real).
func testKey32(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestParseKey_Base64(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString(testKey32(t))
	k, err := ParseKey(raw)
	if err != nil {
		t.Fatalf("ParseKey(base64): %v", err)
	}
	if len(k) != 32 {
		t.Fatalf("len = %d, want 32", len(k))
	}
}

func TestParseKey_Raw32(t *testing.T) {
	k, err := ParseKey("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("ParseKey(raw32): %v", err)
	}
	if len(k) != 32 {
		t.Fatalf("len = %d, want 32", len(k))
	}
}

func TestParseKey_Invalid(t *testing.T) {
	for _, raw := range []string{"", "corta", "0123456789abcdef", strings.Repeat("a", 33)} {
		if _, err := ParseKey(raw); err == nil {
			t.Errorf("ParseKey(%q) = nil, want error", raw)
		}
	}
}

func TestCrypter_RoundTrip(t *testing.T) {
	c, err := NewCrypter(testKey32(t))
	if err != nil {
		t.Fatalf("NewCrypter: %v", err)
	}
	plain := []byte("REDACTED-fixture-token")
	blob, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(blob, plain) {
		t.Fatal("ciphertext == plaintext")
	}
	got, err := c.Decrypt(blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("roundtrip mismatch")
	}
}

func TestCrypter_NonceRandom(t *testing.T) {
	c, err := NewCrypter(testKey32(t))
	if err != nil {
		t.Fatalf("NewCrypter: %v", err)
	}
	a, _ := c.Encrypt([]byte("same"))
	b, _ := c.Encrypt([]byte("same"))
	if bytes.Equal(a, b) {
		t.Error("dos cifrados iguales: el nonce no aleatoriza")
	}
}

func TestCrypter_DecryptTampered(t *testing.T) {
	c, err := NewCrypter(testKey32(t))
	if err != nil {
		t.Fatalf("NewCrypter: %v", err)
	}
	blob, _ := c.Encrypt([]byte("x"))
	blob[len(blob)-1] ^= 0xff
	if _, err := c.Decrypt(blob); err == nil {
		t.Error("Decrypt(tampered) = nil, want error")
	}
	if _, err := c.Decrypt([]byte("corto")); err == nil {
		t.Error("Decrypt(short) = nil, want error")
	}
}

func TestCrypter_WrongKeySize(t *testing.T) {
	if _, err := NewCrypter([]byte("corta")); err == nil {
		t.Error("NewCrypter(5B) = nil, want error")
	}
}

// TestCrypter_WrongKeyDecrypt: blob de una clave no abre con otra.
func TestCrypter_WrongKeyDecrypt(t *testing.T) {
	a, _ := NewCrypter(testKey32(t))
	b, _ := NewCrypter(testKey32(t))
	blob, _ := a.Encrypt([]byte("secreto"))
	if _, err := b.Decrypt(blob); err == nil {
		t.Error("Decrypt con otra clave = nil, want error")
	}
}
