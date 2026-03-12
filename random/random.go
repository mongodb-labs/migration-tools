// Package random provides convenience wrappers around crypto/rand.
//
// NB: In tests it’s convenient to wrap these functions with lo.Must().
package random

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ccoveille/go-safecast/v2"
)

// Float64 returns, as a float64, a cryptographically secure pseudo-random number
// in the half-open interval [0.0, 1.0).
// It returns an error if the underlying system fails to provide random data.
func Float64() (float64, error) {
	v, err := Uint64()
	if err != nil {
		return 0, err
	}

	// Shift right by 11 to get exactly 53 bits of randomness (64 - 11 = 53).
	// 1 << 53 is 9007199254740992, the maximum exact integer in float64.
	// Dividing our 53 random bits by this number guarantees a float in [0.0, 1.0).
	return safecast.MustConvert[float64](v>>11) / (1 << 53), nil
}

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

// Uint64N returns a cryptographically secure random number in the range [0, n).
// It avoids math/big allocations by using rejection sampling.
func Uint64N(n uint64) (uint64, error) {
	if n == 0 {
		return 0, fmt.Errorf("invalid argument to Uint64N: n must be > 0")
	}

	// If n is a power of 2, we can just mask it (fastest path).
	if n&(n-1) == 0 {
		v, err := Uint64()
		if err != nil {
			return 0, err
		}
		return v & (n - 1), nil
	}

	// Calculate the maximum unbiased value.
	maxUnbiased := math.MaxUint64 - (math.MaxUint64 % n)

	// Rejection sampling loop
	for {
		v, err := Uint64()
		if err != nil {
			return 0, err
		}

		// If the value is within the unbiased range, return the modulo.
		// Otherwise, the loop restarts and we draw a new number.
		if v <= maxUnbiased {
			return v % n, nil
		}
	}
}
