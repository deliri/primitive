package runnercontrol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
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
	// ElapsedSeconds is Go's finite float64 scalar, including signed zero.
	// Absence projects zero; nonnegative-duration policy belongs to consumers.
	ElapsedSeconds float64 `json:"Elapsed"`
}

func (GoEventFrame) runnerControlProtocolFact() {}
func (f GoEventFrame) Validate() error {
	if math.IsNaN(f.ElapsedSeconds) || math.IsInf(f.ElapsedSeconds, 0) {
		return goJSONFailure()
	}
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
	// OnBenchmark, when set, receives every Go benchmark result row, typed.
	// A row may span consecutive output events of one package (test2json
	// splits lines longer than its buffer); it is delivered as its newline
	// is decoded and, like a fragment, is provisional until that event's
	// OnEvent. A row cut by another package, a non-output event or EOF is
	// reported interrupted. Nothing accumulates beyond the pending row.
	OnBenchmark func(runprotocol.GoBenchmarkRow) error
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
func ReadGoEventStream(ctx context.Context, request GoEventStreamRequest) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx})) }()
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
	if request.OnBenchmark != nil {
		parser.rows = &goBenchmarkRows{deliver: func(row runprotocol.GoBenchmarkRow) error {
			if err := contextstate.Validate(ctx); err != nil {
				return err
			}
			if err := row.Validate(); err != nil {
				return err
			}
			return request.OnBenchmark(row)
		}}
	}
	emit := func(projection goJSONProjection) error {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		frame := GoEventFrame{Action: projection.action, OutputKind: projection.outputType, ElapsedSeconds: projection.elapsedSeconds}
		if err := frame.Validate(); err != nil {
			return err
		}
		if err := request.OnEvent(frame); err != nil {
			return err
		}
		if parser.rows == nil {
			return nil
		}
		return parser.rows.endEvent(projection)
	}
	if err := readGoEventFragments(ctx, reader, &parser, emit); err != nil || parser.rows == nil {
		return err
	}
	// The row cut by EOF is refused with the same identity as a completed one.
	if err := parser.rows.interrupt(); err != nil {
		return fmt.Errorf("go event stream refused: %w", errors.Join(core.ErrPrimitiveContract, err))
	}
	return nil
}

func readGoEventFragments(ctx context.Context, reader *lineio.Reader, parser *goJSONStream, emit func(goJSONProjection) error) error {
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		fragment, readErr := reader.ReadFragment(ctx)
		data := fragment.Bytes
		for index := 0; index < len(data); index++ {
			index += parser.consumeStringRun(data[index:])
			if index == len(data) {
				break
			}
			if err := parser.consume(data[index], emit); err != nil {
				if readErr == io.EOF {
					readErr = nil // Actual EOF is framing, not an accompanying source refusal.
				}
				return fmt.Errorf("go event stream refused: %w", errors.Join(core.ErrPrimitiveContract, err, readErr))
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, core.ErrLineIOScan) || !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("go event source refused: %w", errors.Join(core.ErrPrimitiveContract, readErr))
		}
		if err := parser.finish(emit); err != nil {
			return fmt.Errorf("go event stream incomplete: %w", errors.Join(core.ErrPrimitiveContract, err))
		}
		return contextstate.Validate(ctx)
	}
}
