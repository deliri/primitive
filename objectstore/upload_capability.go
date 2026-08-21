package objectstore

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// CapabilityURLMaximumBytes bounds one received upload or download bearer.
	// Vendor V4 signed URLs carry their whole credential in the query string,
	// so the bound is deliberately generous rather than tight.
	CapabilityURLMaximumBytes = 8 * 1024
	// CapabilityJSONMaximumBytes bounds one received bearer document in either
	// direction. Both shapes carry the same bounded provider, method, URL,
	// expiry, and signed-header fields.
	// Every term is derived rather than chosen: the URL bound, the
	// signed-header aggregate this package already owns, the JSON punctuation
	// each header object adds beyond its name and value, and the punctuation and
	// member names of the widest possible document.
	CapabilityJSONMaximumBytes = canonicalJSONStringMaximumExpansion*CapabilityURLMaximumBytes +
		canonicalJSONStringMaximumExpansion*SignedHeaderMaximumBytes +
		SignedHeaderMaximumCount*uploadCapabilityHeaderSyntaxBytes +
		uploadCapabilityDocumentSyntaxBytes
	// canonicalJSONStringMaximumExpansion is the widest canonical JSON
	// expansion of one admitted source byte. encoding/json emits HTML-sensitive
	// ASCII and control bytes as six-byte Unicode escapes. Receiver and issuer
	// therefore share a bound over the bytes actually carried on the wire.
	canonicalJSONStringMaximumExpansion = 6

	// UploadMethodTokenSignedPut is the wire token for a whole-object signed
	// PUT, which Amazon S3 and Google Cloud Storage publish.
	UploadMethodTokenSignedPut = "signed_put"
	// UploadMethodTokenMultipartPost is the wire token for a one-time multipart
	// POST, which Cloudflare Images publishes.
	UploadMethodTokenMultipartPost = "multipart_post"
	// UploadCapabilityCommitmentDomain separates a capability commitment from
	// every other SHA-256 use. The zero separator closes the domain before the
	// exact canonical capability document begins.
	UploadCapabilityCommitmentDomain = "primitive/objectstore/upload-capability-commitment/v1"
	// UploadCapabilityCommitmentFrameSeparator terminates the commitment domain
	// before the canonical capability document.
	UploadCapabilityCommitmentFrameSeparator byte = 0

	// uploadCapabilityHeaderSyntaxBytes is the exact JSON punctuation and member
	// names one header object adds beyond the name and value already counted by
	// SignedHeaderMaximumBytes.
	uploadCapabilityHeaderSyntaxBytes = len(`{"name":"","value":""},`)
	// uploadCapabilityDocumentSyntaxBytes is the exact punctuation and member
	// names of the widest document: the longest provider and method tokens, the
	// longest signed nanosecond expiry, and empty url and header payloads, both
	// of which are counted separately.
	uploadCapabilityDocumentSyntaxBytes = len(
		`{"provider":"google_cloud_storage","method":"multipart_post",` +
			`"url":"","expires_at":-9223372036854775808,"headers":[]}`,
	)

	uploadCapabilityReceiverErrorText           = "nil upload capability receiver"
	uploadCapabilityMemberErrorText             = "upload capability member is absent"
	uploadCapabilityURLExtentErrorText          = "upload capability url extent is outside the supported range"
	uploadCapabilityDocumentErrorText           = "upload capability document extent is outside the supported range"
	uploadCapabilityProviderErrorText           = "upload capability provider token is unknown"
	uploadCapabilityMethodErrorText             = "upload capability method is not published by the named vendor"
	uploadCapabilityUnsetErrorText              = "upload capability is unset"
	uploadCapabilityProjectionErrorText         = "upload capability projection is unset"
	uploadCapabilityCommitmentErrorText         = "upload capability commitment is unset"
	uploadCapabilityCommitmentReceiverErrorText = "nil upload capability commitment receiver"
	uploadCapabilityUTF8ErrorText               = "upload capability member is not valid utf-8"
)

