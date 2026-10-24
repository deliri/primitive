package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Write publishes only the bytes the original native descriptor accepted.
// No reader cursor, callback or destination acknowledgement is awaited.
func (s *AppendSpool) Write(data []byte) (int, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.sealed || s.closing {
		return 0, contractError(fs.ErrClosed)
	}
	if err := scopeCancellationError(s.ctx, contextstate.Validate(s.ctx)); err != nil {
		return 0, err
	}
	if s.terminal != nil {
		return 0, s.terminal
	}
	length, err := core.NewByteLength(uint64(s.extent) + uint64(len(data)))
	if err != nil {
		return 0, sizeError(err)
	}
	if _, err := length.Int64(); err != nil {
		return 0, sizeError(err)
	}
	written, writeErr := s.file.Write(data)
	_, digestErr := s.digest.Write(data[:written])
	s.extent += int64(written)
	if err := errors.Join(writeErr, digestErr); err != nil {
		s.terminal = destinationError(err)
	}
	if written > 0 || s.terminal != nil {
		s.notify()
	}
	return written, s.terminal
}

// Seal ends production exactly once. A producer error aborts without syncing
// or releasing content proof. Successful sealing syncs the original writer and
// verifies its actual native extent before acknowledging the digest. Existing
// readers drain the prefix then receive EOF, or the preserved producer cause.
func (s *AppendSpool) Seal(ctx context.Context, producerError error) (ContentIndexEntry, error) {
	if err := s.Validate(); err != nil {
		return ContentIndexEntry{}, err
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.sealed || s.closing {
		return ContentIndexEntry{}, contractError(fs.ErrClosed)
	}
	err := errors.Join(producerError, s.terminal, scopeCancellationError(ctx, contextstate.Validate(ctx)), scopeCancellationError(s.ctx, contextstate.Validate(s.ctx)))
	if err != nil {
		s.sealFailure(sourceError(err))
		return ContentIndexEntry{}, s.terminal
	}
	metadata, err := s.file.Stat()
	if err == nil && metadata.Size() != s.extent {
		err = errors.Join(core.ErrFilestoreSize, io.ErrUnexpectedEOF)
	}
	if err == nil {
		err = s.file.Sync()
	}
	if err != nil {
		s.sealFailure(destinationError(err))
		return ContentIndexEntry{}, s.terminal
	}
	digest, extent, err := s.digest.Seal()
	err = errors.Join(err, scopeCancellationError(ctx, contextstate.Validate(ctx)), scopeCancellationError(s.ctx, contextstate.Validate(s.ctx)))
	if err != nil {
		s.sealFailure(err)
		return ContentIndexEntry{}, s.terminal
	}
	s.sealed = true
	s.notify()
	return ContentIndexEntry{Digest: digest, Extent: extent}, nil
}

var _ io.Writer = (*AppendSpool)(nil)
