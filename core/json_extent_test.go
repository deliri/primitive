package core

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
)

type extentDocument struct {
	Values []string `json:"values"`
	Tail   string   `json:"tail"`
}

func (d extentDocument) Validate() error { return nil }

// Increasing a caller-owned byte/array budget must not weaken strict decoding.
// Poison at the tail must be checked even when the valid prefix crosses the
// old defaults; no accepted prefix or partially populated value may escape.
func TestJSONCallerExtentPreservesStrictTailValidation(t *testing.T) {
	t.Parallel()
	limits := DefaultStrictJSONLimits()
	var err error
	limits.DocumentMaximumBytes, err = NewByteCount(math.MaxInt - 1)
	if err != nil {
		t.Fatal(err)
	}
	limits.ArrayItemMaximum = math.MaxUint32
	source := extentDocument{Values: make([]string, jsonArrayItemCountMaximum+1), Tail: strings.Repeat("x", JSONDocumentMaximumBytes+1)}
	for i := range source.Values {
		source.Values[i] = "value"
	}
	encoded, err := MarshalCanonicalJSONDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		input []byte
		want  error
	}{
		{name: "all items and final byte retained", input: encoded},
		{name: "unknown field after large prefix", input: append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"unknown":true}`)...), want: ErrJSONContract},
		{name: "duplicate field after large prefix", input: append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"tail":"replacement"}`)...), want: ErrJSONContract},
		{name: "second document after large prefix", input: append(bytes.Clone(encoded), []byte(`{}`)...), want: ErrJSONContract},
		{name: "missing final delimiter", input: encoded[:len(encoded)-1], want: ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeStrictJSON[extentDocument](bytes.NewReader(tc.input), limits)
			if !errors.Is(err, tc.want) {
				t.Fatalf("decode = %v, want %v", err, tc.want)
			}
			if tc.want != nil {
				if len(got.Values) != 0 || got.Tail != "" {
					t.Fatalf("refused document = %d values/%d tail bytes, want zero", len(got.Values), len(got.Tail))
				}
				return
			}
			if len(got.Values) != len(source.Values) || got.Tail != source.Tail {
				t.Fatalf("document = %d values/%d tail bytes, want %d/%d", len(got.Values), len(got.Tail), len(source.Values), len(source.Tail))
			}
			for i, v := range got.Values {
				if v != source.Values[i] {
					t.Fatalf("item %d changed", i)
				}
			}
		})
	}
	t.Run("reader failure after complete large JSON is retained", func(t *testing.T) {
		t.Parallel()
		got, err := DecodeStrictJSON[extentDocument](io.MultiReader(bytes.NewReader(encoded), extentFailureReader{}), limits)
		if !errors.Is(err, io.ErrUnexpectedEOF) || len(got.Values) != 0 || got.Tail != "" {
			t.Fatalf("reader failure = %v with %d retained items", err, len(got.Values))
		}
	})
}

type extentFailureReader struct{}

func (extentFailureReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