// UploadCapability is one already-issued upload capability as it arrives over
// an API. It exists because this package deliberately keeps its execution
// values off the wire: SignedURL has no string accessor and redacts under every
// formatting verb, so a capability received as JSON cannot be projected onto a
// transfer by any package that cannot validate a signed URL. Objectstore is
// that package.
//
// It decodes only. The type therefore implements json.Unmarshaler and not
// json.Marshaler, and it never retains the received wire text: the URL is
// parsed into an opaque SignedURL and the source document is dropped. No public
// operation can emit the received bearer. Commitment derives only an opaque
// digest from the package-owned canonical projection. An issuer that already
// owns a signed UploadTarget uses the nominal UploadCapabilityProjection.
//
// It is not a grant. A grant binds a capability to an authorization, an object
// identity, a declaration, and a receipt; that binding stays with the issuing
// protocol. This type carries only what a transfer needs.
//
// The zero value is unset. Construct it by decoding one capability document.
type UploadCapability struct {
	target   UploadTarget
	provider Provider
	set      bool
}

// UploadCapabilityProjection is the encode-only projection of one
// already-issued UploadTarget. It does not create a bucket, credential,
// signature, grant, or signed URL. The caller owns those authorization facts;
// Objectstore owns only the exact vendor capability document its receiver and
// streaming transfer consume.
//
// The type implements json.Marshaler and deliberately does not implement
// json.Unmarshaler. Its signed URL has no accessor and every formatting verb is
// redacted. The only operation that emits the bearer is an explicit JSON
// marshal at the issuing boundary.
//
// The zero value is unset. Construct a value with
// NewUploadCapabilityProjection.
type UploadCapabilityProjection struct {
	target   UploadTarget
	provider Provider
	set      bool
}

// UploadCapabilityCommitment is the domain-separated SHA-256 closure of one
// exact canonical upload capability document. It carries no bearer material
// and is safe for a higher protocol to sign beside the separately transported
// capability. The zero value is invalid.
type UploadCapabilityCommitment struct {
	digest core.SHA256Digest
}

func newUploadCapabilityCommitment(
	digest core.SHA256Digest,
) (UploadCapabilityCommitment, error) {
	candidate := UploadCapabilityCommitment{digest: digest}
	if err := candidate.Validate(); err != nil {
		return UploadCapabilityCommitment{}, err
	}
	return candidate, nil
}

// Validate rejects an unset digest.
func (c UploadCapabilityCommitment) Validate() error {
	if err := c.digest.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityCommitmentErrorText), err)
	}
	return nil
}

// MarshalJSON emits the non-secret digest as canonical lowercase hexadecimal.
func (c UploadCapabilityCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(c.digest)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	return encoded, nil
}

// UnmarshalJSON accepts one canonical SHA-256 commitment and preserves the
// receiver on every rejection.
func (c *UploadCapabilityCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityCommitmentReceiverErrorText))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	candidate, err := newUploadCapabilityCommitment(digest)
	if err != nil {
		return err
	}
	*c = candidate
	return nil
}

// NewUploadCapabilityProjection validates and owns the projection of one
// already-issued provider target.
func NewUploadCapabilityProjection(
	provider Provider,
	target UploadTarget,
) (UploadCapabilityProjection, error) {
	candidate := UploadCapabilityProjection{
		target:   target,
		provider: provider,
		set:      true,
	}
	if err := candidate.Validate(); err != nil {
		return UploadCapabilityProjection{}, err
	}
	return candidate, nil
}

// uploadCapabilityWire is the private exact wire temporary shared by the
// nominal issuer projection and receiver. Every required member is a pointer
// so receiver-side absence is refused explicitly rather than arriving as a
// zero value that a later check would have to guess about. Headers is optional
// on receipt; the issuer emits the canonical empty array.
type uploadCapabilityWire struct {
	Provider  *string                      `json:"provider"`
	Method    *string                      `json:"method"`
	URL       *string                      `json:"url"`
	ExpiresAt *temporal.NumericInstant     `json:"expires_at"`
	Headers   []uploadCapabilityHeaderWire `json:"headers"`
}

