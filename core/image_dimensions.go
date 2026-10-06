package core

// PixelDimension measures physical image pixels. Zero denotes absence only
// when a containing intent explicitly permits an unconstrained axis.
type PixelDimension uint64

// NewPixelDimension admits a nonzero physical pixel measurement without a
// product or provider size ceiling.
func NewPixelDimension(value uint64) (PixelDimension, error) {
	dimension := PixelDimension(value)
	if err := dimension.Validate(); err != nil {
		return 0, err
	}
	return dimension, nil
}

// Validate refuses absence; native representability is the only upper bound.
func (d PixelDimension) Validate() error {
	if d == 0 {
		return ErrPrimitiveContract
	}
	return nil
}

// ImageDimensions describes an observed raster size, independent of provider,
// layout, encoding, byte length and display density. Its zero value is invalid.
type ImageDimensions struct {
	// Width is the observed horizontal extent in physical pixels.
	Width PixelDimension `json:"width"`
	// Height is the observed vertical extent in physical pixels.
	Height PixelDimension `json:"height"`
}

// Validate requires both measured axes to be present.
func (d ImageDimensions) Validate() error {
	if err := d.Width.Validate(); err != nil {
		return err
	}
	return d.Height.Validate()
}
