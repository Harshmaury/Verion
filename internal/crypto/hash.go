package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const argon2SaltLen = 16 // 128-bit salt

// hashArgon2id hashes code using Argon2id with parameters from cfg.
// A fresh 16-byte salt is generated from crypto/rand for every call.
// Returns a hash string in the format: $argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<salt_b64>$<hash_b64>
func hashArgon2id(code string, cfg *Config) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("argon2id hash: generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(code),
		salt,
		cfg.Argon2Time,
		cfg.Argon2Memory,
		cfg.Argon2Threads,
		cfg.Argon2KeyLen,
	)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		cfg.Argon2Memory,
		cfg.Argon2Time,
		cfg.Argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// verifyArgon2id verifies code against a hash produced by hashArgon2id.
// Uses constant-time comparison to prevent timing attacks.
func verifyArgon2id(code, hashStr string) (bool, error) {
	params, salt, expectedHash, err := parseArgon2idHash(hashStr)
	if err != nil {
		return false, fmt.Errorf("argon2id verify: %w", err)
	}

	actualHash := argon2.IDKey(
		[]byte(code),
		salt,
		params.time,
		params.memory,
		params.threads,
		uint32(len(expectedHash)),
	)

	if subtle.ConstantTimeCompare(actualHash, expectedHash) == 1 {
		return true, nil
	}
	return false, nil
}

type argon2Params struct {
	time    uint32
	memory  uint32
	threads uint8
}

// parseArgon2idHash parses the encoded hash string produced by hashArgon2id.
// Expected format: $argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<salt_b64>$<hash_b64>
func parseArgon2idHash(encoded string) (params argon2Params, salt, hash []byte, err error) {
	parts := strings.Split(encoded, "$")
	// parts[0] = "" (before first $)
	// parts[1] = "argon2id"
	// parts[2] = "v=19"
	// parts[3] = "m=...,t=...,p=..."
	// parts[4] = salt_b64
	// parts[5] = hash_b64
	if len(parts) != 6 {
		return params, nil, nil, fmt.Errorf("invalid hash format: expected 6 segments, got %d", len(parts))
	}
	if parts[1] != "argon2id" {
		return params, nil, nil, fmt.Errorf("invalid hash format: not argon2id")
	}

	var version int
	if _, scanErr := fmt.Sscanf(parts[2], "v=%d", &version); scanErr != nil {
		return params, nil, nil, fmt.Errorf("invalid hash format: bad version: %w", scanErr)
	}
	if version != argon2.Version {
		return params, nil, nil, fmt.Errorf("invalid hash format: unsupported argon2 version %d", version)
	}

	var mem, t, p uint32
	if _, scanErr := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); scanErr != nil {
		return params, nil, nil, fmt.Errorf("invalid hash format: bad params: %w", scanErr)
	}
	params = argon2Params{time: t, memory: mem, threads: uint8(p)}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return params, nil, nil, fmt.Errorf("invalid hash format: bad salt: %w", err)
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return params, nil, nil, fmt.Errorf("invalid hash format: bad hash: %w", err)
	}

	return params, salt, hash, nil
}
