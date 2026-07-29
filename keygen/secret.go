package keygen

import (
	"crypto/rand"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type entropyRead func([]byte) (int, error)

// SecretRequest carries the exact requested generic-secret extent.
type SecretRequest struct {
	Size core.ByteCount
}

// Validate admits the complete Core-owned secret-material size interval.
func (r SecretRequest) Validate() error {
	if err := r.Size.Validate(); err != nil {
		return contractError(err)
	}
	size, err := r.Size.Uint64()
	if err != nil {
		return contractError(err)
	}
	if size < core.SecretMaterialMinimumBytes ||
		size > core.SecretMaterialMaximumBytes {
		return contractError(errors.New("keygen secret size is outside the admitted interval"))
	}
	return nil
}

// GenerateSecret constructs caller-sized generic secret material with
// crypto/rand.Read. Go 1.26 guarantees that Read fills the complete buffer or
// terminates the process irrecoverably when the secure source fails.
func GenerateSecret(request SecretRequest) (core.SecretMaterial, error) {
	return generateSecretWithRead(request, func(destination []byte) (int, error) {
		return rand.Read(destination)
	})
}

func generateSecretWithRead(
	request SecretRequest,
	read entropyRead,
) (core.SecretMaterial, error) {
	if err := request.Validate(); err != nil {
		return core.SecretMaterial{}, err
	}
	if read == nil {
		return core.SecretMaterial{}, contractError(errors.New("keygen entropy reader is missing"))
	}
	size, err := request.Size.Uint64()
	if err != nil {
		return core.SecretMaterial{}, contractError(err)
	}
	raw := make([]byte, size)
	defer clear(raw)
	count, err := read(raw)
	if err != nil || count != len(raw) {
		return core.SecretMaterial{}, entropyError(err)
	}
	material, err := core.NewSecretMaterial(raw)
	if err != nil {
		// Core owns the all-zero rejection, so the classification asks Core
		// through its identity instead of re-deriving the rule over raw. A local
		// predicate would be a second home for one fact and could disagree with
		// Core silently.
		if errors.Is(err, core.ErrSecretMaterialAllZero) {
			return core.SecretMaterial{}, entropyError(err)
		}
		return core.SecretMaterial{}, contractError(err)
	}
	return material, nil
}
