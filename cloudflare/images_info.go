package cloudflare

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// ImageOriginalInfo is metadata reported by the image service. It is not
// a digest proof. FileSize describes the original, never the negotiated image.
type ImageOriginalInfo struct {
	core.ImageDimensions
	Format   core.HTTPMediaType `json:"format"`
	FileSize core.ByteLength    `json:"file_size"`
}

func (i ImageOriginalInfo) Validate() error {
	if err := errors.Join(i.ImageDimensions.Validate(), i.Format.Validate(), i.FileSize.Validate()); err != nil {
		return responseError(err)
	}
	if i.FileSize.Uint64() == 0 || !strings.HasPrefix(i.Format.String(), core.CloudflareImageMediaTypePrefix) {
		return core.ErrCloudflareResponse
	}
	return nil
}

// ImageInfo reports the actual resized dimensions and original metadata.
// It does not claim a negotiated encoding or its byte length.
type ImageInfo struct {
	core.ImageDimensions
	Original ImageOriginalInfo `json:"original"`
}

func (i ImageInfo) Validate() error {
	if err := i.Original.Validate(); err != nil {
		return err
	}
	if err := i.ImageDimensions.Validate(); err != nil {
		return responseError(err)
	}
	return nil
}

// ReadImageInfo decodes the provider's closed, fixed-shape format=json
// response directly from the reader. Whitespace can span any number of
// windows; there is no whole-document buffer or invented document quota.
func ReadImageInfo(source io.Reader) (ImageInfo, error) {
	if core.ReaderIsNil(source) {
		return ImageInfo{}, core.ErrCloudflareResponse
	}
	var result ImageInfo
	if err := json.UnmarshalRead(source, &result, json.RejectUnknownMembers(true)); err != nil {
		return ImageInfo{}, responseError(err)
	}
	if err := result.Validate(); err != nil {
		return ImageInfo{}, err
	}
	return result, nil
}

type imageInfoDecoded struct {
	value ImageInfo
	err   error
}

// InspectResize performs one credential-free metadata GET through Exchange.
// The decoder and network stream share an unbuffered pipe: either failure
// closes its peer, and the caller joins the decoder before returning. Image
// pixels are neither transferred nor materialized by this operation.
func (c ImagesClient) InspectResize(ctx context.Context, request ImageResizeRequest, policy exchange.StreamPolicy) (ImageInfo, error) {
	if err := errors.Join(c.Validate(), request.Validate(), validatePolicy(policy)); err != nil {
		return ImageInfo{}, err
	}
	options, err := request.options(core.CloudflareImageFormatJSON)
	if err != nil {
		return ImageInfo{}, err
	}
	target, err := request.Source.address(options + "," + core.CloudflareImageAnimationOff)
	if err != nil {
		return ImageInfo{}, err
	}
	reader, writer := io.Pipe()
	finished := make(chan imageInfoDecoded, 1)
	go func() {
		value, decodeErr := ReadImageInfo(reader)
		closeErr := reader.CloseWithError(decodeErr)
		finished <- imageInfoDecoded{value: value, err: errors.Join(decodeErr, closeErr)}
	}()
	_, transferErr := exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.client, Policy: policy,
		Request: exchange.DownloadRequest{Target: target, Destination: writer,
			Semantics:      exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: core.HTTPMediaTypeJSON()}})
	closeErr := writer.CloseWithError(transferErr)
	decoded := <-finished
	if err := errors.Join(transferErr, closeErr, decoded.err); err != nil {
		return ImageInfo{}, err
	}
	return decoded.value, nil
}

func (ImageOriginalInfo) cloudflareProtocolFact() {}
func (ImageInfo) cloudflareProtocolFact()         {}
func (imageInfoDecoded) cloudflareInternalFlow()  {}
