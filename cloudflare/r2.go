package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type R2Bucket struct{ value string }

func ParseR2Bucket(value string) (R2Bucket, error) {
	if len(value) < core.CloudflareR2BucketMinimumBytes || len(value) > core.CloudflareR2BucketMaximumBytes || value[0] == '-' || value[len(value)-1] == '-' {
		return R2Bucket{}, core.ErrCloudflareBinding
	}
	for _, c := range value {
		if !r2BucketRune(c) {
			return R2Bucket{}, core.ErrCloudflareBinding
		}
	}
	return R2Bucket{value: value}, nil
}
func r2BucketRune(c rune) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' }

func (b R2Bucket) Validate() error { _, err := ParseR2Bucket(b.value); return err }
func (b R2Bucket) String() string  { return b.value }

type R2Key struct{ value string }

func ParseR2Key(value string) (R2Key, error) {
	if value == "" || len(value) > core.CloudflareR2ObjectKeyMaximumBytes || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return R2Key{}, core.ErrCloudflareBinding
	}
	return R2Key{value: value}, nil
}
func (k R2Key) Validate() error { _, err := ParseR2Key(k.value); return err }
func (k R2Key) String() string  { return k.value }

// R2Jurisdiction selects the actual endpoint, not a signing region.
// https://developers.cloudflare.com/r2/api/tokens/
type R2Jurisdiction uint8

const (
	R2JurisdictionUnknown R2Jurisdiction = iota
	R2JurisdictionDefault
	R2JurisdictionEU
	R2JurisdictionUS
	R2JurisdictionFedRAMP
)

func (j R2Jurisdiction) IsValid() bool { return j.Validate() == nil }
func (j R2Jurisdiction) String() string {
	if j == R2JurisdictionDefault {
		return "default"
	}
	label, err := j.label()
	if err != nil {
		return core.UnknownEnumDiagnostic
	}
	return strings.TrimPrefix(label, ".")
}
func (j R2Jurisdiction) Validate() error { _, err := j.label(); return err }
func (R2Jurisdiction) OffWireEnum()      {}
func (j R2Jurisdiction) label() (string, error) {
	switch j {
	case R2JurisdictionDefault:
		return "", nil
	case R2JurisdictionEU:
		return ".eu", nil
	case R2JurisdictionUS:
		return ".us", nil
	case R2JurisdictionFedRAMP:
		return ".fedramp", nil
	default:
		return "", core.ErrCloudflareBinding
	}
}

type R2Credentials struct {
	accessKey []byte
	secretKey []byte
}

func ParseR2Credentials(accessKey, secretKey []byte) (R2Credentials, error) {
	if !visibleSecret(accessKey) || !visibleSecret(secretKey) {
		return R2Credentials{}, core.ErrCloudflareAuthentication
	}
	return R2Credentials{accessKey: bytes.Clone(accessKey), secretKey: bytes.Clone(secretKey)}, nil
}
func (c R2Credentials) Validate() error {
	if !visibleSecret(c.accessKey) || !visibleSecret(c.secretKey) {
		return core.ErrCloudflareAuthentication
	}
	return nil
}
func (c *R2Credentials) Close() error {
	if c == nil {
		return core.ErrCloudflareContract
	}
	clear(c.accessKey)
	clear(c.secretKey)
	*c = R2Credentials{}
	return nil
}
func (R2Credentials) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// R2Server signs one object operation locally: cheap reads do not require a
// preliminary network request. The official SigV4 signer owns the algorithm;
// Cloudflare owns endpoint, region, service, credentials and admitted operations.
type R2Server struct {
	account      AccountID
	credentials  R2Credentials
	jurisdiction R2Jurisdiction
}

