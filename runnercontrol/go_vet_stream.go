package runnercontrol

import (
	"context"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// GoVetObservationKind distinguishes a native diagnostic from an analyzer
// that could not complete. Neither kind derives a product verdict from an exit.
type GoVetObservationKind uint8

const (
	GoVetObservationUnknown GoVetObservationKind = iota
	GoVetObservationDiagnostic
	GoVetObservationAnalyzerError
)

func (k GoVetObservationKind) Validate() error {
	if k != GoVetObservationDiagnostic && k != GoVetObservationAnalyzerError {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k GoVetObservationKind) IsValid() bool { return k.Validate() == nil }
func (GoVetObservationKind) OffWireEnum()    {}
func (k GoVetObservationKind) String() string {
	switch k {
	case GoVetObservationDiagnostic:
		return "diagnostic"
	case GoVetObservationAnalyzerError:
		return "analyzer error"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// GoVetObservation carries one native analysis record. Text is source data:
// Unit and Analyzer are Go's emitted labels, positions retain Go's rendering,
// and Message is not an error identity. No unit or diagnostic inventory is held.
type GoVetObservation struct {
	Unit     string
	Analyzer string
	Position string
	End      string
	Message  string
	Kind     GoVetObservationKind
}

func (GoVetObservation) runnerControlProtocolFact() {}
func (o GoVetObservation) Validate() error {
	if err := o.Kind.Validate(); err != nil {
		return err
	}
	if o.Unit == "" || o.Analyzer == "" {
		return core.ErrPrimitiveContract
	}
	for _, text := range [...]string{o.Unit, o.Analyzer, o.Position, o.End, o.Message} {
		if !utf8.ValidString(text) {
			return core.ErrPrimitiveContract
		}
	}
	if o.Kind == GoVetObservationDiagnostic && o.Position == "" || o.Kind == GoVetObservationAnalyzerError && (o.Position != "" || o.End != "") {
		return core.ErrPrimitiveContract
	}
	return nil
}

// GoVetStreamRequest lends a source and synchronous visitor. Each observation's
// strings allocate for that one typed record; consumers own any retained text.
// Reader and source lifetime remain with the caller.
type GoVetStreamRequest struct {
	Source io.Reader
	Visit  func(GoVetObservation) error
}

func (GoVetStreamRequest) runnerControlCapabilityWrapper() {}
func (r GoVetStreamRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.Visit == nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

type goVetDiagnosticWire struct {
	Position *string `json:"posn"`
	End      string  `json:"end"`
	Message  *string `json:"message"`
}

type goVetAnalyzerErrorWire struct {
	Error *string `json:"error"`
}

func (goVetDiagnosticWire) runnerControlInternalFlow()    {}
func (goVetAnalyzerErrorWire) runnerControlInternalFlow() {}

// ReadGoVetStream projects cmd/go's analyzer-keyed JSON through Go's decoder.
// Empty package objects and empty diagnostic arrays emit no observations. JSON
// formatting and analyzer/module cardinality never become diagnostic counts.
// A completed record observes only its prefix; clean EOF closes the stream.
// Source, cancellation, and visitor refusals retain their native identities.
func ReadGoVetStream(ctx context.Context, request GoVetStreamRequest) (resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, contextstate.Validate(ctx))
		if resultErr != nil {
			resultErr = errors.Join(core.ErrPrimitiveContract, resultErr)
		}
	}()
	decoder, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: request.Source, NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return err
	}
	for {
		kind, err := decoder.PeekKind(ctx)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if kind != jsonio.TokenObjectStart {
			return core.ErrJSONContract
		}
		if _, err := decoder.Decode(ctx); err != nil {
			return err
		}
		for {
			token, err := decoder.Decode(ctx)
			if err != nil {
				return err
			}
			if token.Kind == jsonio.TokenObjectEnd {
				break
			}
			if token.Kind != jsonio.TokenString || token.Text == "" {
				return core.ErrJSONContract
			}
			if err := readGoVetUnit(ctx, decoder, token.Text, request.Visit); err != nil {
				return err
			}
		}
	}
}

func readGoVetUnit(ctx context.Context, decoder *jsonio.TokenDecoder, unit string, visit func(GoVetObservation) error) error {
	token, err := decoder.Decode(ctx)
	if err != nil {
		return err
	}
	if token.Kind != jsonio.TokenObjectStart {
		return core.ErrJSONContract
	}
	for {
		token, err := decoder.Decode(ctx)
		if err != nil {
			return err
		}
		if token.Kind == jsonio.TokenObjectEnd {
			return nil
		}
		if token.Kind != jsonio.TokenString || token.Text == "" {
			return core.ErrJSONContract
		}
		if err := readGoVetAnalyzer(ctx, decoder, GoVetObservation{Unit: unit, Analyzer: token.Text}, visit); err != nil {
			return err
		}
	}
}

func readGoVetAnalyzer(ctx context.Context, decoder *jsonio.TokenDecoder, observation GoVetObservation, visit func(GoVetObservation) error) error {
	kind, err := decoder.PeekKind(ctx)
	if err != nil {
		return err
	}
	if kind == jsonio.TokenObjectStart {
		wire, err := jsonio.DecodeProjectionAt[goVetAnalyzerErrorWire](ctx, decoder, jsonio.MembersIgnoreExtra)
		if err != nil {
			return err
		}
		if wire.Error == nil {
			return core.ErrJSONContract
		}
		observation.Kind, observation.Message = GoVetObservationAnalyzerError, *wire.Error
		if err := observation.Validate(); err != nil {
			return err
		}
		return visit(observation)
	}
	if kind != jsonio.TokenArrayStart {
		return core.ErrJSONContract
	}
	if _, err := decoder.Decode(ctx); err != nil {
		return err
	}
	for {
		kind, err := decoder.PeekKind(ctx)
		if err != nil {
			return err
		}
		if kind == jsonio.TokenArrayEnd {
			_, err := decoder.Decode(ctx)
			return err
		}
		wire, err := jsonio.DecodeProjectionAt[goVetDiagnosticWire](ctx, decoder, jsonio.MembersIgnoreExtra)
		if err != nil {
			return err
		}
		if wire.Position == nil || wire.Message == nil {
			return core.ErrJSONContract
		}
		observation.Kind, observation.Position, observation.End, observation.Message = GoVetObservationDiagnostic, *wire.Position, wire.End, *wire.Message
		if err := observation.Validate(); err != nil {
			return err
		}
		if err := visit(observation); err != nil {
			return err
		}
	}
}
