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

const (
	algorithm = "argon2id"
	version   = argon2.Version
	memoryKiB = 19 * 1024
	timeCost  = 2
	threads   = 1
	saltBytes = 16
	keyBytes  = 32
)

var ErrInvalidHash = errors.New("invalid password hash")

func Hash(plain string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read random salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plain), salt, timeCost, memoryKiB, threads, keyBytes)
	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		algorithm,
		version,
		memoryKiB,
		timeCost,
		threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func Verify(plain, encoded string) (bool, error) {
	params, salt, expected, err := parse(encoded)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey([]byte(plain), salt, params.timeCost, params.memoryKiB, params.threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

type parameters struct {
	memoryKiB uint32
	timeCost  uint32
	threads   uint8
}

func parse(encoded string) (parameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != algorithm {
		return parameters{}, nil, nil, ErrInvalidHash
	}

	versionPart := strings.TrimPrefix(parts[2], "v=")
	parsedVersion, err := strconv.Atoi(versionPart)
	if err != nil || parsedVersion != version {
		return parameters{}, nil, nil, ErrInvalidHash
	}

	params, err := parseParameters(parts[3])
	if err != nil {
		return parameters{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return parameters{}, nil, nil, ErrInvalidHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return parameters{}, nil, nil, ErrInvalidHash
	}

	return params, salt, hash, nil
}

func parseParameters(raw string) (parameters, error) {
	values := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return parameters{}, ErrInvalidHash
		}
		values[key] = value
	}

	memory, err := strconv.ParseUint(values["m"], 10, 32)
	if err != nil {
		return parameters{}, ErrInvalidHash
	}
	iterations, err := strconv.ParseUint(values["t"], 10, 32)
	if err != nil {
		return parameters{}, ErrInvalidHash
	}
	parallelism, err := strconv.ParseUint(values["p"], 10, 8)
	if err != nil {
		return parameters{}, ErrInvalidHash
	}

	return parameters{
		memoryKiB: uint32(memory),
		timeCost:  uint32(iterations),
		threads:   uint8(parallelism),
	}, nil
}
