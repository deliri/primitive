package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestJSONCapturedValueOwnsBytesAcrossNativeCursorReuse(t *testing.T) {
	t.Parallel()
	canonical, err := jsonv2.Marshal(scalarObject{Name: "first", Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := jsonv2.Marshal(scalarObject{Name: "other", Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	source := io.MultiReader(bytes.NewReader(canonical), strings.NewReader(strings.Repeat(string(second), 257)))
	decoder, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: iotest.OneByteReader(source), NestingDepthMaximum: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, err := decoder.ReadValue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for range 257 {
		if err := decoder.SkipValue(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if err := decoder.SkipValue(t.Context()); err != io.EOF {
		t.Fatalf("cursor closure = %v, want EOF", err)
	}
	if !bytes.Equal(got, canonical) {
		t.Fatalf("owned capture after native buffer reuse = %q, want %q", got, canonical)
	}
}

func TestJSONDeclaredObjectBeforeWriteAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &output})
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonio.EncodeObject(t.Context(), encoder, scalarObject{}); !errors.Is(err, core.ErrSourceObservationContract) || output.Len() != 0 {
		t.Fatalf("owner refusal = %v, bytes %d, want typed refusal without output", err, output.Len())
	}
	if err := jsonio.EncodeObject(t.Context(), encoder, opaqueRootObject{Name: "bypass"}); !errors.Is(err, core.ErrJSONContract) || output.Len() != 0 {
		t.Fatalf("opaque hook = %v, bytes %d, want declared shape refusal", err, output.Len())
	}
	var pointer *scalarObject
	if err := jsonio.EncodeObject(t.Context(), encoder, pointer); !errors.Is(err, core.ErrJSONContract) || output.Len() != 0 {
		t.Fatalf("typed nil = %v, bytes %d", err, output.Len())
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := jsonio.EncodeObject(ctx, encoder, scalarObject{Name: "unused"}); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("cancelled = %v, bytes %d", err, output.Len())
	}
	if err := jsonio.EncodeObject(t.Context(), encoder, scalarObject{Name: "published", Count: 7}); err != nil || !jsontext.Value(output.Bytes()).IsValid() {
		t.Fatalf("positive object = %q/%v, want valid JSON", output.Bytes(), err)
	}
}

func FuzzJSONCursorCaptureSkipAndCustomMarshalClosure(f *testing.F) {
	var seed bytes.Buffer
	encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &seed})
	if err != nil {
		f.Fatal(err)
	}
	if err := jsonio.EncodeMarshaledObject(f.Context(), encoder, ownedCustomObject{Name: "seed"}); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes())
	f.Add([]byte(`{"a":1,"a":2}`))
	f.Add([]byte(`[{]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: bytes.NewReader(data), NestingDepthMaximum: core.JSONNestingDepthMaximum})
		if err != nil {
			t.Fatal(err)
		}
		got, err := cursor.ReadValue(t.Context())
		native := jsontext.NewDecoder(bytes.NewReader(data))
		want, wantErr := native.ReadValue()
		if err != nil {
			if got != nil || wantErr == nil || err != io.EOF && !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("capture refusal = %q/%v, native %v", got, err, wantErr)
			}
		} else if wantErr != nil || !bytes.Equal(got, want) || !jsontext.Value(got).IsValid() {
			t.Fatalf("capture = %q/%v, native %q/%v", got, err, want, wantErr)
		}
		skip, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: bytes.NewReader(data), NestingDepthMaximum: core.JSONNestingDepthMaximum})
		if err != nil {
			t.Fatal(err)
		}
		if gotErr := skip.SkipValue(t.Context()); (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("skip outcome = %v, native capture %v", gotErr, wantErr)
		}
		var out bytes.Buffer
		encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &out})
		if err != nil {
			t.Fatal(err)
		}
		marshalErr := jsonio.EncodeMarshaledObject(t.Context(), encoder, ownedCustomObject{Name: "owner", Wire: string(data)})
		whole := jsontext.Value(data)
		wantObject := whole.Kind() == jsontext.KindBeginObject && whole.IsValid()
		// The custom fixture's empty Wire deliberately selects its typed path.
		if len(data) == 0 {
			wantObject = true
		}
		if !wantObject {
			if !errors.Is(marshalErr, core.ErrJSONContract) || out.Len() != 0 {
				t.Fatalf("custom refusal = %v, bytes %d", marshalErr, out.Len())
			}
		} else if marshalErr != nil || !jsontext.Value(out.Bytes()).IsValid() {
			t.Fatalf("custom accepted = %q/%v", out.Bytes(), marshalErr)
		}
	})
}
