package auth_test

import (
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/auth"
)

func TestTheDefaultParametersAreOWASPs(t *testing.T) {
	t.Parallel()

	// OWASP Password Storage Cheat Sheet, Argon2id, first choice, read 2026-11-17: 19 MiB of memory, 2 iterations,
	// 1 degree of parallelism.
	if got := auth.OWASP; got.MemoryKiB != 19*1024 || got.Time != 2 || got.Threads != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestAPasswordIsHashedAndVerified(t *testing.T) {
	t.Parallel()

	hasher := auth.NewArgon2id(auth.OWASP)

	hash, err := hasher.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") || strings.Contains(hash, "correct horse") {
		t.Fatalf("hash %q is not the standard encoding of the OWASP parameters", hash)
	}

	for password, want := range map[string]bool{"correct horse battery": true, "correct horse batterY": false, "": false} {
		got, err := hasher.Verify(password, hash)
		if err != nil || got != want {
			t.Fatalf("Verify(%q): %v, %v, want %v", password, got, err, want)
		}
	}
}

func TestTwoHashesOfOnePasswordDiffer(t *testing.T) {
	t.Parallel()

	hasher := auth.NewArgon2id(cheap)

	first, err1 := hasher.Hash("same password here")
	second, err2 := hasher.Hash("same password here")

	if err1 != nil || err2 != nil || first == second {
		t.Fatalf("a salt of its own makes every hash different: %v %v", err1, err2)
	}
}

func TestVerifyReadsTheParametersOutOfTheHash(t *testing.T) {
	t.Parallel()

	// A hash made with other parameters still verifies: a later release may raise the cost, and old hashes keep working.
	old, err := auth.NewArgon2id(cheap).Hash("an older password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := auth.NewArgon2id(auth.OWASP).Verify("an older password", old)
	if err != nil || !ok {
		t.Fatalf("Verify: %v, %v", ok, err)
	}
}

func TestAMalformedHashIsAnErrorNotAWrongPassword(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty":             "",
		"not argon2":        "$2a$10$abcdefghijklmnopqrstuuxyz",
		"missing parts":     "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA",
		"bad version":       "$argon2id$v=x$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"bad parameters":    "$argon2id$v=19$m=x,t=2,p=1$c2FsdA$aGFzaA",
		"bad salt":          "$argon2id$v=19$m=19456,t=2,p=1$***$aGFzaA",
		"bad hash":          "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$***",
		"zero iterations":   "$argon2id$v=19$m=19456,t=0,p=1$c2FsdA$aGFzaA",
		"too little memory": "$argon2id$v=19$m=1,t=2,p=1$c2FsdA$aGFzaA",
	}

	hasher := auth.NewArgon2id(cheap)

	for name, hash := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := hasher.Verify("whatever it is", hash); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}
