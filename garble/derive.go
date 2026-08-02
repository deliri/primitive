package garble

import (
	"crypto/hkdf"
	"crypto/sha256"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// DerivationSalt domain-separates Primitive Garble seed derivation.
	DerivationSalt      = "primitive-garble-seed"
	derivationInfoBytes = 1 + sha256.Size
)

func derivationGenerationLabels() [derivationGenerationLimit]string {
	return [...]string{
		DerivationGenerationOne: "one",
	}
}

// DerivationGeneration is the closed Garble derivation protocol generation.
type DerivationGeneration uint8

const (
	// DerivationGenerationUnknown is the invalid zero generation.
	DerivationGenerationUnknown DerivationGeneration = iota
	// DerivationGenerationOne is the current HKDF-SHA-256 protocol.
	DerivationGenerationOne
	derivationGenerationLimit
)

// CurrentDerivationGeneration returns the one reviewed protocol generation.
func CurrentDerivationGeneration() DerivationGeneration {
	return DerivationGenerationOne
}

// Validate rejects generations outside the closed protocol domain.
func (g DerivationGeneration) Validate() error {
	if !g.IsValid() {
		return contractError(errors.New("garble derivation generation is outside the admitted domain"))
	}
	return nil
}

// IsValid reports membership in the closed derivation-generation domain.
func (g DerivationGeneration) IsValid() bool {
	return g > DerivationGenerationUnknown && g < derivationGenerationLimit &&
		derivationGenerationLabels()[g] != ""
}

// OffWireEnum declares DerivationGeneration as a local derivation parameter
// rather than a wire encoding.
func (DerivationGeneration) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for g.
func (g DerivationGeneration) String() string {
	if !g.IsValid() {
		return unknownEnumLabel
	}
	return derivationGenerationLabels()[g]
}

// DerivationIdentity is a canonical release identity projected as a SHA-256
// digest by its owning package.
type DerivationIdentity struct {
	digest core.SHA256Digest
}

// NewDerivationIdentity admits one set canonical release digest.
func NewDerivationIdentity(digest core.SHA256Digest) (DerivationIdentity, error) {
	if err := digest.Validate(); err != nil {
		return DerivationIdentity{}, contractError(err)
	}
	return DerivationIdentity{digest: digest}, nil
}

// Validate rejects an unset release identity.
func (i DerivationIdentity) Validate() error {
	if err := i.digest.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// DeriveRequest carries every input to one deterministic derivation.
type DeriveRequest struct {
	Custody    Custody
	Identity   DerivationIdentity
	Generation DerivationGeneration
}

type derivationFrame struct {
	generation DerivationGeneration
	digest     core.SHA256Digest
}

// Validate checks every derivation input at the package boundary.
func (r DeriveRequest) Validate() error {
	if err := r.Custody.Validate(); err != nil {
		return err
	}
	if err := r.Identity.Validate(); err != nil {
		return err
	}
	return r.Generation.Validate()
}

// Derive applies HKDF-SHA-256 to the complete custody root and canonical
// release identity, returning the exact seed width accepted by Garble.
func Derive(request DeriveRequest) (Seed, error) {
	if err := request.Validate(); err != nil {
		return Seed{}, derivationError(err)
	}
	secret, err := request.Custody.material.CopyBytes()
	if err != nil {
		return Seed{}, derivationError(contractError(err))
	}
	defer clear(secret)
	frame, err := request.derivationFrame()
	if err != nil {
		return Seed{}, derivationError(err)
	}
	info, err := frame.standardLibraryInfo()
	if err != nil {
		return Seed{}, derivationError(err)
	}
	derived, err := hkdf.Key(sha256.New, secret, []byte(DerivationSalt), info, SeedBytes)
	if err != nil {
		return Seed{}, derivationError(err)
	}
	defer clear(derived)
	var fixed [SeedBytes]byte
	copy(fixed[:], derived)
	return NewSeed(fixed), nil
}

func (r DeriveRequest) derivationFrame() (derivationFrame, error) {
	frame := derivationFrame{
		generation: r.Generation,
		digest:     r.Identity.digest,
	}
	if err := frame.Validate(); err != nil {
		return derivationFrame{}, err
	}
	return frame, nil
}

func (f derivationFrame) Validate() error {
	if err := f.generation.Validate(); err != nil {
		return err
	}
	if err := f.digest.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (f derivationFrame) standardLibraryInfo() (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	digest, err := f.digest.Bytes()
	if err != nil {
		return "", contractError(err)
	}
	var info [derivationInfoBytes]byte
	info[0] = byte(f.generation)
	copy(info[1:], digest[:])
	return string(info[:]), nil
}