// uploadCapabilityHeaderWire is one received signed request field. Sending a
// different set than the issuer signed invalidates the signature at the vendor,
// so the set is carried rather than reconstructed.
type uploadCapabilityHeaderWire struct {
	Name  *string `json:"name"`
	Value *string `json:"value"`
}

// UnmarshalJSON decodes one capability document into a private temporary,
// projects it onto this package's owned values, and only then writes the
// receiver. Every rejection leaves the receiver untouched.
func (c *UploadCapability) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityReceiverErrorText))
	}
	wire, err := decodeUploadCapabilityWire(data)
	if err != nil {
		return err
	}
	candidate, err := projectUploadCapability(wire)
	if err != nil {
		return err
	}
	*c = candidate
	return nil
}

func decodeUploadCapabilityWire(data []byte) (uploadCapabilityWire, error) {
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

func projectUploadCapability(wire uploadCapabilityWire) (UploadCapability, error) {
	if wire.Provider == nil || wire.Method == nil ||
		wire.URL == nil || wire.ExpiresAt == nil {
		return UploadCapability{}, errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityMemberErrorText))
	}
	provider, err := parseProviderToken(*wire.Provider)
	if err != nil {
		return UploadCapability{}, err
	}
	if err := validateUploadMethodToken(provider, *wire.Method); err != nil {
		return UploadCapability{}, err
	}
	target, err := projectUploadCapabilityTarget(wire, *wire.URL)
	if err != nil {
		return UploadCapability{}, err
	}
	candidate := UploadCapability{target: target, provider: provider, set: true}
	if err := candidate.Validate(); err != nil {
		return UploadCapability{}, err
	}
	return candidate, nil
}

func projectUploadCapabilityTarget(
	wire uploadCapabilityWire,
	rawURL string,
) (UploadTarget, error) {
	if err := validateUploadCapabilityURLExtent(rawURL); err != nil {
		return UploadTarget{}, err
	}
	signed, err := ParseSignedURL(rawURL)
	if err != nil {
		return UploadTarget{}, err
	}
	expiresAt, err := wire.ExpiresAt.Instant()
	if err != nil {
		return UploadTarget{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	headers, err := projectUploadCapabilityHeaders(wire.Headers)
	if err != nil {
		return UploadTarget{}, err
	}
	return UploadTarget{Headers: headers, URL: signed, ExpiresAt: expiresAt}, nil
}

func projectUploadCapabilityHeaders(
	wire []uploadCapabilityHeaderWire,
) (SignedHeaders, error) {
	projected := make([]SignedHeader, 0, len(wire))
	for _, received := range wire {
		if received.Name == nil || received.Value == nil {
			return SignedHeaders{}, errors.Join(core.ErrObjectStoreContract,
				errors.New(uploadCapabilityMemberErrorText))
		}
		name, err := core.ParseHTTPHeaderName(*received.Name)
		if err != nil {
			return SignedHeaders{}, errors.Join(core.ErrObjectStoreContract, err)
		}
		header, err := NewSignedHeader(name, *received.Value)
		if err != nil {
			return SignedHeaders{}, err
		}
		projected = append(projected, header)
	}
	return NewSignedHeaders(projected)
}

// parseProviderToken admits one exact provider token. The accepted spelling is
// Provider's own name, so the wire vocabulary and the execution domain cannot
// drift into two tables that disagree.
func parseProviderToken(token string) (Provider, error) {
	for provider := ProviderUnknown + 1; provider < providerLimit; provider++ {
		if provider.String() == token {
			return provider, nil
		}
	}
	return ProviderUnknown, errors.Join(core.ErrObjectStoreContract,
		errors.New(uploadCapabilityProviderErrorText))
}

// validateUploadMethodToken refuses a declared method the named vendor does not
// publish. The token is not a second source of truth for how the transfer runs;
// the vendor specification already decides that. It is the issuer's assertion
// about what its signature covers, and a disagreement means the capability
// would be spent on a request the vendor will reject.
func validateUploadMethodToken(provider Provider, token string) error {
	spec, err := Spec(provider)
	if err != nil {
		return err
	}
	expected, err := uploadMethodToken(spec)
	if err != nil {
		return err
	}
	if token != expected {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityMethodErrorText))
	}
	return nil
}