func NewR2Server(credentials R2Credentials, account AccountID, jurisdiction R2Jurisdiction) (R2Server, error) {
	if err := errors.Join(credentials.Validate(), account.Validate(), jurisdiction.Validate()); err != nil {
		return R2Server{}, err
	}
	owned, err := ParseR2Credentials(credentials.accessKey, credentials.secretKey)
	return R2Server{credentials: owned, account: account, jurisdiction: jurisdiction}, err
}
func (s R2Server) Validate() error {
	return errors.Join(s.credentials.Validate(), s.account.Validate(), s.jurisdiction.Validate())
}
func (s *R2Server) Close() error {
	if s == nil {
		return core.ErrCloudflareContract
	}
	return s.credentials.Close()
}

type R2PresignRequest struct {
	Bucket       R2Bucket
	Key          R2Key
	ContentType  core.HTTPMediaType
	SignedAt     temporal.Instant
	Expires      temporal.Duration
	Method       exchange.Method
	Conditions   R2WriteConditions
	CacheControl R2CacheControl
}

func (r R2PresignRequest) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Key.Validate(), r.SignedAt.Validate(), r.Expires.Validate(), r.Method.Validate()); err != nil {
		return contractError(err)
	}
	seconds := r.Expires.Nanoseconds() / 1_000_000_000
	if seconds < 1 || seconds > core.CloudflareR2PresignMaximumSeconds || r.Expires.Nanoseconds()%1_000_000_000 != 0 {
		return core.ErrCloudflareContract
	}
	if _, err := r.SignedAt.CompactUTC(); err != nil {
		return contractError(err)
	}
	if !r.ContentType.IsZero() && (r.Method != exchange.MethodPut || r.ContentType.Validate() != nil) {
		return core.ErrCloudflareBinding
	}
	if err := r.Conditions.validateMethod(r.Method); err != nil {
		return err
	}
	if err := r.CacheControl.validateWrite(r.Method == exchange.MethodPut); err != nil {
		return err
	}
	return validateR2Method(r.Method)
}
func validateR2Method(method exchange.Method) error {
	switch method {
	case exchange.MethodGet, exchange.MethodHead, exchange.MethodPut, exchange.MethodDelete:
		return nil
	default:
		return core.ErrCloudflareBinding
	}
}

// R2Grant binds a presigned endpoint to its exact HTTP method and signed media
// type. Possession is authority; diagnostic formatting never reveals the URL.
type R2Grant struct {
	contentType  core.HTTPMediaType
	endpoint     core.HTTPEndpoint
	method       exchange.Method
	conditions   R2WriteConditions
	cacheControl R2CacheControl
}

func (g R2Grant) Validate() error {
	if err := errors.Join(g.endpoint.Validate(), validateR2Method(g.method)); err != nil {
		return err
	}
	return errors.Join(g.conditions.validateMethod(g.method), g.cacheControl.validateWrite(g.method == exchange.MethodPut))
}
func (g R2Grant) Endpoint() (core.HTTPEndpoint, error) { return g.endpoint, g.Validate() }
func (R2Grant) Format(state fmt.State, _ rune)         { _, _ = io.WriteString(state, core.RedactedValueText) }

func (s R2Server) Presign(ctx context.Context, intent R2PresignRequest) (R2Grant, error) {
	if err := errors.Join(contextstate.Validate(ctx), s.Validate(), intent.Validate()); err != nil {
		return R2Grant{}, err
	}
	label, err := s.jurisdiction.label()
	if err != nil {
		return R2Grant{}, err
	}
	target := url.URL{Scheme: core.SchemeHTTPS, Host: s.account.value + label + core.CloudflareR2HostSuffix, Path: "/" + intent.Bucket.value + "/" + intent.Key.value}
	query := make(url.Values)
	query.Set(core.CloudflareR2QueryExpires, strconv.FormatInt(intent.Expires.Nanoseconds()/1_000_000_000, 10))
	target.RawQuery = query.Encode()
	unsigned, err := core.ParseHTTPEndpoint(target.String())
	if err != nil {
		return R2Grant{}, contractError(err)
	}
	parsed, err := exchange.PresignV4(ctx, exchange.V4PresignRequest{
		AccessKey: s.credentials.accessKey, SecretKey: s.credentials.secretKey,
		Region: core.CloudflareR2SigningRegion, Service: core.CloudflareR2SigningService,
		PayloadHash: core.CloudflareR2UnsignedPayload, Target: unsigned, Method: intent.Method,
		SignedAt: intent.SignedAt, ContentType: intent.ContentType, Headers: r2WriteHeaders(intent.Conditions, intent.CacheControl), DisableURIPathEscaping: true,
	})
	if err != nil {
		return R2Grant{}, authenticationError(err)
	}
	return R2Grant{endpoint: parsed, method: intent.Method, contentType: intent.ContentType, conditions: intent.Conditions, cacheControl: intent.CacheControl}, nil
}

