package cloudflare

import (
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// streamWatermarkWire admits the optional documented direct-upload response
// member. It is provider wire data, not application media/accounting state.
// https://developers.cloudflare.com/api/resources/stream/subresources/direct_upload/methods/create/
type streamWatermarkWire struct {
	Height         *float64 `json:"height,omitempty"`
	Width          *float64 `json:"width,omitempty"`
	Size           *float64 `json:"size,omitempty"`
	Opacity        *float64 `json:"opacity,omitempty"`
	Padding        *float64 `json:"padding,omitempty"`
	Scale          *float64 `json:"scale,omitempty"`
	Created        string   `json:"created,omitempty"`
	DownloadedFrom string   `json:"downloadedFrom,omitempty"`
	Name           string   `json:"name,omitempty"`
	Position       string   `json:"position,omitempty"`
	UID            string   `json:"uid,omitempty"`
}

func (w streamWatermarkWire) Validate() error {
	if err := validateWatermarkIdentity(w); err != nil {
		return err
	}
	if err := validateWatermarkNumbers([]*float64{w.Height, w.Width, w.Size}, math.MaxFloat64); err != nil {
		return err
	}
	return validateWatermarkNumbers([]*float64{w.Opacity, w.Padding, w.Scale}, 1)
}
func validateWatermarkNumbers(values []*float64, maximum float64) error {
	for _, value := range values {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > maximum) {
			return core.ErrCloudflareResponse
		}
	}
	return nil
}
func validateWatermarkIdentity(w streamWatermarkWire) error {
	if w.UID != "" {
		if _, err := ParseStreamVideoID(w.UID); err != nil {
			return err
		}
	}
	if w.Created != "" {
		if _, err := temporal.ParseRFC3339(w.Created); err != nil {
			return responseError(err)
		}
	}
	if w.DownloadedFrom != "" {
		if _, err := core.ParseHTTPEndpoint(w.DownloadedFrom); err != nil {
			return responseError(err)
		}
	}
	switch w.Position {
	case "", core.CloudflareWatermarkUpperRight, core.CloudflareWatermarkUpperLeft, core.CloudflareWatermarkLowerRight, core.CloudflareWatermarkLowerLeft, core.CloudflareWatermarkCenter:
		return nil
	default:
		return core.ErrCloudflareResponse
	}
}
