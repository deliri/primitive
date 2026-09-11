package process

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type planIngressCase struct {
	name           string
	change         func(*planWire)
	damage         func([]byte) []byte
	omitZero       bool
	canonicalBytes int
	wantErr        error
}

func planIngressCases() []planIngressCase {
	return []planIngressCase{
		{name: "positive/order and values survive"},
		{name: "positive/whitespace does not change intent", damage: func(b []byte) []byte { return append(append([]byte(" \n\t"), b...), '\r') }},
		{name: "positive/escaped argument remains exact", change: func(w *planWire) { w.Arguments = []string{"\"\\\r\n\t\x01"} }},
		{name: "positive/unicode argument remains exact", change: func(w *planWire) { w.Arguments = []string{"日本語", "é", "e\u0301", "😀"} }},
		{name: "positive/environment equals remains a value", change: func(w *planWire) { w.Environment = []string{"V=a=b=c"} }},
		{name: "positive/omitted empty environment stays exact", change: func(w *planWire) { w.Environment = nil }, omitZero: true},
		{name: "positive/omitted empty argv stays empty", change: func(w *planWire) { w.Arguments = nil }, omitZero: true},
		{name: "neutral/both omitted vectors stay empty", change: func(w *planWire) { w.Arguments = nil; w.Environment = nil }, omitZero: true},
		{name: "boundary/empty argument is not absent", change: func(w *planWire) { w.Arguments = []string{""} }},
		{name: "boundary/empty value is not absent", change: func(w *planWire) { w.Environment = []string{"V="} }},
		{name: "negative/unknown version preserves receiver", change: func(w *planWire) { w.SchemaVersion++ }, wantErr: core.ErrProcessContract},
		{name: "negative/omitted command preserves receiver", change: func(w *planWire) { w.Command = core.AbsolutePath{} }, omitZero: true, wantErr: core.ErrProcessContract},
		{name: "negative/omitted directory preserves receiver", change: func(w *planWire) { w.WorkingDirectory = core.AbsolutePath{} }, omitZero: true, wantErr: core.ErrProcessContract},
		{name: "negative/unknown isolation preserves receiver", change: func(w *planWire) { w.Isolation = IsolationUnknown.String() }, wantErr: core.ErrProcessContract},
		{name: "negative/unknown cancellation preserves receiver", change: func(w *planWire) { w.CancelSignal = CancelSignalUnknown.String() }, wantErr: core.ErrProcessContract},
		{name: "negative/NUL argument preserves receiver", change: func(w *planWire) { w.Arguments = []string{"before", "bad\x00arg", "after"} }, wantErr: core.ErrProcessContract},
		{name: "negative/NUL environment preserves receiver", change: func(w *planWire) { w.Environment = []string{"V=bad\x00value"} }, wantErr: core.ErrProcessContract},
		{name: "negative/duplicate environment preserves receiver", change: func(w *planWire) { w.Environment = []string{"V=old", "V=new"} }, wantErr: core.ErrProcessContract},
		{name: "negative/environment without separator preserves receiver", change: func(w *planWire) { w.Environment = []string{"missing"} }, wantErr: core.ErrProcessContract},
		{name: "negative/empty environment name preserves receiver", change: func(w *planWire) { w.Environment = []string{"=value"} }, wantErr: core.ErrProcessContract},
		{name: "negative/omitted output budget preserves receiver", change: func(w *planWire) { w.OutputPolicy.Maximum = core.ByteCount{} }, omitZero: true, wantErr: core.ErrProcessContract},
		{name: "negative/omitted wait budget preserves receiver", change: func(w *planWire) { w.WaitDelay = temporal.Duration{} }, omitZero: true, wantErr: core.ErrProcessContract},
		{name: "negative/unknown field preserves receiver", damage: func(b []byte) []byte { return append([]byte(`{"unowned":true,`), b[1:]...) }, wantErr: core.ErrProcessContract},
		{name: "negative/duplicate fields preserve receiver", damage: func(b []byte) []byte { return append(append(bytes.Clone(b[:len(b)-1]), ','), b[1:]...) }, wantErr: core.ErrProcessContract},
		{name: "negative/trailing document preserves receiver", damage: func(b []byte) []byte { return append(b, []byte("{}")...) }, wantErr: core.ErrProcessContract},
		{name: "negative/truncated document preserves receiver", damage: func(b []byte) []byte { return b[:len(b)-1] }, wantErr: core.ErrProcessContract},
		{name: "negative/null document preserves receiver", damage: func([]byte) []byte { return []byte("null") }, wantErr: core.ErrProcessContract},
		{name: "negative/array document preserves receiver", damage: func(b []byte) []byte { return append(append([]byte{'['}, b...), ']') }, wantErr: core.ErrProcessContract},
		{name: "negative/empty input preserves receiver", damage: func([]byte) []byte { return nil }, wantErr: core.ErrProcessContract},
		{name: "negative/invalid UTF8 preserves receiver", damage: func(b []byte) []byte { return append(b, 0xff) }, wantErr: core.ErrProcessContract},
		{name: "boundary/omitted environment expands below publication cap", canonicalBytes: core.JSONDocumentMaximumBytes - 1, omitZero: true},
		{name: "boundary/omitted environment expands to publication cap", canonicalBytes: core.JSONDocumentMaximumBytes, omitZero: true},
		{name: "boundary/omitted environment expands above publication cap", canonicalBytes: core.JSONDocumentMaximumBytes + 1, omitZero: true, wantErr: core.ErrProcessContract},
	}
}

func TestPlanJSONIngressLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range planIngressCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver, err := planContractFixture(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			before := receiver
			before.Arguments = slices.Clone(receiver.Arguments)
			before.Environment.Variables = slices.Clone(receiver.Environment.Variables)
			wire, input, err := planIngressFixture(receiver, tc)
			if err != nil {
				t.Fatal(err)
			}
			inputBefore := bytes.Clone(input)
			err = receiver.UnmarshalJSON(input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decode error=%v, want %v", err, tc.wantErr)
			}
			if !bytes.Equal(input, inputBefore) {
				t.Fatalf("decoder changed caller bytes: got=%x want=%x", input, inputBefore)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, core.ErrJSONContract) || receiver.Command != before.Command || receiver.WorkingDirectory != before.WorkingDirectory || receiver.OutputPolicy != before.OutputPolicy || receiver.WaitDelay != before.WaitDelay || receiver.Containment != before.Containment || receiver.SchemaVersion != before.SchemaVersion || receiver.Environment.Mode != before.Environment.Mode || !slices.Equal(receiver.Arguments, before.Arguments) || !slices.Equal(receiver.Environment.Variables, before.Environment.Variables) || (receiver.Arguments == nil) != (before.Arguments == nil) || (receiver.Environment.Variables == nil) != (before.Environment.Variables == nil) {
					t.Fatalf("refusal lost identity or changed receiver: %v", err)
				}
				return
			}
			arguments := make([]string, len(receiver.Arguments))
			for i, argument := range receiver.Arguments {
				value, err := argument.Value()
				if err != nil {
					t.Fatal(err)
				}
				arguments[i] = value
			}
			environment, projectionErr := receiver.Environment.Strings()
			if projectionErr != nil || receiver.Validate() != nil || receiver.Command != wire.Command || receiver.WorkingDirectory != wire.WorkingDirectory || receiver.OutputPolicy != wire.OutputPolicy || receiver.WaitDelay != wire.WaitDelay || receiver.SchemaVersion != wire.SchemaVersion || receiver.Containment.Isolation.String() != wire.Isolation || receiver.Containment.CancelSignal.String() != wire.CancelSignal || receiver.Environment.Mode != EnvironmentModeExact || !slices.Equal(arguments, wire.Arguments) || !slices.Equal(environment, wire.Environment) {
				t.Fatalf("accepted document changed intent: %v", projectionErr)
			}
			canonical, err := receiver.MarshalJSON()
			if err != nil || len(canonical) > core.JSONDocumentMaximumBytes || tc.canonicalBytes != 0 && len(canonical) != tc.canonicalBytes {
				t.Fatalf("accepted document cannot publish within exact budget: %d bytes, %v", len(canonical), err)
			}
			var next Plan
			if err := next.UnmarshalJSON(canonical); err != nil {
				t.Fatal(err)
			}
			again, err := next.MarshalJSON()
			if err != nil || !bytes.Equal(canonical, again) {
				t.Fatalf("canonical projection is unstable: %v", err)
			}
		})
	}
}

// The ordinary seed comes through production construction and publication.
// Go's encoder makes controlled wire changes, including omission of zero
// fields. It deliberately does not impose Primitive's document cap here.
func planIngressFixture(p Plan, tc planIngressCase) (planWire, []byte, error) {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return planWire{}, nil, err
	}
	var wire planWire
	if err := json.Unmarshal(encoded, &wire); err != nil {
		return planWire{}, nil, err
	}
	if tc.change != nil {
		tc.change(&wire)
	}
	if tc.canonicalBytes != 0 {
		wire.Environment = nil
		wire.Arguments = []string{""}
		base, err := json.Marshal(wire)
		if err != nil {
			return planWire{}, nil, err
		}
		wire.Arguments = []string{strings.Repeat("x", tc.canonicalBytes-len(base))}
	}
	encoded, err = json.Marshal(wire, json.OmitZeroStructFields(tc.omitZero))
	if err != nil {
		return planWire{}, nil, err
	}
	if tc.damage != nil {
		encoded = tc.damage(encoded)
	}
	return wire, encoded, nil
}
