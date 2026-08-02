package objectstore

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// SignedHeaderMaximumCount bounds caller-owned signed request fields.
	SignedHeaderMaximumCount = 32
	// SignedHeaderMaximumBytes bounds their aggregate HTTP wire extent.
	SignedHeaderMaximumBytes    = 32 * 1024
	httpsScheme                 = "https"
	objectstoreOwnedHeaderCount = 15
	googleCloudStorageHost      = "storage.googleapis.com"
	googleCloudStorageMTLSHost  = "storage.mtls.googleapis.com"
	amazonWebServicesDNSRoot    = "amazonaws.com"
	amazonWebServicesChinaRoot  = "amazonaws.com.cn"
)

// Direction identifies one transfer operation.
type Direction uint8

const (
	// DirectionUnknown is the invalid zero direction.
	DirectionUnknown Direction = iota
	// DirectionUpload is one provider upload.
	DirectionUpload
	// DirectionDownload is one provider download.
	DirectionDownload
	directionLimit
)

// Commitment reports what one call established about remote acceptance.
type Commitment uint8

const (
	// CommitmentUnknown is the invalid zero commitment.
	CommitmentUnknown Commitment = iota
	// CommitmentNotAttempted means no provider call began.
	CommitmentNotAttempted
	// CommitmentRejected means the request could not commit or the provider
	// explicitly rejected it.
	CommitmentRejected
	// CommitmentConfirmed means the provider accepted an exact verified stream.
	CommitmentConfirmed
	// CommitmentIndeterminate means an attempted upload lost confirmation.
	CommitmentIndeterminate
	commitmentLimit
)

// ProviderVersion is one validated provider-issued object identity.
type ProviderVersion struct {
	value    string
	provider Provider
	set      bool
}

// Provider returns the vendor that issued the version.
func (v ProviderVersion) Provider() Provider { return v.provider }

// String returns the validated provider wire value.
func (v ProviderVersion) String() string {
	if v.Validate() != nil {
		return ""
	}
	return v.value
}

// IsZero reports whether no provider version is present.
func (v ProviderVersion) IsZero() bool { return !v.set }

// Validate closes the provider-specific version domain.
func (v ProviderVersion) Validate() error {
	if !v.set {
		return core.ErrObjectStoreContract
	}
	switch v.provider {
	case ProviderAmazonS3:
		return validateAmazonS3Version(v.value)
	case ProviderGoogleCloudStorage:
		return validateGoogleCloudStorageGeneration(v.value)
	case ProviderCloudflareImages:
		return core.ErrObjectStoreContract
	default:
		return core.ErrObjectStoreContract
	}
}

func validateAmazonS3Version(value string) error {
	if len(value) == 0 ||
		len(value) > AmazonS3VersionIDMaximumBytes ||
		!utf8.ValidString(value) ||
		core.ValidateHTTPFieldValue(value) != nil {
		return core.ErrObjectStoreContract
	}
	return nil
}

func validateGoogleCloudStorageGeneration(value string) error {
	generation, err := strconv.ParseUint(value, 10, 64)
	if err != nil || generation == 0 ||
		strconv.FormatUint(generation, 10) != value {
		return core.ErrObjectStoreContract
	}
	return nil
}

func newProviderVersion(
	provider Provider,
	value string,
) (ProviderVersion, error) {
	version := ProviderVersion{value: value, provider: provider, set: true}
	if err := version.Validate(); err != nil {
		return ProviderVersion{}, err
	}
	return version, nil
}

// SignedURL is one opaque HTTPS capability. It has no string accessor and all
// formatting is redacted.
type SignedURL struct {
	value url.URL
	set   bool
}

