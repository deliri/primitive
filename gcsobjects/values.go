package gcsobjects

import (
	"errors"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// GCSBucketMinimumBytes is the provider's minimum bucket-name extent.
	// Source: https://cloud.google.com/storage/docs/buckets#naming
	GCSBucketMinimumBytes = 3
	// GCSBucketMaximumBytes is the provider's maximum dotted bucket-name extent.
	// Source: https://cloud.google.com/storage/docs/buckets#naming
	GCSBucketMaximumBytes = 222
	// GCSBucketComponentMaximumBytes bounds each DNS-shaped component.
	// Source: https://cloud.google.com/storage/docs/buckets#naming
	GCSBucketComponentMaximumBytes = 63
	// GCSObjectNameMaximumBytes is the flat-namespace object-name ceiling.
	// Source: https://cloud.google.com/storage/docs/objects#naming
	GCSObjectNameMaximumBytes = 1024
	// GCSCacheControlMaximumBytes is Primitive's custody ceiling for response
	// cache policy metadata; Google publishes the property but no field extent.
	// Source: https://cloud.google.com/storage/docs/metadata#cache-control
	GCSCacheControlMaximumBytes = 1024
	// GCSDeleteMaximumObjects is Primitive's largest bounded destructive sweep;
	// it is not a Google request limit because the SDK deletes one object per call.
	// Source: https://cloud.google.com/storage/docs/deleting-objects#storage-delete-object-go
	GCSDeleteMaximumObjects = 10_000
	// GCSListMaximumObjects is Primitive's visitor-call custody ceiling; Google
	// paginates provider listings and publishes no whole-inventory maximum.
	// Source: https://cloud.google.com/storage/docs/listing-objects#storage-list-objects-go
	GCSListMaximumObjects = 10_000
)

// GCSAuthentication selects exactly how the official SDK finds credentials.
type GCSAuthentication uint8

const (
	// GCSAuthenticationUnknown is the invalid zero authentication mode.
	GCSAuthenticationUnknown GCSAuthentication = iota
	// GCSAuthenticationApplicationDefault uses Application Default Credentials.
	GCSAuthenticationApplicationDefault
	// GCSAuthenticationServiceAccountFile uses one explicit credential file.
	GCSAuthenticationServiceAccountFile
	gcsAuthenticationLimit
)

