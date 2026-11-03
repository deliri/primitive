package jsonio_test

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestNativeIntegerObjectRetainsGoProjection(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		value map[string]any
	}{
		{"empty object is a document", map[string]any{}},
		{"native signed maximum", map[string]any{"integer": int64(math.MaxInt64)}},
		{"native signed minimum", map[string]any{"integer": int64(math.MinInt64)}},
		{"above floating precision", map[string]any{"integer": int64(1<<53 + 1)}},
		{"neutral native zero", map[string]any{"integer": int64(0)}},
		{"nested null and empty collections", map[string]any{"array": []any{nil, false, "123", int64(-7)}, "empty": map[string]any{}}},
		{"unicode ordering and escaping belongs to Go", map[string]any{"\U00010000": "a & b < c", "\ue000": "é\n\"\\"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			want, controlErr := jsonv2.Marshal(row.value, jsonv2.Deterministic(true))
			got, err := jsonio.MarshalNativeObject(row.value)
			if controlErr != nil || err != nil || !bytes.Equal(got, want) {
				t.Fatalf("native object bytes/error = %q/%v, want Go %q/%v", got, err, want, controlErr)
			}
			decoded, err := jsonio.DecodeNativeIntegerObject(got)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := jsonio.MarshalNativeObject(decoded)
			if err != nil || !bytes.Equal(roundTrip, want) {
				t.Fatalf("native object round trip = %q/%v, want exact %q", roundTrip, err, want)
			}
			if integer, exists := row.value["integer"]; exists && decoded["integer"] != integer {
				t.Fatalf("integer = %T(%v), want exact %T(%v)", decoded["integer"], decoded["integer"], integer, integer)
			}
		})
	}
	for _, value := range []map[string]any{nil, {"invalid": math.NaN()}, {"invalid": math.Inf(1)}} {
		got, err := jsonio.MarshalNativeObject(value)
		if !errors.Is(err, core.ErrJSONContract) || got != nil {
			t.Fatalf("unspellable object bytes/error = %q/%v, want nil/JSON refusal", got, err)
		}
	}
}

func TestNativeIntegerObjectRefusesBeforePublishingPartialValues(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		`null`, `[]`, `"object"`, ``, `{`, `{} {}`,
		`{"value":1.0}`, `{"value":1e0}`, `{"value":9223372036854775808}`, `{"value":-9223372036854775809}`,
		`{"value":1,"value":2}`, `{"nested":{"done":1,"broken":0.5}}`, `{"array":[1,2,3e0]}`,
		"{\"text\":\"\xff\"}", `{"text":"\ud800"}`,
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			got, err := jsonio.DecodeNativeIntegerObject([]byte(text))
			if !errors.Is(err, core.ErrJSONContract) || got != nil {
				t.Fatalf("refused native object = %+v/%v, want nil/JSON refusal", got, err)
			}
		})
	}
}

func FuzzNativeIntegerObjectSemanticClosure(f *testing.F) {
	seed, err := jsonio.MarshalNativeObject(map[string]any{"integer": int64(math.MaxInt64), "nested": []any{int64(math.MinInt64), nil, "é"}})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"integer":1e0}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := jsonio.DecodeNativeIntegerObject(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || got != nil {
				t.Fatalf("refusal published = %+v/%v, want nil/JSON contract", got, err)
			}
			return
		}
		wire, err := jsonio.MarshalNativeObject(got)
		if err != nil {
			t.Fatal(err)
		}
		control, err := jsonv2.Marshal(got, jsonv2.Deterministic(true))
		if err != nil || !bytes.Equal(wire, control) {
			t.Fatalf("native wire = %q, want Go %q, error=%v", wire, control, err)
		}
		second, err := jsonio.DecodeNativeIntegerObject(wire)
		if err != nil {
			t.Fatal(err)
		}
		repeated, err := jsonio.MarshalNativeObject(second)
		if err != nil || !bytes.Equal(repeated, wire) {
			t.Fatalf("native fixed point = %q/%v, want %q", repeated, err, wire)
		}
	})
}
