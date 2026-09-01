package gcsobjects

import (
	"bytes"
	"context"
	"errors"
	"hash/crc32"
	"io"
	"slices"
	"strconv"
	"unicode/utf8"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"
)

// GCSClient is an authenticated capability over the official Cloud Storage SDK.
type GCSClient struct{ client *storage.Client }

// NewGCSClient constructs an authenticated production Cloud Storage client.
func NewGCSClient(ctx context.Context, config GCSClientConfig) (*GCSClient, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	options, err := gcsClientOptions(ctx, config)
	if err != nil {
		return nil, err
	}
	client, err := storage.NewClient(ctx, options...)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	client.SetRetry(storage.WithPolicy(storage.RetryNever))
	return &GCSClient{client: client}, nil
}

const (
	// GCSCredentialJSONMaximumBytes bounds one service-account credential file
	// before its bytes enter Google's authentication SDK.
	GCSCredentialJSONMaximumBytes = 64 << 10
	// GCSAuthenticationResponseMaximumBytes bounds provider credential exchange.
	GCSAuthenticationResponseMaximumBytes = 64 << 10
	gcsSDKAlternateRepresentationQuery    = "alt"
	gcsSDKMediaRepresentation             = "media"
)

func gcsSDKResponseMethods() [7]exchange.Method {
	return [...]exchange.Method{
		exchange.MethodGet,
		exchange.MethodHead,
		exchange.MethodPost,
		exchange.MethodPut,
		exchange.MethodPatch,
		exchange.MethodDelete,
		exchange.MethodOptions,
	}
}

func gcsProviderHTTPClientOption(
	ctx context.Context,
	config GCSClientConfig,
) (option.ClientOption, error) {
	credentialJSON, err := gcsCredentialJSON(ctx, config)
	if err != nil {
		return nil, err
	}
	defer clear(credentialJSON)

	authContext, err := gcsAuthenticationContext(ctx)
	if err != nil {
		return nil, err
	}
	boundaries, err := gcsProviderResponseBoundaries()
	if err != nil {
		return nil, err
	}
	providerTransport, err := exchange.NewStandardOfficialSDKResponseTransport(boundaries[0])
	if err != nil {
		return nil, err
	}
	for _, boundary := range boundaries[1:] {
		providerTransport, err = exchange.NewOfficialSDKResponseTransport(
			exchange.OfficialSDKResponseTransportRequest{
				Base: providerTransport, Boundary: boundary,
			},
		)
		if err != nil {
			return nil, err
		}
	}
	providerTransport, err = htransport.NewTransport(
		authContext,
		providerTransport,
		gcsProviderAuthenticationOptions(credentialJSON)...,
	)
	if err != nil {
		return nil, err
	}
	client, err := exchange.NewOfficialSDKHTTPClient(providerTransport)
	if err != nil {
		return nil, err
	}
	return option.WithHTTPClient(client), nil
}

func gcsProviderResponseBoundaries() ([7]exchange.OfficialSDKResponseBoundary, error) {
	methods := gcsSDKResponseMethods()
	var boundaries [7]exchange.OfficialSDKResponseBoundary
	for index, method := range methods {
		boundary, err := gcsProviderResponseBoundary(method)
		if err != nil {
			return [7]exchange.OfficialSDKResponseBoundary{}, err
		}
		boundaries[index] = boundary
	}
	return boundaries, nil
}

func gcsProviderAuthenticationOptions(credentialJSON []byte) []option.ClientOption {
	options := []option.ClientOption{option.WithScopes(storage.ScopeFullControl)}
	if len(credentialJSON) != 0 {
		options = append(
			options,
			option.WithAuthCredentialsJSON(option.ServiceAccount, credentialJSON),
		)
	}
	return options
}

func gcsClientOptions(ctx context.Context, config GCSClientConfig) ([]option.ClientOption, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	clientOption, err := gcsProviderHTTPClientOption(ctx, config)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	return []option.ClientOption{clientOption, storage.WithJSONReads()}, nil
}

func gcsCredentialJSON(ctx context.Context, config GCSClientConfig) ([]byte, error) {
	if config.Authentication == GCSAuthenticationApplicationDefault {
		return nil, nil
	}
	location, err := filestore.OpenParent(ctx, config.CredentialFile)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	maximum, err := core.NewByteCount(GCSCredentialJSONMaximumBytes)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, location.Root.Close(), err)
	}
	var destination bytes.Buffer
	_, readErr := filestore.Read(ctx, filestore.ReadRequest{
		Destination:  &destination,
		Location:     location,
		MaximumBytes: maximum,
	})
	closeErr := location.Root.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, readErr, closeErr)
	}
	return destination.Bytes(), nil
}

