package tenants

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// nonceSize es el nonce de 12 B que exige GCM.
const nonceSize = 12

// ParseKey valida TENANT_TOKEN_ENC_KEY y devuelve los 32 B de clave.
// Acepta base64 de 32 B o string crudo de 32 B exactos; cualquier otra
// cosa es fail-fast. Nunca loguear la clave ni el plaintext.
func ParseKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("tenants: empty encryption key")
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, fmt.Errorf("tenants: encryption key must decode to exactly 32 bytes (base64 of 32 B or raw 32-char string)")
}

// Crypter cifra y descifra tokens de bot con AES-GCM. El nonce (12 B de
// crypto/rand) va preprended al ciphertext (nonce‖ciphertext).
type Crypter struct {
	gcm cipher.AEAD
}

// NewCrypter construye el cifrador sobre la clave ya validada.
func NewCrypter(key []byte) (*Crypter, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("tenants: encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("tenants: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("tenants: new GCM: %w", err)
	}
	return &Crypter{gcm: gcm}, nil
}

// Encrypt cifra el token en claro y devuelve nonce‖ciphertext.
func (c *Crypter) Encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("tenants: encrypt nonce: %w", err)
	}
	return c.gcm.Seal(nonce, nonce, plain, nil), nil
}

// Decrypt descifra un blob nonce‖ciphertext al token en claro.
func (c *Crypter) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) < nonceSize {
		return nil, fmt.Errorf("tenants: ciphertext too short")
	}
	plain, err := c.gcm.Open(nil, blob[:nonceSize], blob[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("tenants: decrypt: %w", err)
	}
	return plain, nil
}
