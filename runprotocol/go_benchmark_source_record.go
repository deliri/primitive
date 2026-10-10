package runprotocol

import (
	"context"
	"errors"
	"io"
	"iter"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// GoBenchmarkNameExtent is an admitted name's original native byte range.
// It retains no name payload and no source handle.
type GoBenchmarkNameExtent struct{ extent GoBenchmarkSourceExtent }

// SourceExtent returns the admitted name's exact bytes.
func (n GoBenchmarkNameExtent) SourceExtent() GoBenchmarkSourceExtent { return n.extent }

// Validate checks a nonempty native range.
func (n GoBenchmarkNameExtent) Validate() error {
	if err := n.extent.Validate(); err != nil {
		return err
	}
	if n.extent.Bytes.Uint64() == 0 {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// GoBenchmarkSourceRecord retains scalar facts and source coordinates only.
type GoBenchmarkSourceRecord struct {
	name        GoBenchmarkNameExtent
	source      GoBenchmarkSourceExtent
	Iterations  int64
	Nanoseconds float64
	Bytes       int64
	Allocations int64
	Presence    GoBenchmarkRecordPresence
	Fields      GoBenchmarkMetricFields
}

// Name returns the admitted name range without copying its payload.
func (r GoBenchmarkSourceRecord) Name() GoBenchmarkNameExtent { return r.name }

// SourceExtent returns all original row bytes, including framing.
func (r GoBenchmarkSourceRecord) SourceExtent() GoBenchmarkSourceExtent { return r.source }

// Refusal exposes the immutable refusal for a complete malformed row.
func (r GoBenchmarkSourceRecord) Refusal() error {
	if r.Presence == GoBenchmarkRecordRefused {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// Validate checks numeric authority and range containment.
func (r GoBenchmarkSourceRecord) Validate() error {
	if err := errors.Join(r.source.Validate(), r.Presence.Validate(), r.Fields.Validate()); err != nil {
		return err
	}
	if r.source.Bytes.Uint64() == 0 {
		return core.ErrGoToolchainOutput
	}
	if r.Presence == GoBenchmarkRecordAbsent || r.Presence == GoBenchmarkRecordRefused {
		if r.name != (GoBenchmarkNameExtent{}) || r.Iterations != 0 || r.Nanoseconds != 0 || r.Bytes != 0 || r.Allocations != 0 || r.Fields != GoBenchmarkMetricFieldsNone {
			return core.ErrGoToolchainOutput
		}
		return nil
	}
	if err := r.name.Validate(); err != nil {
		return err
	}
	name := r.name.extent
	if name.Offset < r.source.Offset || name.Bytes.Uint64() > r.source.Bytes.Uint64() || uint64(name.Offset-r.source.Offset) > r.source.Bytes.Uint64()-name.Bytes.Uint64() {
		return core.ErrGoToolchainOutput
	}
	if r.Iterations < 0 || r.Nanoseconds < 0 || math.IsNaN(r.Nanoseconds) || math.IsInf(r.Nanoseconds, 0) || r.Bytes < 0 || r.Allocations < 0 {
		return core.ErrGoToolchainOutput
	}
	if r.Fields&GoBenchmarkMetricTime == 0 && r.Nanoseconds != 0 || r.Fields&GoBenchmarkMetricBytes == 0 && r.Bytes != 0 || r.Fields&GoBenchmarkMetricAllocations == 0 && r.Allocations != 0 {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// GoBenchmarkNameSourceRequest lends the same native source inside its scope.
type GoBenchmarkNameSourceRequest struct {
	Source GoBenchmarkRecordSource
	Name   GoBenchmarkNameExtent
}

// Validate checks the source and admitted range.
func (r GoBenchmarkNameSourceRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return r.Name.Validate()
}

// GoBenchmarkNameFragment lends bytes until the next iterator advance.
// Consumers must finish processing or copying the fragment before advancing.
type GoBenchmarkNameFragment struct{ bytes []byte }

// Bytes returns the current borrowed fragment.
func (f GoBenchmarkNameFragment) Bytes() []byte { return f.bytes }

// Validate checks a nonempty fragment.
func (f GoBenchmarkNameFragment) Validate() error {
	if len(f.bytes) == 0 {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// GoBenchmarkNameFragments reads an admitted name with fixed working memory.
// No input-length quota or payload copy applies. The iterator retains no source
// past consumption, and never reads after the consumer stops.
func GoBenchmarkNameFragments(ctx context.Context, request GoBenchmarkNameSourceRequest) iter.Seq2[GoBenchmarkNameFragment, error] {
	return func(yield func(GoBenchmarkNameFragment, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoBenchmarkNameFragment{}, err)
			return
		}
		extent := request.Name.extent
		size, err := extent.Bytes.Int64()
		if err != nil {
			yield(GoBenchmarkNameFragment{}, err)
			return
		}
		source := goBenchmarkSourceReader{ctx: ctx, source: io.NewSectionReader(request.Source, int64(extent.Offset), size)}
		var buffer [4096]byte
		for remaining := size; remaining > 0; {
			count := min(int64(len(buffer)), remaining)
			n, err := io.ReadFull(&source, buffer[:int(count)])
			if err = errors.Join(err, source.readErr); err != nil {
				yield(GoBenchmarkNameFragment{}, errors.Join(core.ErrGoToolchainOutput, err))
				return
			}
			remaining -= int64(n)
			if !yield(GoBenchmarkNameFragment{bytes: buffer[:n]}, nil) {
				return
			}
		}
	}
}

func (GoBenchmarkNameExtent) runProtocolFact()        {}
func (GoBenchmarkSourceRecord) runProtocolFact()      {}
func (GoBenchmarkNameSourceRequest) runProtocolFact() {}
func (GoBenchmarkNameFragment) runProtocolFact()      {}
