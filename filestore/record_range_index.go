package filestore

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// RecordRangeIndexBytes is the canonical offset, extent and framing tuple.
const RecordRangeIndexBytes = 17

func WriteRecordRangeIndex(destination io.Writer, record lineio.RecordRange) error {
	if core.WriterIsNil(destination) {
		return core.ErrFilestoreContract
	}
	if err := record.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	var data [RecordRangeIndexBytes]byte
	binary.BigEndian.PutUint64(data[:8], uint64(record.Offset))
	binary.BigEndian.PutUint64(data[8:16], record.Bytes.Uint64())
	data[16] = byte(record.Framing)
	written, err := destination.Write(data[:])
	if written < 0 || written > len(data) {
		return contractError(err)
	}
	if err != nil {
		return destinationError(err)
	}
	if written != len(data) {
		return destinationError(io.ErrShortWrite)
	}
	return nil
}
func ReadRecordRangeIndex(source io.Reader) (lineio.RecordRange, error) {
	if core.ReaderIsNil(source) {
		return lineio.RecordRange{}, core.ErrFilestoreContract
	}
	var data [RecordRangeIndexBytes]byte
	if err := readIndexRecord(source, data[:]); err != nil {
		return lineio.RecordRange{}, err
	}
	offset := binary.BigEndian.Uint64(data[:8])
	if offset > math.MaxInt64 {
		return lineio.RecordRange{}, core.ErrFilestoreContract
	}
	extent, err := core.NewByteLength(binary.BigEndian.Uint64(data[8:16]))
	if err != nil {
		return lineio.RecordRange{}, errors.Join(core.ErrFilestoreContract, err)
	}
	record := lineio.RecordRange{Offset: lineio.SourceByteOffset(offset), Bytes: extent, Framing: lineio.RecordFraming(data[16])}
	if err := record.Validate(); err != nil {
		return lineio.RecordRange{}, errors.Join(core.ErrFilestoreContract, err)
	}
	return record, nil
}
