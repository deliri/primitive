package cloudflare

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// R2MultipartLayout describes equal non-final parts without allocating a part
// list. PartBytes is a transport extent, not a required in-memory buffer.
type R2MultipartLayout struct {
	Bytes     core.ByteLength
	PartBytes core.ByteLength
	Parts     uint16
}

func (l R2MultipartLayout) Validate() error {
	if err := errors.Join(l.Bytes.Validate(), l.PartBytes.Validate()); err != nil {
		return contractError(err)
	}
	total, part := l.Bytes.Uint64(), l.PartBytes.Uint64()
	if total == 0 || total > core.CloudflareR2MultipartMaximumObjectBytes ||
		part < core.CloudflareR2MultipartMinimumPartBytes || part > core.CloudflareR2MultipartMaximumPartBytes ||
		l.Parts == 0 || l.Parts > core.CloudflareR2MultipartMaximumParts {
		return core.ErrCloudflareBinding
	}
	if (total-1)/part+1 != uint64(l.Parts) {
		return core.ErrCloudflareBinding
	}
	return nil
}

// PlanR2Multipart retains the caller's preferred extent unless more than the
// documented part count would be required. Only integer metadata is processed.
func PlanR2Multipart(length, preferred core.ByteLength) (R2MultipartLayout, error) {
	if err := errors.Join(length.Validate(), preferred.Validate()); err != nil {
		return R2MultipartLayout{}, contractError(err)
	}
	total, part := length.Uint64(), preferred.Uint64()
	if total == 0 || total > core.CloudflareR2MultipartMaximumObjectBytes ||
		part < core.CloudflareR2MultipartMinimumPartBytes || part > core.CloudflareR2MultipartMaximumPartBytes {
		return R2MultipartLayout{}, core.ErrCloudflareBinding
	}
	part = max(part, (total-1)/core.CloudflareR2MultipartMaximumParts+1)
	width, err := core.NewByteLength(part)
	if err != nil {
		return R2MultipartLayout{}, contractError(err)
	}
	layout := R2MultipartLayout{Bytes: length, PartBytes: width, Parts: uint16((total-1)/part + 1)}
	if err := layout.Validate(); err != nil {
		return R2MultipartLayout{}, err
	}
	return layout, nil
}

// PartExtent calculates only the requested range; callers never need a list
// containing every range before starting a transfer.
func (l R2MultipartLayout) PartExtent(number uint16) (offset uint64, length core.ByteLength, err error) {
	if err := l.Validate(); err != nil {
		return 0, core.ByteLength{}, err
	}
	if number == 0 || number > l.Parts {
		return 0, core.ByteLength{}, core.ErrCloudflareBinding
	}
	offset = uint64(number-1) * l.PartBytes.Uint64()
	length, err = core.NewByteLength(min(l.PartBytes.Uint64(), l.Bytes.Uint64()-offset))
	return offset, length, err
}

func (R2MultipartLayout) cloudflareProtocolFact() {}
