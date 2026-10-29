package runprotocol

import (
	"bufio"
	"context"
	"errors"
	"go/token"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// GoFuzzTarget is a compiler identifier admitted by cmd/go's fuzz-name rule.
type GoFuzzTarget struct{ name string }

func ParseGoFuzzTarget(name string) (GoFuzzTarget, error) {
	target := GoFuzzTarget{name: name}
	if err := target.Validate(); err != nil {
		return GoFuzzTarget{}, err
	}
	return target, nil
}

func (t GoFuzzTarget) Validate() error {
	if !token.IsIdentifier(t.name) || !strings.HasPrefix(t.name, "Fuzz") {
		return core.ErrGoToolchainContract
	}
	if suffix := t.name[len("Fuzz"):]; suffix != "" {
		value, _ := utf8.DecodeRuneInString(suffix)
		if unicode.IsLower(value) {
			return core.ErrGoToolchainContract
		}
	}
	return nil
}

func (t GoFuzzTarget) String() string { return t.name }

// GoFuzzCompletion records a native testing.F terminal frame, or its absence.
// It does not infer completion from process exit or aggregate fuzz metrics.
type GoFuzzCompletion uint8

const (
	GoFuzzCompletionUnknown GoFuzzCompletion = iota
	GoFuzzCompletionAbsent
	GoFuzzCompletionPassed
	GoFuzzCompletionSkipped
	GoFuzzCompletionFailed
)

func (c GoFuzzCompletion) Validate() error {
	switch c {
	case GoFuzzCompletionAbsent, GoFuzzCompletionPassed, GoFuzzCompletionSkipped, GoFuzzCompletionFailed:
		return nil
	default:
		return core.ErrGoToolchainOutput
	}
}

func (GoFuzzCompletion) OffWireEnum() {}

// GoFuzzCompletionRequest borrows the completed native stdout source. The
// caller owns process wait and the source lifetime; no source escapes the call.
type GoFuzzCompletionRequest struct {
	Source lineio.RecordSource
	Target GoFuzzTarget
}

func (r GoFuzzCompletionRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return r.Target.Validate()
}

// GoFuzzCompletionObservation retains one selected frame's native range.
// Unknown/absent differs from a real selected target's completed empty run.
type GoFuzzCompletionObservation struct {
	target     GoFuzzTarget
	completion GoFuzzCompletion
	record     lineio.RecordRange
}

func (o GoFuzzCompletionObservation) Validate() error {
	if err := errors.Join(o.target.Validate(), o.completion.Validate()); err != nil {
		return err
	}
	if o.completion == GoFuzzCompletionAbsent {
		if o.record != (lineio.RecordRange{}) {
			return core.ErrGoToolchainOutput
		}
		return nil
	}
	return o.record.Validate()
}

func (o GoFuzzCompletionObservation) Target() GoFuzzTarget         { return o.target }
func (o GoFuzzCompletionObservation) Completion() GoFuzzCompletion { return o.completion }
func (o GoFuzzCompletionObservation) Record() lineio.RecordRange   { return o.record }

// ObserveGoFuzzCompletion consumes the entire native source with fixed working
// memory. Go's regexp engine recognizes testing.F's exact top-level terminal
// frame; subtargets, diagnostics and package PASS cannot certify the target.
// Duplicate selected terminal frames refuse even when their labels agree.
// Any native read/cancellation failure discards the observation.
func ObserveGoFuzzCompletion(ctx context.Context, request GoFuzzCompletionRequest) (GoFuzzCompletionObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return GoFuzzCompletionObservation{}, err
	}
	buffer, err := core.NewByteCount(4096)
	if err != nil {
		return GoFuzzCompletionObservation{}, err
	}
	pattern := regexp.MustCompile(`\A--- (?:(PASS)|(SKIP)|(FAIL)): ` + regexp.QuoteMeta(request.Target.String()) + ` \([0-9]+\.[0-9]{2}s\)\n\z`)
	observation := GoFuzzCompletionObservation{target: request.Target, completion: GoFuzzCompletionAbsent}
	for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: request.Source, BufferBytes: buffer}) {
		if err != nil {
			return GoFuzzCompletionObservation{}, err
		}
		if record.Framing != lineio.RecordFramingLF {
			continue
		}
		length, extentErr := record.Bytes.Int64()
		if extentErr != nil {
			return GoFuzzCompletionObservation{}, extentErr
		}
		guard := &goFuzzFrameSource{ctx: ctx, source: io.NewSectionReader(request.Source, int64(record.Offset), length)}
		source := goFuzzSpanReader(guard, length)
		match := pattern.FindReaderSubmatchIndex(source)
		_, drainErr := io.Copy(io.Discard, source)
		if extentErr := errors.Join(guard.readErr, drainErr, contextstate.Validate(ctx)); extentErr != nil {
			return GoFuzzCompletionObservation{}, extentErr
		}
		if match == nil {
			continue
		}
		if observation.completion != GoFuzzCompletionAbsent {
			return GoFuzzCompletionObservation{}, core.ErrGoToolchainOutput
		}
		for index, completion := range [...]GoFuzzCompletion{GoFuzzCompletionPassed, GoFuzzCompletionSkipped, GoFuzzCompletionFailed} {
			if match[2+index*2] >= 0 {
				observation.completion = completion
			}
		}
		observation.record = record
	}
	if err := observation.Validate(); err != nil {
		return GoFuzzCompletionObservation{}, err
	}
	return observation, nil
}

type goFuzzFrameSource struct {
	ctx     context.Context
	source  io.Reader
	readErr error
}

// goFuzzSpanReaderMaximumBytes is the buffered working window for one span.
const goFuzzSpanReaderMaximumBytes = 4096

// goFuzzSpanReader buffers one exact span through a window no larger than the
// span itself, so a short physical record never pays for the full window.
// Buffer size changes only read chunking, never the bytes or runes observed.
func goFuzzSpanReader(guard *goFuzzFrameSource, spanBytes int64) *bufio.Reader {
	return bufio.NewReaderSize(guard, int(min(max(spanBytes, 1), goFuzzSpanReaderMaximumBytes)))
}

func (s *goFuzzFrameSource) Read(p []byte) (int, error) {
	if err := contextstate.Validate(s.ctx); err != nil {
		s.readErr = errors.Join(s.readErr, err)
		return 0, err
	}
	n, err := s.source.Read(p)
	if err != nil && err != io.EOF {
		s.readErr = errors.Join(s.readErr, err)
	}
	return n, err
}

func (GoFuzzTarget) runProtocolFact()                            {}
func (GoFuzzCompletionRequest) runProtocolFact()                 {}
func (GoFuzzCompletionObservation) runProtocolSealedProjection() {}
func (goFuzzFrameSource) runProtocolInternalFlowCarrier()        {}

func (c GoFuzzCompletion) IsValid() bool { return c.Validate() == nil }
func (c GoFuzzCompletion) String() string {
	switch c {
	case GoFuzzCompletionAbsent:
		return "absent"
	case GoFuzzCompletionPassed:
		return "passed"
	case GoFuzzCompletionSkipped:
		return "skipped"
	case GoFuzzCompletionFailed:
		return "failed"
	default:
		return "unknown"
	}
}
