package cloudflare

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// R2UploadID is the opaque provider session coordinate. It is never a key,
// identity, or permission. The application must retain its owner binding.
type R2UploadID struct{ value string }

func ParseR2UploadID(value string) (R2UploadID, error) {
	if value == "" || len(value) > core.CloudflareR2UploadIDMaximumBytes || !utf8.ValidString(value) {
		return R2UploadID{}, core.ErrCloudflareBinding
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return R2UploadID{}, core.ErrCloudflareBinding
		}
	}
	return R2UploadID{value: value}, nil
}
func (id R2UploadID) Validate() error { _, err := ParseR2UploadID(id.value); return err }
func (id R2UploadID) String() string  { return id.value }

// Each operation has one protocol shape. Part URLs grant only a particular
// upload ID and part number; completing or aborting remains server-owned.
type R2MultipartAction uint8

const (
	R2MultipartUnknown R2MultipartAction = iota
	R2MultipartCreate
	R2MultipartPart
	R2MultipartComplete
	R2MultipartAbort
)

func (a R2MultipartAction) method() (exchange.Method, error) {
	switch a {
	case R2MultipartCreate, R2MultipartComplete:
		return exchange.MethodPost, nil
	case R2MultipartPart:
		return exchange.MethodPut, nil
	case R2MultipartAbort:
		return exchange.MethodDelete, nil
	default:
		return 0, core.ErrCloudflareBinding
	}
}
func (a R2MultipartAction) Validate() error { _, err := a.method(); return err }
func (R2MultipartAction) OffWireEnum()      {}

type R2MultipartPresignRequest struct {
	Bucket      R2Bucket
	Key         R2Key
	UploadID    R2UploadID
	ContentType core.HTTPMediaType
	SignedAt    temporal.Instant
	Expires     temporal.Duration
	PartNumber  uint16
	Action      R2MultipartAction
}

func (r R2MultipartPresignRequest) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Key.Validate(), r.SignedAt.Validate(), r.Expires.Validate(), r.Action.Validate()); err != nil {
		return contractError(err)
	}
	nanos := r.Expires.Nanoseconds()
	if nanos < 1_000_000_000 || nanos%1_000_000_000 != 0 || nanos/1_000_000_000 > core.CloudflareR2PresignMaximumSeconds {
		return core.ErrCloudflareBinding
	}
	if r.Action == R2MultipartCreate {
		if r.UploadID != (R2UploadID{}) || r.ContentType.Validate() != nil {
			return core.ErrCloudflareBinding
		}
	} else if r.UploadID.Validate() != nil || !r.ContentType.IsZero() {
		return core.ErrCloudflareBinding
	}
	if r.Action == R2MultipartPart {
		if r.PartNumber < 1 || r.PartNumber > core.CloudflareR2MultipartMaximumParts {
			return core.ErrCloudflareBinding
		}
	} else if r.PartNumber != 0 {
		return core.ErrCloudflareBinding
	}
	return nil
}

type R2MultipartGrant struct {
	intent   R2MultipartPresignRequest
	endpoint core.HTTPEndpoint
}

func (g R2MultipartGrant) Validate() error {
	return errors.Join(g.intent.Validate(), g.endpoint.Validate())
}
func (g R2MultipartGrant) Endpoint() (core.HTTPEndpoint, error) { return g.endpoint, g.Validate() }
func (R2MultipartGrant) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func (s R2Server) PresignMultipart(ctx context.Context, r R2MultipartPresignRequest) (R2MultipartGrant, error) {
	if err := errors.Join(contextstate.Validate(ctx), s.Validate(), r.Validate()); err != nil {
		return R2MultipartGrant{}, err
	}
	label, err := s.jurisdiction.label()
	if err != nil {
		return R2MultipartGrant{}, err
	}
	target := url.URL{Scheme: core.SchemeHTTPS, Host: s.account.value + label + core.CloudflareR2HostSuffix, Path: "/" + r.Bucket.value + "/" + r.Key.value}
	query := make(url.Values)
	query.Set(core.CloudflareR2QueryExpires, strconv.FormatInt(r.Expires.Nanoseconds()/1_000_000_000, 10))
	if r.Action == R2MultipartCreate {
		query.Set(core.CloudflareR2QueryUploads, "")
	} else {
		query.Set(core.CloudflareR2QueryUploadID, r.UploadID.value)
	}
	if r.Action == R2MultipartPart {
		query.Set(core.CloudflareR2QueryPartNumber, strconv.Itoa(int(r.PartNumber)))
	}
	target.RawQuery = query.Encode()
	unsigned, err := core.ParseHTTPEndpoint(target.String())
	if err != nil {
		return R2MultipartGrant{}, contractError(err)
	}
	method, err := r.Action.method()
	if err != nil {
		return R2MultipartGrant{}, err
	}
	signed, err := exchange.PresignV4(ctx, exchange.V4PresignRequest{
		AccessKey: s.credentials.accessKey, SecretKey: s.credentials.secretKey, Region: core.CloudflareR2SigningRegion, Service: core.CloudflareR2SigningService,
		PayloadHash: core.CloudflareR2UnsignedPayload, Target: unsigned, Method: method, SignedAt: r.SignedAt, ContentType: r.ContentType, DisableURIPathEscaping: true,
	})
	if err != nil {
		return R2MultipartGrant{}, authenticationError(err)
	}
	return R2MultipartGrant{intent: r, endpoint: signed}, nil
}

func (R2UploadID) cloudflareCapabilityWrapper()           {}
func (R2MultipartGrant) cloudflareCapabilityWrapper()     {}
func (R2MultipartPresignRequest) cloudflareProtocolFact() {}
