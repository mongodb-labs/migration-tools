// Package random provides convenience wrappers around crypto/rand.
package random

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// Uint64 generates a cryptographically secure random uint64.
// It returns an error if the underlying system fails to provide random data.
func Uint64() (uint64, error) {
	var b [8]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return 0, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Endianness doesn't matter for pure randomness.
	return binary.NativeEndian.Uint64(b[:]), nil
}

// MustUint64 generates a cryptographically secure random uint64.
// It panics if the underlying system fails to provide random data.
// This is useful for initialization where a failure to get entropy is fatal.
func MustUint64() uint64 {
	val, err := Uint64()

	assertNoError(err)

	return val
}

func assertNoError(err error) {
	if err != nil {
		panic(fmt.Sprintf("critical entropy failure: %v", err))
	}
}
