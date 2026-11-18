// Package auth is the owner's sign-in: passwords stored only as Argon2id hashes, sessions whose text is never kept, the
// limit on failed attempts, and the idle sign-out.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the cost of one Argon2id hash.
type Params struct {
	// MemoryKiB is the memory one hash uses, in KiB.
	MemoryKiB uint32
	// Time is the number of passes over that memory.
	Time uint32
	// Threads is the degree of parallelism.
	Threads uint8
}

// OWASP is the first choice of OWASP's Password Storage Cheat Sheet for Argon2id, read on 2026-11-17
// (https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html): 19 MiB of memory, 2 iterations,
// 1 degree of parallelism.
var OWASP = Params{MemoryKiB: 19 * 1024, Time: 2, Threads: 1}

const (
	saltSize = 16
	keySize  = 32
	// minMemoryPerThread is the smallest memory Argon2 accepts for each thread, in KiB.
	minMemoryPerThread = 8
)

// Hasher makes and checks password hashes.
type Hasher interface {
	// Hash returns the encoded hash of a password, with a new salt.
	Hash(password string) (string, error)
	// Verify says whether the password is the one the encoded hash was made from. A hash that cannot be read is an
	// error, never "wrong password".
	Verify(password, encoded string) (bool, error)
}

// Argon2id is the Hasher of this program: the PHC string format, `$argon2id$v=19$m=…,t=…,p=…$<salt>$<hash>`.
type Argon2id struct {
	params Params
	random io.Reader
}

// NewArgon2id returns a hasher that makes hashes of the given cost. It checks every hash with the cost the hash states.
func NewArgon2id(params Params) Argon2id {
	return Argon2id{params: params, random: rand.Reader}
}

// Hash implements Hasher.
func (h Argon2id) Hash(password string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(h.random, salt); err != nil {
		return "", fmt.Errorf("make a salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, h.params.Time, h.params.MemoryKiB, h.params.Threads, keySize)
	enc := base64.RawStdEncoding

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.MemoryKiB, h.params.Time, h.params.Threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify implements Hasher.
func (Argon2id) Verify(password, encoded string) (bool, error) {
	params, salt, want, err := parse(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, params.Time, params.MemoryKiB, params.Threads, uint32(len(want))) //nolint:gosec // G115: the length of a hash that was decoded from a short text.

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

var errMalformedHash = errors.New("the stored password hash cannot be read")

func parse(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, fmt.Errorf("%w: it is not an argon2id hash", errMalformedHash)
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, fmt.Errorf("%w: version %q", errMalformedHash, parts[2])
	}

	params, err := parseParams(parts[3])
	if err != nil {
		return Params{}, nil, nil, err
	}

	enc := base64.RawStdEncoding

	salt, err := enc.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Params{}, nil, nil, fmt.Errorf("%w: its salt", errMalformedHash)
	}

	key, err := enc.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, fmt.Errorf("%w: its hash", errMalformedHash)
	}

	return params, salt, key, nil
}

func parseParams(text string) (Params, error) {
	fields := strings.Split(text, ",")
	if len(fields) != 3 {
		return Params{}, fmt.Errorf("%w: its parameters", errMalformedHash)
	}

	var numbers [3]uint64

	for i, prefix := range []string{"m=", "t=", "p="} {
		value, ok := strings.CutPrefix(fields[i], prefix)
		if !ok {
			return Params{}, fmt.Errorf("%w: its parameters", errMalformedHash)
		}

		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return Params{}, fmt.Errorf("%w: its parameters", errMalformedHash)
		}

		numbers[i] = n
	}

	memory, passes, threads := numbers[0], numbers[1], numbers[2]
	if passes < 1 || threads < 1 || threads > 255 || memory < minMemoryPerThread*threads {
		return Params{}, fmt.Errorf("%w: its parameters are out of range", errMalformedHash)
	}

	return Params{MemoryKiB: uint32(memory), Time: uint32(passes), Threads: uint8(threads)}, nil //nolint:gosec // G115: bounded by the checks above.
}
