package jsonio

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
)

// Reader-supplied codec-shaped errors are source refusals, not native grammar
// findings. The real checked reader must conserve both facts through Go.
func TestJSONReadRefusalKeepsNativeIOProvenance(t *testing.T) {
	t.Parallel()
	document := struct{ Value int }{Value: 7}
	wire, err := core.MarshalCanonicalJSONDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, door := range []struct {
		name string
		read func(*testing.T, io.Reader) error
	}{
		{name: "tokens", read: func(t *testing.T, source io.Reader) error {
			for _, err := range Tokens(t.Context(), TokenRequest{Source: source, NestingDepthMaximum: core.JSONNestingDepthMaximum}) {
				if err != nil {
					return err
				}
			}
			return nil
		}},
		{name: "projection withdrawal", read: func(t *testing.T, source io.Reader) error {
			got, err := DecodeProjectionReader[struct{ Value int }](t.Context(), source, ProjectionReadPolicy{})
			if err != nil && got != (struct{ Value int }{}) {
				t.Fatalf("refused projection=%+v error=%v, want zero", got, err)
			}
			return err
		}},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			for _, row := range []struct {
				name  string
				cause error
			}{
				{name: "ordinary native I/O", cause: io.ErrClosedPipe},
				{name: "syntactic-shaped native I/O", cause: &jsontext.SyntacticError{Err: io.ErrClosedPipe}},
				{name: "semantic-shaped native I/O", cause: &json.SemanticError{Err: io.ErrClosedPipe}},
				{name: "EOF sibling cannot erase native I/O", cause: errors.Join(io.EOF, io.ErrClosedPipe)},
			} {
				t.Run(row.name, func(t *testing.T) {
					t.Parallel()
					err := door.read(t, io.MultiReader(bytes.NewReader(wire), iotest.ErrReader(row.cause)))
					var native *nativeJSONError
					if !errors.As(err, &native) || !IsIOError(err) || !errors.Is(err, row.cause) || !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, core.ErrJSONContract) {
						t.Fatalf("native door=%s source=%T error=%v private origin=%t, want retained source origin/cause and no invented JSON identity", door.name, row.cause, err, native != nil)
					}
				})
			}
		})
	}
}

func TestJSONIOProvenanceDoesNotAuthorizeGrammarOrEmptyInput(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, wire  string
		wantGrammar bool
	}{
		{name: "empty token stream"},
		{name: "complete token stream", wire: "{}"},
		{name: "actual malformed source", wire: "{", wantGrammar: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var refusal error
			for _, err := range Tokens(t.Context(), TokenRequest{Source: bytes.NewBufferString(row.wire), NestingDepthMaximum: core.JSONNestingDepthMaximum}) {
				if err != nil {
					refusal = err
					break
				}
			}
			if IsIOError(refusal) || errors.Is(refusal, core.ErrJSONContract) != row.wantGrammar {
				t.Fatalf("source=%q error=%v I/O origin=%t, want no I/O origin and grammar=%t", row.wire, refusal, IsIOError(refusal), row.wantGrammar)
			}
		})
	}
	if IsIOError(&jsontext.SyntacticError{Err: io.ErrClosedPipe}) || IsIOError(&json.SemanticError{Err: io.ErrClosedPipe}) {
		t.Fatal("unowned codec shape acquired native I/O provenance, want false")
	}
}

func TestJSONWriteRefusalKeepsNativeIOProvenanceAcrossCancellation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name              string
		cancelDuringWrite bool
	}{
		{name: "writer refusal without cancellation"},
		{name: "writer refusal beside owned cancellation", cancelDuringWrite: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			writer := &jsonIORefusingWriter{cause: &json.SemanticError{Err: io.ErrClosedPipe}}
			if row.cancelDuringWrite {
				writer.cancel = cancel
			}
			encoder, err := NewObjectEncoder[jsonIOProbe](ObjectDestinationRequest{Destination: writer})
			if err != nil {
				t.Fatal(err)
			}
			err = encoder.Encode(ctx, jsonIOProbe{Value: 7})
			if writer.calls != 1 || !IsIOError(err) || !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, context.Canceled) != row.cancelDuringWrite || errors.Is(err, io.ErrNoProgress) != row.cancelDuringWrite {
				t.Fatalf("native writes=%d error=%v I/O origin=%t, want one refused write, closed-pipe origin and cancellation=%t", writer.calls, err, IsIOError(err), row.cancelDuringWrite)
			}
		})
	}
}

