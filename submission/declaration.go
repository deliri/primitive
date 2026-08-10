package submission

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

// Declaration is the exact immutable object an installation asks the
// authority to accept. It carries integrity and media type only; customer
// names, paths, source, output, and other product data have no protocol slot.
type Declaration struct {
	ContentType core.HTTPMediaType `json:"content_type"`
	Extent      core.ByteLength    `json:"extent_bytes"`
	SHA256      core.SHA256Digest  `json:"sha256"`
	CRC32C      core.CRC32C        `json:"crc32c"`
}

// Validate closes the media type and delegates the integrity rule to
// Objectstore, which owns the transfer-side interpretation.
func (d Declaration) Validate() error {
	if err := d.ContentType.Validate(); err != nil {
		return contractError(err)
	}
	if d.Extent.Uint64() == 0 {
		return contractError(errors.New("submission object is empty"))
	}
	if err := d.Integrity().Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// Integrity returns the exact Objectstore transfer declaration.
func (d Declaration) Integrity() objectstore.Integrity {
	return objectstore.Integrity{Length: d.Extent, SHA256: d.SHA256, CRC32C: d.CRC32C}
}

var _ core.Validatable = Declaration{}
