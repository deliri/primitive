package cloudflare

import (
	"errors"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// ImageDeliveryAccount is the public Images account hash. It is distinct from
// the hexadecimal AccountID used by the credentialed management API. Zero is
// invalid. This SDK admits URL-safe alphanumeric, hyphen and underscore tokens.
type ImageDeliveryAccount struct{ value string }

func ParseImageDeliveryAccount(value string) (ImageDeliveryAccount, error) {
	if !imageDeliveryToken(value) {
		return ImageDeliveryAccount{}, core.ErrCloudflareBinding
	}
	return ImageDeliveryAccount{value: value}, nil
}

func (a ImageDeliveryAccount) Validate() error {
	_, err := ParseImageDeliveryAccount(a.value)
	return err
}
func (a ImageDeliveryAccount) String() string { return a.value }

// ImageVariantName names one predefined variant. It does not encode transform
// instructions. Zero is invalid; the same URL-safe token grammar applies.
type ImageVariantName struct{ value string }

func ParseImageVariantName(value string) (ImageVariantName, error) {
	if !imageDeliveryToken(value) || len(value) > core.CloudflareImageVariantMaximumCharacters {
		return ImageVariantName{}, core.ErrCloudflareBinding
	}
	return ImageVariantName{value: value}, nil
}

func (v ImageVariantName) Validate() error { _, err := ParseImageVariantName(v.value); return err }
func (v ImageVariantName) String() string  { return v.value }

func imageDeliveryToken(value string) bool {
	if len(value) == 0 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// ImageDeliveryRequest binds a predefined variant to a caller-owned HTTPS
// origin. Address performs only URL projection; it does not claim the domain,
// image or variant exists. The caller owns public access policy and layout.
type ImageDeliveryRequest struct {
	Origin  core.HTTPEndpoint
	Account ImageDeliveryAccount
	Image   ImageID
	Variant ImageVariantName
}

func (r ImageDeliveryRequest) Validate() error {
	source := ImageDeliverySource{Origin: r.Origin, Account: r.Account, Image: r.Image}
	if err := errors.Join(source.Validate(), r.Variant.Validate()); err != nil {
		return errors.Join(core.ErrCloudflareBinding, err)
	}
	return nil
}

func validateImageDeliveryPath(value string) error {
	if strings.ContainsAny(value, "\\\x00\r\n\t") {
		return core.ErrCloudflareBinding
	}
	for part := range strings.SplitSeq(value, "/") {
		if part == "" || part == "." || part == ".." {
			return core.ErrCloudflareBinding
		}
	}
	return nil
}

func (r ImageDeliveryRequest) Address() (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return (ImageDeliverySource{Origin: r.Origin, Account: r.Account, Image: r.Image}).address(r.Variant.value)
}

func (s ImageDeliverySource) address(options string) (core.HTTPEndpoint, error) {
	if err := s.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	u := s.Origin.HTTPURL()
	var path strings.Builder
	path.WriteString(core.CloudflareImagesCustomDeliveryPath)
	path.WriteString(s.Account.value)
	for part := range strings.SplitSeq(s.Image.value, "/") {
		path.WriteByte('/')
		path.WriteString(url.PathEscape(part))
	}
	path.WriteByte('/')
	path.WriteString(options)
	address, err := core.ParseHTTPEndpoint(u.Scheme + "://" + u.Host + path.String())
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrCloudflareBinding, err)
	}
	return address, nil
}

// PublicAddress projects an observed public variant onto the requested custom
// origin only after binding account, image and variant. It does not fetch bytes
// or prove domain ownership. Draft or private metadata cannot issue an unsigned
// address, and a foreign provider variant is never silently rewritten.
func (d ImageDetails) PublicAddress(r ImageDeliveryRequest) (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	if err := d.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	if d.ID != r.Image {
		return core.HTTPEndpoint{}, core.ErrCloudflareBinding
	}
	if d.Draft || d.RequireSignedURLs || len(d.Variants) == 0 {
		return core.HTTPEndpoint{}, core.ErrCloudflareResponse
	}
	wantPath := "/" + r.Account.value + "/" + r.Image.value + "/" + r.Variant.value
	for _, variant := range d.Variants {
		if variant.HTTPURL().Path == wantPath {
			return r.Address()
		}
	}
	return core.HTTPEndpoint{}, core.ErrCloudflareBinding
}

func (ImageDeliveryAccount) cloudflareCapabilityWrapper() {}
func (ImageVariantName) cloudflareCapabilityWrapper()     {}
func (ImageDeliveryRequest) cloudflareProtocolFact()      {}
