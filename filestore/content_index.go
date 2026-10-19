package filestore

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// ContentIndexEntry binds an opaque content digest to its exact byte extent.
// It carries no path, product identity, or interpretation of the content.
type ContentIndexEntry struct {
	Digest core.SHA256Digest
	Extent core.ByteLength
}

// ContentIndexRecordBytes is one canonical record: the raw SHA-256 bytes,
// followed by the unsigned byte extent in network byte order. A stream is
// zero or more complete records; partial final records are refused.
const ContentIndexRecordBytes = core.SHA256DigestBytes + 8

func (e ContentIndexEntry) Validate() error {
	if err := errors.Join(e.Digest.Validate(), e.Extent.Validate()); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if e.Extent.Uint64() == 0 && e.Digest != core.SHA256Of(nil) {
		return core.ErrFilestoreContract
	}
	return nil
}

// WriteContentIndexEntry writes one complete typed record. The caller owns the
// destination and must discard provisional output after any write failure.
func WriteContentIndexEntry(destination io.Writer, entry ContentIndexEntry) error {
	if core.WriterIsNil(destination) {
		return core.ErrFilestoreContract
	}
	if err := entry.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	digest, err := entry.Digest.Bytes()
	if err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	data := append(appendIndexRecordScratch(destination), digest[:]...)
	data = binary.BigEndian.AppendUint64(data, entry.Extent.Uint64())
	written, err := destination.Write(data)
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

// ReadContentIndexEntry admits one complete record. Clean EOF returns a zero
// entry and io.EOF; every other refusal also returns zero. Source lifetime
// remains with the caller, and total stream length is unrestricted.
func ReadContentIndexEntry(source io.Reader) (ContentIndexEntry, error) {
	if core.ReaderIsNil(source) {
		return ContentIndexEntry{}, core.ErrFilestoreContract
	}
	if data, buffered, err := bufferedIndexRecord(source, ContentIndexRecordBytes); buffered {
		if err != nil {
			return ContentIndexEntry{}, err
		}
		entry, err := decodeContentIndexEntry(data)
		if _, discardErr := source.(*bufio.Reader).Discard(len(data)); discardErr != nil {
			return ContentIndexEntry{}, sourceError(discardErr)
		}
		return entry, err
	}
	var data [ContentIndexRecordBytes]byte
	if err := readIndexRecord(source, data[:]); err != nil {
		return ContentIndexEntry{}, err
	}
	return decodeContentIndexEntry(data[:])
}

func decodeContentIndexEntry(data []byte) (ContentIndexEntry, error) {
	var digest [core.SHA256DigestBytes]byte
	copy(digest[:], data[:core.SHA256DigestBytes])
	extent, err := core.NewByteLength(binary.BigEndian.Uint64(data[core.SHA256DigestBytes:]))
	if err != nil {
		return ContentIndexEntry{}, errors.Join(core.ErrFilestoreContract, err)
	}
	entry := ContentIndexEntry{Digest: core.NewSHA256Digest(digest), Extent: extent}
	if err := entry.Validate(); err != nil {
		return ContentIndexEntry{}, err
	}
	return entry, nil
}
