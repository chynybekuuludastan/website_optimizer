// internal/utils/password/password.go
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameters for Argon2id password hashing
const (
	// Recommended values as of 2023
	memory      = 64 * 1024 // 64MB
	iterations  = 3         // Number of iterations
	parallelism = 4         // Number of threads to use
	saltLength  = 16        // 16 bytes salt
	keyLength   = 32        // 32 bytes of key
)

var (
	ErrInvalidHash         = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
)

// Hash generates a secure hash for the provided password
func Hash(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Generate the hash using argon2id
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	// Base64 encode the salt and hash
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format as $argon2id$v=19$m=memory,t=iterations,p=parallelism$salt$hash
	encodedHash := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory,
		iterations,
		parallelism,
		b64Salt,
		b64Hash,
	)

	return encodedHash, nil
}

// Verify checks if the provided password matches the stored hash
func Verify(password, encodedHash string) (bool, error) {
	// Extract the parameters, salt and hash from the encoded hash
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	// Compute the hash of the provided password using same parameters
	otherHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLength,
	)

	// Check if the computed hash matches the stored hash
	// Use subtle.ConstantTimeCompare to prevent timing attacks
	return subtle.ConstantTimeCompare(hash, otherHash) == 1, nil
}

// Params holds the parameters used for Argon2id hashing
type Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	keyLength   uint32
}

// decodeHash extracts parameters, salt and hash from an encoded hash string
func decodeHash(encodedHash string) (params Params, salt, hash []byte, err error) {
	// Split the encodedHash into its parts
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return Params{}, nil, nil, ErrInvalidHash
	}

	// Check the algorithm and version
	if parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrInvalidHash
	}

	var version int
	_, err = fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return Params{}, nil, nil, err
	}
	if version != argon2.Version {
		return Params{}, nil, nil, ErrIncompatibleVersion
	}

	// Parse the parameters
	var memory, iterations uint32
	var parallelism uint8
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return Params{}, nil, nil, err
	}

	// Decode the salt
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, err
	}

	// Decode the hash
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, err
	}
	keyLength := uint32(len(hash))

	params = Params{
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
		keyLength:   keyLength,
	}

	return params, salt, hash, nil
}
