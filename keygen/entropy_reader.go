package keygen

import (
	"crypto/rand"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// EntropyReader is a narrow capability over Go's production CSPRNG. Its fields
// are deliberately private so only NewEntropyReader can issue a usable value.
type EntropyReader struct{ admitted bool }

// NewEntropyReader issues one bounded entropy-reader capability.
func NewEntropyReader() EntropyReader { return EntropyReader{admitted: true} }

// Validate rejects the zero, unissued capability.
func (r EntropyReader) Validate() error {
	if !r.admitted {
		return contractError(errors.New("keygen entropy reader is not issued"))
	}
	return nil
}

// Read fills one bounded destination from Go's production CSPRNG.
func (r EntropyReader) Read(destination []byte) (int, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	if len(destination) == 0 {
		return 0, nil
	}
	if len(destination) > core.SecretMaterialMaximumBytes {
		return 0, contractError(errors.New("keygen entropy read exceeds its bound"))
	}
	count, err := rand.Read(destination)
	if err != nil {
		return count, entropyError(err)
	}
	return count, nil
}

var _ core.Validatable = EntropyReader{}
