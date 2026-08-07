package keygen

import (
	"crypto/rand"
	"encoding/binary"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// RandomTokenMaximumBytes bounds one public random-token draw. A token is a
// nonce or device label, not a stream: a caller wanting more than this wanted
// a different primitive.
const RandomTokenMaximumBytes = 64

// RandomUint64 draws one uniform 64-bit value from Go's production CSPRNG.
//
// It exists for the one bounded need a caller otherwise reaches into
// crypto/rand for: a seed or salt that is a single integer. keygen owns the
// entropy boundary, so the draw is made here rather than in every consumer
// that happens to want a random number.
func RandomUint64() (uint64, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, entropyError(err)
	}
	return binary.BigEndian.Uint64(raw[:]), nil
}

// RandomTokenRequest carries the exact requested public-token extent.
type RandomTokenRequest struct {
	Size core.ByteCount
}

// Validate admits a token size from one byte up to the token ceiling.
func (r RandomTokenRequest) Validate() error {
	if err := r.Size.Validate(); err != nil {
		return contractError(err)
	}
	size, err := r.Size.Uint64()
	if err != nil {
		return contractError(err)
	}
	if size == 0 || size > RandomTokenMaximumBytes {
		return contractError(errors.New("keygen token size is outside the admitted interval"))
	}
	return nil
}

// RandomToken fills one caller-sized public random token from Go's production
// CSPRNG.
//
// A token is public, not secret: unlike GenerateSecret it carries no all-zero
// rejection, because an all-zero draw is a legitimate nonce that a caller must
// not have to retry. It is a bounded one-shot value, not an entropy provider:
// the size is fixed and validated before the draw, so nothing here streams.
func RandomToken(request RandomTokenRequest) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	size, err := request.Size.Uint64()
	if err != nil {
		return nil, contractError(err)
	}
	token := make([]byte, size)
	if _, err := rand.Read(token); err != nil {
		return nil, entropyError(err)
	}
	return token, nil
}
