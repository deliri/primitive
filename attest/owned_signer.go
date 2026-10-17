package attest

import (
	"crypto"
	"crypto/ed25519"
	"io"
	"sync"

	"github.com/deliri/primitive/v2026/core"
)

// SignerRequest admits a local Ed25519 key or an opaque external signer.
// Local bytes are copied; external capabilities remain owned by their provider.
type SignerRequest struct{ Signer crypto.Signer }

func (r SignerRequest) Validate() error {
	capability, err := newSigningCapability(r.Signer)
	if err == nil {
		capability.close()
	}
	return err
}

// Signer owns one serialized signing capability and its admitted public identity.
// Copies share custody. Destroy invalidates all copies; the zero value refuses.
// It implements crypto.Signer for standard-library protocol boundaries.
type Signer struct{ owner *serializedSigner }

type serializedSigner struct {
	mu         sync.Mutex
	capability *signingCapability
}

func NewSigner(request SignerRequest) (Signer, error) {
	capability, err := newSigningCapability(request.Signer)
	if err != nil {
		return Signer{}, err
	}
	return Signer{owner: &serializedSigner{capability: capability}}, nil
}

func (s Signer) Validate() error {
	_, err := s.PublicKey()
	return err
}

// PublicKey returns the immutable public identity admitted at construction.
func (s Signer) PublicKey() (core.Ed25519PublicKey, error) {
	if s.owner == nil {
		return core.Ed25519PublicKey{}, core.ErrAttestContract
	}
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	if s.owner.capability == nil {
		return core.Ed25519PublicKey{}, core.ErrAttestContract
	}
	return s.owner.capability.publicKey, nil
}

// Public returns an independent standard-library public-key projection.
func (s Signer) Public() crypto.PublicKey {
	public, err := s.PublicKey()
	if err != nil {
		return nil
	}
	data, err := public.Bytes()
	if err != nil {
		return nil
	}
	return ed25519.PublicKey(data)
}

// Sign serializes native observation and execution across every copied handle.
// A provider identity change withdraws the signature, including a change made
// during Sign. The provider's native failure remains reachable with errors.Is.
func (s Signer) Sign(random io.Reader, message []byte, options crypto.SignerOpts) ([]byte, error) {
	if s.owner == nil || options == nil {
		return nil, core.ErrAttestContract
	}
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	capability := s.owner.capability
	if capability == nil {
		return nil, core.ErrAttestContract
	}
	if err := observeSignerIdentity(capability); err != nil {
		return nil, err
	}
	signer := capability.signer
	signature, err := guardedCall(func() ([]byte, error) {
		return signer.Sign(random, message, options)
	})
	if err != nil {
		clear(signature)
		return nil, contractError(err)
	}
	if err := observeSignerIdentity(capability); err != nil {
		clear(signature)
		return nil, err
	}
	return signature, nil
}

func observeSignerIdentity(capability *signingCapability) error {
	observed, err := newExternalSigningCapability(capability.signer)
	if err != nil {
		return err
	}
	defer observed.close()
	if observed.publicKey != capability.publicKey {
		return core.ErrAttestSignerChanged
	}
	return nil
}

func (s Signer) Destroy() error {
	if s.owner == nil {
		return core.ErrAttestContract
	}
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	if s.owner.capability == nil {
		return core.ErrAttestContract
	}
	s.owner.capability.close()
	s.owner.capability = nil
	return nil
}

var (
	_ crypto.Signer    = Signer{}
	_ core.Validatable = Signer{}
	_ core.Validatable = SignerRequest{}
)
