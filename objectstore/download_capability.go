package objectstore

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// DownloadMethodTokenSignedGet is the one whole-object retrieval method.
	DownloadMethodTokenSignedGet = "signed_get"
	// DownloadCapabilityCommitmentDomain separates retrieval bearers from every
	// other digest, including an upload bearer with otherwise identical text.
	DownloadCapabilityCommitmentDomain = "primitive/objectstore/download-capability-commitment/v1"
	// DownloadCapabilityCommitmentFrameSeparator closes the commitment domain.
	DownloadCapabilityCommitmentFrameSeparator byte = 0

	downloadCapabilityUnsetDiagnostic       = "download capability is unset"
	downloadProjectionUnsetDiagnostic       = "download capability projection is unset"
	downloadCommitmentUnsetDiagnostic       = "download capability commitment is unset"
	downloadReceiverNilDiagnostic           = "nil download capability receiver"
	downloadCommitmentReceiverNilDiagnostic = "nil download capability commitment receiver"
	downloadMemberAbsentDiagnostic          = "download capability member is absent"
	downloadMethodDiagnostic                = "download capability method is unsupported"
	downloadExtentDiagnostic                = "download capability extent is outside the supported range"
	downloadUTF8Diagnostic                  = "download capability member is not valid utf-8"
)

// DownloadCapability is the receive-only form of one already-issued exact
// whole-object retrieval bearer. It deliberately cannot marshal itself.
type DownloadCapability struct {
	target   DownloadTarget
	provider Provider
	set      bool
}

// DownloadCapabilityProjection is the issue-only form of the same bearer.
// MarshalJSON is its sole disclosure boundary.
type DownloadCapabilityProjection struct {
	target   DownloadTarget
	provider Provider
	set      bool
}

// DownloadCapabilityCommitment is the non-secret closure signed by a higher
// retrieval protocol beside the separately transported bearer.
type DownloadCapabilityCommitment struct {
	digest core.SHA256Digest
}

// NewDownloadCapabilityProjection validates and owns an already-issued target.
func NewDownloadCapabilityProjection(provider Provider, target DownloadTarget) (DownloadCapabilityProjection, error) {
	candidate := DownloadCapabilityProjection{provider: provider, target: target, set: true}
	if err := candidate.Validate(); err != nil {
		return DownloadCapabilityProjection{}, err
	}
	return candidate, nil
}

func newDownloadCapabilityCommitment(digest core.SHA256Digest) (DownloadCapabilityCommitment, error) {
	candidate := DownloadCapabilityCommitment{digest: digest}
	if err := candidate.Validate(); err != nil {
		return DownloadCapabilityCommitment{}, err
	}
	return candidate, nil
}

// Validate rejects an unset receive-only bearer and rechecks its provider.
func (c DownloadCapability) Validate() error {
	if !c.set {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadCapabilityUnsetDiagnostic))
	}
	return validateDownloadCapabilityTarget(c.provider, c.target)
}

// IsZero reports whether no bearer has been decoded.
func (c DownloadCapability) IsZero() bool { return !c.set }

// Provider returns the compiler-owned transfer selector.
func (c DownloadCapability) Provider() (Provider, error) {
	if err := c.Validate(); err != nil {
		return ProviderUnknown, err
	}
	return c.provider, nil
}

// Target returns the validated target while its URL remains opaque.
func (c DownloadCapability) Target() (DownloadTarget, error) {
	if err := c.Validate(); err != nil {
		return DownloadTarget{}, err
	}
	return c.target, nil
}

// Commitment returns the non-secret closure of this exact bearer.
func (c DownloadCapability) Commitment() (DownloadCapabilityCommitment, error) {
	if err := c.Validate(); err != nil {
		return DownloadCapabilityCommitment{}, err
	}
	return deriveDownloadCapabilityCommitment(c.provider, c.target)
}

// UnmarshalJSON admits one bounded capability without mutating on refusal.
func (c *DownloadCapability) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadReceiverNilDiagnostic))
	}
	wire, err := decodeDownloadCapabilityWire(data)
	if err != nil {
		return err
	}
	candidate, err := projectDownloadCapability(wire)
	if err != nil {
		return err
	}
	*c = candidate
	return nil
}

// Format redacts the bearer under every formatting verb.
func (DownloadCapability) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// Validate rejects an unset issue-only projection.
func (p DownloadCapabilityProjection) Validate() error {
	if !p.set {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadProjectionUnsetDiagnostic))
	}
	return validateDownloadCapabilityTarget(p.provider, p.target)
}

// IsZero reports whether the projection has been constructed.
func (p DownloadCapabilityProjection) IsZero() bool { return !p.set }

// Commitment returns the non-secret closure of the emitted bearer.
func (p DownloadCapabilityProjection) Commitment() (DownloadCapabilityCommitment, error) {
	if err := p.Validate(); err != nil {
		return DownloadCapabilityCommitment{}, err
	}
	return deriveDownloadCapabilityCommitment(p.provider, p.target)
}

// MarshalJSON is the sole issuer boundary that discloses the bearer.
func (p DownloadCapabilityProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return marshalDownloadCapability(p.provider, p.target)
}

// Format redacts the issue-only bearer projection.
func (DownloadCapabilityProjection) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// Validate rejects an unset commitment.
func (c DownloadCapabilityCommitment) Validate() error {
	if err := c.digest.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadCommitmentUnsetDiagnostic), err)
	}
	return nil
}

