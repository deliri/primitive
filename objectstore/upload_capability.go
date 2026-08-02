package objectstore

import (
	"errors"
	"fmt"
	"io"

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
	UploadCapabilityJSONMaximumBytes = UploadCapabilityURLMaximumBytes +
		SignedHeaderMaximumBytes +
		SignedHeaderMaximumCount*uploadCapabilityHeaderSyntaxBytes +
		uploadCapabilityDocumentSyntaxBytes

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

	uploadCapabilityReceiverErrorText  = "nil upload capability receiver"
	uploadCapabilityMemberErrorText    = "upload capability member is absent"
	uploadCapabilityURLExtentErrorText = "upload capability url extent is outside the supported range"
	uploadCapabilityProviderErrorText  = "upload capability provider token is unknown"
	uploadCapabilityMethodErrorText    = "upload capability method is not published by the named vendor"
	uploadCapabilityUnsetErrorText     = "upload capability is unset"
)

// UploadCapability is one already-issued upload capability as it arrives over
// an API. It exists because this package deliberately keeps its execution
// values off the wire: SignedURL has no string accessor and redacts under every
// formatting verb, so a capability received as JSON cannot be projected onto a
// transfer by any package that cannot validate a signed URL. Objectstore is
// that package.
//
// It decodes only. Emitting a capability is issuing one, which this package's
// documented boundary excludes along with buckets, credentials, and signed-URL
// creation. The type therefore implements json.Unmarshaler and not
// json.Marshaler, and it never retains the received URL text: the bytes are
// parsed into an opaque SignedURL and dropped, so re-serializing the bearer is
// not merely discouraged but structurally impossible.
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

// uploadCapabilityWire is the private decode temporary. Every required member
// is a pointer so absence is refused explicitly rather than arriving as a zero
// value that a later check would have to guess about. Headers is optional;
// absence and an empty array both project to the one empty SignedHeaders value.
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
	if len(rawURL) == 0 || len(rawURL) > UploadCapabilityURLMaximumBytes {
		return UploadTarget{}, errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityURLExtentErrorText))
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

// Validate rejects an unset capability and rechecks the projected target under
// the provider it names, which is the same gate the transfer entry point
// applies.
func (c UploadCapability) Validate() error {
	if !c.set {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadCapabilityUnsetErrorText))
	}
	if err := c.provider.Validate(); err != nil {
		return err
	}
	if err := c.target.validateFor(c.provider); err != nil {
		return err
	}
	return validateProviderSignedHeaders(c.provider, c.target)
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

var (
	_ core.Validatable = UploadCapability{}
	_ fmt.Formatter    = UploadCapability{}
)
