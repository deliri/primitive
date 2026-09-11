package runnercontrol

import (
	"context"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// GoEventStringFragment borrows decoded UTF-8 from one cmd/go string field.
// Final closes that field. Fields can arrive in any order. Fragments are
// provisional until OnEvent receives the validated containing frame; neither
// callback proves that the remaining source will be admitted.
type GoEventStringFragment struct {
	Data  []byte
	Field GoEventField
	Final bool
}

func (GoEventStringFragment) runnerControlProtocolFact() {}
func (f GoEventStringFragment) Validate() error {
	if err := f.Field.Validate(); err != nil {
		return err
	}
	if f.Field == GoEventFieldElapsed || !utf8.Valid(f.Data) || f.Final && len(f.Data) != 0 {
		return goJSONFailure()
	}
	return nil
}

// GoEventFrame closes one syntactically complete cmd/go event. Its meaning and
// any cross-event accounting belong to the consumer, not this streaming reader.
type GoEventFrame struct {
	Action     GoEventAction     `json:"Action"`
	OutputKind GoEventOutputKind `json:"OutputType"`
}

func (GoEventFrame) runnerControlProtocolFact() {}
func (f GoEventFrame) Validate() error {
	return errors.Join(f.Action.Validate(), f.OutputKind.Validate())
}

// GoEventStreamRequest exposes the existing cmd/go scalar decoder without
// retaining package inventories, benchmark arrays, complete lines or strings.
// Source lifetime belongs to the caller. Callbacks must consume borrowed data
// before returning and retain their own bounded projections.
type GoEventStreamRequest struct {
	Source   io.Reader
	OnString func(GoEventStringFragment) error
	OnEvent  func(GoEventFrame) error
}

func (GoEventStreamRequest) runnerControlCapabilityWrapper() {}
func (r GoEventStreamRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.OnString == nil || r.OnEvent == nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

// ReadGoEventStream reads to clean EOF with constant working memory. There is
// no total-stream, event-count or string-length quota. It returns source and
// callback refusals unchanged inside the observation error identity. Previously
// emitted frames remain observations of their prefix, never whole-stream proof.
func ReadGoEventStream(ctx context.Context, request GoEventStreamRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	size, err := core.NewByteCount(32 << 10)
	if err != nil {
		return err
	}
	reader, err := lineio.New(lineio.Request{Source: request.Source, BufferBytes: size})
	if err != nil {
		return err
	}
	parser := goJSONStream{observeString: func(fragment GoEventStringFragment) error {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		if err := fragment.Validate(); err != nil {
			return err
		}
		return request.OnString(fragment)
	}}
	emit := func(projection goJSONProjection) error {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		frame := GoEventFrame{Action: projection.action, OutputKind: projection.outputType}
		if err := frame.Validate(); err != nil {
			return err
		}
		return request.OnEvent(frame)
	}
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		fragment, readErr := reader.ReadFragment()
		for _, value := range fragment.Bytes {
			if err := parser.consume(value, emit); err != nil {
				return observationFailure("go event stream refused", err, readErr)
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, core.ErrLineIOScan) || !errors.Is(readErr, io.EOF) {
			return observationFailure("go event source refused", readErr)
		}
		if err := parser.finish(emit); err != nil {
			return observationFailure("go event stream incomplete", err)
		}
		return contextstate.Validate(ctx)
	}
}
