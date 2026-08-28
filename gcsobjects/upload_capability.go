package gcsobjects

import (
	"context"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

const (
	// GCSServiceAccountMaximumBytes is the documented Internet mailbox ceiling
	// applied to one Google service-account principal.
	GCSServiceAccountMaximumBytes = 254
	// GCSCapabilityMaximumDays is the provider's V4 signed URL lifetime
	// ceiling in exact 24-hour days.
	GCSCapabilityMaximumDays uint64 = 7
	// GCSSignatureMaximumBytes bounds one decoded provider signature before it
	// can become bearer material.
	GCSSignatureMaximumBytes = 1024
	// GCSSignatureMaximumEncodedBytes is the canonical base64 ceiling for one
	// provider signature.
	GCSSignatureMaximumEncodedBytes = ((GCSSignatureMaximumBytes + 2) / 3) * 4
	gcsServiceAccountDomainSuffix   = ".gserviceaccount.com"
	gcsSignBlobResourcePrefix       = "projects/-/serviceAccounts/"
	gcsSignedURLHeaderSeparator     = ":"
)

// GCSServiceAccount is one validated Google service-account signing principal.
type GCSServiceAccount struct{ value string }

// ParseGCSServiceAccount admits one canonical Google service-account address.
func ParseGCSServiceAccount(value string) (GCSServiceAccount, error) {
	candidate := GCSServiceAccount{value: value}
	if err := candidate.Validate(); err != nil {
		return GCSServiceAccount{}, err
	}
	return candidate, nil
}

// String returns the validated provider principal.
func (a GCSServiceAccount) String() string { return a.value }

// Validate rejects unset, noncanonical, or foreign mailbox identities.
func (a GCSServiceAccount) Validate() error {
	if len(a.value) == 0 || len(a.value) > GCSServiceAccountMaximumBytes ||
		!gcsServiceAccountASCII(a.value) ||
		!strings.HasSuffix(a.value, gcsServiceAccountDomainSuffix) {
		return core.ErrObjectStoreContract
	}
	parsed, err := mail.ParseAddress(a.value)
	if err != nil || parsed.Name != "" || parsed.Address != a.value {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

func gcsServiceAccountASCII(value string) bool {
	for index := range len(value) {
		if value[index] < '!' || value[index] > '~' {
			return false
		}
	}
	return true
}

// GCSCapabilityIssuer is an authenticated capability over the official
// IAM Credentials SDK. It mints no credential; it asks the named provider
// principal to sign one Objectstore-owned raw object request.
type GCSCapabilityIssuer struct {
	service *iamcredentials.Service
}

// NewGCSCapabilityIssuer constructs the official IAM Credentials client
// through the same closed credential discovery contract as GCSClient.
func NewGCSCapabilityIssuer(
	ctx context.Context,
	config GCSClientConfig,
) (*GCSCapabilityIssuer, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	options, err := gcsClientOptions(ctx, config)
	if err != nil {
		return nil, err
	}
	service, err := iamcredentials.NewService(ctx, options...)
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	return &GCSCapabilityIssuer{service: service}, nil
}

// Validate rejects an unconstructed signing capability.
func (i *GCSCapabilityIssuer) Validate() error {
	if i == nil || i.service == nil {
		return core.ErrObjectStoreContract
	}
	return nil
}

// GCSUploadCapabilityRequest is one exact create-only raw-object capability
// issuance intent. It carries provider facts only; grants, users, products,
// media roles, and workflow policy remain downstream.
type GCSUploadCapabilityRequest struct {
	Bucket         GCSBucket
	Name           GCSObjectName
	ServiceAccount GCSServiceAccount
	ContentType    core.HTTPMediaType
	Integrity      objectstore.Integrity
	Lifetime       temporal.Duration
}

// gcsUploadURLRequest is the owner-only projection passed to the official
// Storage signing leaf after every nominal input has crossed validation.
type gcsUploadURLRequest struct {
	capability GCSUploadCapabilityRequest
	expiresAt  temporal.Instant
}

func (r gcsUploadURLRequest) Validate() error {
	return errors.Join(r.capability.Validate(), r.expiresAt.Validate())
}

func (r gcsUploadURLRequest) signingHeaderLines() ([]string, error) {
	signedHeaders, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		return nil, err
	}
	headers, err := objectstore.NewUploadSigningHeaders(
		objectstore.ProviderGoogleCloudStorage,
		signedHeaders,
		r.capability.Integrity,
	)
	if err != nil {
		return nil, err
	}
	lines := make([]string, len(headers.Values))
	for index, header := range headers.Values {
		if err := header.Validate(); err != nil || len(header.Values) != 1 {
			return nil, errors.Join(core.ErrObjectStoreContract, err)
		}
		value, valueErr := header.Values[0].Value()
		if valueErr != nil {
			return nil, errors.Join(core.ErrObjectStoreContract, valueErr)
		}
		lines[index] = header.Name.String() + gcsSignedURLHeaderSeparator + value
	}
	return lines, nil
}

// Validate closes every fact before a provider signing request begins.
func (r GCSUploadCapabilityRequest) Validate() error {
	for _, err := range []error{
		r.Bucket.Validate(),
		r.Name.Validate(),
		r.ServiceAccount.Validate(),
		validateAuthenticatedGCSIntegrity(r.Integrity),
		r.ContentType.Validate(),
		validateGCSCapabilityLifetime(r.Lifetime),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

func validateGCSCapabilityLifetime(lifetime temporal.Duration) error {
	maximum, err := temporal.DurationFromDays(GCSCapabilityMaximumDays)
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := lifetime.Validate(); err != nil || lifetime.IsZero() ||
		lifetime.Nanoseconds() > maximum.Nanoseconds() {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// IssueGCSUploadCapability binds Objectstore's exact request fields into one
// official V4 signed URL and returns only the opaque Objectstore projection.
func IssueGCSUploadCapability(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
	request GCSUploadCapabilityRequest,
) (objectstore.UploadCapabilityProjection, error) {
	if err := request.Validate(); err != nil {
		return objectstore.UploadCapabilityProjection{}, err
	}
	if err := validateGCSCapabilityIssuer(ctx, issuer); err != nil {
		return objectstore.UploadCapabilityProjection{}, err
	}
	expiresAt, err := gcsCapabilityExpiry(request.Lifetime)
	if err != nil {
		return objectstore.UploadCapabilityProjection{}, err
	}
	rawURL, err := issueGCSUploadURL(ctx, issuer, gcsUploadURLRequest{
		capability: request,
		expiresAt:  expiresAt,
	})
	if err != nil {
		return objectstore.UploadCapabilityProjection{}, err
	}
	return projectGCSUploadCapability(rawURL, expiresAt)
}

func gcsCapabilityExpiry(lifetime temporal.Duration) (temporal.Instant, error) {
	observation, err := temporal.Observe()
	if err != nil {
		return temporal.Instant{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	now, err := observation.Instant()
	if err != nil {
		return temporal.Instant{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	expiresAt, err := now.Add(lifetime)
	if err != nil {
		return temporal.Instant{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return expiresAt, nil
}

func validateGCSCapabilityIssuer(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return issuer.Validate()
}

func issueGCSUploadURL(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
	request gcsUploadURLRequest,
) (string, error) {
	if err := request.Validate(); err != nil {
		return "", errors.Join(core.ErrObjectStoreContract, err)
	}
	headerLines, err := request.signingHeaderLines()
	if err != nil {
		return "", err
	}
	expiration, err := request.expiresAt.Time()
	if err != nil {
		return "", errors.Join(core.ErrObjectStoreContract, err)
	}
	spec, err := objectstore.Spec(objectstore.ProviderGoogleCloudStorage)
	if err != nil {
		return "", err
	}
	rawURL, err := storage.SignedURL(request.capability.Bucket.String(), request.capability.Name.String(), &storage.SignedURLOptions{
		GoogleAccessID: request.capability.ServiceAccount.String(),
		SignBytes: func(payload []byte) ([]byte, error) {
			return signGCSCapabilityBytes(ctx, issuer, request.capability.ServiceAccount, payload)
		},
		Method:      spec.UploadMethod.String(),
		Expires:     expiration,
		ContentType: request.capability.ContentType.String(),
		Headers:     headerLines,
		Scheme:      storage.SigningSchemeV4,
	})
	if err != nil {
		return "", errors.Join(core.ErrObjectStoreDestination, err)
	}
	return rawURL, nil
}

func signGCSCapabilityBytes(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
	account GCSServiceAccount,
	payload []byte,
) ([]byte, error) {
	if len(payload) == 0 {
		return nil, core.ErrObjectStoreContract
	}
	response, err := issuer.service.Projects.ServiceAccounts.SignBlob(
		gcsSignBlobResourcePrefix+account.String(),
		&iamcredentials.SignBlobRequest{Payload: base64.StdEncoding.EncodeToString(payload)},
	).Context(ctx).Do()
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreDestination, err)
	}
	if response == nil || response.SignedBlob == "" {
		return nil, core.ErrObjectStoreDestination
	}
	return decodeGCSCapabilitySignature(response.SignedBlob)
}

func decodeGCSCapabilitySignature(encoded string) ([]byte, error) {
	if len(encoded) == 0 || len(encoded) > GCSSignatureMaximumEncodedBytes {
		return nil, core.ErrObjectStoreDestination
	}
	signed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(signed) == 0 || len(signed) > GCSSignatureMaximumBytes ||
		base64.StdEncoding.EncodeToString(signed) != encoded {
		return nil, errors.Join(core.ErrObjectStoreDestination, err)
	}
	return signed, nil
}

func projectGCSUploadCapability(
	rawURL string,
	expiresAt temporal.Instant,
) (objectstore.UploadCapabilityProjection, error) {
	signedURL, err := objectstore.ParseSignedURL(rawURL)
	if err != nil {
		return objectstore.UploadCapabilityProjection{}, err
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		return objectstore.UploadCapabilityProjection{}, err
	}
	return objectstore.NewUploadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage,
		objectstore.UploadTarget{URL: signedURL, Headers: headers, ExpiresAt: expiresAt},
	)
}

var (
	_ core.Validatable = GCSServiceAccount{}
	_ core.Validatable = (*GCSCapabilityIssuer)(nil)
	_ core.Validatable = GCSUploadCapabilityRequest{}
)
