// Package auth owns credentials: argon2id password hashing, JWT issue/verify,
// and the typed context keys that carry the authenticated user and their org
// role through a request. Domain packages and the HTTP layer both lean on it,
// but it imports neither — it is a leaf.
//
// Course mapping: Chapter 15 — password hashing (this file); Chapter 16 — JWT
// (token.go); Chapter 17 — RBAC role context (context.go).
//
// NOTE: The course stores password parameters in Config and supports lazy
// rehash-on-login. We keep a single fixed OWASP-recommended parameter set here
// to stay simple; the encoded hash still records its own params, so a future
// pass can add rotation without breaking stored hashes.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen = 16
	keyLen  = 32
)

// params controls the cost of a single hash. The values are recorded inside
// each encoded hash, so changing them does not invalidate old hashes. These
// follow current OWASP guidance for argon2id (m=64 MiB, t=3, p=1).
type params struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
}

var defaultParams = params{memory: 64 * 1024, iterations: 3, parallelism: 1}

// Hash turns a plaintext password into an argon2id-encoded string suitable to
// store in the database as-is. A fresh 16-byte salt is drawn from crypto/rand
// (never math/rand — the salt must be unguessable).
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		defaultParams.iterations,
		defaultParams.memory,
		defaultParams.parallelism,
		keyLen,
	)
	return encode(hash, salt, defaultParams), nil
}

// Verify reports whether password matches the argon2id-encoded string. The
// comparison is constant-time, defeating timing side channels. A malformed
// encoded string returns an error (distinct from a clean false) so logs can
// tell "wrong password" apart from "corrupt hash".
func Verify(password, encoded string) (bool, error) {
	p, salt, want, err := decode(encoded)
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	got := argon2.IDKey(
		[]byte(password),
		salt,
		p.iterations, p.memory, p.parallelism,
		uint32(len(want)),
	)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func encode(hash, salt []byte, p params) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		p.memory, p.iterations, p.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

func decode(s string) (params, []byte, []byte, error) {
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return params{}, nil, nil, errors.New("not an argon2id hash")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return params{}, nil, nil, errors.New("malformed version segment")
	}
	if version != argon2.Version {
		return params{}, nil, nil, errors.New("argon2 version mismatch")
	}

	var p params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return params{}, nil, nil, errors.New("malformed params segment")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return params{}, nil, nil, errors.New("malformed salt")
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return params{}, nil, nil, errors.New("malformed hash")
	}
	return p, salt, hash, nil
}
