package filestore

import (
	"bufio"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ContentPartitionSourceRequest borrows one forward stream. Its owner keeps
// the stream alive through the final partition or refusal.
type ContentPartitionSourceRequest struct{ Source io.Reader }

func (r ContentPartitionSourceRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrFilestoreContract
	}
	return nil
}

// ContentPartitionSource retains only Go's fixed buffered-reader window.
// It is a sequential borrowed cursor, with no filenames or partition population.
type ContentPartitionSource struct{ reader *bufio.Reader }

func NewContentPartitionSource(ctx context.Context, request ContentPartitionSourceRequest) (*ContentPartitionSource, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return nil, err
	}
	return &ContentPartitionSource{reader: bufio.NewReader(&streamReader{ctx: ctx, source: request.Source})}, nil
}

func (s *ContentPartitionSource) Validate() error {
	if s == nil || s.reader == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// ContentPartitionKind distinguishes exhaustion, a full nonterminal window,
// and the final nonempty window. Each observation stays provisional until EOF.
type ContentPartitionKind uint8

const (
	ContentPartitionUnknown ContentPartitionKind = iota
	ContentPartitionAbsent
	ContentPartitionMore
	ContentPartitionLast
)

func (k ContentPartitionKind) Validate() error {
	if k < ContentPartitionAbsent || k > ContentPartitionLast {
		return core.ErrFilestoreContract
	}
	return nil
}

func (ContentPartitionKind) OffWireEnum() {}

// CopyPartitionRequest declares a partition size, never an input admission
// ceiling. A later call continues from the exact next byte in Source.
type CopyPartitionRequest struct {
	Source      *ContentPartitionSource
	Destination io.Writer
	Maximum     core.ByteCount
}

func (r CopyPartitionRequest) Validate() error {
	if err := r.Source.Validate(); err != nil {
		return err
	}
	if core.WriterIsNil(r.Destination) {
		return core.ErrFilestoreContract
	}
	maximum, err := r.Maximum.Int64()
	if err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if maximum <= 0 {
		return core.ErrFilestoreContract
	}
	return nil
}

type ContentPartitionObservation struct {
	Content ContentIndexEntry
	Kind    ContentPartitionKind
}

func (o ContentPartitionObservation) Validate() error {
	if err := o.Kind.Validate(); err != nil {
		return err
	}
	if o.Kind == ContentPartitionAbsent {
		if o.Content != (ContentIndexEntry{}) {
			return core.ErrFilestoreContract
		}
		return nil
	}
	if err := o.Content.Validate(); err != nil {
		return err
	}
	if o.Content.Extent.Uint64() == 0 {
		return core.ErrFilestoreContract
	}
	return nil
}

// CopyPartition delegates the partition transfer to Go's limited reader and
// copy loop. Go's one-byte Peek distinguishes exact multiples without consuming
// the next partition. Source failures withdraw the observation; output remains
// provisional and caller-owned. This operation does not seal a destination.
func CopyPartition(ctx context.Context, request CopyPartitionRequest) (ContentPartitionObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ContentPartitionObservation{}, err
	}
	if _, err := request.Source.reader.Peek(1); err != nil {
		if err == io.EOF {
			if err := contextstate.Validate(ctx); err != nil {
				return ContentPartitionObservation{}, err
			}
			return ContentPartitionObservation{Kind: ContentPartitionAbsent}, nil
		}
		return ContentPartitionObservation{}, errors.Join(core.ErrFilestoreSource, err, contextstate.Validate(ctx))
	}
	maximum, err := request.Maximum.Int64()
	if err != nil {
		return ContentPartitionObservation{}, err
	}
	source := io.LimitedReader{R: request.Source.reader, N: maximum}
	content, err := CopyContent(ctx, CopyContentRequest{Source: &source, Destination: request.Destination})
	if err != nil {
		return ContentPartitionObservation{}, err
	}
	kind := ContentPartitionMore
	if _, err := request.Source.reader.Peek(1); err != nil {
		if err != io.EOF {
			return ContentPartitionObservation{}, errors.Join(core.ErrFilestoreSource, err, contextstate.Validate(ctx))
		}
		kind = ContentPartitionLast
	}
	observation := ContentPartitionObservation{Content: content, Kind: kind}
	if err := errors.Join(observation.Validate(), contextstate.Validate(ctx)); err != nil {
		return ContentPartitionObservation{}, err
	}
	return observation, nil
}
