package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type StreamServer struct{ api apiServer }

func NewStreamServer(client exchange.Client, options ServerOptions) (StreamServer, error) {
	server, err := newAPIServer(client, options)
	return StreamServer{api: server}, err
}
func (s StreamServer) Validate() error { return s.api.Validate() }
func (s *StreamServer) Close() error {
	if s == nil {
		return core.ErrCloudflareContract
	}
	return s.api.options.Token.Close()
}

// StreamDurationSeconds is Cloudflare's whole-second video reservation unit.
// It is distinct from nanoseconds, timestamps, and an observed media duration.
// https://developers.cloudflare.com/api/resources/stream/subresources/direct_upload/methods/create/
type StreamDurationSeconds uint32

// StreamDurationFromTemporal refuses fractional seconds rather than truncating
// the caller's reservation. Cloudflare accepts only 1..36000 whole seconds.
func StreamDurationFromTemporal(duration temporal.Duration) (StreamDurationSeconds, error) {
	nanos, err := core.CheckedUint64FromInt64(duration.Nanoseconds())
	if err != nil || duration.Validate() != nil {
		return 0, contractError(err)
	}
	seconds := nanos / temporal.NanosecondsPerSecond
	if nanos%temporal.NanosecondsPerSecond != 0 || seconds == 0 || seconds > core.CloudflareStreamDurationMaximumSeconds {
		return 0, core.ErrCloudflareContract
	}
	return StreamDurationSeconds(seconds), nil
}

func (d StreamDurationSeconds) Validate() error {
	if d == 0 || d > core.CloudflareStreamDurationMaximumSeconds {
		return core.ErrCloudflareContract
	}
	return nil
}
func (d StreamDurationSeconds) Duration() (temporal.Duration, error) {
	if err := d.Validate(); err != nil {
		return temporal.Duration{}, err
	}
	return temporal.DurationFromSeconds(uint64(d))
}

// StreamDirectUploadRequest reserves capacity, not completed storage. Product
// accounting must later use the provider's actual processed duration.
type StreamDirectUploadRequest struct {
	Creator                string
	Expiry                 temporal.Instant
	MaximumDurationSeconds StreamDurationSeconds
	RequireSignedURLs      bool
}

func (r StreamDirectUploadRequest) Validate() error {
	if err := r.MaximumDurationSeconds.Validate(); err != nil {
		return err
	}
	if !utf8.ValidString(r.Creator) || utf8.RuneCountInString(r.Creator) > core.CloudflareStreamCreatorMaximumCharacters {
		return core.ErrCloudflareContract
	}
	if r.Expiry.IsSet() {
		return r.Expiry.Validate()
	}
	return nil
}

type streamDirectUploadRequestWire struct {
	Creator                string                `json:"creator,omitempty"`
	Expiry                 string                `json:"expiry,omitempty"`
	MaximumDurationSeconds StreamDurationSeconds `json:"maxDurationSeconds"`
	RequireSignedURLs      bool                  `json:"requireSignedURLs"`
}
type streamDirectUploadWire struct {
	ScheduledDeletion *string              `json:"scheduledDeletion,omitempty"`
	Watermark         *streamWatermarkWire `json:"watermark,omitempty"`
	UID               string               `json:"uid"`
	UploadURL         string               `json:"uploadURL"`
}

func (r streamDirectUploadWire) Validate() error {
	if r.Watermark != nil {
		if err := r.Watermark.Validate(); err != nil {
			return err
		}
	}
	if r.ScheduledDeletion != nil {
		if _, err := temporal.ParseRFC3339(*r.ScheduledDeletion); err != nil {
			return responseError(err)
		}
	}
	_, idErr := ParseStreamVideoID(r.UID)
	_, urlErr := uploadEndpoint(r.UploadURL, core.CloudflareStreamUploadHost)
	return errors.Join(idErr, urlErr)
}

type StreamUpload struct {
	ID       StreamVideoID
	endpoint core.HTTPEndpoint
}

func (u StreamUpload) Validate() error {
	if err := u.ID.Validate(); err != nil {
		return err
	}
	_, err := uploadEndpoint(u.endpoint.String(), core.CloudflareStreamUploadHost)
	return err
}
func (u StreamUpload) Endpoint() (core.HTTPEndpoint, error) { return u.endpoint, u.Validate() }
func ParseStreamUpload(id StreamVideoID, endpoint string) (StreamUpload, error) {
	u, err := uploadEndpoint(endpoint, core.CloudflareStreamUploadHost)
	if err != nil {
		return StreamUpload{}, err
	}
	result := StreamUpload{ID: id, endpoint: u}
	if err := result.Validate(); err != nil {
		return StreamUpload{}, err
	}
	return result, nil
}

func (s StreamServer) CreateDirectUpload(ctx context.Context, request StreamDirectUploadRequest, policy exchange.StreamPolicy) (StreamUpload, error) {
	if err := request.Validate(); err != nil {
		return StreamUpload{}, err
	}
	wireRequest := streamDirectUploadRequestWire{Creator: request.Creator, MaximumDurationSeconds: request.MaximumDurationSeconds, RequireSignedURLs: request.RequireSignedURLs}
	if request.Expiry.IsSet() {
		text, err := request.Expiry.RFC3339Nano()
		if err != nil {
			return StreamUpload{}, err
		}
		wireRequest.Expiry = text
	}
	body, err := core.MarshalCanonicalJSONDocument(wireRequest)
	if err != nil {
		return StreamUpload{}, err
	}
	wire, err := executeAPI[streamDirectUploadWire](ctx, s.api, apiIntent{suffix: "/stream/direct_upload", method: exchange.MethodPost, source: bytes.NewReader(body), media: core.HTTPMediaTypeJSON()}, policy)
	if err != nil {
		return StreamUpload{}, err
	}
	id, err := ParseStreamVideoID(wire.UID)
	if err != nil {
		return StreamUpload{}, err
	}
	return ParseStreamUpload(id, wire.UploadURL)
}

type StreamClient struct{ client exchange.Client }

func NewStreamClient(client exchange.Client) (StreamClient, error) {
	if err := client.Validate(); err != nil {
		return StreamClient{}, contractError(err)
	}
	return StreamClient{client: client}, nil
}
func (c StreamClient) Validate() error { return c.client.Validate() }

// UploadBasic executes one multipart POST. It deliberately does not pretend to
// resume; Cloudflare requires tus for larger or resumable uploads.
func (c StreamClient) UploadBasic(ctx context.Context, authority StreamUpload, source MediaUpload, policy exchange.StreamPolicy) (exchange.StreamRoundTripResponse, error) {
	if err := errors.Join(c.Validate(), authority.Validate(), source.validateMaximum(core.CloudflareStreamBasicUploadMaximumBytes)); err != nil {
		return exchange.StreamRoundTripResponse{}, err
	}
	return source.transfer(ctx, c.client, authority.endpoint, policy)
}

func (StreamUpload) Format(state fmt.State, _ rune) {
	// fmt.Formatter cannot return the destination error; it owns that failure.
	_, _ = io.WriteString(state, core.RedactedValueText)
}
