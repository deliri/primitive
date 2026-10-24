package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

// S3FileUpload spends explicit static credentials on one sealed, known-size
// source. Source remains borrowed and is never closed, rewound or retried.
// Endpoint is an HTTPS S3 service origin, not a bucket URL or signed bearer.
// Expires is a whole number of seconds in AWS's inclusive 1..604800 range.
type S3FileUpload struct {
	CredentialFile core.AbsolutePath
	// CredentialMaximumBytes is the caller's explicit whole-file memory
	// admission budget. It is not an AWS credential-size limit or a default.
	CredentialMaximumBytes core.ByteCount
	Endpoint               core.HTTPEndpoint
	Region                 S3Region
	Bucket                 S3Bucket
	Key                    S3Key
	Source                 io.Reader
	Integrity              Integrity
	ContentType            core.HTTPMediaType
	SignedAt               temporal.Instant
	Expires                temporal.Duration
	Policy                 Policy
}

func (r S3FileUpload) Validate() error {
	if err := errors.Join(r.CredentialFile.Validate(), r.Endpoint.Validate(), r.Region.Validate(), r.Bucket.Validate(), r.Key.Validate(), r.Integrity.Validate(), r.ContentType.Validate(), r.SignedAt.Validate(), r.Expires.Validate(), r.Policy.Validate()); err != nil {
		return s3FileRefusal(err)
	}
	if core.ReaderIsNil(r.Source) {
		return core.ErrObjectStoreSource
	}
	if _, err := s3CredentialReadLimit(r.CredentialMaximumBytes); err != nil {
		return s3FileRefusal(err)
	}
	u := r.Endpoint.HTTPURL()
	first, _, _ := strings.Cut(u.Hostname(), ".")
	if u.Scheme != core.SchemeHTTPS || !amazonS3DataHost(u.Hostname()) || (first != "s3" && first != "s3-fips" && first != "s3-"+r.Region.value) || (u.Port() != "" && u.Port() != core.SignatureV4HTTPSPort) || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery {
		return core.ErrObjectStoreContract
	}
	nanos := r.Expires.Nanoseconds()
	if nanos%1_000_000_000 != 0 || nanos/1_000_000_000 < core.S3PresignMinimumSeconds || nanos/1_000_000_000 > core.S3PresignMaximumSeconds {
		return core.ErrObjectStoreContract
	}
	if _, err := s3FileExpiration(r.SignedAt, r.Expires); err != nil {
		return s3FileRefusal(err)
	}
	_, err := NewUploadSigningHeaders(ProviderAmazonS3, SignedHeaders{}, r.Integrity)
	return err
}

// Format never renders the private credential path, source or request target.
func (S3FileUpload) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// UploadS3File closes the credential-file lifetime before spending its private
// presigned target through UploadS3. Existing create-only, exact extent, local
// SHA256/CRC32C, native provider limits and single-attempt rules remain owned
// there. Every failure publishes zero Transfer, including uncertain commits.
// A confirmed PUT relies on AWS's successful-PUT/checksum contract; it is not
// an independent remote readback or a guarantee that versioning is enabled.
func UploadS3File(ctx context.Context, client Client, request S3FileUpload) (Transfer, error) {
	if err := errors.Join(contextstate.Validate(ctx), client.Validate(), request.Validate()); err != nil {
		return Transfer{}, s3FileRefusal(err)
	}
	credentials, err := readS3FileCredentials(ctx, request.CredentialFile, request.CredentialMaximumBytes)
	if err != nil {
		return Transfer{}, s3FileRefusal(err)
	}
	defer credentials.destroy()
	target, err := presignS3File(ctx, request, credentials)
	if err != nil {
		return Transfer{}, s3FileRefusal(err)
	}
	// No key material survives the signing operation in this owner's storage.
	credentials.destroy()
	result, err := UploadS3(ctx, client, UploadRequest{Source: request.Source, ContentType: request.ContentType, Target: target, Integrity: request.Integrity, Policy: request.Policy})
	if err != nil {
		return Transfer{}, s3FileRefusal(err)
	}
	return result, nil
}

// s3CredentialValue owns mutable text bytes, not a base64 wire value. Go's
// JSON decoder owns its temporary buffers; this clears the owner's retained
// copies without claiming to erase every native parser allocation.
type s3CredentialValue struct{ value []byte }