func gcsProviderResponseBoundary(method exchange.Method) (exchange.OfficialSDKResponseBoundary, error) {
	limit, err := core.NewByteCount(GCSProviderResponseMaximumBytes)
	if err != nil {
		return exchange.OfficialSDKResponseBoundary{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	if method == exchange.MethodGet {
		boundary, streamErr := exchange.NewOfficialSDKStreamingSuccessCeiling(
			exchange.OfficialSDKStreamingSuccessCeilingRequest{
				Method:                  method,
				StreamQueryName:         gcsSDKAlternateRepresentationQuery,
				StreamQueryValue:        gcsSDKMediaRepresentation,
				AggregateRepresentation: exchange.OfficialSDKResponseRepresentationJSON,
				AggregateMaximumBytes:   limit,
			},
		)
		if streamErr != nil {
			return exchange.OfficialSDKResponseBoundary{}, errors.Join(core.ErrObjectStoreContract, streamErr)
		}
		return boundary, nil
	}
	boundary, err := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{
		Method: method, Representation: exchange.OfficialSDKResponseRepresentationJSON,
		MaximumBytes: limit,
	})
	if err != nil {
		return exchange.OfficialSDKResponseBoundary{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return boundary, nil
}

func gcsAuthenticationContext(ctx context.Context) (context.Context, error) {
	limit, err := core.NewByteCount(GCSAuthenticationResponseMaximumBytes)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	getBoundary, err := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{
		Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationJSON,
		MaximumBytes: limit,
	})
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	authTransport, err := exchange.NewStandardOfficialSDKResponseTransport(getBoundary)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	postBoundary, err := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{
		Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationJSON,
		MaximumBytes: limit,
	})
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	authTransport, err = exchange.NewOfficialSDKResponseTransport(
		exchange.OfficialSDKResponseTransportRequest{
			Base: authTransport, Boundary: postBoundary,
		},
	)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	authClient, err := exchange.NewOfficialSDKHTTPClient(authTransport)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	return context.WithValue(ctx, oauth2.HTTPClient, authClient), nil
}

// Validate rejects an unconstructed or closed client capability.
func (c *GCSClient) Validate() error {
	if c == nil || c.client == nil {
		return core.ErrObjectStoreContract
	}
	return nil
}

// Close releases the official SDK client's resources exactly once.
func (c *GCSClient) Close() error {
	if err := c.Validate(); err != nil {
		return err
	}
	err := c.client.Close()
	c.client = nil
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// CreateBucket creates one new bucket through the official Cloud Storage SDK.
// Provider conflict is preserved as a typed object-store conflict.
func CreateBucket(
	ctx context.Context,
	client *GCSClient,
	request GCSBucketCreateRequest,
) (GCSBucketProvisioning, error) {
	if err := request.Validate(); err != nil {
		return GCSBucketProvisioning{}, err
	}
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSBucketProvisioning{}, err
	}
	attrs := &storage.BucketAttrs{
		Location:                 request.Location.String(),
		UniformBucketLevelAccess: storage.UniformBucketLevelAccess{Enabled: true},
		PublicAccessPrevention:   storage.PublicAccessPreventionInherited,
	}
	if request.Namespace == GCSNamespaceHierarchical {
		attrs.HierarchicalNamespace = &storage.HierarchicalNamespace{Enabled: true}
	}
	if err := client.client.Bucket(request.Bucket.String()).Create(
		ctx, request.Project.String(), attrs,
	); err != nil {
		return GCSBucketProvisioning{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	result := GCSBucketProvisioning{request: request, set: true}
	return result, result.Validate()
}

const (
	// GCSProviderResponseMaximumBytes bounds one complete provider response
	// before an official Google SDK decodes it.
	GCSProviderResponseMaximumBytes              = 1 << 20
	gcsPublicReadRole               iam.RoleName = "roles/storage.legacyObjectReader"
	gcsJSONBucketPathPrefix                      = "/storage/v1/b/"
	gcsIAMPolicyPathSuffix                       = "/iam"
)

// GrantGCSBucketPublicRead idempotently adds the provider's unauthenticated,
// object-get-only membership to one bucket IAM policy. It performs no upload,
// listing, retry, or product policy and returns proof only after reading the
// provider policy back and observing the exact membership.
func GrantGCSBucketPublicRead(
	ctx context.Context,
	client *GCSClient,
	request GCSBucketPublicReadRequest,
) (GCSBucketPublicReadGrant, error) {
	if err := request.Validate(); err != nil {
		return GCSBucketPublicReadGrant{}, err
	}
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSBucketPublicReadGrant{}, err
	}
	handle := client.client.Bucket(request.Bucket.String()).IAM().V3()
	policy, err := readGCSBucketIAMPolicy(ctx, handle)
	if err != nil {
		return GCSBucketPublicReadGrant{}, err
	}
	if gcsPolicyHasPublicRead(policy) {
		return newGCSBucketPublicReadGrant(request.Bucket, GCSBucketPublicReadUnchanged)
	}
	addGCSBucketPublicRead(policy)
	if err := handle.SetPolicy(ctx, policy); err != nil {
		return GCSBucketPublicReadGrant{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	confirmed, err := readGCSBucketIAMPolicy(ctx, handle)
	if err != nil {
		return GCSBucketPublicReadGrant{}, err
	}
	if !gcsPolicyHasPublicRead(confirmed) {
		return GCSBucketPublicReadGrant{}, errors.Join(
			core.ErrObjectStoreContract,
			core.ErrObjectStoreDestination,
			core.ErrObjectStoreConflict,
		)
	}
	return newGCSBucketPublicReadGrant(request.Bucket, GCSBucketPublicReadGranted)
}

// readGCSBucketIAMPolicy confines the official SDK's provider-response
// projection. The SDK currently panics when a successful IAM response contains
// JSON null; Primitive converts that malformed external response into a typed
// refusal while preserving an error-valued native panic cause.
func readGCSBucketIAMPolicy(ctx context.Context, handle *iam.Handle3) (policy *iam.Policy3, err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		policy = nil
		err = errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
		if cause, ok := recovered.(error); ok {
			err = errors.Join(err, cause)
		}
	}()
	policy, err = handle.Policy(ctx)
	if err != nil {
		return nil, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	if policy == nil {
		return nil, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	if err := validateGCSBucketIAMPolicy(policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func validateGCSBucketIAMPolicy(policy *iam.Policy3) error {
	if policy == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	for _, binding := range policy.Bindings {
		if !validGCSBucketIAMBindingText(binding) {
			return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
		}
	}
	return nil
}

func validGCSBucketIAMBindingText(binding *iampb.Binding) bool {
	if binding == nil || !utf8.ValidString(binding.Role) {
		return false
	}
	for _, member := range binding.Members {
		if !utf8.ValidString(member) {
			return false
		}
	}
	if binding.Condition == nil {
		return true
	}
	return utf8.ValidString(binding.Condition.Expression) &&
		utf8.ValidString(binding.Condition.Title) &&
		utf8.ValidString(binding.Condition.Description) &&
		utf8.ValidString(binding.Condition.Location)
}

func gcsPolicyHasPublicRead(policy *iam.Policy3) bool {
	if policy == nil {
		return false
	}
	for _, binding := range policy.Bindings {
		if binding == nil || binding.Condition != nil || binding.Role != string(gcsPublicReadRole) {
			continue
		}
		if slices.Contains(binding.Members, iam.AllUsers) {
			return true
		}
	}
	return false
}

func addGCSBucketPublicRead(policy *iam.Policy3) {
	for _, binding := range policy.Bindings {
		if binding == nil || binding.Condition != nil || binding.Role != string(gcsPublicReadRole) {
			continue
		}
		binding.Members = append(binding.Members, iam.AllUsers)
		return
	}
	policy.Bindings = append(policy.Bindings, &iampb.Binding{
		Role: string(gcsPublicReadRole), Members: []string{iam.AllUsers},
	})
}

func newGCSBucketPublicReadGrant(
	bucket GCSBucket,
	change GCSBucketPublicReadChange,
) (GCSBucketPublicReadGrant, error) {
	grant := GCSBucketPublicReadGrant{bucket: bucket, change: change, set: true}
	if err := grant.Validate(); err != nil {
		return GCSBucketPublicReadGrant{}, err
	}
	return grant, nil
}

// GCSMediaUpload is create-only ingress for an object a browser or CDN will
// fetch: it carries the content type that makes the bytes render and an
// optional cache-control the edge may honor.
type GCSMediaUpload struct {
	Source       io.Reader
	Bucket       GCSBucket
	Name         GCSObjectName
	ContentType  core.HTTPMediaType
	CacheControl GCSCacheControl
	Integrity    objectstore.Integrity
	CustomTime   temporal.Instant
}

// Validate rejects incomplete or contradictory media ingress. The cache
// directive is optional: a served asset without one lets the edge apply its own
// default, so an absent cache-control is admitted and a present one must be a
// legal field value.
func (r GCSMediaUpload) Validate() error {
	if r.Source == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	for _, err := range []error{
		r.Bucket.Validate(), r.Name.Validate(), validateAuthenticatedGCSIntegrity(r.Integrity),
		r.ContentType.Validate(), r.CustomTime.Validate(),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	if r.CacheControl.String() != "" {
		if err := r.CacheControl.Validate(); err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

// GCSFileUpload is create-only ingress for a stored integrity-bound blob the
// system retrieves and verifies. It carries no content type or cache-control:
// the object is written as application/octet-stream with no cache directive,
// because nothing serves it to a browser.
type GCSFileUpload struct {
	Source     io.Reader
	Bucket     GCSBucket
	Name       GCSObjectName
	Integrity  objectstore.Integrity
	CustomTime temporal.Instant
}

// Validate rejects incomplete or contradictory file ingress.
func (r GCSFileUpload) Validate() error {
	if r.Source == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	for _, err := range []error{
		r.Bucket.Validate(), r.Name.Validate(), validateAuthenticatedGCSIntegrity(r.Integrity),
		r.CustomTime.Validate(),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

// gcsWrite is the owner-only projection both uploads share at the effect leaf.
type gcsWrite struct {
	Source       io.Reader
	Bucket       GCSBucket
	Name         GCSObjectName
	ContentType  core.HTTPMediaType
	CacheControl GCSCacheControl
	Integrity    objectstore.Integrity
	CustomTime   temporal.Instant
}

// GCSReadRequest is exact authenticated object egress.
type GCSReadRequest struct {
	Destination filestore.StageDestinationRequest
	Bucket      GCSBucket
	Name        GCSObjectName
	Integrity   objectstore.Integrity
}

// GCSReadResult transfers ownership of one integrity-verified local stage and
// the exact provider metadata that produced it.
type GCSReadResult struct {
	metadata GCSObjectMetadata
	staged   filestore.StagedFile
}

func (r GCSReadResult) Validate() error {
	return errors.Join(r.metadata.Validate(), r.staged.Validate())
}

func (r GCSReadResult) Metadata() (GCSObjectMetadata, error) {
	if err := r.Validate(); err != nil {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return r.metadata, nil
}

func (r GCSReadResult) Staged() (filestore.StagedFile, error) {
	if err := r.Validate(); err != nil {
		return filestore.StagedFile{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return r.staged, nil
}

// GCSListRequest is one bounded authenticated prefix inventory.
type GCSListRequest struct {
	Bucket     GCSBucket
	Prefix     GCSObjectPrefix
	MaxObjects core.ByteCount
}

// GCSListedReadRequest reads one exact generation returned by ListGCSObjects.
type GCSListedReadRequest struct {
	Destination io.Writer
	Object      GCSObjectMetadata
	Maximum     core.ByteCount
}

// GCSObjectVisitor consumes one provider-sealed object without requiring a slice.
type GCSObjectVisitor func(GCSObjectMetadata) error

// GCSUploadObservationRequest names the authority-owned provider object whose
// exact generation must agree with one authenticated client upload result.
type GCSUploadObservationRequest struct {
	Bucket   GCSBucket
	Name     GCSObjectName
	Evidence objectstore.TransferEvidence
}

// Validate rejects any observation that does not name one exact GCS upload
// generation. The generation is provider evidence, not a caller convention.
func (r GCSUploadObservationRequest) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Name.Validate(), r.Evidence.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	_, err := gcsGenerationFromEvidence(r.Evidence)
	return err
}

func gcsGenerationFromEvidence(evidence objectstore.TransferEvidence) (GCSGeneration, error) {
	if evidence.Provider() != objectstore.ProviderGoogleCloudStorage ||
		evidence.Direction() != objectstore.DirectionUpload {
		return GCSGeneration{}, core.ErrObjectStoreContract
	}
	version, present := evidence.Version()
	if !present || version.Provider() != objectstore.ProviderGoogleCloudStorage {
		return GCSGeneration{}, core.ErrObjectStoreContract
	}
	value, err := strconv.ParseInt(version.String(), 10, 64)
	if err != nil {
		return GCSGeneration{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return NewGCSGeneration(value)
}

// Validate rejects incomplete or contradictory read ingress.
func (r GCSReadRequest) Validate() error {
	for _, err := range []error{r.Bucket.Validate(), r.Name.Validate(), r.Integrity.Validate(), r.Destination.Validate()} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	if r.Integrity.Length.Uint64() > objectstore.GoogleCloudStorageObjectMaximumBytes ||
		r.Destination.ExpectedBytes != r.Integrity.Length {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
	}
	return nil
}

func (r GCSListRequest) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Prefix.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	maximum, err := r.MaxObjects.Uint64()
	if err != nil || maximum > GCSListMaximumObjects {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize, err)
	}
	return nil
}

func (r GCSListedReadRequest) Validate() error {
	if r.Destination == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	if err := r.Object.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	maximum, err := r.Maximum.Uint64()
	if err != nil || maximum > objectstore.GoogleCloudStorageObjectMaximumBytes || r.Object.Length().Uint64() > maximum {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize, err)
	}
	return nil
}

func validateAuthenticatedGCSIntegrity(integrity objectstore.Integrity) error {
	if err := integrity.Validate(); err != nil {
		return err
	}
	if integrity.Length.Uint64() > objectstore.GoogleCloudStorageObjectMaximumBytes {
		return core.ErrObjectStoreSize
	}
	return nil
}

// GCSDeleteRequest is one bounded, generation-safe destructive prefix sweep.
type GCSDeleteRequest struct {
	Bucket     GCSBucket
	Prefix     GCSObjectPrefix
	MaxObjects core.ByteCount
}

// GCSDeleteObjectRequest is one exact generation-safe object deletion.
type GCSDeleteObjectRequest struct {
	Bucket GCSBucket
	Name   GCSObjectName
}

// Validate rejects an unset bucket or exact object name.
func (r GCSDeleteObjectRequest) Validate() error {
	return errors.Join(r.Bucket.Validate(), r.Name.Validate())
}

// Validate rejects unbounded or whole-bucket destructive ingress.
func (r GCSDeleteRequest) Validate() error {
	if err := r.Bucket.Validate(); err != nil {
		return err
	}
	if err := r.Prefix.Validate(); err != nil {
		return err
	}
	maximum, err := r.MaxObjects.Uint64()
	if err != nil || maximum > GCSDeleteMaximumObjects {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// UploadMedia confirms unauthenticated object read on the exact destination
// bucket, then streams one create-only object a browser or CDN will fetch. The
// upload never succeeds merely because its metadata looks browser-compatible:
// the provider IAM policy must first prove that the resulting address is
// publicly readable.
func UploadMedia(ctx context.Context, client *GCSClient, request GCSMediaUpload) (GCSObjectMetadata, error) {
	if err := request.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	if _, err := GrantGCSBucketPublicRead(ctx, client, GCSBucketPublicReadRequest{
		Bucket: request.Bucket,
	}); err != nil {
		return GCSObjectMetadata{}, err
	}
	return createGCSObject(ctx, client, gcsWrite(request))
}

// UploadFile streams one create-only integrity-bound blob the system stores
// and later retrieves, written as application/octet-stream with no cache
// directive because nothing serves it to a browser.
func UploadFile(ctx context.Context, client *GCSClient, request GCSFileUpload) (GCSObjectMetadata, error) {
	if err := request.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return createGCSObject(ctx, client, gcsWrite{
		Source:      request.Source,
		Bucket:      request.Bucket,
		Name:        request.Name,
		ContentType: core.HTTPMediaTypeOctetStream(),
		Integrity:   request.Integrity,
		CustomTime:  request.CustomTime,
	})
}

// createGCSObject streams one exact object through the official SDK under a
// generation-zero precondition. Existing objects are conflicts, never writes.
func createGCSObject(ctx context.Context, client *GCSClient, request gcsWrite) (GCSObjectMetadata, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSObjectMetadata{}, err
	}
	object := client.client.Bucket(request.Bucket.String()).Object(request.Name.String()).If(
		storage.Conditions{DoesNotExist: true},
	)
	writeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	writer := object.NewWriter(writeContext)
	writer.ContentType = request.ContentType.String()
	writer.CacheControl = request.CacheControl.String()
	customTime, err := request.CustomTime.Time()
	if err != nil {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	writer.CustomTime = customTime
	checksum, err := request.Integrity.CRC32C.Uint32()
	if err != nil {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	writer.CRC32C = checksum
	writer.SendCRC32C = true
	if err := streamGCSWrite(writer, cancel, request); err != nil {
		return GCSObjectMetadata{}, err
	}
	if err := writer.Close(); err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	return metadataFromGCSAttrs(writer.Attrs())
}

func streamGCSWrite(writer *storage.Writer, cancel context.CancelFunc, request gcsWrite) error {
	exact, err := objectstore.NewExactReader(request.Source, request.Integrity.Length)
	if err != nil {
		return err
	}
	digest := core.NewDigestWriter()
	if request.Integrity.Length.Uint64() == 0 {
		err = exact.ProveEmpty()
	} else {
		_, err = io.Copy(writer, io.TeeReader(exact, digest))
	}
	if err != nil {
		cancel()
		closeErr := writer.Close()
		if exact.Failure() != nil {
			return errors.Join(exact.Failure(), projectGCSAbortError(closeErr))
		}
		return errors.Join(projectGCSError(err, core.ErrObjectStoreDestination), projectGCSAbortError(closeErr))
	}
	actualDigest, actualLength, err := digest.Seal()
	if err != nil || actualDigest != request.Integrity.SHA256 || actualLength != request.Integrity.Length {
		cancel()
		closeErr := writer.Close()
		return errors.Join(core.ErrObjectStoreSource, core.ErrObjectStoreIntegrity, err, projectGCSAbortError(closeErr))
	}
	return nil
}

func projectGCSAbortError(err error) error {
	if err == nil {
		return nil
	}
	return projectGCSError(err, core.ErrObjectStoreDestination)
}

type gcsReadSession struct {
	reader    *storage.Reader
	metadata  GCSObjectMetadata
	integrity objectstore.Integrity
}

func openGCSReadSession(ctx context.Context, client *GCSClient, request GCSReadRequest) (gcsReadSession, error) {
	object := client.client.Bucket(request.Bucket.String()).Object(request.Name.String())
	reader, err := object.NewReader(ctx)
	if err != nil {
		return gcsReadSession{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	metadata, err := metadataFromReader(ctx, object, reader)
	if err != nil {
		_ = reader.Close()
		return gcsReadSession{}, err
	}
	integrity, err := readIntegrityFromMetadata(request, metadata)
	if err != nil {
		_ = reader.Close()
		return gcsReadSession{}, err
	}
	return gcsReadSession{reader: reader, metadata: metadata, integrity: integrity}, nil
}

func (s gcsReadSession) stage(ctx context.Context, request filestore.StageDestinationRequest) (GCSReadResult, error) {
	destination, err := filestore.OpenStageDestination(ctx, request)
	if err != nil {
		_ = s.reader.Close()
		return GCSReadResult{}, errors.Join(core.ErrObjectStoreDestination, err)
	}
	file, err := destination.File()
	if err != nil {
		_ = s.reader.Close()
		return GCSReadResult{}, errors.Join(core.ErrObjectStoreDestination, filestore.AbandonStageDestination(destination), err)
	}
	if err := streamGCSRead(s.reader, file, s.integrity); err != nil {
		return GCSReadResult{}, errors.Join(err, s.reader.Close(), filestore.AbandonStageDestination(destination))
	}
	if err := s.reader.Close(); err != nil {
		return GCSReadResult{}, errors.Join(projectGCSError(err, core.ErrObjectStoreSource), filestore.AbandonStageDestination(destination))
	}
	staged, err := filestore.FinishStageDestination(ctx, destination)
	if err != nil {
		return GCSReadResult{}, errors.Join(core.ErrObjectStoreDestination, err)
	}
	result := GCSReadResult{metadata: s.metadata, staged: staged}
	return result, result.Validate()
}

// ReadGCSObject streams one exact generation into a caller-owned destination.
func ReadGCSObject(ctx context.Context, client *GCSClient, request GCSReadRequest) (GCSReadResult, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSReadResult{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSReadResult{}, err
	}
	session, err := openGCSReadSession(ctx, client, request)
	if err != nil {
		return GCSReadResult{}, err
	}
	return session.stage(ctx, request.Destination)
}

// ListGCSObjects streams a bounded, lexicographically ordered prefix inventory.
func ListGCSObjects(ctx context.Context, client *GCSClient, request GCSListRequest, visit GCSObjectVisitor) error {
	if err := validateGCSCall(ctx, client); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if visit == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	objects := client.client.Bucket(request.Bucket.String()).Objects(ctx, &storage.Query{Prefix: request.Prefix.String(), Projection: storage.ProjectionNoACL})
	maximum, _ := request.MaxObjects.Uint64()
	return visitGCSObjects(objects, maximum, visit)
}

type gcsObjectIterator interface {
	Next() (*storage.ObjectAttrs, error)
}

func visitGCSObjects(objects gcsObjectIterator, maximum uint64, visit GCSObjectVisitor) error {
	var count uint64
	for {
		attrs, err := objects.Next()
		if errors.Is(err, iterator.Done) {
			return nil
		}
		if err != nil {
			return projectGCSError(err, core.ErrObjectStoreSource)
		}
		if count == maximum {
			return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
		}
		metadata, err := metadataFromGCSAttrs(attrs)
		if err != nil {
			return err
		}
		if err := visit(metadata); err != nil {
			return err
		}
		count++
	}
}

// ReadListedGCSObject streams the exact listed generation and verifies its
// provider extent and CRC32C before returning.
func ReadListedGCSObject(ctx context.Context, client *GCSClient, request GCSListedReadRequest) (GCSObjectMetadata, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSObjectMetadata{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	generation, err := request.Object.Generation().Int64()
	if err != nil {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	object := client.client.Bucket(request.Object.Bucket().String()).Object(request.Object.Name().String()).Generation(generation)
	reader, err := object.NewReader(ctx)
	if err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	metadata, err := metadataFromReader(ctx, object, reader)
	if err != nil {
		_ = reader.Close()
		return GCSObjectMetadata{}, err
	}
	if !sameListedGCSObject(request.Object, metadata) {
		_ = reader.Close()
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreConflict)
	}
	if err := streamGCSListedRead(reader, request.Destination, request.Object); err != nil {
		return GCSObjectMetadata{}, errors.Join(err, reader.Close())
	}
	if err := reader.Close(); err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	return metadata, nil
}

func sameListedGCSObject(listed, read GCSObjectMetadata) bool {
	return listed.Bucket() == read.Bucket() && listed.Name() == read.Name() && listed.Generation() == read.Generation() &&
		listed.Length() == read.Length() && listed.CRC32C() == read.CRC32C()
}

func streamGCSListedRead(reader *storage.Reader, destination io.Writer, listed GCSObjectMetadata) error {
	exact, err := objectstore.NewExactReader(newGCSExactSource(reader, listed.Length()), listed.Length())
	if err != nil {
		return err
	}
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	output := io.MultiWriter(destination, checksum)
	if listed.Length().Uint64() == 0 {
		err = exact.ProveEmpty()
	} else {
		_, err = io.Copy(output, exact)
	}
	if err != nil {
		if exact.Failure() != nil {
			return exact.Failure()
		}
		return errors.Join(core.ErrObjectStoreDestination, err)
	}
	if core.NewCRC32C(checksum.Sum32()) != listed.CRC32C() {
		return core.ErrObjectStoreIntegrity
	}
	return nil
}

// ObserveGCSUpload reads only the official provider metadata for the exact
// generation authenticated by a client completion. It downloads no object
// bytes and releases proof only when provider identity, extent, and CRC32C all
// agree with that completion.
func ObserveGCSUpload(
	ctx context.Context,
	client *GCSClient,
	request GCSUploadObservationRequest,
) (VerifiedGCSUpload, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return VerifiedGCSUpload{}, err
	}
	if err := request.Validate(); err != nil {
		return VerifiedGCSUpload{}, err
	}
	generation, err := gcsGenerationFromEvidence(request.Evidence)
	if err != nil {
		return VerifiedGCSUpload{}, err
	}
	value, err := generation.Int64()
	if err != nil {
		return VerifiedGCSUpload{}, err
	}
	attrs, err := client.client.Bucket(request.Bucket.String()).
		Object(request.Name.String()).Generation(value).Attrs(ctx)
	if err != nil {
		return VerifiedGCSUpload{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	metadata, err := metadataFromGCSAttrs(attrs)
	if err != nil {
		return VerifiedGCSUpload{}, errors.Join(core.ErrObjectStoreSource, err)
	}
	if metadata.Bucket() != request.Bucket || metadata.Name() != request.Name {
		return VerifiedGCSUpload{}, errors.Join(core.ErrObjectStoreIntegrity, core.ErrObjectStoreSource)
	}
	return verifiedGCSUpload(metadata, request.Evidence)
}

func verifiedGCSUpload(
	metadata GCSObjectMetadata,
	evidence objectstore.TransferEvidence,
) (VerifiedGCSUpload, error) {
	version, present := evidence.Version()
	if !present {
		return VerifiedGCSUpload{}, core.ErrObjectStoreContract
	}
	observation, err := objectstore.VerifyProviderUpload(objectstore.ProviderUploadObservationRequest{
		Evidence: evidence, Version: version,
		Bytes: metadata.Length(), CRC32C: metadata.CRC32C(),
		ContentType: metadata.ContentType(), OccurredAt: metadata.CreatedAt(),
	})
	if err != nil {
		return VerifiedGCSUpload{}, err
	}
	verified := VerifiedGCSUpload{metadata: metadata, observation: observation, set: true}
	if err := verified.Validate(); err != nil {
		return VerifiedGCSUpload{}, err
	}
	return verified, nil
}

func metadataFromReader(ctx context.Context, object *storage.ObjectHandle, reader *storage.Reader) (GCSObjectMetadata, error) {
	attrs, err := object.Generation(reader.Attrs.Generation).Attrs(ctx)
	if err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	return metadataFromGCSAttrs(attrs)
}

func readIntegrityFromMetadata(request GCSReadRequest, metadata GCSObjectMetadata) (objectstore.Integrity, error) {
	if metadata.Length() != request.Integrity.Length || metadata.CRC32C() != request.Integrity.CRC32C {
		return objectstore.Integrity{}, errors.Join(core.ErrObjectStoreSize, core.ErrObjectStoreIntegrity)
	}
	return request.Integrity, nil
}

func streamGCSRead(reader *storage.Reader, destinationWriter io.Writer, integrity objectstore.Integrity) error {
	exact, err := objectstore.NewExactReader(newGCSExactSource(reader, integrity.Length), integrity.Length)
	if err != nil {
		return err
	}
	digest := core.NewDigestWriter()
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	destination := io.MultiWriter(destinationWriter, digest, checksum)
	if integrity.Length.Uint64() == 0 {
		err = exact.ProveEmpty()
	} else {
		_, err = io.Copy(destination, exact)
	}
	if err != nil {
		if exact.Failure() != nil {
			return exact.Failure()
		}
		return errors.Join(core.ErrObjectStoreDestination, err)
	}
	actualDigest, actualLength, err := digest.Seal()
	if err != nil || actualDigest != integrity.SHA256 ||
		actualLength != integrity.Length ||
		core.NewCRC32C(checksum.Sum32()) != integrity.CRC32C {
		return errors.Join(core.ErrObjectStoreIntegrity, err)
	}
	return nil
}

type gcsExactSource struct {
	source    io.Reader
	remaining uint64
}

func newGCSExactSource(source io.Reader, length core.ByteLength) *gcsExactSource {
	return &gcsExactSource{source: source, remaining: length.Uint64()}
}

func (s *gcsExactSource) Read(destination []byte) (int, error) {
	count, err := s.source.Read(destination)
	if count < 0 || uint64(count) > s.remaining {
		return 0, core.ErrObjectStoreIntegrity
	}
	s.remaining -= uint64(count)
	return count, err
}

func (s *gcsExactSource) RemainingBytes() uint64 { return s.remaining }

// DeleteGCSObject permanently deletes one exact current generation and proves
// that no current object remains at the name. Soft-delete buckets are refused.
func DeleteGCSObject(ctx context.Context, client *GCSClient, request GCSDeleteObjectRequest) (GCSDeleteObjectResult, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSDeleteObjectResult{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSDeleteObjectResult{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	bucket := client.client.Bucket(request.Bucket.String())
	if err := requirePermanentGCSDeletion(ctx, bucket); err != nil {
		return GCSDeleteObjectResult{}, err
	}
	object := bucket.Object(request.Name.String())
	generation, value, err := currentGCSGeneration(ctx, object)
	if err != nil {
		return GCSDeleteObjectResult{}, err
	}
	if err := object.Generation(value).If(storage.Conditions{GenerationMatch: value}).Delete(ctx); err != nil &&
		!errors.Is(err, storage.ErrObjectNotExist) {
		return GCSDeleteObjectResult{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	if err := confirmGCSObjectAbsent(ctx, object); err != nil {
		return GCSDeleteObjectResult{}, err
	}
	return GCSDeleteObjectResult{name: request.Name, generation: generation}, nil
}

// currentGCSGeneration observes the exact live generation a delete
// precondition binds to. Absence surfaces from the attribute read itself;
// attributes carrying no object is a contract violation rather than an
// absence.
func currentGCSGeneration(ctx context.Context, object *storage.ObjectHandle) (GCSGeneration, int64, error) {
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return GCSGeneration{}, 0, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	if attrs == nil {
		return GCSGeneration{}, 0, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	generation, err := NewGCSGeneration(attrs.Generation)
	if err != nil {
		return GCSGeneration{}, 0, err
	}
	value, err := generation.Int64()
	if err != nil {
		return GCSGeneration{}, 0, err
	}
	return generation, value, nil
}

func confirmGCSObjectAbsent(ctx context.Context, object *storage.ObjectHandle) error {
	_, err := object.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	if err != nil {
		return projectGCSError(err, core.ErrObjectStoreDestination)
	}
	return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreConflict)
}

// DeleteGCSObjects permanently deletes every live object under one bounded
// prefix, using each listed generation as a delete precondition, then proves
// the prefix absent. Buckets with soft deletion enabled are refused.
func DeleteGCSObjects(ctx context.Context, client *GCSClient, request GCSDeleteRequest) (GCSDeleteResult, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSDeleteResult{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSDeleteResult{}, err
	}
	bucket := client.client.Bucket(request.Bucket.String())
	if err := requirePermanentGCSDeletion(ctx, bucket); err != nil {
		return GCSDeleteResult{}, err
	}
	maximum, _ := request.MaxObjects.Uint64()
	deleted, err := deleteGCSPrefix(ctx, bucket, request.Prefix, maximum)
	if err != nil {
		return GCSDeleteResult{}, err
	}
	if err := confirmGCSPrefixAbsent(ctx, bucket, request.Prefix); err != nil {
		return GCSDeleteResult{}, err
	}
	count, err := core.NewByteLength(deleted)
	if err != nil {
		return GCSDeleteResult{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return GCSDeleteResult{prefix: request.Prefix, deleted: count}, nil
}

func requirePermanentGCSDeletion(ctx context.Context, bucket *storage.BucketHandle) error {
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		return projectGCSError(err, core.ErrObjectStoreDestination)
	}
	if attrs.SoftDeletePolicy != nil && attrs.SoftDeletePolicy.RetentionDuration > 0 {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreConflict)
	}
	return nil
}

func deleteGCSPrefix(ctx context.Context, bucket *storage.BucketHandle, prefix GCSObjectPrefix, maximum uint64) (uint64, error) {
	objects := bucket.Objects(ctx, &storage.Query{Prefix: prefix.String(), Projection: storage.ProjectionNoACL})
	var deleted uint64
	for {
		attrs, err := objects.Next()
		if errors.Is(err, iterator.Done) {
			return deleted, nil
		}
		if err != nil {
			return deleted, projectGCSError(err, core.ErrObjectStoreDestination)
		}
		if deleted == maximum {
			return deleted, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
		}
		metadata, err := metadataFromGCSAttrs(attrs)
		if err != nil {
			return deleted, err
		}
		generation, err := metadata.Generation().Int64()
		if err != nil {
			return deleted, err
		}
		object := bucket.Object(metadata.Name().String()).Generation(generation).If(
			storage.Conditions{GenerationMatch: generation},
		)
		if err := object.Delete(ctx); errors.Is(err, storage.ErrObjectNotExist) {
			continue
		} else if err != nil {
			return deleted, projectGCSError(err, core.ErrObjectStoreDestination)
		}
		deleted++
	}
}

func confirmGCSPrefixAbsent(ctx context.Context, bucket *storage.BucketHandle, prefix GCSObjectPrefix) error {
	objects := bucket.Objects(ctx, &storage.Query{Prefix: prefix.String(), Projection: storage.ProjectionNoACL})
	_, err := objects.Next()
	if errors.Is(err, iterator.Done) {
		return nil
	}
	if err != nil {
		return projectGCSError(err, core.ErrObjectStoreDestination)
	}
	return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreConflict)
}

func validateGCSCall(ctx context.Context, client *GCSClient) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return client.Validate()
}

func metadataFromGCSAttrs(attrs *storage.ObjectAttrs) (GCSObjectMetadata, error) {
	if attrs == nil {
		return GCSObjectMetadata{}, core.ErrObjectStoreContract
	}
	identity, err := gcsIdentityFromAttrs(attrs)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	properties, err := gcsPropertiesFromAttrs(attrs)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	times, err := gcsTimesFromAttrs(attrs)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	metadata := GCSObjectMetadata{
		bucket: identity.bucket, name: identity.name, generation: identity.generation,
		length: properties.length, crc32c: core.NewCRC32C(attrs.CRC32C),
		contentType: properties.contentType, cacheControl: properties.cacheControl,
		createdAt: times.createdAt, updatedAt: times.updatedAt, customTime: times.customTime,
	}
	if err := metadata.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return metadata, nil
}

type gcsObjectIdentity struct {
	bucket     GCSBucket
	name       GCSObjectName
	generation GCSGeneration
}

func gcsIdentityFromAttrs(attrs *storage.ObjectAttrs) (gcsObjectIdentity, error) {
	bucket, err := ParseGCSBucket(attrs.Bucket)
	if err != nil {
		return gcsObjectIdentity{}, err
	}
	name, err := ParseGCSObjectName(attrs.Name)
	if err != nil {
		return gcsObjectIdentity{}, err
	}
	generation, err := NewGCSGeneration(attrs.Generation)
	if err != nil {
		return gcsObjectIdentity{}, err
	}
	return gcsObjectIdentity{bucket: bucket, name: name, generation: generation}, nil
}

type gcsObjectProperties struct {
	contentType  core.HTTPMediaType
	cacheControl GCSCacheControl
	length       core.ByteLength
}

func gcsPropertiesFromAttrs(attrs *storage.ObjectAttrs) (gcsObjectProperties, error) {
	lengthValue, err := core.CheckedUint64FromInt64(attrs.Size)
	if err != nil {
		return gcsObjectProperties{}, errors.Join(core.ErrObjectStoreSize, err)
	}
	length, err := core.NewByteLength(lengthValue)
	if err != nil {
		return gcsObjectProperties{}, errors.Join(core.ErrObjectStoreSize, err)
	}
	contentType, err := core.ParseHTTPMediaType(attrs.ContentType)
	if err != nil {
		return gcsObjectProperties{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	cacheControl, err := optionalGCSCacheControl(attrs.CacheControl)
	if err != nil {
		return gcsObjectProperties{}, err
	}
	return gcsObjectProperties{length: length, contentType: contentType, cacheControl: cacheControl}, nil
}

// optionalGCSCacheControl reads a possibly-absent provider cache directive. A
// stored file is written with no Cache-Control, so an empty field is the absent
// value rather than a malformed one; a present value must still be a legal HTTP
// field value.
func optionalGCSCacheControl(value string) (GCSCacheControl, error) {
	if value == "" {
		return GCSCacheControl{}, nil
	}
	return ParseGCSCacheControl(value)
}

type gcsObjectTimes struct {
	createdAt  temporal.Instant
	updatedAt  temporal.Instant
	customTime temporal.Instant
}

func gcsTimesFromAttrs(attrs *storage.ObjectAttrs) (gcsObjectTimes, error) {
	createdAt, err := temporal.NewInstant(attrs.Created)
	if err != nil {
		return gcsObjectTimes{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	updatedAt, err := temporal.NewInstant(attrs.Updated)
	if err != nil {
		return gcsObjectTimes{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	var customTime temporal.Instant
	if !attrs.CustomTime.IsZero() {
		customTime, err = temporal.NewInstant(attrs.CustomTime)
		if err != nil {
			return gcsObjectTimes{}, errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return gcsObjectTimes{createdAt: createdAt, updatedAt: updatedAt, customTime: customTime}, nil
}

func projectGCSError(cause error, operation core.ErrorIdentity) error {
	projected := []error{core.ErrObjectStoreContract, operation, cause}
	if errors.Is(cause, core.ErrExchangeBodyLimit) {
		projected = append(projected, core.ErrObjectStoreSize)
	}
	if errors.Is(cause, storage.ErrObjectNotExist) {
		projected = append(projected, core.ErrObjectStoreAbsent)
	}
	provider, hasProvider := errors.AsType[*googleapi.Error](cause)
	if hasProvider {
		switch provider.Code {
		case 404:
			projected = append(projected, core.ErrObjectStoreAbsent)
		case 409, 412:
			projected = append(projected, core.ErrObjectStoreConflict)
		}
	}
	return errors.Join(projected...)
}
