package firestoredocuments

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// The native projection must spell exactly what the canonical whole-document
// encoder spells for the same shape, so extents and member order are stable
// across the SDK edge. The encoder is the control; the projection is the subject.
func TestNativeProjectionMatchesCanonicalEncoderControl(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		native any
	}{
		{"null", nil}, {"true", true}, {"false", false}, {"empty string", ""},
		{"escaped unicode string", "Élaine\n\"日本語\t\u0001\\"},
		{"integer bounds", []any{int64(0), int64(math.MinInt64), int64(math.MaxInt64), int64(-1)}},
		{"bytes", []byte{0, 255, 7}},
		{"empty containers", map[string]any{"a": []any{}, "b": map[string]any{}}},
		{"member order is UTF-16", map[string]any{"b": int64(1), "a": int64(2), "": int64(3), "\U0001F600": int64(4), "Z": int64(5), "aa": int64(6)}},
		{"nested rows", map[string]any{"rows": []any{map[string]any{"value": int64(1)}, map[string]any{"value": int64(2), "name": "x"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := canonicalControl(t, tc.native)
			got, err := nativeToJSON(t.Context(), tc.native, true)
			if err != nil || string(got) != string(want) {
				t.Fatalf("nativeToJSON = %s/%v, want control %s", got, err, want)
			}
		})
	}
}

func TestNativeProjectionRefusesForeignKindsAndDoublesWhenIntegerOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		native      any
		integerOnly bool
	}{
		{"integer-only refuses a double", map[string]any{"value": float64(1)}, true},
		{"nested integer-only refuses a double", map[string]any{"rows": []any{float64(0)}}, true},
		{"single precision is not a JSON kind", float32(1), false},
		{"time is not a JSON kind", time.Unix(0, 0), false},
		{"struct is not a JSON kind", struct{ A int }{1}, false},
		{"not a number has no JSON spelling", math.NaN(), false},
		{"infinity has no JSON spelling", math.Inf(1), false},
		{"int32 is not the SDK integer kind", int32(1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := nativeToJSON(t.Context(), tc.native, tc.integerOnly)
			if !errors.Is(err, core.ErrJSONContract) || got != nil {
				t.Fatalf("nativeToJSON = %s/%v, want nil and ErrJSONContract", got, err)
			}
		})
	}
}

func TestNativeDoubleTextMatchesCanonicalEncoderControl(t *testing.T) {
	t.Parallel()
	for _, value := range []float64{0, math.Copysign(0, -1), 1, -1, 2.5, 1e-7, 1e-6, 123456789.125, 1e20, 1e21, 1.5e300, math.SmallestNonzeroFloat64, math.MaxFloat64, 0.1, 100, 1e6, 123e-9} {
		want, err := core.MarshalCanonicalJSONDocument(value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := nativeDoubleText(value)
		if err != nil || got != string(want) {
			t.Fatalf("nativeDoubleText(%g) = %q/%v, want control %q", value, got, err, want)
		}
	}
}

func FuzzNativeDoubleTextSemanticClosure(f *testing.F) {
	for _, seed := range []float64{0, 1, -1, 1e-7, 1e21, 0.3, 7e22, 5e-324} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value float64) {
		want, wantErr := core.MarshalCanonicalJSONDocument(value)
		got, err := nativeDoubleText(value)
		if (wantErr != nil) != (err != nil) || err == nil && got != string(want) {
			t.Fatalf("nativeDoubleText(%v) = %q/%v, want control %q/%v", value, got, err, want, wantErr)
		}
	})
}

func TestJSONToNativeIntegerShapeClosure(t *testing.T) {
	t.Parallel()
	native := map[string]any{"owner": "x", "rows": []any{map[string]any{"value": int64(math.MinInt64)}, int64(math.MaxInt64), nil, true, false, "s"}, "empty": map[string]any{}, "none": []any{}}
	data, err := nativeToJSON(t.Context(), native, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsonToNative(t.Context(), data)
	if err != nil || !reflect.DeepEqual(got, native) {
		t.Fatalf("jsonToNative round trip = %#v/%v, want %#v", got, err, native)
	}
	for _, tc := range []struct {
		name string
		data string
	}{
		{"fraction", `{"v":1.5}`}, {"exponent", `{"v":1e3}`}, {"overflow", `{"v":9223372036854775808}`}, {"negative zero fraction", `{"v":-0.0}`},
		{"duplicate member", `{"v":1,"v":2}`}, {"trailing value", `{"v":1} 2`}, {"truncated", `{"v":1`}, {"empty", ``}, {"invalid utf8", "{\"v\":\"\xff\"}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonToNative(t.Context(), []byte(tc.data))
			if !errors.Is(err, core.ErrJSONContract) || got != nil {
				t.Fatalf("jsonToNative(%q) = %#v/%v, want nil and ErrJSONContract", tc.data, got, err)
			}
		})
	}
	if _, _, err := integerJSONWriteValue(t.Context(), nonObjectRecord("scalar")); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("non-object record projection = %v, want ErrJSONContract", err)
	}
}

// canonicalControl is the oracle: Go's whole-document encoder canonicalized by
// jsonio (UTF-16 member order, exact integer spellings, RFC 8785 doubles).
func canonicalControl(t testing.TB, native any) []byte {
	t.Helper()
	encoded, err := core.MarshalCanonicalJSONDocument(native)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := jsonio.Canonicalize(encoded, jsonio.CanonicalPolicy{NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

type nonObjectRecord string

func (nonObjectRecord) Validate() error { return nil }

func FuzzNativeProjectionSemanticClosure(f *testing.F) {
	f.Add(`{"owner":"x","value":1}`)
	f.Add(`{"rows":[{"value":-9223372036854775808},null,true,"s",[]]}`)
	f.Add(`[]`)
	f.Add(`"é"`)
	f.Fuzz(func(t *testing.T, data string) {
		native, err := jsonToNative(t.Context(), []byte(data))
		if err != nil {
			return
		}
		canonical, err := nativeToJSON(t.Context(), native, true)
		if err != nil {
			t.Fatalf("admitted native %#v cannot be projected: %v", native, err)
		}
		again, err := jsonToNative(t.Context(), canonical)
		if err != nil || !reflect.DeepEqual(again, native) {
			t.Fatalf("projection is not closed: %s -> %#v -> %s -> %#v/%v", data, native, canonical, again, err)
		}
		if control := canonicalControl(t, native); string(control) != string(canonical) {
			t.Fatalf("projection %s differs from control %s", canonical, control)
		}
	})
}