// MarshalJSON emits only the non-secret digest.
func (c DownloadCapabilityCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONDocument(c.digest)
}

// UnmarshalJSON accepts one commitment transactionally.
func (c *DownloadCapabilityCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadCommitmentReceiverNilDiagnostic))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	candidate, err := newDownloadCapabilityCommitment(digest)
	if err != nil {
		return err
	}
	*c = candidate
	return nil
}

func decodeDownloadCapabilityWire(data []byte) (uploadCapabilityWire, error) {
	limit, err := core.NewByteCount(uint64(CapabilityJSONMaximumBytes))
	if err != nil {
		return uploadCapabilityWire{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = limit
	wire, err := core.DecodeStrictJSONStructure[uploadCapabilityWire](data, limits)
	if err != nil {
		return uploadCapabilityWire{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return wire, nil
}

func projectDownloadCapability(wire uploadCapabilityWire) (DownloadCapability, error) {
	if wire.Provider == nil || wire.Method == nil || wire.URL == nil || wire.ExpiresAt == nil {
		return DownloadCapability{}, errors.Join(core.ErrObjectStoreContract, errors.New(downloadMemberAbsentDiagnostic))
	}
	provider, err := parseProviderToken(*wire.Provider)
	if err != nil {
		return DownloadCapability{}, err
	}
	if *wire.Method != DownloadMethodTokenSignedGet {
		return DownloadCapability{}, errors.Join(core.ErrObjectStoreContract, errors.New(downloadMethodDiagnostic))
	}
	target, err := projectDownloadCapabilityTarget(wire)
	if err != nil {
		return DownloadCapability{}, err
	}
	candidate := DownloadCapability{provider: provider, target: target, set: true}
	if err := candidate.Validate(); err != nil {
		return DownloadCapability{}, err
	}
	return candidate, nil
}

func projectDownloadCapabilityTarget(wire uploadCapabilityWire) (DownloadTarget, error) {
	if len(*wire.URL) == 0 || len(*wire.URL) > CapabilityURLMaximumBytes {
		return DownloadTarget{}, errors.Join(core.ErrObjectStoreContract, errors.New(downloadExtentDiagnostic))
	}
	signed, err := ParseSignedURL(*wire.URL)
	if err != nil {
		return DownloadTarget{}, err
	}
	expiresAt, err := wire.ExpiresAt.Instant()
	if err != nil {
		return DownloadTarget{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	headers, err := projectUploadCapabilityHeaders(wire.Headers)
	if err != nil {
		return DownloadTarget{}, err
	}
	return DownloadTarget{Headers: headers, URL: signed, ExpiresAt: expiresAt}, nil
}

func validateDownloadCapabilityTarget(provider Provider, target DownloadTarget) error {
	if err := provider.Validate(); err != nil {
		return err
	}
	if err := target.Validate(); err != nil {
		return err
	}
	if len(target.URL.value.String()) == 0 || len(target.URL.value.String()) > CapabilityURLMaximumBytes {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadExtentDiagnostic))
	}
	if !utf8.ValidString(target.URL.value.String()) {
		return errors.Join(core.ErrObjectStoreContract, errors.New(downloadUTF8Diagnostic))
	}
	for _, header := range target.Headers.values {
		if !utf8.ValidString(header.name.String()) ||
			!utf8.ValidString(*header.value) {
			return errors.Join(core.ErrObjectStoreContract, errors.New(downloadUTF8Diagnostic))
		}
	}
	return target.ValidateFor(provider)
}

func projectDownloadCapabilityWire(provider Provider, target DownloadTarget) (uploadCapabilityWire, error) {
	if err := validateDownloadCapabilityTarget(provider, target); err != nil {
		return uploadCapabilityWire{}, err
	}
	expiresAt, err := temporal.NewNumericInstant(target.ExpiresAt)
	if err != nil {
		return uploadCapabilityWire{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	providerToken := provider.String()
	method := DownloadMethodTokenSignedGet
	rawURL := target.URL.value.String()
	return uploadCapabilityWire{
		Provider: &providerToken, Method: &method, URL: &rawURL, ExpiresAt: &expiresAt,
		Headers: projectUploadCapabilityHeaderWire(target.Headers),
	}, nil
}

func marshalDownloadCapability(provider Provider, target DownloadTarget) ([]byte, error) {
	wire, err := projectDownloadCapabilityWire(provider, target)
	if err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil || len(encoded) > CapabilityJSONMaximumBytes {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	return encoded, nil
}

func deriveDownloadCapabilityCommitment(provider Provider, target DownloadTarget) (DownloadCapabilityCommitment, error) {
	encoded, err := marshalDownloadCapability(provider, target)
	if err != nil {
		return DownloadCapabilityCommitment{}, err
	}
	digest, err := capabilityCommitmentDigest(
		DownloadCapabilityCommitmentDomain,
		DownloadCapabilityCommitmentFrameSeparator,
		encoded,
	)
	if err != nil {
		return DownloadCapabilityCommitment{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return newDownloadCapabilityCommitment(digest)
}

var (
	_ core.Validatable            = DownloadCapability{}
	_ fmt.Formatter               = DownloadCapability{}
	_ core.ValidatedJSONMarshaler = DownloadCapabilityProjection{}
	_ fmt.Formatter               = DownloadCapabilityProjection{}
	_ core.ValidatedJSONMarshaler = DownloadCapabilityCommitment{}
)
