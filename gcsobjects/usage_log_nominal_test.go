package gcsobjects

import (
	"encoding"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

var _ core.OffWireEnum = GCSUsageIPv4

// Exhaust the complete byte domain; repeated unknown arms are not claimed as
// distinct hostile classes. This is a closed domain, not an invented JSON API.
func TestGCSUsageIPTypeCompleteDomain(t *testing.T) {
	t.Parallel()
	for raw := 0; raw < 256; raw++ {
		t.Run(strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			got := GCSUsageIPType(raw)
			if raw == 1 || raw == 2 {
				if got.Validate() != nil || !got.IsValid() || got.String() != strconv.Itoa(raw) {
					t.Fatalf("IP type=%d valid=%v text=%q,want documented closed value", raw, got.IsValid(), got.String())
				}
				return
			}
			if !errors.Is(got.Validate(), core.ErrObjectStoreSource) || got.IsValid() || got.String() != "" {
				t.Fatalf("IP type=%d valid=%v text=%q,want typed refusal/false/empty", raw, got.IsValid(), got.String())
			}
		})
	}
	for _, receiver := range []reflect.Type{reflect.TypeFor[GCSUsageIPType](), reflect.TypeFor[*GCSUsageIPType]()} {
		for _, wire := range []reflect.Type{reflect.TypeFor[json.Marshaler](), reflect.TypeFor[json.Unmarshaler](), reflect.TypeFor[encoding.TextMarshaler](), reflect.TypeFor[encoding.TextUnmarshaler](), reflect.TypeFor[encoding.BinaryMarshaler](), reflect.TypeFor[encoding.BinaryUnmarshaler]()} {
			if receiver.Implements(wire) {
				t.Fatalf("receiver=%v implements %v,want in-process enum without generic wire methods", receiver, wire)
			}
		}
	}
}

// net/http's real request constructor independently admits RFC HTTP tokens; no
// HTTP execution or provider call occurs. Empty defaults in net/http are not an
// observed method and must remain an explicit nominal refusal here.
func FuzzGCSUsageMethodSemanticClosure(f *testing.F) {
	for _, value := range []string{http.MethodGet, http.MethodTrace, http.MethodConnect, "PROPFIND"} {
		method, err := ParseGCSUsageMethod(value)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(method.String())
	}
	f.Add("")
	f.Add("BAD METHOD")
	f.Add("GET\x00")
	f.Add("méthod")
	f.Fuzz(func(t *testing.T, input string) {
		value := input[:min(len(input), 64*1024)]
		request, referenceErr := http.NewRequest(value, "https://example.invalid/", nil)
		got, err := ParseGCSUsageMethod(value)
		if value == "" || referenceErr != nil {
			if !errors.Is(err, core.ErrObjectStoreSource) || got != (GCSUsageMethod{}) {
				t.Fatalf("method admission=%v/%v,want zero/typed token refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != value || request.Method != value {
			t.Fatalf("method admission=%q/%v,want exact standard token %q", got.String(), err, value)
		}
		round, roundErr := ParseGCSUsageMethod(got.String())
		if roundErr != nil || round != got {
			t.Fatalf("nominal roundtrip=%v/%v,want exact %v/nil", round, roundErr, got)
		}
	})
}
