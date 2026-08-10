package objectstore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// UploadCapabilityRequest binds one received bearer to the exact stream facts
// the caller declared before asking an authority for permission.
type UploadCapabilityRequest struct {
	Source      io.Reader
	Observer    ProgressObserver
	ContentType core.HTTPMediaType
	Capability  UploadCapability
	Integrity   Integrity
	Policy      Policy
}

// Validate closes the received bearer and ordinary upload ingress together.
func (r UploadCapabilityRequest) Validate() error {
	_, _, err := r.execution()
	return err
}

func (r UploadCapabilityRequest) execution() (Provider, UploadRequest, error) {
	if err := r.Capability.Validate(); err != nil {
		return ProviderUnknown, UploadRequest{}, err
	}
	request := UploadRequest{
		Source: r.Source, Target: r.Capability.target,
		ContentType: r.ContentType, Integrity: r.Integrity, Policy: r.Policy,
		Observer: r.Observer,
	}
	if err := request.validateFor(r.Capability.provider); err != nil {
		return ProviderUnknown, UploadRequest{}, err
	}
	return r.Capability.provider, request, nil
}

// DownloadCapabilityRequest binds one received bearer to one exact expected
// object and the caller-owned streaming destination.
type DownloadCapabilityRequest struct {
	Destination io.Writer
	Observer    ProgressObserver
	ContentType core.HTTPMediaType
	Capability  DownloadCapability
	Integrity   Integrity
	Policy      Policy
}

// Validate closes the received bearer and ordinary download ingress together.
func (r DownloadCapabilityRequest) Validate() error {
	_, _, err := r.execution()
	return err
}

func (r DownloadCapabilityRequest) execution() (Provider, DownloadRequest, error) {
	if err := r.Capability.Validate(); err != nil {
		return ProviderUnknown, DownloadRequest{}, err
	}
	request := DownloadRequest{
		Destination: r.Destination, Target: r.Capability.target,
		ContentType: r.ContentType, Integrity: r.Integrity, Policy: r.Policy,
		Observer: r.Observer,
	}
	if err := request.validateFor(r.Capability.provider); err != nil {
		return ProviderUnknown, DownloadRequest{}, err
	}
	return r.Capability.provider, request, nil
}

// Upload streams through the exact provider operation selected by one
// received, validated capability.
func Upload(
	ctx context.Context,
	client Client,
	request UploadCapabilityRequest,
) (Transfer, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Transfer{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	provider, ordinary, err := request.execution()
	if err != nil {
		return Transfer{}, err
	}
	switch provider {
	case ProviderAmazonS3, ProviderGoogleCloudStorage, ProviderCloudflareImages:
		return client.upload(ctx, ordinary, provider)
	case ProviderUnknown, providerLimit:
		return Transfer{}, core.ErrObjectStoreContract
	}
	return Transfer{}, core.ErrObjectStoreContract
}

// Download streams through the exact provider operation selected by one
// received, validated capability.
func Download(
	ctx context.Context,
	client Client,
	request DownloadCapabilityRequest,
) (Transfer, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Transfer{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	provider, ordinary, err := request.execution()
	if err != nil {
		return Transfer{}, err
	}
	switch provider {
	case ProviderAmazonS3, ProviderGoogleCloudStorage:
		return client.download(ctx, ordinary, provider)
	case ProviderUnknown, ProviderCloudflareImages, providerLimit:
		return Transfer{}, core.ErrObjectStoreContract
	}
	return Transfer{}, core.ErrObjectStoreContract
}

var (
	_ core.Validatable = UploadCapabilityRequest{}
	_ core.Validatable = DownloadCapabilityRequest{}
)
