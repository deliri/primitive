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

type readerOwnedRoot struct {
	Value int `json:"value"`
}

func (r *readerOwnedRoot) UnmarshalJSON([]byte) error { r.Value = 7; return nil }

func TestProjectionReaderSingleDocumentNativeClosure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		body     string
		want     canonicalProjection
		refused  bool
		multiple bool
	}{
		{body: `{"z":9007199254740993,"a":"x"}`, want: canonicalProjection{Z: 9007199254740993, A: "x"}},
		{body: `{"a":"partial","z":"wrong"}`, refused: true},
		{body: `{"z":1,"z":2}`, refused: true},
		{body: `{"z":1,"unknown":2}`, refused: true},
		{body: `null`, refused: true},
		{body: `{} {}`, refused: true, multiple: true},
		{body: `{} {`, refused: true, multiple: true},
	} {
		got, err := jsonio.DecodeProjectionReader[canonicalProjection](t.Context(), strings.NewReader(test.body), jsonio.ProjectionReadPolicy{})
		if test.refused {
			if got != (canonicalProjection{}) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("body=%q value=%+v err=%v", test.body, got, err)
			}
			var multiple *jsonio.MultipleValuesError
			if test.multiple && !errors.As(err, &multiple) {
				t.Fatalf("multiple-value identity lost: %v", err)
			}
		} else if err != nil || got != test.want {
			t.Fatalf("body=%q value=%+v err=%v", test.body, got, err)
		}
	}
	got, err := jsonio.DecodeProjectionReader[canonicalProjection](t.Context(), strings.NewReader(`null`), jsonio.ProjectionReadPolicy{RootNulls: jsonio.RootNullNativeZero})
	if err != nil || got != (canonicalProjection{}) {
		t.Fatalf("explicit neutral null=%+v %v", got, err)
	}
	for _, body := range []string{`{"a":"partial",`, `{"a":"complete","z":1}`} {
		got, err := jsonio.DecodeProjectionReader[canonicalProjection](t.Context(), io.MultiReader(strings.NewReader(body), iotest.ErrReader(io.ErrClosedPipe)), jsonio.ProjectionReadPolicy{})
		if got != (canonicalProjection{}) || !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("late native body=%q value=%+v err=%v", body, got, err)
		}
	}
	cancelled, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	if got, err := jsonio.DecodeProjectionReader[canonicalProjection](cancelled, strings.NewReader(`{}`), jsonio.ProjectionReadPolicy{}); got != (canonicalProjection{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("cancelled=%+v %v", got, err)
	}
}

func TestProjectionReaderExplicitRootHookAdmissionAndNativeLexicalGate(t *testing.T) {
	t.Parallel()
	if got, err := jsonio.DecodeProjectionReader[readerOwnedRoot](t.Context(), strings.NewReader(`{}`), jsonio.ProjectionReadPolicy{}); got != (readerOwnedRoot{}) || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("unadmitted root=%+v %v", got, err)
	}
	policy := jsonio.ProjectionReadPolicy{Root: jsonio.RootOwnedMethods}
	if got, err := jsonio.DecodeProjectionReader[readerOwnedRoot](t.Context(), strings.NewReader(`{}`), policy); err != nil || got.Value != 7 {
		t.Fatalf("admitted hook=%+v %v", got, err)
	}
	for _, body := range [][]byte{[]byte(`{"x":1,"x":2}`), []byte(`{"x":`), {'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}} {
		got, err := jsonio.DecodeProjectionReader[readerOwnedRoot](t.Context(), bytes.NewReader(body), policy)
		if got != (readerOwnedRoot{}) || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("opaque lexical gate body=%q value=%+v err=%v", body, got, err)
		}
		if got, err := jsonio.DecodeValue[readerOwnedRoot](body, policy); got != (readerOwnedRoot{}) || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("borrowed-byte lexical gate=%+v %v", got, err)
		}
	}
}

func TestProjectionReaderPolicyClosedEnums(t *testing.T) {
	t.Parallel()
	for value := 0; value < 256; value++ {
		policy := jsonio.RootNullPolicy(value)
		if policy.IsValid() != (value < 2) {
			t.Fatalf("null arm %d admitted=%t", value, policy.IsValid())
		}
	}
	if jsonio.IsSemanticError(errors.ErrUnsupported) {
		t.Fatal("unrelated failure classified as typed JSON error")
	}
	_, err := jsonio.DecodeProjectionReader[canonicalProjection](t.Context(), strings.NewReader(`{"z":"wrong"}`), jsonio.ProjectionReadPolicy{})
	if !jsonio.IsSemanticError(err) {
		t.Fatalf("native typed decode identity lost: %v", err)
	}
}

func TestValidateBytesConservesTruncatedDepthAndNativeClosureIdentities(t *testing.T) {
	t.Parallel()
	for _, depth := range []int{63, 64, 65} {
		err := jsonio.ValidateBytes([]byte(strings.Repeat("[", depth)))
		if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("depth=%d native truncation lost: %v", depth, err)
		}
		var observed *jsonio.DepthError
		if errors.As(err, &observed) != (depth > 64) {
			t.Fatalf("depth=%d depth observation=%v err=%v", depth, observed, err)
		}
	}
	err := jsonio.ValidateBytes([]byte(`{} {`))
	var multiple *jsonio.MultipleValuesError
	if !errors.As(err, &multiple) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("trailing truncation identities lost: %v", err)
	}
}

func FuzzProjectionReaderMatchesNativeDeclaredValue(f *testing.F) {
	for _, body := range []string{`{"z":9007199254740993,"a":"x"}`, `{"z":"wrong"}`, `{} {}`, `null`, `{"x":1}`, `{"z":1,"z":2}`} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var want canonicalProjection
		nativeErr := jsonv2.Unmarshal(data, &want, jsonv2.RejectUnknownMembers(true))
		admitted := nativeErr == nil && jsontext.Value(data).IsValid() && jsontext.Value(data).Kind() == jsontext.KindBeginObject
		got, err := jsonio.DecodeProjectionReader[canonicalProjection](t.Context(), bytes.NewReader(data), jsonio.ProjectionReadPolicy{})
		if !admitted {
			if err == nil || got != (canonicalProjection{}) {
				t.Fatalf("unadmitted source=%q output=%+v err=%v", data, got, err)
			}
			return
		}
		if err != nil || got != want {
			t.Fatalf("native=%+v err=%v primitive=%+v err=%v", want, nativeErr, got, err)
		}
		borrowed, err := jsonio.DecodeValue[canonicalProjection](data, jsonio.ProjectionReadPolicy{})
		if err != nil || borrowed != want {
			t.Fatalf("borrowed=%+v %v want=%+v", borrowed, err, want)
		}
	})
}
