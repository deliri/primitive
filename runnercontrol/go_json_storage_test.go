package runnercontrol

import (
	json "encoding/json/v2"
	"reflect"
	"strconv"
	"testing"
)

// This structural ratchet guards storage shape; behavioral streaming tests
// independently prove the windows can consume inputs larger than themselves.
func TestGoJSONParserStorageHasNoGrowingInputCarrier(t *testing.T) {
	t.Parallel()
	for _, typ := range []reflect.Type{reflect.TypeFor[goJSONStream](), reflect.TypeFor[goJSONFloat](), reflect.TypeFor[goJSONProjection](), reflect.TypeFor[goBenchmarkStream]()} {
		for field := range typ.Fields() {
			switch field.Type.Kind() {
			case reflect.Slice, reflect.Map, reflect.String:
				t.Fatalf("%s.%s has growing carrier %s, want fixed storage or owned nominal value", typ.Name(), field.Name, field.Type)
			}
		}
	}
}

func FuzzGoEventElapsedNativeFloatAgreement(f *testing.F) {
	for _, value := range []float64{0, 1, -1, 0.125, 1.7976931348623157e308, 5e-324} {
		f.Add(strconv.FormatFloat(value, 'g', -1, 64))
	}
	f.Add("1.7976931348623159e308")
	f.Add("0e999999999999999999999999999999999")
	f.Add("1e-99999999999999999999999999999999")
	f.Fuzz(func(t *testing.T, number string) {
		var native float64
		nativeErr := json.Unmarshal([]byte(number), &native)
		// Null is accepted by the generic decoder but isn't Go's numeric field.
		// Other nonnumeric JSON representations aren't this scalar domain either.
		if len(number) == 0 || number[0] != '-' && (number[0] < '0' || number[0] > '9') {
			return
		}
		for _, character := range number {
			if (character < '0' || character > '9') && character != '.' && character != 'e' && character != 'E' && character != '+' && character != '-' {
				return
			}
		}
		var parser goJSONStream
		count := 0
		emit := func(goJSONProjection) error { count++; return nil }
		var parseErr error
		for _, part := range []string{`{"Elapsed":`, number, "}"} {
			for index := 0; index < len(part); index++ {
				parseErr = parser.consume(part[index], emit)
				if parseErr != nil {
					break
				}
			}
			if parseErr != nil {
				break
			}
		}
		if parseErr == nil {
			parseErr = parser.finish(emit)
		}
		if (parseErr == nil) != (nativeErr == nil) {
			t.Fatalf("elapsed %q acceptance = %v, native float acceptance = %v", number, parseErr, nativeErr)
		}
		if parseErr == nil && count != 1 {
			t.Fatalf("projection count = %d, want 1", count)
		}
	})
}