// Validate closes the credential-discovery domain.
func (a GCSAuthentication) Validate() error {
	if a <= GCSAuthenticationUnknown || a >= gcsAuthenticationLimit ||
		gcsAuthenticationDiagnostics()[a] == "" {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether a belongs to the closed authentication domain.
func (a GCSAuthentication) IsValid() bool { return a.Validate() == nil }

// String returns the diagnostic authentication name.
func (a GCSAuthentication) String() string {
	if !a.IsValid() {
		return ""
	}
	return gcsAuthenticationDiagnostics()[a]
}

func gcsAuthenticationDiagnostics() [gcsAuthenticationLimit]string {
	return [...]string{"", "application_default", "service_account_file"}
}

// OffWireEnum declares GCSAuthentication as execution policy, not wire syntax.
func (GCSAuthentication) OffWireEnum() {}

// GCSClientConfig is authenticated SDK construction ingress.
type GCSClientConfig struct {
	CredentialFile core.AbsolutePath
	Authentication GCSAuthentication
}

// Validate binds the selected mode to its exact credential input.
func (c GCSClientConfig) Validate() error {
	if err := c.Authentication.Validate(); err != nil {
		return err
	}
	switch c.Authentication {
	case GCSAuthenticationApplicationDefault:
		if c.CredentialFile.String() != "" {
			return core.ErrObjectStoreContract
		}
	case GCSAuthenticationServiceAccountFile:
		if err := c.CredentialFile.Validate(); err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	default:
		return core.ErrObjectStoreContract
	}
	return nil
}

// GCSBucket is one validated Google Cloud Storage bucket name.
type GCSBucket struct{ value string }

// ParseGCSBucket admits one provider-valid bucket name.
func ParseGCSBucket(value string) (GCSBucket, error) {
	bucket := GCSBucket{value: value}
	if err := bucket.Validate(); err != nil {
		return GCSBucket{}, err
	}
	return bucket, nil
}

// String returns the validated bucket name.
func (b GCSBucket) String() string { return b.value }

// Validate rejects names that cannot identify one GCS bucket.
func (b GCSBucket) Validate() error {
	value := b.value
	if !validGCSBucketExtent(value) || !gcsLowerAlphaNumeric(value[0]) ||
		!gcsLowerAlphaNumeric(value[len(value)-1]) || reservedGCSBucket(value) {
		return core.ErrObjectStoreContract
	}
	return validateGCSBucketComponents(value)
}

func validGCSBucketExtent(value string) bool {
	return len(value) >= GCSBucketMinimumBytes && len(value) <= gcsBucketMaximumBytes(value)
}

func reservedGCSBucket(value string) bool {
	return net.ParseIP(value) != nil || strings.HasPrefix(value, "goog") ||
		strings.Contains(value, "google") || strings.Contains(value, "g00gle")
}

func validateGCSBucketComponents(value string) error {
	componentBytes := 0
	for index := range len(value) {
		if !gcsBucketByte(value[index]) {
			return core.ErrObjectStoreContract
		}
		if value[index] == '.' {
			if componentBytes == 0 {
				return core.ErrObjectStoreContract
			}
			componentBytes = 0
			continue
		}
		componentBytes++
		if componentBytes > GCSBucketComponentMaximumBytes {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}

func gcsBucketMaximumBytes(value string) int {
	if strings.ContainsRune(value, '.') {
		return GCSBucketMaximumBytes
	}
	return GCSBucketComponentMaximumBytes
}

func gcsBucketByte(value byte) bool {
	return gcsLowerAlphaNumeric(value) || value == '-' || value == '_' || value == '.'
}

func gcsLowerAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

// GCSObjectName is one exact flat-namespace object name.
type GCSObjectName struct{ value string }

// ParseGCSObjectName validates an exact provider object name.
func ParseGCSObjectName(value string) (GCSObjectName, error) {
	name := GCSObjectName{value: value}
	if err := name.Validate(); err != nil {
		return GCSObjectName{}, err
	}
	return name, nil
}

// String returns the validated object name.
func (n GCSObjectName) String() string { return n.value }

// Validate rejects reserved, malformed, or oversized object names.
func (n GCSObjectName) Validate() error {
	if !validGCSObjectText(n.value) {
		return core.ErrObjectStoreContract
	}
	return nil
}

// GCSObjectPrefix is a nonempty object-list boundary. It must identify a
// directory-shaped namespace, never the whole bucket or an ambiguous leaf.
type GCSObjectPrefix struct{ value string }

// ParseGCSObjectPrefix admits a confined slash-terminated object prefix.
func ParseGCSObjectPrefix(value string) (GCSObjectPrefix, error) {
	prefix := GCSObjectPrefix{value: value}
	if err := prefix.Validate(); err != nil {
		return GCSObjectPrefix{}, err
	}
	return prefix, nil
}

// String returns the validated object prefix.
func (p GCSObjectPrefix) String() string { return p.value }

// Validate rejects whole-bucket, leaf-shaped, and ambiguous prefixes.
func (p GCSObjectPrefix) Validate() error {
	if !validGCSObjectText(p.value) || !strings.HasSuffix(p.value, "/") ||
		strings.HasPrefix(p.value, "/") || strings.Contains(p.value, "//") {
		return core.ErrObjectStoreContract
	}
	segments := strings.SplitSeq(strings.TrimSuffix(p.value, "/"), "/")
	for segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}

func validGCSObjectText(value string) bool {
	return len(value) > 0 && len(value) <= GCSObjectNameMaximumBytes &&
		utf8.ValidString(value) && value != "." && value != ".." &&
		!strings.ContainsAny(value, "\r\n") &&
		!strings.HasPrefix(value, ".well-known/acme-challenge/")
}

// GCSCacheControl is one bounded HTTP field value stored as object metadata.
type GCSCacheControl struct{ value string }

// ParseGCSCacheControl admits one nonempty Cache-Control field value.
func ParseGCSCacheControl(value string) (GCSCacheControl, error) {
	policy := GCSCacheControl{value: value}
	if err := policy.Validate(); err != nil {
		return GCSCacheControl{}, err
	}
	return policy, nil
}

// String returns the validated field value.
func (c GCSCacheControl) String() string { return c.value }

// Validate rejects empty, oversized, or invalid HTTP field values.
func (c GCSCacheControl) Validate() error {
	if len(c.value) == 0 || len(c.value) > GCSCacheControlMaximumBytes ||
		core.ValidateHTTPFieldValue(c.value) != nil {
		return core.ErrObjectStoreContract
	}
	return nil
}

// GCSGeneration is one positive provider-issued immutable object generation.
type GCSGeneration struct{ value int64 }

// NewGCSGeneration admits one positive provider generation.
func NewGCSGeneration(value int64) (GCSGeneration, error) {
	generation := GCSGeneration{value: value}
	if err := generation.Validate(); err != nil {
		return GCSGeneration{}, err
	}
	return generation, nil
}

// Int64 returns the validated generation.
func (g GCSGeneration) Int64() (int64, error) {
	if err := g.Validate(); err != nil {
		return 0, err
	}
	return g.value, nil
}

// Validate rejects the unset and impossible generation domains.
func (g GCSGeneration) Validate() error {
	if g.value <= 0 {
		return core.ErrObjectStoreContract
	}
	return nil
}

// GCSObjectMetadata is immutable provider evidence for one exact generation.
type GCSObjectMetadata struct {
	bucket       GCSBucket
	name         GCSObjectName
	contentType  core.HTTPMediaType
	cacheControl GCSCacheControl
	createdAt    temporal.Instant
	updatedAt    temporal.Instant
	customTime   temporal.Instant
	generation   GCSGeneration
	length       core.ByteLength
	crc32c       core.CRC32C
}

func (m GCSObjectMetadata) Bucket() GCSBucket               { return m.bucket }
func (m GCSObjectMetadata) Name() GCSObjectName             { return m.name }
func (m GCSObjectMetadata) Generation() GCSGeneration       { return m.generation }
func (m GCSObjectMetadata) Length() core.ByteLength         { return m.length }
func (m GCSObjectMetadata) CRC32C() core.CRC32C             { return m.crc32c }
func (m GCSObjectMetadata) ContentType() core.HTTPMediaType { return m.contentType }
func (m GCSObjectMetadata) CacheControl() GCSCacheControl   { return m.cacheControl }
func (m GCSObjectMetadata) CreatedAt() temporal.Instant     { return m.createdAt }
func (m GCSObjectMetadata) UpdatedAt() temporal.Instant     { return m.updatedAt }
func (m GCSObjectMetadata) CustomTime() temporal.Instant    { return m.customTime }

// Validate proves the provider evidence is complete and internally usable. A
// stored file carries no cache directive, so an absent cache-control is valid
// provider evidence; a present one must be a legal field value.
func (m GCSObjectMetadata) Validate() error {
	for _, err := range []error{
		m.bucket.Validate(), m.name.Validate(), m.generation.Validate(),
		m.length.Validate(), m.crc32c.Validate(), m.contentType.Validate(),
		m.createdAt.Validate(), m.updatedAt.Validate(),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	if m.cacheControl.String() != "" {
		if err := m.cacheControl.Validate(); err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	if m.customTime != (temporal.Instant{}) {
		if err := m.customTime.Validate(); err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

// VerifiedGCSUpload is sealed proof that official provider metadata for one
// exact generation agreed with authenticated upload evidence.
type VerifiedGCSUpload struct {
	metadata    GCSObjectMetadata
	observation objectstore.VerifiedProviderUpload
	set         bool
}

// Validate rechecks every fact retained by the observation proof.
func (v VerifiedGCSUpload) Validate() error {
	if !v.set {
		return core.ErrObjectStoreContract
	}
	if err := errors.Join(v.metadata.Validate(), v.observation.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return errors.Join(v.validateEvidenceBinding(), v.validateProviderProjection())
}

func (v VerifiedGCSUpload) validateEvidenceBinding() error {
	evidence, err := v.observation.Evidence()
	if err != nil {
		return err
	}
	generation, err := gcsGenerationFromEvidence(evidence)
	if err != nil {
		return err
	}
	if v.metadata.Generation() != generation ||
		v.metadata.Length() != evidence.Bytes() ||
		v.metadata.CRC32C() != evidence.CRC32C() {
		return errors.Join(core.ErrObjectStoreIntegrity, core.ErrObjectStoreSource)
	}
	return nil
}

func (v VerifiedGCSUpload) validateProviderProjection() error {
	contentType, contentTypeErr := v.observation.ContentType()
	occurredAt, occurredAtErr := v.observation.OccurredAt()
	if err := errors.Join(contentTypeErr, occurredAtErr); err != nil {
		return err
	}
	if v.metadata.ContentType() != contentType || v.metadata.CreatedAt() != occurredAt {
		return errors.Join(core.ErrObjectStoreIntegrity, core.ErrObjectStoreSource)
	}
	return nil
}

// Metadata returns the exact authenticated provider metadata.
func (v VerifiedGCSUpload) Metadata() (GCSObjectMetadata, error) {
	if err := v.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return v.metadata, nil
}

// Evidence returns the exact authenticated client evidence reconciled by the
// provider observation.
func (v VerifiedGCSUpload) Evidence() (objectstore.TransferEvidence, error) {
	if err := v.Validate(); err != nil {
		return objectstore.TransferEvidence{}, err
	}
	return v.observation.Evidence()
}

// ProviderObservation returns the provider-neutral exact-upload proof used by
// authority reconciliation.
func (v VerifiedGCSUpload) ProviderObservation() (objectstore.VerifiedProviderUpload, error) {
	if err := v.Validate(); err != nil {
		return objectstore.VerifiedProviderUpload{}, err
	}
	return v.observation, nil
}

// GCSDeleteResult is sealed evidence that a bounded prefix was absent after a
// generation-matched destructive sweep.
type GCSDeleteResult struct {
	prefix  GCSObjectPrefix
	deleted core.ByteLength
}

// GCSDeleteObjectResult is sealed evidence that one exact generation was
// removed and its current name was proved absent.
type GCSDeleteObjectResult struct {
	name       GCSObjectName
	generation GCSGeneration
}

func (r GCSDeleteObjectResult) Name() GCSObjectName       { return r.name }
func (r GCSDeleteObjectResult) Generation() GCSGeneration { return r.generation }

// Validate rejects incomplete exact-object deletion evidence.
func (r GCSDeleteObjectResult) Validate() error {
	return errors.Join(r.name.Validate(), r.generation.Validate())
}

func (r GCSDeleteResult) Prefix() GCSObjectPrefix  { return r.prefix }
func (r GCSDeleteResult) Deleted() core.ByteLength { return r.deleted }

// Validate rejects unconfirmed or malformed purge evidence.
func (r GCSDeleteResult) Validate() error {
	if err := r.prefix.Validate(); err != nil {
		return err
	}
	return r.deleted.Validate()
}

var (
	_ core.OffWireEnum = GCSAuthenticationUnknown
	_ core.Validatable = GCSUploadObservationRequest{}
	_ core.Validatable = VerifiedGCSUpload{}
)
