package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const aesNonceSize = 12 // 96-bit nonce for GCM

// aesEncrypt encrypts plaintext with AES-256-GCM using the provided 32-byte key.
// A fresh 12-byte IV (nonce) is generated from crypto/rand for every call.
// Returns ciphertext and IV separately; both must be stored.
func aesEncrypt(key, plaintext []byte) (ciphertext []byte, iv []byte, err error) {
	if len(key) != 32 {
		return nil, nil, fmt.Errorf("aes encrypt: master key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("aes encrypt: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("aes encrypt: create gcm: %w", err)
	}

	// Generate a fresh nonce for this encryption call.
	iv = make([]byte, aesNonceSize)
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return nil, nil, fmt.Errorf("aes encrypt: generate iv: %w", err)
	}

	// Seal appends the authentication tag to the ciphertext.
	// dst is nil so GCM allocates the output slice.
	ciphertext = gcm.Seal(nil, iv, plaintext, nil)
	return ciphertext, iv, nil
}

// aesDecrypt decrypts AES-256-GCM ciphertext using the provided key and IV.
func aesDecrypt(key, ciphertext, iv []byte) (plaintext []byte, err error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("aes decrypt: master key must be 32 bytes, got %d", len(key))
	}
	if len(iv) != aesNonceSize {
		return nil, fmt.Errorf("aes decrypt: iv must be %d bytes, got %d", aesNonceSize, len(iv))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes decrypt: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes decrypt: create gcm: %w", err)
	}

	plaintext, err = gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		// Don't leak details — authentication failure is opaque to callers.
		return nil, errors.New("aes decrypt: authentication failed")
	}
	return plaintext, nil
}
