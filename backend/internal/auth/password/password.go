package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrInvalidHash indicates a password hash is malformed or unsupported.
var ErrInvalidHash = errors.New("invalid password hash")

// Params defines Argon2id parameters used to hash passwords.
type Params struct {
	Memory      uint32
	Time        uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams are the default Argon2id settings for this service.
var DefaultParams = Params{
	Memory:      64 * 1024,
	Time:        3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// Hash hashes a password using Argon2id with DefaultParams.
func Hash(password string) (string, error) {
	return HashWithParams(password, DefaultParams)
}

// HashWithParams hashes a password using Argon2id and returns an encoded hash string.
func HashWithParams(password string, params Params) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	if params.SaltLength == 0 || params.KeyLength == 0 {
		return "", errors.New("invalid params")
	}

	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Parallelism, params.KeyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Time,
		params.Parallelism,
		b64Salt,
		b64Key,
	)
	return encoded, nil
}

// Verify checks whether password matches the encoded Argon2id hash string.
func Verify(password, encodedHash string) (bool, error) {
	if password == "" {
		return false, errors.New("password is required")
	}
	params, salt, hash, err := decode(encodedHash)
	if err != nil {
		return false, err
	}

	if len(hash) > int(^uint32(0)) {
		return false, ErrInvalidHash
	}
	keyLen := uint32(len(hash)) //nolint:gosec // bounded by check above

	otherHash := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Parallelism, keyLen)
	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

func decode(encodedHash string) (Params, []byte, []byte, error) {
	var params Params

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return params, nil, nil, ErrInvalidHash
	}
	if parts[1] != "argon2id" {
		return params, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return params, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return params, nil, nil, ErrInvalidHash
	}

	if err := parseParams(parts[3], &params); err != nil {
		return params, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return params, nil, nil, ErrInvalidHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) == 0 {
		return params, nil, nil, ErrInvalidHash
	}

	if len(salt) > int(^uint32(0)) || len(hash) > int(^uint32(0)) {
		return params, nil, nil, ErrInvalidHash
	}

	params.SaltLength = uint32(len(salt)) //nolint:gosec // bounded by check above
	params.KeyLength = uint32(len(hash))  //nolint:gosec // bounded by check above
	return params, salt, hash, nil
}

func parseParams(input string, params *Params) error {
	if params == nil {
		return ErrInvalidHash
	}

	memory, err := parseUint32KV(input, "m")
	if err != nil {
		return ErrInvalidHash
	}
	time, err := parseUint32KV(input, "t")
	if err != nil {
		return ErrInvalidHash
	}
	parallelism, err := parseUint32KV(input, "p")
	if err != nil || parallelism == 0 || parallelism > 255 {
		return ErrInvalidHash
	}

	params.Memory = memory
	params.Time = time
	params.Parallelism = uint8(parallelism)
	return nil
}

func parseUint32KV(input, key string) (uint32, error) {
	for _, part := range strings.Split(input, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if k != key {
			continue
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, err
		}
		return uint32(n), nil
	}
	return 0, ErrInvalidHash
}
