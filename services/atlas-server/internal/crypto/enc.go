// Package crypto provides ENC(...) AES-GCM helpers for managed model API keys.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// DefaultModelSecret is the baked-in shared secret (client + server).
// Override with ATLAS_MODEL_SECRET_KEY / GROK_MODEL_SECRET_KEY.
const DefaultModelSecret = "atlas-managed-model-secret-v1"

const encPrefix = "ENC("
const encSuffix = ")"

// ResolveModelSecret returns env override or the baked-in default.
func ResolveModelSecret() string {
	if v := strings.TrimSpace(os.Getenv("ATLAS_MODEL_SECRET_KEY")); v != "" {
		return v
	}
	return DefaultModelSecret
}

// DeriveKey SHA-256s the secret into a 32-byte AES-256 key.
func DeriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// IsEnc reports whether s looks like ENC(...).
func IsEnc(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, encPrefix) && strings.HasSuffix(s, encSuffix)
}

// Encrypt encrypts plaintext and returns ENC(<base64(nonce||ciphertext)>).
func Encrypt(plaintext, secret string) (string, error) {
	if plaintext == "" {
		return "", errors.New("empty plaintext")
	}
	if IsEnc(plaintext) {
		return strings.TrimSpace(plaintext), nil
	}
	block, err := aes.NewCipher(DeriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(out) + encSuffix, nil
}

// Decrypt decrypts ENC(...) or returns plaintext unchanged when not wrapped.
func Decrypt(encOrPlain, secret string) (string, error) {
	s := strings.TrimSpace(encOrPlain)
	if s == "" {
		return "", nil
	}
	if !IsEnc(s) {
		return s, nil
	}
	payload := s[len(encPrefix) : len(s)-len(encSuffix)]
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("invalid ENC payload: %w", err)
	}
	block, err := aes.NewCipher(DeriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ENC payload too short")
	}
	plain, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}
	return string(plain), nil
}

// MaskEnc returns a display-safe hint for admin UIs.
func MaskEnc(enc string) string {
	s := strings.TrimSpace(enc)
	if s == "" {
		return ""
	}
	if IsEnc(s) {
		if len(s) > 16 {
			return s[:8] + "…" + s[len(s)-4:]
		}
		return "ENC(…)"
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
