package runnercontrol_test

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoEventGrammarHasNativeIdentity(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		wantGrammar  bool
		wantFrames   int
	}{
		{name: "empty source remains neutral"},
		{name: "valid frame remains an observation", source: `{"Action":"pass"}`, wantFrames: 1},
		{name: "unclosed record", source: `{`, wantGrammar: true},
		{name: "foreign member", source: `{"Action":"pass","foreign":true}`, wantGrammar: true},
		{name: "wrong action type", source: `{"Action":42}`, wantGrammar: true},
		{name: "duplicate member", source: `{"Action":"pass","Action":"fail"}`, wantGrammar: true},
		{name: "unknown action", source: `{"Action":"future"}`, wantGrammar: true},
		{name: "invalid string escape", source: `{"Action":"output","Output":"\q"}`, wantGrammar: true},
		{name: "nested field is outside flat native protocol", source: `{"Action":"output","Output":{}}`, wantGrammar: true},
		{name: "negative elapsed remains consumer policy", source: `{"Action":"pass","Elapsed":-1}`, wantFrames: 1},
		{name: "valid prefix remains provisional beside bad tail", source: "{\"Action\":\"pass\"}\n{", wantFrames: 1, wantGrammar: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			frames := 0
			err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader(row.source), OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { frames++; return nil }})
			got := runnercontrol.IsGoEventGrammarError(err)
			if got != row.wantGrammar || errors.Is(err, core.ErrGoEventGrammar) != row.wantGrammar || errors.Is(err, core.ErrJSONContract) != row.wantGrammar || frames != row.wantFrames || (err != nil) != row.wantGrammar {
				t.Fatalf("native source=%q grammar=%t error=%v frames=%d, want grammar=%t frames=%d and exact typed JSON ownership", row.source, got, err, frames, row.wantGrammar, row.wantFrames)
			}
		})
	}
}

func TestGoEventGrammarCannotHideNativeCause(t *testing.T) {
	t.Parallel()
	grammar := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader("{"), OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { return nil }})
	for _, cause := range []error{io.ErrClosedPipe, context.Canceled, core.ErrFilestoreSource, errors.Join(io.EOF, io.ErrClosedPipe)} {
		err := fmt.Errorf("native scope: %w", errors.Join(grammar, cause))
		if runnercontrol.IsGoEventGrammarError(err) || !errors.Is(err, cause) {
			t.Fatalf("mixed native grammar=%t error=%v, want false and retained cause=%v", runnercontrol.IsGoEventGrammarError(err), err, cause)
		}
	}
	for _, cause := range []error{nil, core.ErrJSONContract, core.ErrPrimitiveContract, io.ErrClosedPipe} {
		if runnercontrol.IsGoEventGrammarError(cause) {
			t.Fatalf("unowned cause=%v acquired Go grammar authority, want false", cause)
		}
	}
	for _, body := range []string{"{", `{"Action":"pass"}`} {
		for _, cause := range []error{io.ErrClosedPipe, errors.Join(io.EOF, io.ErrClosedPipe)} {
			source := iotest.DataErrReader(io.MultiReader(strings.NewReader(body), goEventFailureReader{cause: cause}))
			err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: source, OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { return nil }})
			if !errors.Is(err, io.ErrClosedPipe) || runnercontrol.IsGoEventGrammarError(err) {
				t.Fatalf("source=%q native cause=%v grammar=%t, want closed-pipe identity and no grammar-only authority", body, err, runnercontrol.IsGoEventGrammarError(err))
			}
		}
	}
	source := iotest.DataErrReader(strings.NewReader("{"))
	err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: source, OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { return nil }})
	if !runnercontrol.IsGoEventGrammarError(err) || !errors.Is(err, core.ErrGoEventGrammar) {
		t.Fatalf("bad record with actual EOF error=%v, want typed grammar-only refusal", err)
	}
}

func FuzzGoEventGrammarPreservesJoinedNativeCause(f *testing.F) {
	for _, data := range [][]byte{{}, {0}, {1}, {2, 0}, {3}, {0, 1}} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader("{"), OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { return nil }})
		want := true
		for _, value := range data {
			if value&1 != 0 {
				err = errors.Join(err, io.ErrClosedPipe)
				want = false
			} else {
				err = fmt.Errorf("native: %w", errors.Join(core.ErrPrimitiveContract, err))
			}
		}
		if got := runnercontrol.IsGoEventGrammarError(err); got != want || (!want && !errors.Is(err, io.ErrClosedPipe)) {
			t.Fatalf("native branches=%x grammar=%t error=%v, want %t and conserved native failures", data, got, err, want)
		}
	})
}

func BenchmarkGoEventGrammarCauseClassification(b *testing.B) {
	b.ReportAllocs()
	err := runnercontrol.ReadGoEventStream(b.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader("{"), OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { return nil }})
	if !errors.Is(err, core.ErrGoEventGrammar) {
		b.Fatalf("native setup error=%v, want %v", err, core.ErrGoEventGrammar)
	}
	for b.Loop() {
		if !runnercontrol.IsGoEventGrammarError(err) {
			b.Fatalf("native grammar query refused %v, want true", err)
		}
	}
}

func TestGoEventGrammarRejectsBorrowedCodecCause(t *testing.T) {
	t.Parallel()
	grammar := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader("{"), OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { return nil }})
	if !runnercontrol.IsGoEventGrammarError(grammar) {
		t.Fatalf("native grammar setup error=%v, want actual owned parser refusal", grammar)
	}
	for _, row := range []struct {
		name  string
		cause error
	}{
		{name: "syntactic wrapper cannot lend source ownership", cause: &jsontext.SyntacticError{Err: io.ErrClosedPipe}},
		{name: "semantic wrapper cannot lend source ownership", cause: &json.SemanticError{Err: io.ErrClosedPipe}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			joined := errors.Join(grammar, row.cause)
			if got := runnercontrol.IsGoEventGrammarError(joined); got || !errors.Is(joined, io.ErrClosedPipe) {
				t.Fatalf("borrowed codec cause type=%T grammar=%t error=%v, want false and conserved closed pipe", row.cause, got, joined)
			}
		})
	}
}

func TestGoEventGrammarOwnsNativeUnquoteCause(t *testing.T) {
	t.Parallel()
	wire := "\"" + string([]byte{0xff}) + "\""
	_, native := jsontext.AppendUnquote(nil, wire)
	var syntax *jsontext.SyntacticError
	if !errors.As(native, &syntax) || syntax.Err == nil {
		t.Fatalf("native unquote error=%v, want syntax refusal with a concrete cause", native)
	}
	body := "{\"Action\":\"output\",\"Output\":" + wire + "}"
	err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader(body), OnString: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { return nil }})
	if !runnercontrol.IsGoEventGrammarError(err) || !errors.Is(err, core.ErrGoEventGrammar) || !errors.Is(err, syntax.Err) {
		t.Fatalf("native unquote error=%v grammar=%t, want owned grammar and %v", err, runnercontrol.IsGoEventGrammarError(err), syntax.Err)
	}
}
