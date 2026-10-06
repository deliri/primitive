package jsonio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

type scalarObject struct {
	Name  string
	Count uint64
}

func (o scalarObject) Validate() error {
	if o.Name == "" {
		return core.ErrSourceObservationContract
	}
	return nil
}

func TestJSONObjectStreamClosedShapeAndRefusal(t *testing.T) {
	t.Parallel()
	first := scalarObject{Name: "one", Count: 1}
	var encoded bytes.Buffer
	writer, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: &encoded})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Encode(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		count      int
		refuse     bool
	}{
		{"empty", "", 0, false}, {"whitespace", " \t\n", 0, false},
		{"canonical", encoded.String(), 1, false}, {"adjacent objects", encoded.String() + encoded.String(), 2, false},
		{"root null", "null", 0, true}, {"root array", "[]", 0, true}, {"root string", `"x"`, 0, true},
		{"root number", "1", 0, true}, {"root true", "true", 0, true}, {"root false", "false", 0, true},
		{"unknown member", `{"Name":"one","extra":1}`, 0, true}, {"case variant", `{"name":"one"}`, 0, true},
		{"duplicate member", `{"Name":"one","Name":"two"}`, 0, true},
		{"escaped duplicate", `{"Name":"one","\u004eame":"two"}`, 0, true},
		{"missing meaning", `{}`, 0, true}, {"empty meaning", `{"Name":""}`, 0, true},
		{"null required meaning", `{"Name":null}`, 0, true}, {"wrong name type", `{"Name":1}`, 0, true},
		{"null count cannot become zero", `{"Name":"one","Count":null}`, 0, true},
		{"wrong count type", `{"Name":"one","Count":"1"}`, 0, true},
		{"negative count", `{"Name":"one","Count":-1}`, 0, true},
		{"fraction count", `{"Name":"one","Count":1.5}`, 0, true},
		{"count overflow", `{"Name":"one","Count":18446744073709551616}`, 0, true},
		{"array instead of name", `{"Name":[]}`, 0, true}, {"object instead of name", `{"Name":{}}`, 0, true},
		{"truncated", `{"Name":"one"`, 0, true}, {"unterminated string", `{"Name":"one}`, 0, true},
		{"invalid escape", `{"Name":"\q"}`, 0, true}, {"invalid surrogate", `{"Name":"\ud800"}`, 0, true},
		{"invalid UTF8", "{\"Name\":\"\xff\"}", 0, true},
		{"trailing comma", `{"Name":"one",}`, 0, true}, {"missing colon", `{"Name" "one"}`, 0, true},
		{"valid prefix malformed tail", encoded.String() + "broken", 1, true},
		{"valid prefix truncated tail", encoded.String() + "{", 1, true},
		{"valid prefix null tail", encoded.String() + "null", 1, true},
		{"reordered fields", `{"Count":1,"Name":"one"}`, 1, false},
		{"escaped known field", `{"N\u0061me":"one","Count":1}`, 1, false},
		{"whitespace in object", " { \"Name\" : \"one\" , \"Count\" : 1 } ", 1, false},
		{"uppercase name member", `{"NAME":"one"}`, 0, true},
		{"uppercase count member", `{"Name":"one","COUNT":1}`, 0, true},
		{"empty unknown key", `{"Name":"one","":1}`, 0, true},
		{"boolean name", `{"Name":true}`, 0, true},
		{"false name", `{"Name":false}`, 0, true},
		{"fractional name", `{"Name":1.5}`, 0, true},
		{"array count", `{"Name":"one","Count":[]}`, 0, true},
		{"object count", `{"Name":"one","Count":{}}`, 0, true},
		{"boolean count", `{"Name":"one","Count":true}`, 0, true},
		{"incomplete count", `{"Name":"one","Count":`, 0, true},
		{"wrong closing delimiter", `{"Name":"one"]`, 0, true},
		{"valid prefix invalid meaning tail", encoded.String() + `{}`, 1, true},
		{"single quoted name", `{'Name':'one'}`, 0, true},
		{"literal newline in name", "{\"Name\":\"one\n\"}", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seen, refused := 0, false
			for object, err := range jsonio.Objects[scalarObject](t.Context(), jsonio.ObjectSourceRequest{Source: iotest.OneByteReader(strings.NewReader(tc.body))}) {
				if err != nil {
					if refused || !errors.Is(err, core.ErrJSONContract) || object != (scalarObject{}) {
						t.Fatalf("refusal = (%+v,%v), want zero typed refusal once", object, err)
					}
					refused = true
				} else {
					if refused || object != first || object.Validate() != nil {
						t.Fatalf("object = %+v, want exact %+v before refusal", object, first)
					}
					seen++
				}
			}
			if seen != tc.count || refused != tc.refuse {
				t.Fatalf("stream = (%d,%v), want (%d,%v)", seen, refused, tc.count, tc.refuse)
			}
		})
	}
}

type objectWriteDestination struct {
	bytes.Buffer
	maximum int
	err     error
	cancel  context.CancelCauseFunc
}

func (w *objectWriteDestination) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data[:min(len(data), w.maximum)])
	if w.cancel != nil {
		w.cancel(nil)
	}
	return n, errors.Join(err, w.err)
}

func TestJSONObjectEncoderPreservesWriteRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		maximum         int
		sourceErr, want error
		canceled        bool
	}{
		{"zero acknowledged bytes", 0, nil, io.ErrShortWrite, false},
		{"partial acknowledged bytes", 1, nil, io.ErrShortWrite, false},
		{"partial native failure", 1, io.ErrClosedPipe, io.ErrClosedPipe, false},
		{"complete byte acknowledgement with failure", 1024, io.ErrClosedPipe, io.ErrClosedPipe, false},
		{"cancellation during a completed write", 1024, nil, context.Canceled, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
			if err != nil {
				t.Fatal(err)
			}
			defer cancel(nil)
			writer := &objectWriteDestination{maximum: tc.maximum, err: tc.sourceErr}
			if tc.canceled {
				writer.cancel = cancel
			}
			encoder, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: writer})
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Encode(ctx, scalarObject{Name: "one", Count: 1}); !errors.Is(err, tc.want) {
				t.Fatalf("Encode(refused) = %v, want %v", err, tc.want)
			}
		})
	}
	var nilWriter *bytes.Buffer
	for _, destination := range []io.Writer{nil, nilWriter} {
		encoder, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: destination})
		if encoder != nil || !errors.Is(err, core.ErrJSONContract) {
			t.Fatal(encoder, err)
		}
	}
	var destination bytes.Buffer
	encoder, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: &destination})
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(t.Context(), scalarObject{}); !errors.Is(err, core.ErrSourceObservationContract) || destination.Len() != 0 {
		t.Fatalf("invalid meaning output = (%d,%v), want zero and owner refusal", destination.Len(), err)
	}
	var absent *jsonio.ObjectEncoder[scalarObject]
	if err := absent.Encode(t.Context(), scalarObject{Name: "one"}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatal(err)
	}
}

type opaqueRootObject struct{ Name string }

func (opaqueRootObject) Validate() error              { return nil }
func (opaqueRootObject) MarshalJSON() ([]byte, error) { return []byte("1"), nil }

func TestJSONObjectRootCannotBypassDeclaredShape(t *testing.T) {
	t.Parallel()
	var destination bytes.Buffer
	encoder, err := jsonio.NewObjectEncoder[opaqueRootObject](jsonio.ObjectDestinationRequest{Destination: &destination})
	if encoder != nil || !errors.Is(err, core.ErrJSONContract) || destination.Len() != 0 {
		t.Fatal(encoder, err)
	}
	for document, err := range jsonio.Objects[opaqueRootObject](t.Context(), jsonio.ObjectSourceRequest{Source: strings.NewReader(`{"Name":"one"}`)}) {
		if document != (opaqueRootObject{}) || !errors.Is(err, core.ErrJSONContract) {
			t.Fatal(document, err)
		}
	}
}

func TestJSONObjectStreamCancellationAndSourceIdentity(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "null", `{"Name":"one","Count":1}`} {
		ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for object, err := range jsonio.Objects[scalarObject](ctx, jsonio.ObjectSourceRequest{Source: tokenCancelReader{source: strings.NewReader(body), cancel: cancel}}) {
			seen++
			if object != (scalarObject{}) || !errors.Is(err, context.Canceled) {
				t.Fatal(object, err)
			}
		}
		cancel(nil)
		if seen != 1 {
			t.Fatalf("cancellation refusals = %d, want one", seen)
		}
	}
	for _, identity := range []error{io.ErrClosedPipe, io.ErrUnexpectedEOF, errors.Join(io.EOF, io.ErrClosedPipe)} {
		seen := 0
		for object, err := range jsonio.Objects[scalarObject](t.Context(), jsonio.ObjectSourceRequest{Source: iotest.ErrReader(identity)}) {
			seen++
			if object != (scalarObject{}) || !errors.Is(err, identity) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatal(object, err)
			}
		}
		if seen != 1 {
			t.Fatalf("source refusals = %d, want one", seen)
		}
	}
}

func FuzzJSONObjectEncoderExactMeaning(f *testing.F) {
	f.Add("one", uint64(0))
	f.Add("quoted\"\x00é", ^uint64(0))
	f.Fuzz(func(t *testing.T, name string, count uint64) {
		object := scalarObject{Name: name, Count: count}
		canonical, stringErr := core.MarshalCanonicalJSONDocument(object)
		if object.Validate() != nil || stringErr != nil {
			return
		}
		var buffer bytes.Buffer
		encoder, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: &buffer})
		if err != nil {
			t.Fatal(err)
		}
		if err := encoder.Encode(t.Context(), object); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buffer.Bytes(), append(canonical, '\n')) {
			t.Fatal("encoded stream lost string meaning or LF")
		}
		seen := 0
		for got, err := range jsonio.Objects[scalarObject](t.Context(), jsonio.ObjectSourceRequest{Source: &buffer}) {
			seen++
			if err != nil || got != object || got.Validate() != nil {
				t.Fatalf("decoded = (%+v,%v), want %+v", got, err, object)
			}
		}
		if seen != 1 {
			t.Fatalf("objects = %d, want one", seen)
		}
	})
}