type jsonIOProbe struct{ Value int }

func (p jsonIOProbe) Validate() error {
	if p.Value < 1 {
		return core.ErrJSONContract
	}
	return nil
}

type jsonIORefusingWriter struct {
	cause  error
	cancel context.CancelCauseFunc
	calls  int
}

func (w *jsonIORefusingWriter) Write([]byte) (int, error) {
	w.calls++
	if w.cancel != nil {
		w.cancel(io.ErrNoProgress)
	}
	return 0, w.cause
}

type jsonIORefusingSource struct {
	cause    error
	observed bool
}

func (r *jsonIORefusingSource) Read([]byte) (int, error) { r.observed = true; return 0, r.cause }

func FuzzJSONIOProvenanceConservesNativeRefusal(f *testing.F) {
	wire, err := core.MarshalCanonicalJSONDocument(jsonIOProbe{Value: 7})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(wire, false)
	f.Add([]byte("{"), true)
	f.Add([]byte{}, true)
	f.Fuzz(func(t *testing.T, data []byte, contractSibling bool) {
		var cause error = &jsontext.SyntacticError{Err: io.ErrClosedPipe}
		if contractSibling {
			cause = errors.Join(cause, core.ErrJSONContract)
		}
		source := &jsonIORefusingSource{cause: cause}
		var refusal error
		for _, err := range Tokens(t.Context(), TokenRequest{Source: io.MultiReader(bytes.NewReader(data), source), NestingDepthMaximum: core.JSONNestingDepthMaximum}) {
			if err != nil {
				refusal = err
				break
			}
		}
		if refusal == nil || IsIOError(refusal) != source.observed || errors.Is(refusal, io.ErrClosedPipe) != source.observed || source.observed && !errors.Is(refusal, cause) || !source.observed && !errors.Is(refusal, core.ErrJSONContract) {
			t.Fatalf("native source observed=%t error=%v I/O origin=%t data=%x, want conserved source origin and complete native cause or actual grammar refusal", source.observed, refusal, IsIOError(refusal), data)
		}
	})
}

func BenchmarkJSONNativeIOProvenance(b *testing.B) {
	b.ReportAllocs()
	var refusal error
	for _, err := range Tokens(b.Context(), TokenRequest{Source: iotest.ErrReader(&jsontext.SyntacticError{Err: io.ErrClosedPipe}), NestingDepthMaximum: core.JSONNestingDepthMaximum}) {
		if err != nil {
			refusal = err
			break
		}
	}
	if !errors.Is(refusal, io.ErrClosedPipe) || !IsIOError(refusal) {
		b.Fatalf("native setup error=%v, want owned source failure", refusal)
	}
	for b.Loop() {
		if !IsIOError(refusal) {
			b.Fatalf("native source origin lost for %v", refusal)
		}
	}
}

