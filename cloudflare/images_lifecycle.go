package cloudflare

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"errors"
	"net/url"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// ImageDetails is provider metadata, not a checksum assertion about original
// bytes. A draft exists before upload; Ready distinguishes that state.
// https://developers.cloudflare.com/images/storage/upload-images/direct-creator-upload/
type ImageDetails struct {
	ID                ImageID
	Creator           string
	Filename          string
	Uploaded          temporal.Instant
	Variants          []core.HTTPEndpoint
	Draft             bool
	RequireSignedURLs bool
}

func (d ImageDetails) Validate() error {
	if err := d.ID.Validate(); err != nil {
		return err
	}
	if !utf8.ValidString(d.Creator) || utf8.RuneCountInString(d.Creator) > core.CloudflareImageCreatorMaximumCharacters || !utf8.ValidString(d.Filename) || len(d.Filename) > core.CloudflareMultipartFilenameMaximumBytes {
		return core.ErrCloudflareResponse
	}
	if err := d.Uploaded.Validate(); err != nil {
		return responseError(err)
	}
	for _, variant := range d.Variants {
		if err := validateImageVariant(variant, d.ID); err != nil {
			return err
		}
	}
	return nil
}

func (d ImageDetails) Ready() bool { return d.Validate() == nil && !d.Draft && len(d.Variants) != 0 }

type imageDetailsWire struct {
	ID                string         `json:"id"`
	Creator           string         `json:"creator,omitempty"`
	Filename          string         `json:"filename,omitempty"`
	Uploaded          string         `json:"uploaded"`
	Variants          []string       `json:"variants"`
	Draft             bool           `json:"draft,omitempty"`
	RequireSignedURLs bool           `json:"requireSignedURLs"`
	Meta              jsontext.Value `json:"meta,omitempty"`
	Metadata          jsontext.Value `json:"metadata,omitempty"`
}

func (w imageDetailsWire) Validate() error { _, err := w.details(); return err }
func (w imageDetailsWire) details() (ImageDetails, error) {
	id, err := ParseImageID(w.ID)
	if err != nil {
		return ImageDetails{}, responseError(err)
	}
	stamp, err := temporal.ParseRFC3339(w.Uploaded)
	if err != nil {
		return ImageDetails{}, responseError(err)
	}
	d := ImageDetails{ID: id, Creator: w.Creator, Filename: w.Filename, Uploaded: stamp, Draft: w.Draft, RequireSignedURLs: w.RequireSignedURLs, Variants: make([]core.HTTPEndpoint, len(w.Variants))}
	for i, raw := range w.Variants {
		d.Variants[i], err = core.ParseHTTPEndpoint(raw)
		if err != nil {
			return ImageDetails{}, responseError(err)
		}
	}
	if err := d.Validate(); err != nil {
		return ImageDetails{}, err
	}
	return d, nil
}

func validateImageVariant(endpoint core.HTTPEndpoint, id ImageID) error {
	if err := endpoint.Validate(); err != nil {
		return responseError(err)
	}
	u := endpoint.HTTPURL()
	if u.Scheme != core.SchemeHTTPS || u.Host != core.CloudflareImagesDeliveryHost || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return core.ErrCloudflareBinding
	}
	// One account delivery hash, the exact custom image ID (possibly containing
	// slashes), then one variant. A foreign ID is never silently admitted.
	path := u.Path
	first := 1
	for first < len(path) && path[first] != '/' {
		first++
	}
	if first <= 1 || first == len(path) {
		return core.ErrCloudflareBinding
	}
	rest := path[first+1:]
	prefix := id.value + "/"
	if len(rest) <= len(prefix) || rest[:len(prefix)] != prefix {
		return core.ErrCloudflareBinding
	}
	for _, r := range rest[len(prefix):] {
		if r == '/' {
			return core.ErrCloudflareBinding
		}
	}
	return nil
}

// Details performs exactly one authenticated metadata request. It never fetches
// image bytes and never treats draft creation as upload completion.
func (s ImagesServer) Details(ctx context.Context, id ImageID, policy exchange.StreamPolicy) (ImageDetails, error) {
	if err := id.Validate(); err != nil {
		return ImageDetails{}, err
	}
	wire, err := executeAPI[imageDetailsWire](ctx, s.api, apiIntent{suffix: core.CloudflareImagesV1Path + url.PathEscape(id.value), method: exchange.MethodGet}, policy)
	if err != nil {
		return ImageDetails{}, err
	}
	result, err := wire.details()
	if err != nil || result.ID != id {
		return ImageDetails{}, errors.Join(core.ErrCloudflareBinding, err)
	}
	return result, nil
}

// The API declares deletion's result opaque. Preserve its presence and JSON
// validity while the envelope owns explicit success and refusal. A missing
// result must not become successful deletion through a zero-value record.
type imageDeleteWire struct{ body jsontext.Value }

func (w imageDeleteWire) Validate() error {
	if len(w.body) == 0 || !w.body.IsValid() {
		return core.ErrCloudflareResponse
	}
	return nil
}

func (w *imageDeleteWire) UnmarshalJSON(data []byte) error {
	if w == nil || !jsontext.Value(data).IsValid() {
		return core.ErrCloudflareResponse
	}
	w.body = bytes.Clone(data)
	return nil
}

func (w imageDeleteWire) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return bytes.Clone(w.body), nil
}

func (s ImagesServer) Delete(ctx context.Context, id ImageID, policy exchange.StreamPolicy) error {
	if err := id.Validate(); err != nil {
		return err
	}
	_, err := executeAPI[imageDeleteWire](ctx, s.api, apiIntent{suffix: core.CloudflareImagesV1Path + url.PathEscape(id.value), method: exchange.MethodDelete}, policy)
	return err
}

func (ImageDetails) cloudflareProtocolFact()     {}
func (imageDetailsWire) cloudflareProtocolFact() {}
func (imageDeleteWire) cloudflareProtocolFact()  {}

var _ core.ValidatedJSONMarshaler = imageDeleteWire{}