type R2Client struct{ client exchange.Client }

func NewR2Client(client exchange.Client) (R2Client, error) {
	if err := client.Validate(); err != nil {
		return R2Client{}, contractError(err)
	}
	return R2Client{client: client}, nil
}
func (c R2Client) Validate() error { return c.client.Validate() }

// Read performs precisely the granted GET or HEAD and preserves partial bytes
// on failure. It does not list the bucket or fetch metadata before a GET.
func (c R2Client) Read(ctx context.Context, grant R2Grant, destination io.Writer, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	if err := errors.Join(c.Validate(), grant.Validate(), validatePolicy(policy)); err != nil {
		return exchange.StreamResponse{}, err
	}
	if grant.method != exchange.MethodGet && grant.method != exchange.MethodHead {
		return exchange.StreamResponse{}, core.ErrCloudflareBinding
	}
	return exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.client, Policy: policy,
		Request: exchange.DownloadRequest{Target: grant.endpoint, Destination: destination, ExpectedStatus: core.HTTPStatusOK(),
			Semantics: exchange.RequestSemantics{Method: grant.method, Replay: exchange.ReplaySingleAttempt}}})
}

// Put streams exactly one declared object. It does not retry an uncertain PUT.
type R2WriteRequest struct {
	Source io.Reader
	Grant  R2Grant
	Bytes  core.ByteLength
}

func (r R2WriteRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrCloudflareContract
	}
	if r.Grant.method != exchange.MethodPut || r.Bytes.Uint64() > core.CloudflareR2SingleUploadMaximumBytes {
		return core.ErrCloudflareBinding
	}
	return errors.Join(r.Grant.Validate(), r.Bytes.Validate())
}

func (c R2Client) Put(ctx context.Context, request R2WriteRequest, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	if err := errors.Join(c.Validate(), request.Validate(), validatePolicy(policy)); err != nil {
		return exchange.StreamResponse{}, err
	}
	grant, source, length := request.Grant, request.Source, request.Bytes
	media := grant.contentType
	if media.IsZero() {
		media = core.HTTPMediaTypeOctetStream()
	}
	return exchange.Upload(exchange.UploadCall{Context: ctx, Client: c.client, Policy: policy,
		Request: exchange.UploadRequest{Target: grant.endpoint, Source: source, ContentLength: &length, ContentType: media, Headers: r2WriteHeaders(grant.conditions, grant.cacheControl),
			ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPut, Replay: exchange.ReplaySingleAttempt}}})
}

func (c R2Client) Delete(ctx context.Context, grant R2Grant, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	if err := errors.Join(c.Validate(), grant.Validate(), validatePolicy(policy)); err != nil {
		return exchange.StreamResponse{}, err
	}
	if grant.method != exchange.MethodDelete {
		return exchange.StreamResponse{}, core.ErrCloudflareBinding
	}
	var status core.HTTPStatusCode
	if err := status.AdmitInt(204); err != nil {
		return exchange.StreamResponse{}, err
	}
	return exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.client, Policy: policy,
		Request: exchange.DownloadRequest{Target: grant.endpoint, Destination: io.Discard, ExpectedStatus: status,
			Semantics: exchange.RequestSemantics{Method: exchange.MethodDelete, Replay: exchange.ReplaySingleAttempt}}})
}
