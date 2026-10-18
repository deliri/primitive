package jsonio

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// RecordSyntax describes grammar separately from typed projection admission.
type RecordSyntax uint8

const (
	RecordSyntaxUnknown RecordSyntax = iota
	RecordSyntaxBlank
	RecordSyntaxComplete
	RecordSyntaxInvalid
	RecordSyntaxUninspected
)

func (s RecordSyntax) IsValid() bool { return s >= RecordSyntaxBlank && s <= RecordSyntaxUninspected }
func (s RecordSyntax) Validate() error {
	if !s.IsValid() {
		return core.ErrJSONContract
	}
	return nil
}
func (RecordSyntax) OffWireEnum() {}
func (s RecordSyntax) String() string {
	switch s {
	case RecordSyntaxBlank:
		return "blank"
	case RecordSyntaxComplete:
		return "complete"
	case RecordSyntaxInvalid:
		return "invalid"
	case RecordSyntaxUninspected:
		return "uninspected"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// ProjectionRecordRequest lends one Native record reader. SyntaxSource is
// optional: on projection refusal it permits a second grammar-only read of the
// declared physical range. Its bytes must match the original consumed record;
// the implementation proves that agreement with native digest and extent.
type ProjectionRecordRequest struct {
	Reader       *lineio.Reader
	SyntaxSource lineio.RecordSource
	Offset       lineio.SourceByteOffset
	Policy       ProjectionReadPolicy
}

func (r ProjectionRecordRequest) Validate() error {
	if r.SyntaxSource != nil && core.ReaderIsNil(r.SyntaxSource) {
		return core.ErrJSONContract
	}
	return errors.Join(r.Reader.Validate(), r.Offset.Validate(), r.Policy.Validate())
}

// ProjectionRecord carries completed physical framing, trimmed JSON byte
// extent and separate typed/grammar refusals. Value is zero on DecodeError.
// The source owner still owns cleanup; a completed record is not product proof.
type ProjectionRecord[Document any] struct {
	Value        Document
	DecodeError  error
	SyntaxError  error
	Record       lineio.RecordRange
	ContentBytes core.ByteLength
	Syntax       RecordSyntax
}

func (r ProjectionRecord[Document]) Validate() error {
	if err := errors.Join(r.Record.Validate(), r.ContentBytes.Validate(), r.Syntax.Validate()); err != nil {
		return err
	}
	if r.ContentBytes.Uint64() > r.Record.Bytes.Uint64() || (r.ContentBytes.Uint64() == 0) != (r.Syntax == RecordSyntaxBlank) {
		return core.ErrJSONContract
	}
	if r.Syntax == RecordSyntaxInvalid && (r.DecodeError == nil || r.SyntaxError == nil) || r.Syntax == RecordSyntaxUninspected && r.DecodeError == nil {
		return core.ErrJSONContract
	}
	return nil
}

// DecodeProjectionRecord consumes one LF/EOF-framed record without collecting
// its text. Native Jsonio owns decoding; Native Lineio owns framing and drain.
// Only bare io.EOF means no record remains. Source failure returns zero. Native
// JSON still owns any per-value or whitespace-buffer allocation it requires.
func DecodeProjectionRecord[Document any](ctx context.Context, request ProjectionRecordRequest) (result ProjectionRecord[Document], resultErr error) {
	if err := errors.Join(projectionContextError(ctx), request.Validate()); err != nil {
		return ProjectionRecord[Document]{}, err
	}
	capture := projectionRecordCapture{digest: core.NewDigestWriter(), first: -1}
	frame, err := request.Reader.WithRecord(ctx, lineio.RecordReadRequest{Capture: &capture, Use: func(ctx context.Context, source io.Reader) error {
		result.Value, result.DecodeError = DecodeProjectionReader[Document](ctx, source, request.Policy)
		// Typed refusal must not hide the remaining physical source's outcome.
		return nil
	}})
	if err != nil {
		return ProjectionRecord[Document]{}, err
	}
	result.Record = lineio.RecordRange{Offset: request.Offset, Bytes: frame.Bytes, Framing: frame.Framing}
	if err := result.Record.Validate(); err != nil {
		return ProjectionRecord[Document]{}, err
	}
	if capture.first < 0 {
		var zero Document
		result.Value, result.DecodeError, result.Syntax = zero, nil, RecordSyntaxBlank
		result.ContentBytes, err = core.NewByteLength(0)
	} else {
		result.ContentBytes, err = core.NewByteLength(uint64(capture.last - capture.first))
		result.Syntax = RecordSyntaxComplete
		if result.DecodeError != nil {
			result.Syntax = RecordSyntaxUninspected
			if request.SyntaxSource != nil {
				result.SyntaxError, err = projectionRecordSyntax(ctx, request, result.Record, &capture)
				result.Syntax = RecordSyntaxComplete
				if result.SyntaxError != nil {
					result.Syntax = RecordSyntaxInvalid
				}
			}
		}
	}
	if err := errors.Join(err, result.Validate(), projectionContextError(ctx)); err != nil {
		return ProjectionRecord[Document]{}, err
	}
	return result, nil
}

type projectionRecordCapture struct {
	digest   *core.DigestWriter
	first    int64
	last     int64
	consumed int64
}

func (c *projectionRecordCapture) Write(data []byte) (int, error) {
	n, err := c.digest.Write(data)
	for index, value := range data[:n] {
		if strings.IndexByte(core.JSONWhitespace, value) < 0 {
			if c.first < 0 {
				c.first = c.consumed + int64(index)
			}
			c.last = c.consumed + int64(index) + 1
		}
	}
	c.consumed += int64(n)
	return n, err
}

type projectionRecordReplay struct {
	ctx     context.Context
	source  io.Reader
	refusal error
}

func (r *projectionRecordReplay) Read(data []byte) (int, error) {
	if err := projectionContextError(r.ctx); err != nil {
		r.refusal = errors.Join(r.refusal, err)
		return 0, err
	}
	n, err := (checkedJSONSource{source: r.source}).Read(data)
	if terminal := projectionContextError(r.ctx); terminal != nil {
		err = errors.Join(err, terminal)
	}
	if err != nil && err != io.EOF {
		r.refusal = errors.Join(r.refusal, err)
	}
	return n, err
}

func projectionRecordSyntax(ctx context.Context, request ProjectionRecordRequest, record lineio.RecordRange, captured *projectionRecordCapture) (error, error) {
	expected, expectedBytes, err := captured.digest.Seal()
	if err != nil || expectedBytes != record.Bytes {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	length, err := record.Bytes.Int64()
	if err != nil {
		return nil, err
	}
	source := projectionRecordReplay{ctx: ctx, source: io.NewSectionReader(request.SyntaxSource, int64(record.Offset), length)}
	digest := core.NewDigestWriter()
	stream := io.TeeReader(&source, digest)
	cursor, err := NewTokenDecoder(TokenRequest{Source: stream, NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return nil, err
	}
	syntaxErr := cursor.SkipValue(ctx)
	if syntaxErr == nil {
		syntaxErr = cursor.RequireEOF(ctx)
	}
	_, drainErr := io.Copy(io.Discard, stream)
	actual, actualBytes, sealErr := digest.Seal()
	if err := errors.Join(source.refusal, drainErr, sealErr, projectionContextError(ctx)); err != nil {
		return nil, errors.Join(syntaxErr, err)
	}
	if actual != expected || actualBytes != expectedBytes {
		return nil, core.ErrJSONContract
	}
	return syntaxErr, nil
}

var _ core.Validatable = ProjectionRecordRequest{}
