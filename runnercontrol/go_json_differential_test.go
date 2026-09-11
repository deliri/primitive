package runnercontrol

import (
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzGoEventStringProjectionMatchesStandardJSON(f *testing.F) {
	for _, value := range []string{"ordinary", "😀", strings.Repeat("a", 191) + "😀", "\\\"\n", strings.Repeat("x", 2048)} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		// The standard encoder supplies valid UTF-8/escape syntax. Invalid Go strings
		// are independently refused by it and cannot supply a valid provider event.
		data, err := json.Marshal(goTestEventWire{Action: "pass", Package: value})
		if err != nil {
			return
		}
		var parser goJSONStream
		var got goJSONProjection
		count := 0
		emit := func(event goJSONProjection) error { got = event; count++; return nil }
		for _, b := range data {
			if err := parser.consume(b, emit); err != nil {
				t.Fatalf("consume(encoded string) error = %v, want nil", err)
			}
		}
		if err := parser.finish(emit); err != nil {
			t.Fatalf("finish() error = %v, want nil", err)
		}
		if count != 1 || got.packageID != sha256.Sum256([]byte(value)) || got.packagePresent != (value != "") {
			t.Fatalf("string projection = %+v/count %d, want original UTF8 digest and presence/count 1", got, count)
		}
	})
}

func FuzzGoEventFlatGrammarMatchesStandardJSON(f *testing.F) {
	canonical, err := json.Marshal(goTestEventWire{Action: "pass", Package: "selected"})
	if err != nil {
		f.Fatalf("Marshal(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte(`{"Action":"pass","Package":"selected","Elapsed":1e309}`))
	f.Add([]byte(`{"Action":"pass","Package":"\ud800"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var parser goJSONStream
		count := 0
		emit := func(goJSONProjection) error { count++; return nil }
		var gotErr error
		for _, b := range data {
			if gotErr = parser.consume(b, emit); gotErr != nil {
				break
			}
		}
		if gotErr == nil {
			gotErr = parser.finish(emit)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("projection refusal = %v, want typed contract", gotErr)
			}
			return
		}
		if count != 1 {
			return
		} // This target also admits NDJSON and whitespace-only input.
		var decoded goTestEventWire
		if err := json.Unmarshal(data, &decoded); err != nil {
			var build goBuildEventWire
			if buildErr := json.Unmarshal(data, &build); buildErr != nil {
				t.Fatalf("stream accepted %q, standard JSON errors = %v/%v, want valid scalar document", data, err, buildErr)
			}
		}
	})
}

func TestGoJSONFloatNativeRangeAndLongSpelling(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, value string }{
		{"zero", "0"}, {"negative zero", "-0"},
		{"largest finite float", "1.7976931348623157e308"},
		{"rounding above finite float", "1.7976931348623159e308"},
		{"exponent overflow", "1e309"},
		{"zero with arbitrarily spelled exponent", "0e999999999999999999999"},
		{"underflow with arbitrarily spelled exponent", "1e-999999999999999999999"},
		{"long fraction compensated by exponent", "0." + strings.Repeat("0", 2048) + "1e2049"},
		{"long integer compensated by exponent", "1" + strings.Repeat("0", 2048) + "e-2048"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.value
			t.Parallel()
			data := []byte(`{"Action":"pass","Package":"selected","Elapsed":` + value + `}`)
			var decoded goTestEventWire
			wantErr := json.Unmarshal(data, &decoded)
			var parser goJSONStream
			emit := func(goJSONProjection) error { return nil }
			var gotErr error
			for _, b := range data {
				if gotErr = parser.consume(b, emit); gotErr != nil {
					break
				}
			}
			if gotErr == nil {
				gotErr = parser.finish(emit)
			}
			if (gotErr == nil) != (wantErr == nil) {
				t.Fatalf("number %s error = %v, standard decoder error = %v", value, gotErr, wantErr)
			}
		})
	}
}
