package objectstore

import (
	"context"
	"errors"
	"hash/crc32"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/zeebo/blake3"
)

const inspectionBufferBytes = 32 << 10

// InspectionRequest names one bounded source inspection. Inspect consumes the
// source exactly once; callers reopen durable sources for a later transfer.
type InspectionRequest struct {
	Source       io.Reader
	MaximumBytes core.ByteCount
}

func (r InspectionRequest) Validate() error {
	if r.Source == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	if _, err := r.MaximumBytes.Int64(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize, err)
	}
	return nil
}

// Inspection is the exact declaration produced from one complete stream.
type Inspection struct {
	Integrity Integrity
	BLAKE3    BLAKE3Digest
}

func (i Inspection) Validate() error {
	if err := i.Integrity.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := i.BLAKE3.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if i.Integrity.Length.Uint64() == 0 {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
	}
	return nil
}

// Inspect streams one source through SHA-256, CRC32C, and exact byte counting
// in O(1) memory. It refuses empty and over-limit sources and preserves native
// reader errors beneath the typed object-store source identity.
func Inspect(ctx context.Context, request InspectionRequest) (Inspection, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Inspection{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := request.Validate(); err != nil {
		return Inspection{}, err
	}
	maximum, _ := request.MaximumBytes.Int64()
	digest := core.NewDigestWriter()
	contentIdentity := blake3.New()
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	limited := &io.LimitedReader{R: request.Source, N: maximum}
	buffer := make([]byte, inspectionBufferBytes)
	if _, err := io.CopyBuffer(io.MultiWriter(digest, contentIdentity, checksum), limited, buffer); err != nil {
		return Inspection{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource, err)
	}
	over, err := sourceHasAnotherByte(request.Source)
	if err != nil {
		return Inspection{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource, err)
	}
	if over {
		return Inspection{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
	}
	sha256, length, err := digest.Seal()
	if err != nil {
		return Inspection{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	var blake3Sum [BLAKE3DigestBytes]byte
	contentIdentity.Sum(blake3Sum[:0])
	inspection := Inspection{Integrity: Integrity{
		Length: length, SHA256: sha256, CRC32C: core.NewCRC32C(checksum.Sum32()),
	}, BLAKE3: NewBLAKE3Digest(blake3Sum)}
	if err := inspection.Validate(); err != nil {
		return Inspection{}, err
	}
	return inspection, nil
}

func sourceHasAnotherByte(source io.Reader) (bool, error) {
	var probe [1]byte
	read, err := source.Read(probe[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return read != 0, nil
}

var (
	_ core.Validatable = InspectionRequest{}
	_ core.Validatable = Inspection{}
)
