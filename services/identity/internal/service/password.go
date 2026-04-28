package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/argon2"

	"sandbox_url_shortener/services/identity/internal/domain"
)

const (
	argonMemoryKiB   = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonKeyLen      = 32
	argonSaltLen     = 16
)

var dummyPasswordSalt = []byte("identity-dummy!!")

type passwordParams struct {
	MemoryKiB   uint32 `json:"memory_kib"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint8  `json:"parallelism"`
	Salt        string `json:"salt"`
	KeyLen      uint32 `json:"key_len"`
}

func hashPassword(password string) (string, []byte, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", nil, fmt.Errorf("password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyLen)
	params := passwordParams{
		MemoryKiB:   argonMemoryKiB,
		Iterations:  argonIterations,
		Parallelism: argonParallelism,
		Salt:        base64.RawStdEncoding.EncodeToString(salt),
		KeyLen:      argonKeyLen,
	}
	payload, err := json.Marshal(params)
	if err != nil {
		return "", nil, fmt.Errorf("password params: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(hash), payload, nil
}

func verifyPassword(password string, credential *domain.PasswordCredential) (bool, error) {
	if credential.Algorithm != domain.PasswordAlgorithmArgon2ID {
		return false, nil
	}
	var params passwordParams
	if err := json.Unmarshal(credential.Params, &params); err != nil {
		return false, fmt.Errorf("password params decode: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(params.Salt)
	if err != nil {
		return false, fmt.Errorf("password salt decode: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(credential.Hash)
	if err != nil {
		return false, fmt.Errorf("password hash decode: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, params.Iterations, params.MemoryKiB, params.Parallelism, params.KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func validatePasswordPolicy(password string) error {
	if len(password) < PasswordMinLength {
		return fmt.Errorf("%w: password must be at least %d characters", domain.ErrInvalidInput, PasswordMinLength)
	}
	if len([]byte(password)) > PasswordMaxBytes {
		return fmt.Errorf("%w: password must be at most %d bytes", domain.ErrInvalidInput, PasswordMaxBytes)
	}
	return nil
}

func runDummyPasswordHash(password string) {
	_ = argon2.IDKey([]byte(password), dummyPasswordSalt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyLen)
}
