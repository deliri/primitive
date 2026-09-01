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

type inspectionCopier struct {
	destination io.Writer
	source      io.Reader
	buffer      []byte
	remaining   int64
	emptyReads  int
	sourceEnded bool
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
	buffer := make([]byte, inspectionBufferBytes)
	copier := inspectionCopier{
		destination: io.MultiWriter(digest, contentIdentity, checksum),
		source:      request.Source,
		buffer:      buffer,
		remaining:   maximum,
	}
	if err := copier.copy(ctx); err != nil {
		return Inspection{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource, err)
	}
	over, err := inspectionSourceExceeds(request.Source, copier.sourceEnded)
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

func (c *inspectionCopier) copy(ctx context.Context) error {
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		done, err := c.copyChunk()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func (c *inspectionCopier) copyChunk() (bool, error) {
	if c.remaining == 0 {
		return true, nil
	}
	destination := c.buffer
	if int64(len(destination)) > c.remaining {
		destination = destination[:c.remaining]
	}
	read, readErr := c.source.Read(destination)
	if read < 0 || read > len(destination) {
		return false, core.ErrObjectStoreSource
	}
	if err := c.acceptChunk(destination[:read], readErr); err != nil {
		return false, err
	}
	return c.chunkOutcome(readErr)
}

func (c *inspectionCopier) acceptChunk(data []byte, readErr error) error {
	if len(data) == 0 {
		if readErr != nil {
			return nil
		}
		c.emptyReads++
		if c.emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
			return io.ErrNoProgress
		}
		return nil
	}
	c.emptyReads = 0
	c.remaining -= int64(len(data))
	_, err := c.destination.Write(data)
	return err
}

func (c *inspectionCopier) chunkOutcome(readErr error) (bool, error) {
	if errors.Is(readErr, io.EOF) {
		c.sourceEnded = true
		return true, nil
	}
	if c.remaining == 0 {
		return true, readErr
	}
	return false, readErr
}

func inspectionSourceExceeds(source io.Reader, sourceEnded bool) (bool, error) {
	if sourceEnded {
		return false, nil
	}
	remaining, proven, err := exactSourceRemaining(source)
	if err != nil {
		return false, err
	}
	if !proven {
		return false, io.ErrNoProgress
	}
	return remaining != 0, nil
}

var (
	_ core.Validatable = InspectionRequest{}
	_ core.Validatable = Inspection{}
)
