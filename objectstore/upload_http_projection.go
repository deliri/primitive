package objectstore

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	uploadHTTPProjectionUnsetErrorText    = "upload HTTP projection is unset"
	uploadHTTPProjectionEncodingErrorText = "upload capability cannot be spent as one raw browser request"
	uploadHTTPProjectionHeaderErrorText   = "upload HTTP projection contains an unsupported header shape"
	uploadHTTPProjectionVersionErrorText  = "upload HTTP projection has no provider version response field"
)

// UploadHTTPProjection is the encode-only browser-spendable projection of one
// already-issued whole-object upload capability. It carries the same bearer as
// UploadCapabilityProjection and adds only the exact body declaration and the
// Objectstore-owned provider fields that a browser must send with that bearer.
// It does not mint, widen, persist, or decode authority.
//
// Host and Content-Length remain browser-owned framing. Multipart provider
// uploads are refused because their body boundary is not one raw object.
//
// The zero value is unset. Construct a value with NewUploadHTTPProjection.
type UploadHTTPProjection struct {
	contentType core.HTTPMediaType
	capability  UploadCapabilityProjection
	integrity   Integrity
	set         bool
}

// uploadHTTPProjectionWire is the private exact external-output temporary for
// one complete browser request and the one response field needed to identify
// the created object.
type uploadHTTPProjectionWire struct {
	Provider              *string                      `json:"provider"`
	Method                *exchange.Method             `json:"method"`
	URL                   *string                      `json:"url"`
	ExpiresAt             *temporal.NumericInstant     `json:"expires_at"`
	Bytes                 *core.ByteLength             `json:"bytes"`
	SHA256                *core.SHA256Digest           `json:"sha256"`
	CRC32C                *core.CRC32C                 `json:"crc32c"`
	ContentType           *core.HTTPMediaType          `json:"content_type"`
	ResponseVersionHeader *string                      `json:"response_version_header"`
	Headers               []uploadCapabilityHeaderWire `json:"headers"`
}

// NewUploadHTTPProjection validates one issued capability and exact body
// declaration, then owns their complete browser request projection.
func NewUploadHTTPProjection(
	capability UploadCapabilityProjection,
	integrity Integrity,
	contentType core.HTTPMediaType,
) (UploadHTTPProjection, error) {
	candidate := UploadHTTPProjection{
		capability:  capability,
		integrity:   integrity,
		contentType: contentType,
		set:         true,
	}
	if err := candidate.Validate(); err != nil {
		return UploadHTTPProjection{}, err
	}
	return candidate, nil
}

// Validate rejects unset, non-raw, or internally inconsistent projections.
func (p UploadHTTPProjection) Validate() error {
	if !p.set {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadHTTPProjectionUnsetErrorText))
	}
	if err := p.capability.Validate(); err != nil {
		return err
	}
	if err := p.integrity.Validate(); err != nil {
		return err
	}
	if err := p.contentType.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return validateUploadHTTPProvider(p.capability.provider)
}

// IsZero reports whether no complete browser request has crossed the
// constructor.
func (p UploadHTTPProjection) IsZero() bool { return !p.set }

func validateUploadHTTPProvider(provider Provider) error {
	spec, err := Spec(provider)
	if err != nil {
		return err
	}
	if spec.UploadEncoding != UploadEncodingRawObject ||
		spec.UploadMethod != exchange.MethodPut {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(uploadHTTPProjectionEncodingErrorText))
	}
	return nil
}

// Commitment returns the closure of the underlying issued capability. The
// body declaration remains a separately typed fact for its issuing protocol to
// bind beside this commitment.
func (p UploadHTTPProjection) Commitment() (UploadCapabilityCommitment, error) {
	if err := p.Validate(); err != nil {
		return UploadCapabilityCommitment{}, err
	}
	return p.capability.Commitment()
}

// MarshalJSON emits one complete bounded browser request projection.
func (p UploadHTTPProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	wire, err := projectUploadHTTPWire(p)
	if err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	return encoded, nil
}

func projectUploadHTTPWire(
	projection UploadHTTPProjection,
) (uploadHTTPProjectionWire, error) {
	capabilityWire, err := projectUploadCapabilityWire(
		projection.capability.provider,
		projection.capability.target,
	)
	if err != nil {
		return uploadHTTPProjectionWire{}, err
	}
	headers, err := projectUploadHTTPHeaders(projection)
	if err != nil {
		return uploadHTTPProjectionWire{}, err
	}
	versionHeader, err := providerVersionHeader(projection.capability.provider)
	if err != nil || versionHeader == "" {
		return uploadHTTPProjectionWire{}, errors.Join(
			core.ErrObjectStoreContract,
			errors.New(uploadHTTPProjectionVersionErrorText),
			err,
		)
	}
	method := exchange.MethodPut
	return uploadHTTPProjectionWire{
		Provider:              capabilityWire.Provider,
		Method:                &method,
		URL:                   capabilityWire.URL,
		ExpiresAt:             capabilityWire.ExpiresAt,
		Bytes:                 &projection.integrity.Length,
		SHA256:                &projection.integrity.SHA256,
		CRC32C:                &projection.integrity.CRC32C,
		ContentType:           &projection.contentType,
		Headers:               headers,
		ResponseVersionHeader: &versionHeader,
	}, nil
}

func projectUploadHTTPHeaders(
	projection UploadHTTPProjection,
) ([]uploadCapabilityHeaderWire, error) {
	headers, err := uploadHeaders(
		projection.capability.provider,
		projection.capability.target,
		projection.integrity,
	)
	if err != nil {
		return nil, err
	}
	wire := make([]uploadCapabilityHeaderWire, 0, len(headers.Values)+1)
	contentTypeName := core.HTTPHeaderContentType().String()
	contentTypeValue := projection.contentType.String()
	wire = append(wire, uploadCapabilityHeaderWire{
		Name:  &contentTypeName,
		Value: &contentTypeValue,
	})
	for _, header := range headers.Values {
		projected, projectErr := projectUploadHTTPHeader(header)
		if projectErr != nil {
			return nil, projectErr
		}
		wire = append(wire, projected)
	}
	slices.SortFunc(wire, compareUploadHTTPHeaderWire)
	return wire, nil
}

func projectUploadHTTPHeader(
	header exchange.Header,
) (uploadCapabilityHeaderWire, error) {
	if err := header.Validate(); err != nil || len(header.Values) != 1 {
		return uploadCapabilityHeaderWire{}, errors.Join(
			core.ErrObjectStoreContract,
			errors.New(uploadHTTPProjectionHeaderErrorText),
			err,
		)
	}
	value, err := header.Values[0].Value()
	if err != nil {
		return uploadCapabilityHeaderWire{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	name := header.Name.String()
	return uploadCapabilityHeaderWire{Name: &name, Value: &value}, nil
}

func compareUploadHTTPHeaderWire(
	left uploadCapabilityHeaderWire,
	right uploadCapabilityHeaderWire,
) int {
	return strings.Compare(*left.Name, *right.Name)
}

// Format redacts under every formatting verb because the projection carries a
// bearer credential.
func (UploadHTTPProjection) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

var (
	_ core.Validatable            = UploadHTTPProjection{}
	_ core.ValidatedJSONMarshaler = UploadHTTPProjection{}
	_ fmt.Formatter               = UploadHTTPProjection{}
)
