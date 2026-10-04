package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"unicode/utf8"
	"uuid"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// ImagesServer owns authenticated account operations; ImagesClient consumes
// the resulting single-use upload authority without possessing the API token.
type ImagesServer struct{ api apiServer }

func NewImagesServer(client exchange.Client, options ServerOptions) (ImagesServer, error) {
	server, err := newAPIServer(client, options)
	return ImagesServer{api: server}, err
}
func (s ImagesServer) Validate() error { return s.api.Validate() }
func (s *ImagesServer) Close() error {
	if s == nil {
		return core.ErrCloudflareContract
	}
	return s.api.options.Token.Close()
}

// ImageDirectUploadRequest creates a draft. CustomID and Expiry are optional;
// zero Expiry asks Cloudflare for its documented default, not a local clock.
type ImageDirectUploadRequest struct {
	CustomID          ImageID
	Creator           string
	ObservedAt        temporal.Instant
	Expiry            temporal.Instant
	RequireSignedURLs bool
}

func (r ImageDirectUploadRequest) Validate() error {
	if !utf8.ValidString(r.Creator) || utf8.RuneCountInString(r.Creator) > core.CloudflareImageCreatorMaximumCharacters {
		return core.ErrCloudflareContract
	}
	if err := r.validateCustomID(); err != nil {
		return err
	}
	return r.validateExpiry()
}

func (r ImageDirectUploadRequest) validateCustomID() error {
	if r.CustomID.value != "" {
		if err := r.CustomID.Validate(); err != nil {
			return err
		}
		// Custom-ID images cannot require private signed delivery URLs.
		// https://developers.cloudflare.com/images/storage/upload-images/upload-custom-path/
		_, uuidErr := uuid.Parse(r.CustomID.value)
		if r.RequireSignedURLs || uuidErr == nil {
			return core.ErrCloudflareBinding
		}
	}
	return nil
}

func (r ImageDirectUploadRequest) validateExpiry() error {
	if !r.Expiry.IsSet() {
		return nil
	}
	elapsed, err := r.Expiry.Since(r.ObservedAt)
	if err != nil {
		return contractError(err)
	}
	minimum, err := temporal.DurationFromSeconds(core.CloudflareImageExpiryMinimumSeconds)
	maximum, maxErr := temporal.DurationFromSeconds(core.CloudflareImageExpiryMaximumSeconds)
	if err != nil || maxErr != nil || elapsed.Nanoseconds() < minimum.Nanoseconds() || elapsed.Nanoseconds() > maximum.Nanoseconds() {
		return core.ErrCloudflareContract
	}
	return nil
}

type imageDirectUploadWire struct {
	ID        string `json:"id"`
	UploadURL string `json:"uploadURL"`
}

func (r imageDirectUploadWire) Validate() error {
	_, idErr := ParseImageID(r.ID)
	_, urlErr := uploadEndpoint(r.UploadURL, core.CloudflareImagesUploadHost)
	return errors.Join(idErr, urlErr)
}

// ImageUpload is an issued draft, not an uploaded or processed image.
type ImageUpload struct {
	ID       ImageID
	endpoint core.HTTPEndpoint
}

func (u ImageUpload) Validate() error {
	if err := u.ID.Validate(); err != nil {
		return err
	}
	_, err := uploadEndpoint(u.endpoint.String(), core.CloudflareImagesUploadHost)
	return err
}
func (u ImageUpload) Endpoint() (core.HTTPEndpoint, error) { return u.endpoint, u.Validate() }

// ParseImageUpload admits a server-provided authority at the client boundary.
func ParseImageUpload(id ImageID, endpoint string) (ImageUpload, error) {
	u, err := uploadEndpoint(endpoint, core.CloudflareImagesUploadHost)
	if err != nil {
		return ImageUpload{}, err
	}
	result := ImageUpload{ID: id, endpoint: u}
	if err := result.Validate(); err != nil {
		return ImageUpload{}, err
	}
	return result, nil
}

func (s ImagesServer) CreateDirectUpload(ctx context.Context, request ImageDirectUploadRequest, policy exchange.StreamPolicy) (ImageUpload, error) {
	if err := request.Validate(); err != nil {
		return ImageUpload{}, err
	}
	body, media, err := encodeImageDirectUpload(request)
	if err != nil {
		return ImageUpload{}, err
	}
	wire, err := executeAPI[imageDirectUploadWire](ctx, s.api, apiIntent{suffix: "/images/v2/direct_upload", method: exchange.MethodPost, source: bytes.NewReader(body), media: media}, policy)
	if err != nil {
		return ImageUpload{}, err
	}
	id, err := ParseImageID(wire.ID)
	if err != nil {
		return ImageUpload{}, responseError(err)
	}
	if request.CustomID.value != "" && request.CustomID != id {
		return ImageUpload{}, core.ErrCloudflareBinding
	}
	return ParseImageUpload(id, wire.UploadURL)
}

func encodeImageDirectUpload(request ImageDirectUploadRequest) ([]byte, core.HTTPMediaType, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("requireSignedURLs", strconv.FormatBool(request.RequireSignedURLs)); err != nil {
		return nil, core.HTTPMediaType{}, err
	}
	if request.CustomID.value != "" {
		if err := form.WriteField("id", request.CustomID.value); err != nil {
			return nil, core.HTTPMediaType{}, err
		}
	}
	if request.Creator != "" {
		if err := form.WriteField("creator", request.Creator); err != nil {
			return nil, core.HTTPMediaType{}, err
		}
	}
	if err := writeImageExpiry(form, request.Expiry); err != nil {
		return nil, core.HTTPMediaType{}, err
	}
	if err := form.Close(); err != nil {
		return nil, core.HTTPMediaType{}, err
	}
	media, err := core.ParseHTTPMediaType(form.FormDataContentType())
	return body.Bytes(), media, err
}

func writeImageExpiry(form *multipart.Writer, expiry temporal.Instant) error {
	if !expiry.IsSet() {
		return nil
	}
	text, err := expiry.RFC3339Nano()
	if err != nil {
		return err
	}
	return form.WriteField("expiry", text)
}

type ImagesClient struct{ client exchange.Client }

func NewImagesClient(client exchange.Client) (ImagesClient, error) {
	if err := client.Validate(); err != nil {
		return ImagesClient{}, contractError(err)
	}
	return ImagesClient{client: client}, nil
}
func (c ImagesClient) Validate() error { return c.client.Validate() }
func (c ImagesClient) Upload(ctx context.Context, authority ImageUpload, source MediaUpload, policy exchange.StreamPolicy) (exchange.StreamRoundTripResponse, error) {
	if err := errors.Join(c.Validate(), authority.Validate(), source.validateMaximum(core.CloudflareImagesUploadMaximumBytes)); err != nil {
		return exchange.StreamRoundTripResponse{}, err
	}
	return source.transfer(ctx, c.client, authority.endpoint, policy)
}

func (ImageUpload) Format(state fmt.State, _ rune) {
	// fmt.Formatter cannot return the destination error; it owns that failure.
	_, _ = io.WriteString(state, core.RedactedValueText)
}
