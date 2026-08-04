package objectstore

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// UploadCapabilityURLMaximumBytes bounds one received capability URL.
	// Vendor V4 signed URLs carry their whole credential in the query string,
	// so the bound is deliberately generous rather than tight.
	UploadCapabilityURLMaximumBytes = 8 * 1024
	// UploadCapabilityJSONMaximumBytes bounds one received capability document.
	// Every term is derived rather than chosen: the URL bound, the
	// signed-header aggregate this package already owns, the JSON punctuation
	// each header object adds beyond its name and value, and the punctuation and
	// member names of the widest possible document.
	UploadCapabilityJSONMaximumBytes = uploadCapabilityJSONStringMaximumExpansion*UploadCapabilityURLMaximumBytes +
		uploadCapabilityJSONStringMaximumExpansion*SignedHeaderMaximumBytes +
		SignedHeaderMaximumCount*uploadCapabilityHeaderSyntaxBytes +
		uploadCapabilityDocumentSyntaxBytes
	// uploadCapabilityJSONStringMaximumExpansion is the widest canonical JSON
	// expansion of one admitted source byte. encoding/json emits HTML-sensitive
	// ASCII and control bytes as six-byte Unicode escapes. Receiver and issuer
	// therefore share a bound over the bytes actually carried on the wire.
	uploadCapabilityJSONStringMaximumExpansion = 6

	// UploadMethodTokenSignedPut is the wire token for a whole-object signed
	// PUT, which Amazon S3 and Google Cloud Storage publish.
	UploadMethodTokenSignedPut = "signed_put"
	// UploadMethodTokenMultipartPost is the wire token for a one-time multipart
	// POST, which Cloudflare Images publishes.
	UploadMethodTokenMultipartPost = "multipart_post"

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

	uploadCapabilityReceiverErrorText   = "nil upload capability receiver"
	uploadCapabilityMemberErrorText     = "upload capability member is absent"
	uploadCapabilityURLExtentErrorText  = "upload capability url extent is outside the supported range"
	uploadCapabilityDocumentErrorText   = "upload capability document extent is outside the supported range"
	uploadCapabilityProviderErrorText   = "upload capability provider token is unknown"
	uploadCapabilityMethodErrorText     = "upload capability method is not published by the named vendor"
	uploadCapabilityUnsetErrorText      = "upload capability is unset"
	uploadCapabilityProjectionErrorText = "upload capability projection is unset"
	uploadCapabilityUTF8ErrorText       = "upload capability member is not valid utf-8"
)

// UploadCapability is one already-issued upload capability as it arrives over
// an API. It exists because this package deliberately keeps its execution
// values off the wire: SignedURL has no string accessor and redacts under every
// formatting verb, so a capability received as JSON cannot be projected onto a
// transfer by any package that cannot validate a signed URL. Objectstore is
// that package.
//
// It decodes only. The type therefore implements json.Unmarshaler and not
// json.Marshaler, and it never retains the received URL text: the bytes are
// parsed into an opaque SignedURL and dropped, so re-serializing the received
// bearer is structurally impossible. An issuer that already owns a signed
// UploadTarget uses the nominal UploadCapabilityProjection instead.
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
	limit, err := core.NewByteCount(uint64(UploadCapabilityJSONMaximumBytes))
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
	if len(rawURL) == 0 || len(rawURL) > UploadCapabilityURLMaximumBytes {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityURLExtentErrorText))
	}
	return nil
}

func validateUploadCapabilityTarget(provider Provider, target UploadTarget) error {
	if err := provider.Validate(); err != nil {
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
		if !utf8.ValidString(header.name.String()) || !utf8.ValidString(header.value) {
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

// MarshalJSON emits the exact bounded capability document accepted by
// UploadCapability. This is the only operation that exposes the bearer.
func (p UploadCapabilityProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	wire, err := projectUploadCapabilityWire(p.provider, p.target)
	if err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	if len(encoded) > UploadCapabilityJSONMaximumBytes {
		return nil, errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityDocumentErrorText))
	}
	return encoded, nil
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
		value := header.value
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
)