func uploadMethodToken(spec VendorSpec) (string, error) {
	switch {
	case spec.UploadMethod == exchange.MethodPut &&
		spec.UploadEncoding == UploadEncodingRawObject:
		return UploadMethodTokenSignedPut, nil
	case spec.UploadMethod == exchange.MethodPost &&
		spec.UploadEncoding == UploadEncodingMultipartFile:
		return UploadMethodTokenMultipartPost, nil
	default:
		return "", errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityMethodErrorText))
	}
}

func validateUploadCapabilityURLExtent(rawURL string) error {
	if len(rawURL) == 0 || len(rawURL) > CapabilityURLMaximumBytes {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityURLExtentErrorText))
	}
	return nil
}

func validateUploadCapabilityTarget(provider Provider, target UploadTarget) error {
	if err := provider.Validate(); err != nil {
		return err
	}
	if err := target.Validate(); err != nil {
		return err
	}
	if err := validateUploadCapabilityURLExtent(target.URL.value.String()); err != nil {
		return err
	}
	if err := validateUploadCapabilityUTF8(target); err != nil {
		return err
	}
	if err := target.validateFor(provider); err != nil {
		return err
	}
	return validateProviderSignedHeaders(provider, target)
}

func validateUploadCapabilityUTF8(target UploadTarget) error {
	if !utf8.ValidString(target.URL.value.String()) {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityUTF8ErrorText))
	}
	for _, header := range target.Headers.values {
		if !utf8.ValidString(header.name.String()) ||
			!utf8.ValidString(*header.value) {
			return errors.Join(core.ErrObjectStoreContract,
				errors.New(uploadCapabilityUTF8ErrorText))
		}
	}
	return nil
}

// Validate rejects an unset capability and rechecks the projected target under
// the provider it names, which is the same gate the transfer entry point
// applies.
func (c UploadCapability) Validate() error {
	if !c.set {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityUnsetErrorText))
	}
	return validateUploadCapabilityTarget(c.provider, c.target)
}

// IsZero reports whether no capability has been decoded.
func (c UploadCapability) IsZero() bool { return !c.set }

// Provider returns the vendor the capability names, so a caller can select the
// compiler-owned transfer entry point for it. This package publishes no runtime
// provider dispatch; the selection stays an explicit caller switch.
func (c UploadCapability) Provider() (Provider, error) {
	if err := c.Validate(); err != nil {
		return ProviderUnknown, err
	}
	return c.provider, nil
}

// Target returns the validated transfer target. The signed URL inside it stays
// opaque and redacted, so handing out the target does not hand out the bearer.
func (c UploadCapability) Target() (UploadTarget, error) {
	if err := c.Validate(); err != nil {
		return UploadTarget{}, err
	}
	return c.target, nil
}

// Commitment returns the non-secret closure of the exact canonical capability
// this receiver admitted. It does not expose or return the bearer document.
func (c UploadCapability) Commitment() (UploadCapabilityCommitment, error) {
	if err := c.Validate(); err != nil {
		return UploadCapabilityCommitment{}, err
	}
	return deriveUploadCapabilityCommitment(c.provider, c.target)
}

// Format redacts under every formatting verb, including %v and %#v, because the
// capability carries a bearer credential.
func (UploadCapability) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// Validate rejects an unset projection and rechecks the exact provider target
// that will cross the external output boundary.
func (p UploadCapabilityProjection) Validate() error {
	if !p.set {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityProjectionErrorText))
	}
	return validateUploadCapabilityTarget(p.provider, p.target)
}