// ParseSignedURL parses one absolute HTTPS capability.
func ParseSignedURL(value string) (SignedURL, error) {
	endpoint, err := core.ParseHTTPEndpoint(value)
	if err != nil {
		return SignedURL{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	parsed := endpoint.HTTPURL()
	if parsed.Scheme != httpsScheme || parsed.EscapedPath() == "" ||
		parsed.EscapedPath() == "/" {
		return SignedURL{}, core.ErrObjectStoreContract
	}
	return SignedURL{value: parsed, set: true}, nil
}

// Validate rejects unset, non-HTTPS, or root-only capabilities.
func (u SignedURL) Validate() error {
	if !u.set || u.value.Scheme != httpsScheme ||
		u.value.Host == "" || u.value.EscapedPath() == "" ||
		u.value.EscapedPath() == "/" {
		return core.ErrObjectStoreContract
	}
	return nil
}

// Format redacts the bearer for every formatting verb.
func (SignedURL) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// SignedHeader is one immutable signed request field.
type SignedHeader struct {
	name  core.HTTPHeaderName
	value string
}

// NewSignedHeader validates and owns one request field.
func NewSignedHeader(
	name core.HTTPHeaderName,
	value string,
) (SignedHeader, error) {
	header := exchange.Header{Name: name, Values: []string{value}}
	if err := header.Validate(); err != nil {
		return SignedHeader{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	owned, err := objectstoreOwnedHeader(name)
	if err != nil {
		return SignedHeader{}, err
	}
	if owned {
		return SignedHeader{}, core.ErrObjectStoreContract
	}
	return SignedHeader{name: name, value: value}, nil
}

// Validate rejects unset or Objectstore-owned fields.
func (h SignedHeader) Validate() error {
	_, err := NewSignedHeader(h.name, h.value)
	return err
}

// SignedHeaders is an immutable bounded set.
type SignedHeaders struct {
	values []SignedHeader
}

// NewSignedHeaders validates, deduplicates, and copies fields.
func NewSignedHeaders(values []SignedHeader) (SignedHeaders, error) {
	if len(values) > SignedHeaderMaximumCount {
		return SignedHeaders{}, core.ErrObjectStoreContract
	}
	owned := make([]SignedHeader, len(values))
	wireBytes := 0
	for index, value := range values {
		if err := value.Validate(); err != nil {
			return SignedHeaders{}, err
		}
		wireBytes += len(value.name.String()) + len(value.value) +
			headerWireSyntaxBytes
		if wireBytes > SignedHeaderMaximumBytes {
			return SignedHeaders{}, core.ErrObjectStoreContract
		}
		for prior := range index {
			if values[prior].name == value.name {
				return SignedHeaders{}, core.ErrObjectStoreContract
			}
		}
		owned[index] = value
	}
	return SignedHeaders{values: owned}, nil
}

// Validate rechecks the immutable set.
func (h SignedHeaders) Validate() error {
	_, err := NewSignedHeaders(h.values)
	return err
}

// UploadTarget is one already-issued provider upload capability.
type UploadTarget struct {
	Headers   SignedHeaders
	URL       SignedURL
	ExpiresAt temporal.Instant
}

// Validate closes the provider-independent target shape.
func (t UploadTarget) Validate() error {
	if err := t.URL.Validate(); err != nil {
		return err
	}
	if err := t.ExpiresAt.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := t.Headers.Validate(); err != nil {
		return err
	}
	return nil
}

func (t UploadTarget) validateFor(provider Provider) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if err := validateCapability(provider, t.URL.value); err != nil {
		return err
	}
	return validateCallerSignedHeaders(
		provider, t.URL.value, t.Headers, DirectionUpload,
	)
}

// DownloadTarget is one already-issued whole-object download capability.
type DownloadTarget struct {
	Headers   SignedHeaders
	URL       SignedURL
	ExpiresAt temporal.Instant
}

// Validate closes the provider-independent target shape.
func (t DownloadTarget) Validate() error {
	if err := t.URL.Validate(); err != nil {
		return err
	}
	if err := t.ExpiresAt.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := t.Headers.Validate(); err != nil {
		return err
	}
	return nil
}

// ValidateFor binds the target to one compiler-selected whole-object provider
// before a caller performs any preparatory side effects. The transfer entry
// points apply the same gate again at execution ingress.
func (t DownloadTarget) ValidateFor(provider Provider) error {
	if err := t.Validate(); err != nil {
		return err
	}
	spec, err := Spec(provider)
	if err != nil {
		return err
	}
	if spec.Directions != DirectionCapabilityUploadDownload {
		return core.ErrObjectStoreContract
	}
	if err := validateCapability(provider, t.URL.value); err != nil {
		return err
	}
	if err := validateCallerSignedHeaders(
		provider, t.URL.value, t.Headers, DirectionDownload,
	); err != nil {
		return err
	}
	return validateDownloadSignedHeaders(provider, t)
}

// Integrity binds an exact extent to two independently calculated digests.
type Integrity struct {
	SHA256 core.SHA256Digest
	Length core.ByteLength
	CRC32C core.CRC32C
}

// Validate rejects unset digests and lengths outside net/http's exact extent.
func (i Integrity) Validate() error {
	if _, err := i.Length.Int64(); err != nil {
		return errors.Join(core.ErrObjectStoreSize, err)
	}
	if err := i.SHA256.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := i.CRC32C.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// Policy owns one attempt's finite time and rejected-body bounds.
type Policy struct {
	OperationTimeout temporal.Duration
	AttemptTimeout   temporal.Duration
	ErrorBodyLimit   core.ByteCount
}

// Validate closes the one-attempt policy.
func (p Policy) Validate() error {
	return p.exchange().Validate()
}

func (p Policy) exchange() exchange.StreamPolicy {
	return exchange.StreamPolicy{
		OperationTimeout: p.OperationTimeout,
		AttemptTimeout:   p.AttemptTimeout,
		ErrorBodyLimit:   p.ErrorBodyLimit,
		Redirect: exchange.RedirectPolicy{
			Mode: exchange.RedirectReject,
		},
	}
}

// UploadRequest supplies one exact caller-owned source.
type UploadRequest struct {
	Source      io.Reader
	ContentType core.HTTPMediaType
	Target      UploadTarget
	Integrity   Integrity
	Policy      Policy
}

// Validate closes every provider-independent upload ingress before the source
// is read. The selected provider entry point owns its capability and extent
// rules.
func (r UploadRequest) Validate() error {
	if r.Source == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	if err := r.Target.Validate(); err != nil {
		return err
	}
	if err := r.Integrity.Validate(); err != nil {
		return err
	}
	if err := r.ContentType.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := r.Policy.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

func (r UploadRequest) validateFor(provider Provider) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := r.Target.validateFor(provider); err != nil {
		return err
	}
	spec, err := Spec(provider)
	if err != nil {
		return err
	}
	if r.Integrity.Length.Uint64() > spec.UploadMaximum.Uint64() {
		return core.ErrObjectStoreSize
	}
	return validateProviderSignedHeaders(provider, r.Target)
}

// DownloadRequest supplies one exact caller-owned destination.
type DownloadRequest struct {
	Destination io.Writer
	ContentType core.HTTPMediaType
	Target      DownloadTarget
	Integrity   Integrity
	Policy      Policy
}

// Validate closes every provider-independent download ingress before a
// provider call begins. The selected provider entry point owns its capability
// and extent rules.
func (r DownloadRequest) Validate() error {
	if r.Destination == nil {
		return errors.Join(
			core.ErrObjectStoreContract,
			core.ErrObjectStoreDestination,
		)
	}
	if err := r.Target.Validate(); err != nil {
		return err
	}
	if err := r.Integrity.Validate(); err != nil {
		return err
	}
	if err := r.ContentType.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := r.Policy.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

func (r DownloadRequest) validateFor(provider Provider) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := r.Target.ValidateFor(provider); err != nil {
		return err
	}
	spec, err := Spec(provider)
	if err != nil {
		return err
	}
	if r.Integrity.Length.Uint64() > spec.DownloadMaximum.Uint64() {
		return core.ErrObjectStoreSize
	}
	return nil
}

func validateCapability(provider Provider, value url.URL) error {
	if err := validateProviderEndpoint(provider, value); err != nil {
		return err
	}
	query := value.Query()
	switch provider {
	case ProviderAmazonS3:
		if _, ok := uniqueQueryValue(query, queryS3Signature); !ok {
			return core.ErrObjectStoreContract
		}
		if _, ok := uniqueQueryValue(query, queryS3SignedHeaders); !ok {
			return core.ErrObjectStoreContract
		}
	case ProviderGoogleCloudStorage:
		if _, ok := uniqueQueryValue(query, queryGCSSignature); !ok {
			return core.ErrObjectStoreContract
		}
		if _, ok := uniqueQueryValue(query, queryGCSSignedHeaders); !ok {
			return core.ErrObjectStoreContract
		}
	case ProviderCloudflareImages:
	default:
		return core.ErrObjectStoreContract
	}
	return nil
}

// validateProviderEndpoint prevents a provider-labelled bearer received over
// an API from becoming an arbitrary HTTPS upload or download target. The
// admitted hosts are vendor-controlled data-plane DNS names; an explicit port
// may only restate HTTPS's standard port.
func validateProviderEndpoint(provider Provider, value url.URL) error {
	if port := value.Port(); port != "" && port != "443" {
		return core.ErrObjectStoreContract
	}
	host := strings.ToLower(value.Hostname())
	if providerEndpointHost(provider, host) {
		return nil
	}
	return core.ErrObjectStoreContract
}

func providerEndpointHost(provider Provider, host string) bool {
	switch provider {
	case ProviderAmazonS3:
		return amazonS3DataHost(host)
	case ProviderGoogleCloudStorage:
		return googleCloudStorageDataHost(host)
	case ProviderCloudflareImages:
		return host == cloudflareImagesUploadHost
	case ProviderUnknown, providerLimit:
		return false
	}
	return false
}

func googleCloudStorageDataHost(host string) bool {
	return host == googleCloudStorageHost ||
		host == googleCloudStorageMTLSHost ||
		strings.HasSuffix(host, "."+googleCloudStorageHost)
}

func amazonS3DataHost(host string) bool {
	for _, root := range [...]string{
		amazonWebServicesDNSRoot,
		amazonWebServicesChinaRoot,
	} {
		if host != root && !strings.HasSuffix(host, "."+root) {
			continue
		}
		prefix := strings.TrimSuffix(strings.TrimSuffix(host, root), ".")
		for label := range strings.SplitSeq(prefix, ".") {
			if label == "s3" || strings.HasPrefix(label, "s3-") ||
				strings.HasPrefix(label, "s3express-") {
				return true
			}
		}
	}
	return false
}

// objectstoreOwnedHeaderNames resolves the one compiler-sized set of request
// fields Objectstore and Exchange set themselves. Constants cross the typed
// parser on every ingress; there is no process-global cache or mutable slice
// whose contents can become a second runtime contract.
func objectstoreOwnedHeaderNames() ([objectstoreOwnedHeaderCount]core.HTTPHeaderName, error) {
	authorization, err := exchange.StandardHeaderAuthorization.Name()
	if err != nil {
		return [objectstoreOwnedHeaderCount]core.HTTPHeaderName{}, err
	}
	contentRange, err := headerName(headerContentRange)
	if err != nil {
		return [objectstoreOwnedHeaderCount]core.HTTPHeaderName{}, err
	}
	names := [objectstoreOwnedHeaderCount]core.HTTPHeaderName{
		core.HTTPHeaderContentType(),
		core.HTTPHeaderContentLength(),
		core.HTTPHeaderAcceptEncoding(),
		core.HTTPHeaderContentEncoding(),
		core.HTTPHeaderAccept(),
		authorization,
		core.HTTPHeaderIdempotencyKey(),
		contentRange,
	}
	index := 8
	for _, value := range [...]string{
		headerHost,
		headerRange,
		headerIfNoneMatch,
		headerS3ChecksumCRC32C,
		headerS3ChecksumMode,
		headerGCSHash,
		headerGCSGenerationMatch,
	} {
		name, parseErr := headerName(value)
		if parseErr != nil {
			return [objectstoreOwnedHeaderCount]core.HTTPHeaderName{}, parseErr
		}
		names[index] = name
		index++
	}
	return names, nil
}

func objectstoreOwnedHeader(name core.HTTPHeaderName) (bool, error) {
	owned, err := objectstoreOwnedHeaderNames()
	if err != nil {
		return false, err
	}
	for _, candidate := range owned {
		if candidate == name {
			return true, nil
		}
	}
	return false, nil
}

var (
	_ core.Validatable = DirectionUnknown
	_ core.Validatable = CommitmentUnknown
	_ core.Validatable = SignedURL{}
	_ core.Validatable = SignedHeader{}
	_ core.Validatable = SignedHeaders{}
	_ core.Validatable = UploadTarget{}
	_ core.Validatable = DownloadTarget{}
	_ core.Validatable = Integrity{}
	_ core.Validatable = Policy{}
	_ core.Validatable = UploadRequest{}
	_ core.Validatable = DownloadRequest{}
)

// Validate rejects values outside the closed direction domain.
func (d Direction) Validate() error {
	if d <= DirectionUnknown || d >= directionLimit ||
		directionDiagnostics()[d] == "" {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether d belongs to the closed direction domain.
func (d Direction) IsValid() bool { return d.Validate() == nil }

// String returns a diagnostic direction name.
func (d Direction) String() string {
	if !d.IsValid() {
		return ""
	}
	return directionDiagnostics()[d]
}

func directionDiagnostics() [directionLimit]string {
	return [...]string{
		"",
		"upload",
		"download",
	}
}

// OffWireEnum declares Direction as an execution enum.
func (Direction) OffWireEnum() {}

// Validate rejects values outside the closed commitment domain.
func (c Commitment) Validate() error {
	if c <= CommitmentUnknown || c >= commitmentLimit ||
		commitmentDiagnostics()[c] == "" {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether c belongs to the closed commitment domain.
func (c Commitment) IsValid() bool { return c.Validate() == nil }

// String returns a diagnostic commitment name.
func (c Commitment) String() string {
	if !c.IsValid() {
		return ""
	}
	return commitmentDiagnostics()[c]
}

func commitmentDiagnostics() [commitmentLimit]string {
	return [...]string{
		"",
		"not_attempted",
		"rejected",
		"confirmed",
		"indeterminate",
	}
}

// OffWireEnum declares Commitment as an execution enum.
func (Commitment) OffWireEnum() {}
