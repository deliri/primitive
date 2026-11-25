package gcsobjects

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

// GCSDownloadCapabilityRequest is one exact whole-object retrieval capability
// issuance intent. It carries provider facts only; display, attachment, user,
// and workflow policy remain downstream.
type GCSDownloadCapabilityRequest struct {
	Bucket         GCSBucket
	Name           GCSObjectName
	ServiceAccount GCSServiceAccount
	Lifetime       temporal.Duration
}

// Validate closes every fact before a provider signing request begins.
func (r GCSDownloadCapabilityRequest) Validate() error {
	for _, err := range []error{
		r.Bucket.Validate(),
		r.Name.Validate(),
		r.ServiceAccount.Validate(),
		validateGCSCapabilityLifetime(r.Lifetime),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

// gcsGenerationQuery is the XML API GET Object parameter that selects one
// exact object generation. V4 signing places it in the canonical query.
const gcsGenerationQuery = "generation"

// GCSGenerationDownloadCapabilityRequest is one exact whole-object retrieval
// capability issuance intent bound to one provider-issued generation.
type GCSGenerationDownloadCapabilityRequest struct {
	Download   GCSDownloadCapabilityRequest
	Generation GCSGeneration
}

// Validate closes every fact before a provider signing request begins.
func (r GCSGenerationDownloadCapabilityRequest) Validate() error {
	if err := r.Download.Validate(); err != nil {
		return err
	}
	if err := r.Generation.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// IssueGCSGenerationDownloadCapability binds one exact GCS object generation
// into an official V4 signed GET URL whose signed canonical query carries that
// generation, and returns only Objectstore's opaque receive-only projection.
func IssueGCSGenerationDownloadCapability(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
	request GCSGenerationDownloadCapabilityRequest,
) (objectstore.DownloadCapabilityProjection, error) {
	if err := request.Validate(); err != nil {
		return objectstore.DownloadCapabilityProjection{}, err
	}
	generation, err := request.Generation.Int64()
	if err != nil {
		return objectstore.DownloadCapabilityProjection{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return issueGCSDownloadCapability(ctx, issuer, request.Download, url.Values{
		gcsGenerationQuery: {strconv.FormatInt(generation, 10)},
	})
}

// IssueGCSDownloadCapability binds one exact GCS object into an official V4
// signed GET URL and returns only Objectstore's opaque receive-only projection.
func IssueGCSDownloadCapability(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
	request GCSDownloadCapabilityRequest,
) (objectstore.DownloadCapabilityProjection, error) {
	return issueGCSDownloadCapability(ctx, issuer, request, nil)
}

func issueGCSDownloadCapability(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
	request GCSDownloadCapabilityRequest,
	query url.Values,
) (objectstore.DownloadCapabilityProjection, error) {
	if err := request.Validate(); err != nil {
		return objectstore.DownloadCapabilityProjection{}, err
	}
	if err := validateGCSCapabilityIssuer(ctx, issuer); err != nil {
		return objectstore.DownloadCapabilityProjection{}, err
	}
	expiresAt, err := gcsCapabilityExpiry(request.Lifetime)
	if err != nil {
		return objectstore.DownloadCapabilityProjection{}, err
	}
	rawURL, err := issueGCSDownloadURL(ctx, issuer, request, query, expiresAt)
	if err != nil {
		return objectstore.DownloadCapabilityProjection{}, err
	}
	return projectGCSDownloadCapability(rawURL, expiresAt)
}

func issueGCSDownloadURL(
	ctx context.Context,
	issuer *GCSCapabilityIssuer,
	request GCSDownloadCapabilityRequest,
	query url.Values,
	expiresAt temporal.Instant,
) (string, error) {
	if err := errors.Join(request.Validate(), expiresAt.Validate()); err != nil {
		return "", errors.Join(core.ErrObjectStoreContract, err)
	}
	expiration, err := expiresAt.Time()
	if err != nil {
		return "", errors.Join(core.ErrObjectStoreContract, err)
	}
	spec, err := objectstore.Spec(objectstore.ProviderGoogleCloudStorage)
	if err != nil {
		return "", err
	}
	rawURL, err := storage.SignedURL(request.Bucket.String(), request.Name.String(), &storage.SignedURLOptions{
		GoogleAccessID: request.ServiceAccount.String(),
		SignBytes: func(payload []byte) ([]byte, error) {
			return signGCSCapabilityBytes(ctx, issuer, request.ServiceAccount, payload)
		},
		Method:          spec.DownloadMethod.String(),
		Expires:         expiration,
		Scheme:          storage.SigningSchemeV4,
		QueryParameters: query,
	})
	if err != nil {
		return "", errors.Join(core.ErrObjectStoreDestination, err)
	}
	return rawURL, nil
}

func projectGCSDownloadCapability(
	rawURL string,
	expiresAt temporal.Instant,
) (objectstore.DownloadCapabilityProjection, error) {
	signedURL, err := objectstore.ParseSignedURL(rawURL)
	if err != nil {
		return objectstore.DownloadCapabilityProjection{}, err
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		return objectstore.DownloadCapabilityProjection{}, err
	}
	return objectstore.NewDownloadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage,
		objectstore.DownloadTarget{URL: signedURL, Headers: headers, ExpiresAt: expiresAt},
	)
}

var (
	_ core.Validatable = GCSDownloadCapabilityRequest{}
	_ core.Validatable = GCSGenerationDownloadCapabilityRequest{}
)
