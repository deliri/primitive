package cloudflare

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// ImageBlobDownloadRequest borrows the destination. Maximum is the caller's
// export policy; ContentType binds the response representation before copying.
type ImageBlobDownloadRequest struct {
	ID          ImageID
	Destination io.Writer
	ContentType core.HTTPMediaType
	Maximum     core.ByteCount
}

func (r ImageBlobDownloadRequest) Validate() error {
	if err := errors.Join(r.ID.Validate(), r.ContentType.Validate(), r.Maximum.Validate()); err != nil {
		return contractError(err)
	}
	base, err := r.ContentType.Base()
	if err != nil || !strings.HasPrefix(base, core.CloudflareImageMediaTypePrefix) || r.Destination == nil {
		return core.ErrCloudflareContract
	}
	return nil
}

// DownloadOriginal executes the authenticated Images blob API once. Cloudflare
// returns the uploaded original for most images and a near-lossless version for
// larger images. This method makes no original-byte checksum assertion.
// https://developers.cloudflare.com/images/storage/manage-images/export-images/
// A failure may leave a bounded prefix in Destination; the caller owns rollback.
func (s ImagesServer) DownloadOriginal(ctx context.Context, request ImageBlobDownloadRequest, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	var zero exchange.StreamResponse
	if err := errors.Join(s.Validate(), request.Validate(), validatePolicy(policy)); err != nil {
		return zero, err
	}
	maximum, err := request.Maximum.Uint64()
	if err != nil {
		return zero, contractError(err)
	}
	target, err := core.ParseHTTPEndpoint(s.api.root.String() + core.CloudflareImagesV1Path + url.PathEscape(request.ID.value) + core.CloudflareImagesBlobSuffix)
	if err != nil {
		return zero, contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: s.api.token.value})
	if err != nil {
		return zero, authenticationError(err)
	}
	destination := imageBlobDestination{writer: request.Destination, remaining: maximum}
	return exchange.Download(exchange.DownloadCall{Context: ctx, Client: s.api.client, Policy: policy,
		Request: exchange.DownloadRequest{Target: target, Destination: &destination,
			Headers:        exchange.Headers{Values: []exchange.Header{authorization}},
			Semantics:      exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: request.ContentType}})
}

type imageBlobDestination struct {
	writer    io.Writer
	remaining uint64
}

func (w *imageBlobDestination) Write(p []byte) (int, error) {
	if uint64(len(p)) > w.remaining {
		return 0, core.ErrCloudflareResponse
	}
	n, err := w.writer.Write(p)
	if n < 0 || n > len(p) {
		return 0, errors.Join(core.ErrCloudflareResponse, io.ErrShortWrite)
	}
	w.remaining -= uint64(n)
	if n != len(p) && err == nil {
		return n, io.ErrShortWrite
	}
	return n, err
}

func (ImageBlobDownloadRequest) cloudflareInternalFlow() {}
func (imageBlobDestination) cloudflareInternalFlow()     {}

var (
	_ cloudflareInternalFlowRole = ImageBlobDownloadRequest{}
	_ cloudflareInternalFlowRole = imageBlobDestination{}
)
