package cloudflare

import (
	"errors"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

type ImageFit uint8

const (
	ImageFitUnknown ImageFit = iota
	ImageFitScaleDown
	ImageFitContain
	ImageFitCover
	ImageFitCrop
	ImageFitPad
)

func (v ImageFit) parameter() (string, error) {
	switch v {
	case ImageFitScaleDown:
		return core.CloudflareImageFitScaleDown, nil
	case ImageFitContain:
		return core.CloudflareImageFitContain, nil
	case ImageFitCover:
		return core.CloudflareImageFitCover, nil
	case ImageFitCrop:
		return core.CloudflareImageFitCrop, nil
	case ImageFitPad:
		return core.CloudflareImageFitPad, nil
	default:
		return "", core.ErrCloudflareContract
	}
}
func (v ImageFit) Validate() error { _, err := v.parameter(); return err }
func (ImageFit) OffWireEnum()      {}

// ImageFormat chooses negotiation or an explicit encoding. The observed
// HTTP response remains authoritative: the provider can refuse or fall back.
type ImageFormat uint8

const (
	ImageFormatUnknown ImageFormat = iota
	ImageFormatAuto
	ImageFormatAVIF
	ImageFormatWebP
)

func (v ImageFormat) parameter() (string, error) {
	switch v {
	case ImageFormatAuto:
		return core.CloudflareImageFormatAuto, nil
	case ImageFormatAVIF:
		return core.CloudflareImageFormatAVIF, nil
	case ImageFormatWebP:
		return core.CloudflareImageFormatWebP, nil
	default:
		return "", core.ErrCloudflareContract
	}
}
func (v ImageFormat) Validate() error { _, err := v.parameter(); return err }
func (ImageFormat) OffWireEnum()      {}

type ImageMetadata uint8

const (
	ImageMetadataUnknown ImageMetadata = iota
	ImageMetadataNone
	ImageMetadataCopyright
	ImageMetadataKeep
)

func (v ImageMetadata) parameter() (string, error) {
	switch v {
	case ImageMetadataNone:
		return core.CloudflareImageMetadataNone, nil
	case ImageMetadataCopyright:
		return core.CloudflareImageMetadataCopyright, nil
	case ImageMetadataKeep:
		return core.CloudflareImageMetadataKeep, nil
	default:
		return "", core.ErrCloudflareContract
	}
}
func (v ImageMetadata) Validate() error { _, err := v.parameter(); return err }
func (ImageMetadata) OffWireEnum()      {}

// ImageDeliverySource is the exact public image and caller-owned origin.
// It does not assert existence, authorization or product meaning.
type ImageDeliverySource struct {
	Origin  core.HTTPEndpoint
	Account ImageDeliveryAccount
	Image   ImageID
}

func (s ImageDeliverySource) Validate() error {
	if err := errors.Join(s.Origin.Validate(), s.Account.Validate(), s.Image.Validate()); err != nil {
		return errors.Join(core.ErrCloudflareBinding, err)
	}
	u := s.Origin.HTTPURL()
	if u.Scheme != core.SchemeHTTPS || u.Port() != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery {
		return core.ErrCloudflareBinding
	}
	return validateImageDeliveryPath(s.Image.value)
}

// ImageResizeRequest carries one native transform. Zero width/height means
// that axis is unconstrained; at least one axis must be specified. No SDK
// dimension ceiling replaces the native provider's admission or refusal.
type ImageResizeRequest struct {
	Source   ImageDeliverySource
	Width    core.PixelDimension
	Height   core.PixelDimension
	Fit      ImageFit
	Format   ImageFormat
	Metadata ImageMetadata
}

func (r ImageResizeRequest) Validate() error {
	if err := errors.Join(r.Source.Validate(), r.Fit.Validate(), r.Format.Validate(), r.Metadata.Validate()); err != nil {
		return contractError(err)
	}
	if r.Width == 0 && r.Height == 0 {
		return core.ErrCloudflareContract
	}
	return nil
}

func (r ImageResizeRequest) options(format string) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	fit, err := r.Fit.parameter()
	if err != nil {
		return "", err
	}
	metadata, err := r.Metadata.parameter()
	if err != nil {
		return "", err
	}
	var options strings.Builder
	if r.Width != 0 {
		options.WriteString(core.CloudflareImageWidthOption)
		options.WriteString(strconv.FormatUint(uint64(r.Width), 10))
		options.WriteByte(',')
	}
	if r.Height != 0 {
		options.WriteString(core.CloudflareImageHeightOption)
		options.WriteString(strconv.FormatUint(uint64(r.Height), 10))
		options.WriteByte(',')
	}
	options.WriteString(core.CloudflareImageFitOption + fit + "," + core.CloudflareImageMetadataOption + metadata + "," + core.CloudflareImageFormatOption + format)
	return options.String(), nil
}

// Address projects the typed transform without claiming it was executed.
func (r ImageResizeRequest) Address() (core.HTTPEndpoint, error) {
	format, err := r.Format.parameter()
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	options, err := r.options(format)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	return r.Source.address(options)
}

// PublicResize binds an observed public image to the requested account and
// image before projecting its native flexible variant. Flexible variants
// must be enabled by the account owner; the SDK does not change that setting.
func (d ImageDetails) PublicResize(r ImageResizeRequest) (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	if err := d.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	if d.ID != r.Source.Image {
		return core.HTTPEndpoint{}, core.ErrCloudflareBinding
	}
	if d.Draft || d.RequireSignedURLs {
		return core.HTTPEndpoint{}, core.ErrCloudflareResponse
	}
	prefix := "/" + r.Source.Account.value + "/" + r.Source.Image.value + "/"
	for _, variant := range d.Variants {
		if strings.HasPrefix(variant.HTTPURL().Path, prefix) {
			return r.Address()
		}
	}
	return core.HTTPEndpoint{}, core.ErrCloudflareBinding
}

func (ImageDeliverySource) cloudflareProtocolFact() {}
func (ImageResizeRequest) cloudflareProtocolFact()  {}