func TestJSONCursorDoorsConserveReadAheadRefusal(t *testing.T) {
	t.Parallel()
	for _, door := range []struct {
		name string
		read func(*testing.T, io.Reader) error
	}{
		{name: "token", read: func(t *testing.T, r io.Reader) error {
			d, e := NewTokenDecoder(TokenRequest{Source: r, NestingDepthMaximum: core.JSONNestingDepthMaximum})
			if e != nil {
				return e
			}
			got, e := d.Decode(t.Context())
			if e != nil && got != (Token{}) {
				t.Fatalf("refused token=%+v error=%v, want zero", got, e)
			}
			return e
		}},
		{name: "peek", read: func(t *testing.T, r io.Reader) error {
			d, e := NewTokenDecoder(TokenRequest{Source: r, NestingDepthMaximum: core.JSONNestingDepthMaximum})
			if e != nil {
				return e
			}
			got, e := d.PeekKind(t.Context())
			if e != nil && got != TokenUnknown {
				t.Fatalf("refused kind=%v error=%v, want unknown", got, e)
			}
			return e
		}},
		{name: "raw value", read: func(t *testing.T, r io.Reader) error {
			d, e := NewTokenDecoder(TokenRequest{Source: r, NestingDepthMaximum: core.JSONNestingDepthMaximum})
			if e != nil {
				return e
			}
			got, e := d.ReadValue(t.Context())
			if e != nil && len(got) != 0 {
				t.Fatalf("refused value=%x error=%v, want empty", got, e)
			}
			return e
		}},
		{name: "skip", read: func(t *testing.T, r io.Reader) error {
			d, e := NewTokenDecoder(TokenRequest{Source: r, NestingDepthMaximum: core.JSONNestingDepthMaximum})
			if e != nil {
				return e
			}
			return d.SkipValue(t.Context())
		}},
		{name: "projection cursor", read: func(t *testing.T, r io.Reader) error {
			d, e := NewTokenDecoder(TokenRequest{Source: r, NestingDepthMaximum: core.JSONNestingDepthMaximum})
			if e != nil {
				return e
			}
			got, e := DecodeProjectionAt[jsonIOProbe](t.Context(), d, MembersDeclaredOnly)
			if e != nil && got != (jsonIOProbe{}) {
				t.Fatalf("refused projection=%+v error=%v, want zero", got, e)
			}
			return e
		}},
		{name: "value cursor", read: func(t *testing.T, r io.Reader) error {
			d, e := NewTokenDecoder(TokenRequest{Source: r, NestingDepthMaximum: core.JSONNestingDepthMaximum})
			if e != nil {
				return e
			}
			got, e := DecodeValueAt[jsonIOProbe](t.Context(), d, ProjectionReadPolicy{})
			if e != nil && got != (jsonIOProbe{}) {
				t.Fatalf("refused value=%+v error=%v, want zero", got, e)
			}
			return e
		}},
		{name: "object decoder", read: func(t *testing.T, r io.Reader) error {
			d, e := NewObjectDecoder[jsonIOProbe](ObjectSourceRequest{Source: r})
			if e != nil {
				return e
			}
			got, e := d.Decode(t.Context())
			if e != nil && got != (jsonIOProbe{}) {
				t.Fatalf("refused object=%+v error=%v, want zero", got, e)
			}
			return e
		}},
		{name: "whole document validation", read: func(t *testing.T, r io.Reader) error { return ValidateDocument(t.Context(), r) }},
		{name: "bounded documents", read: func(t *testing.T, r io.Reader) error {
			for got, e := range Documents[jsonIOProbe](t.Context(), Request{Source: r, Limits: core.DefaultStrictJSONLimits()}) {
				if e != nil {
					if got != (jsonIOProbe{}) {
						t.Fatalf("refused bounded document=%+v error=%v, want zero", got, e)
					}
					return e
				}
			}
			return nil
		}},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			source := &jsonIORefusingSource{cause: io.ErrClosedPipe}
			err := door.read(t, io.MultiReader(bytes.NewBufferString(","), source))
			if !source.observed || !IsIOError(err) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("door=%s read refusal observed=%t error=%v I/O origin=%t, want actual read-ahead refusal beside grammar and no accepted value", door.name, source.observed, err, IsIOError(err))
			}
		})
	}
}

func TestJSONObjectResetReplacesRefusedNativeSource(t *testing.T) {
	t.Parallel()
	source := &jsonIORefusingSource{cause: io.ErrClosedPipe}
	decoder, err := NewObjectDecoder[jsonIOProbe](ObjectSourceRequest{Source: io.MultiReader(bytes.NewBufferString(","), source)})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := decoder.Decode(t.Context()); got != (jsonIOProbe{}) || !IsIOError(err) || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("initial decode=%+v error=%v, want zero and observed native refusal", got, err)
	}
	if err := decoder.Reset(t.Context(), ObjectSourceRequest{}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("invalid reset=%v, want %v", err, core.ErrJSONContract)
	}
	if got, err := decoder.Decode(t.Context()); got != (jsonIOProbe{}) || !IsIOError(err) {
		t.Fatalf("refused reset changed source: value=%+v error=%v, want original refusal", got, err)
	}
	want := jsonIOProbe{Value: 7}
	wire, err := core.MarshalCanonicalJSONDocument(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.Reset(t.Context(), ObjectSourceRequest{Source: bytes.NewReader(wire)}); err != nil {
		t.Fatal(err)
	}
	if got, err := decoder.Decode(t.Context()); err != nil || got != want {
		t.Fatalf("admitted reset=%+v error=%v, want %+v", got, err, want)
	}
	if got, err := decoder.Decode(t.Context()); got != (jsonIOProbe{}) || err != io.EOF || IsIOError(err) {
		t.Fatalf("fresh source closure=%+v error=%v, want zero and bare EOF without old I/O origin", got, err)
	}
}