// IsZero reports whether no already-issued target has crossed the constructor.
func (p UploadCapabilityProjection) IsZero() bool { return !p.set }

// Commitment returns the non-secret closure of the exact canonical capability
// emitted by this issuer projection.
func (p UploadCapabilityProjection) Commitment() (UploadCapabilityCommitment, error) {
	if err := p.Validate(); err != nil {
		return UploadCapabilityCommitment{}, err
	}
	return deriveUploadCapabilityCommitment(p.provider, p.target)
}

// MarshalJSON emits the exact bounded capability document accepted by
// UploadCapability. This is the only operation that exposes the bearer.
func (p UploadCapabilityProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return marshalUploadCapability(p.provider, p.target)
}

func marshalUploadCapability(
	provider Provider,
	target UploadTarget,
) ([]byte, error) {
	wire, err := projectUploadCapabilityWire(provider, target)
	if err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	if len(encoded) > CapabilityJSONMaximumBytes {
		return nil, errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityDocumentErrorText))
	}
	return encoded, nil
}

func deriveUploadCapabilityCommitment(
	provider Provider,
	target UploadTarget,
) (UploadCapabilityCommitment, error) {
	encoded, err := marshalUploadCapability(provider, target)
	if err != nil {
		return UploadCapabilityCommitment{}, err
	}
	digest, err := capabilityCommitmentDigest(
		UploadCapabilityCommitmentDomain,
		UploadCapabilityCommitmentFrameSeparator,
		encoded,
	)
	if err != nil {
		return UploadCapabilityCommitment{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return newUploadCapabilityCommitment(digest)
}

func capabilityCommitmentDigest(
	domain string,
	separator byte,
	encoded []byte,
) (core.SHA256Digest, error) {
	writer := core.NewDigestWriter()
	if _, err := writer.Write([]byte(domain)); err != nil {
		return core.SHA256Digest{}, err
	}
	if _, err := writer.Write([]byte{separator}); err != nil {
		return core.SHA256Digest{}, err
	}
	if _, err := writer.Write(encoded); err != nil {
		return core.SHA256Digest{}, err
	}
	digest, _, err := writer.Seal()
	return digest, err
}

func projectUploadCapabilityWire(
	provider Provider,
	target UploadTarget,
) (uploadCapabilityWire, error) {
	spec, err := Spec(provider)
	if err != nil {
		return uploadCapabilityWire{}, err
	}
	method, err := uploadMethodToken(spec)
	if err != nil {
		return uploadCapabilityWire{}, err
	}
	expiresAt, err := temporal.NewNumericInstant(target.ExpiresAt)
	if err != nil {
		return uploadCapabilityWire{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	providerToken := provider.String()
	rawURL := target.URL.value.String()
	return uploadCapabilityWire{
		Provider:  &providerToken,
		Method:    &method,
		URL:       &rawURL,
		ExpiresAt: &expiresAt,
		Headers:   projectUploadCapabilityHeaderWire(target.Headers),
	}, nil
}

func projectUploadCapabilityHeaderWire(
	headers SignedHeaders,
) []uploadCapabilityHeaderWire {
	wire := make([]uploadCapabilityHeaderWire, len(headers.values))
	for index, header := range headers.values {
		name := header.name.String()
		value := *header.value
		wire[index] = uploadCapabilityHeaderWire{Name: &name, Value: &value}
	}
	return wire
}

// Format redacts under every formatting verb, including %v and %#v, because
// the projection carries a bearer credential.
func (UploadCapabilityProjection) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

var (
	_ core.Validatable            = UploadCapability{}
	_ fmt.Formatter               = UploadCapability{}
	_ core.ValidatedJSONMarshaler = UploadCapabilityProjection{}
	_ fmt.Formatter               = UploadCapabilityProjection{}
	_ core.ValidatedJSONMarshaler = UploadCapabilityCommitment{}
)