func (v *s3CredentialValue) UnmarshalJSON(data []byte) error {
	if v == nil {
		return core.ErrObjectStoreContract
	}
	text, err := jsonio.DecodeText(data)
	if err != nil {
		return s3FileRefusal(err)
	}
	clear(v.value)
	v.value = []byte(text)
	return nil
}
func (s3CredentialValue) MarshalJSON() ([]byte, error) { return nil, core.ErrObjectStoreContract }
func (s3CredentialValue) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// The declared two-field file deliberately admits no token, profile,
// discovery or refresh settings. JSONIO rejects duplicates and unknown fields.
type s3FileCredentials struct {
	AccessKeyID     s3CredentialValue `json:"access_key_id"`
	SecretAccessKey s3CredentialValue `json:"secret_access_key"`
}

func (c s3FileCredentials) Validate() error {
	access := c.AccessKeyID.value
	if len(access) < 16 || len(access) > 128 || strings.HasPrefix(string(access), "ASIA") || len(c.SecretAccessKey.value) == 0 {
		return core.ErrObjectStoreContract
	}
	for _, b := range access {
		if !(b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_') {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}
func (c *s3FileCredentials) destroy() {
	clear(c.AccessKeyID.value)
	clear(c.SecretAccessKey.value)
	*c = s3FileCredentials{}
}
func (s3FileCredentials) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func decodeS3FileCredentials(ctx context.Context, source io.Reader) (s3FileCredentials, error) {
	credentials, err := jsonio.DecodeProjectionReader[s3FileCredentials](ctx, source, jsonio.ProjectionReadPolicy{})
	if err == nil {
		err = credentials.Validate()
	}
	if err != nil {
		credentials.destroy()
		return s3FileCredentials{}, s3FileRefusal(err)
	}
	return credentials, nil
}

func readS3FileCredentials(ctx context.Context, path core.AbsolutePath, maximum core.ByteCount) (s3FileCredentials, error) {
	var credentials s3FileCredentials
	parent, err := path.Parent()
	if err != nil {
		return credentials, err
	}
	base, err := path.Base()
	if err != nil {
		return credentials, err
	}
	relative, err := core.ParseRelativePath(base.String())
	if err != nil {
		return credentials, err
	}
	rootResult, rootErr := filestore.WithRootScope(ctx, filestore.RootScopeRequest{Directory: parent, Use: func(ctx context.Context, root *os.Root) error {
		result, readErr := filestore.WithInspectedReadScope(ctx, filestore.InspectedReadScopeRequest{Location: filestore.Location{Root: root, Path: relative}, Use: func(ctx context.Context, held filestore.InspectedRead) error {
			var decodeErr error
			credentials, decodeErr = decodeS3InspectedCredentials(ctx, held, maximum)
			return decodeErr
		}})
		return errors.Join(readErr, result.OperationError(), result.CleanupError())
	}})
	if resultErr := errors.Join(rootErr, rootResult.OperationError, rootResult.CleanupError); resultErr != nil {
		credentials.destroy()
		return s3FileCredentials{}, resultErr
	}
	return credentials, nil
}

// The extra byte distinguishes real EOF at the budget from synthetic EOF at
// a limit. Native signed-counter overflow refuses before reading or allocating.
func s3CredentialReadLimit(maximum core.ByteCount) (int64, error) {
	limit, err := maximum.Int64()
	if err != nil {
		return 0, err
	}
	if limit == math.MaxInt64 {
		return 0, core.ErrNumericOverflow
	}
	return limit + 1, nil
}

// Metadata and bytes belong to the same acquired descriptor. A replacement
// pathname cannot substitute a larger or less-private object after inspection.
// Growth of that held object is still bounded by actual max+1 consumption.
func decodeS3InspectedCredentials(ctx context.Context, held filestore.InspectedRead, maximum core.ByteCount) (s3FileCredentials, error) {
	if err := held.Validate(); err != nil {
		return s3FileCredentials{}, s3FileRefusal(err)
	}
	size, err := held.Metadata.SizeBytes()
	if err != nil {
		return s3FileCredentials{}, s3FileRefusal(err)
	}
	permissions, err := held.Metadata.Permissions()
	if err != nil {
		return s3FileCredentials{}, s3FileRefusal(err)
	}
	mode, err := permissions.FileMode()
	if err != nil {
		return s3FileCredentials{}, s3FileRefusal(err)
	}
	limit, err := s3CredentialReadLimit(maximum)
	if err != nil {
		return s3FileCredentials{}, s3FileRefusal(err)
	}
	if mode.Perm()&0o077 != 0 {
		return s3FileCredentials{}, core.ErrObjectStoreContract
	}
	if size.Uint64() > uint64(limit-1) {
		return s3FileCredentials{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
	}
	return decodeS3BoundedCredentials(ctx, held.Source, maximum)
}

func decodeS3BoundedCredentials(ctx context.Context, source io.Reader, maximum core.ByteCount) (s3FileCredentials, error) {
	if core.ReaderIsNil(source) {
		return s3FileCredentials{}, s3FileRefusal(core.ErrObjectStoreSource)
	}
	limit, err := s3CredentialReadLimit(maximum)
	if err != nil {
		return s3FileCredentials{}, s3FileRefusal(err)
	}
	bounded := io.LimitedReader{R: source, N: limit}
	credentials, decodeErr := decodeS3FileCredentials(ctx, &bounded)
	if bounded.N <= 0 {
		credentials.destroy()
		return s3FileCredentials{}, s3FileRefusal(errors.Join(core.ErrObjectStoreSize, decodeErr))
	}
	return credentials, decodeErr
}

func presignS3File(ctx context.Context, r S3FileUpload, c s3FileCredentials) (UploadTarget, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate(), c.Validate()); err != nil {
		return UploadTarget{}, s3FileRefusal(err)
	}
	u := r.Endpoint.HTTPURL()
	u.Path = "/" + r.Bucket.value + "/" + r.Key.value
	// Go's query encoder escapes every reserved byte. S3 keeps only path
	// separators literal and requires %20 rather than form-style spaces.
	// Do not clean dot segments or unescape an operator's literal percent.
	u.RawPath = strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(u.Path), core.SignatureV4FormSpace, core.SignatureV4SpaceEncoding), core.S3EscapedPathSeparator, core.SignatureV4PathSeparator)
	query := u.Query()
	query.Set(core.S3QueryExpires, strconv.FormatInt(r.Expires.Nanoseconds()/1_000_000_000, 10))
	u.RawQuery = query.Encode()
	unsigned, err := core.ParseHTTPEndpoint(u.String())
	if err != nil {
		return UploadTarget{}, err
	}
	headers, err := NewUploadSigningHeaders(ProviderAmazonS3, SignedHeaders{}, r.Integrity)
	if err != nil {
		return UploadTarget{}, err
	}
	signed, err := exchange.PresignV4(ctx, exchange.V4PresignRequest{Region: r.Region.value, Service: core.S3SigningService, PayloadHash: core.S3UnsignedPayload, ContentType: r.ContentType, AccessKey: c.AccessKeyID.value, SecretKey: c.SecretAccessKey.value, Headers: headers, Target: unsigned, SignedAt: r.SignedAt, Method: exchange.MethodPut, DisableURIPathEscaping: true})
	if err != nil {
		return UploadTarget{}, err
	}
	url, err := ParseSignedURL(signed.String())
	if err != nil {
		return UploadTarget{}, err
	}
	expires, err := s3FileExpiration(r.SignedAt, r.Expires)
	if err != nil {
		return UploadTarget{}, err
	}
	target := UploadTarget{URL: url, ExpiresAt: expires}
	return target, target.Validate()
}

// SigV4 publishes whole UTC seconds. Match the actual signed instant rather
// than extending the local spending window by an unpublished fractional second.
func s3FileExpiration(signedAt temporal.Instant, expires temporal.Duration) (temporal.Instant, error) {
	whole, err := signedAt.Truncate(temporal.PrecisionSecond)
	if err != nil {
		return temporal.Instant{}, err
	}
	return whole.Add(expires)
}

// Native causes remain inspectable through Is/As without allowing JSON
// diagnostics, filenames or transport URLs to render credential material.
type s3FileError struct{ cause error }

func (*s3FileError) Error() string                    { return "S3 file upload refused" }
func (e *s3FileError) Unwrap() error                  { return e.cause }
func (e *s3FileError) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, e.Error()) }
func s3FileRefusal(cause error) error {
	if cause == nil {
		return nil
	}
	return &s3FileError{cause: errors.Join(core.ErrObjectStoreContract, cause)}
}

var _ core.Validatable = S3FileUpload{}
var _ core.Validatable = s3FileCredentials{}
