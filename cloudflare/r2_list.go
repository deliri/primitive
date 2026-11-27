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

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// R2 supports the S3 ListObjectsV2 query, including URL-encoded keys and opaque
// continuation tokens. Object Read & Write credentials can list their scoped
// buckets. https://developers.cloudflare.com/r2/api/s3/api/#implemented-object-level-operations
// https://developers.cloudflare.com/r2/api/tokens/#permissions
// https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html
const (
	R2ListMaximumKeys       = 1000
	r2ListTokenMaximumBytes = 2048
	r2ListETagMaximumBytes  = 128
	// Three bytes per encoded key byte, plus scalar XML framing and ETag escaping.
	r2ListRowMaximumBytes  = 3*core.CloudflareR2ObjectKeyMaximumBytes + 2048
	r2ListPageFramingBytes = 32 * 1024
)

type R2ListPrefix struct{ value string }

func ParseR2ListPrefix(value string) (R2ListPrefix, error) {
	if value != "" {
		if _, err := ParseR2Key(value); err != nil {
			return R2ListPrefix{}, err
		}
	}
	return R2ListPrefix{value: value}, nil
}

func (p R2ListPrefix) String() string { return p.value }

type R2ListContinuation struct{ value string }

func ParseR2ListContinuation(value string) (R2ListContinuation, error) {
	if value == "" || len(value) > r2ListTokenMaximumBytes || !utf8.ValidString(value) {
		return R2ListContinuation{}, core.ErrCloudflareBinding
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return R2ListContinuation{}, core.ErrCloudflareBinding
		}
	}
	return R2ListContinuation{value: value}, nil
}

func (c R2ListContinuation) IsSet() bool    { return c.value != "" }
func (c R2ListContinuation) String() string { return c.value }
func (R2ListContinuation) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type R2ListRequest struct {
	Bucket       R2Bucket
	Prefix       R2ListPrefix
	Continuation R2ListContinuation
	SignedAt     temporal.Instant
	MaximumKeys  uint16
}

func (r R2ListRequest) Validate() error {
	if r.MaximumKeys == 0 || r.MaximumKeys > R2ListMaximumKeys {
		return core.ErrCloudflareBinding
	}
	return errors.Join(r.Bucket.Validate(), r.SignedAt.Validate())
}

// The provider ETag is opaque, including for multipart objects.
type R2ListedObject struct {
	Key          R2Key
	ETag         string
	Bytes        core.ByteLength
	LastModified temporal.Instant
}

type R2ListPage struct {
	Objects      []R2ListedObject
	Continuation R2ListContinuation
}

// ListObjectsV2 signs and admits exactly one bounded page. A set continuation
// belongs to the next request. The client performs no automatic pagination.
func (c R2Client) ListObjectsV2(ctx context.Context, server R2Server, request R2ListRequest, policy exchange.StreamPolicy) (R2ListPage, error) {
	if err := errors.Join(c.Validate(), server.Validate(), request.Validate(), validatePolicy(policy)); err != nil {
		return R2ListPage{}, err
	}
	target, err := server.r2ListTarget(request)
	if err != nil {
		return R2ListPage{}, err
	}
	response := boundedResponse{maximum: uint64(request.MaximumKeys)*r2ListRowMaximumBytes + r2ListPageFramingBytes}
	failure := boundedResponse{maximum: r2ListPageFramingBytes}
	_, err = exchange.DownloadV4(exchange.DownloadCall{Context: ctx, Client: c.client, Policy: policy,
		Request: exchange.DownloadRequest{Target: target, Destination: &response, FailureDestination: &failure, ExpectedStatus: core.HTTPStatusOK(),
			Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}}},
		exchange.V4DownloadSigning{AccessKey: server.credentials.accessKey, SecretKey: server.credentials.secretKey,
			Service: core.CloudflareR2SigningService, Region: core.CloudflareR2SigningRegion, SignedAt: request.SignedAt})
	if err != nil {
		return R2ListPage{}, err
	}
	return decodeR2ListPage(&response.buffer, request)
}

func (s R2Server) r2ListTarget(request R2ListRequest) (core.HTTPEndpoint, error) {
	label, err := s.jurisdiction.label()
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	query := url.Values{
		"list-type": {"2"}, "encoding-type": {"url"},
		"max-keys": {strconv.FormatUint(uint64(request.MaximumKeys), 10)},
	}
	if request.Prefix.value != "" {
		query.Set("prefix", request.Prefix.value)
	}
	if request.Continuation.IsSet() {
		query.Set("continuation-token", request.Continuation.value)
	}
	endpoint := url.URL{Scheme: core.SchemeHTTPS, Host: s.account.value + label + core.CloudflareR2HostSuffix,
		Path: "/" + request.Bucket.value, RawQuery: query.Encode()}
	return core.ParseHTTPEndpoint(endpoint.String())
}
