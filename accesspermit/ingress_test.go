package accesspermit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// These cases exercise the public streaming decoder with genuinely signed bytes.
func TestPermitDecoderLayerTriad(t *testing.T) {
	t.Parallel()
	document, keys, _ := permitFixture(t)
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	seed := string(encoded)
	for _, tc := range []struct {
		name, input string
		wantErr     error
	}{
		{"canonical signed facts survive transport", seed, nil},
		{"legal whitespace preserves signed facts", "\n" + seed + "\t", nil},
		{"one below document ceiling", seed + strings.Repeat(" ", DocumentMaximumBytes-1-len(seed)), nil},
		{"exact document ceiling", seed + strings.Repeat(" ", DocumentMaximumBytes-len(seed)), nil},
		{"one above document ceiling", seed + strings.Repeat(" ", DocumentMaximumBytes+1-len(seed)), core.ErrJSONContract},
		{"empty stream emits no agreement", "", core.ErrJSONContract},
		{"explicit null cannot grant access", "null", core.ErrJSONContract},
		{"truncated outer document", seed[:len(seed)-1], core.ErrJSONContract},
		{"trailing second document", seed + seed, core.ErrJSONContract},
		{"unknown outer member", strings.Replace(seed, "{", `{"extra":0,`, 1), core.ErrJSONContract},
		{"duplicate allow member", strings.Replace(seed, `"decision":"allow"`, `"decision":"allow","decision":"allow"`, 1), core.ErrJSONContract},
		{"conflicting decision members", strings.Replace(seed, `"decision":"allow"`, `"decision":"allow","decision":"refuse"`, 1), core.ErrJSONContract},
		{"numeric decision cannot become allow", strings.Replace(seed, `"decision":"allow"`, `"decision":1`, 1), core.ErrJSONContract},
		{"future decision is not refusal", strings.Replace(seed, `"decision":"allow"`, `"decision":"future"`, 1), core.ErrJSONContract},
		{"missing decision cannot default", strings.Replace(seed, `,"decision":"allow"`, "", 1), core.ErrJSONContract},
		{"future revision cannot borrow current signature", strings.Replace(seed, `"revision":1`, `"revision":2`, 1), core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.wantErr != nil && tc.input == seed {
				t.Fatal("hostile input = seed, want load-bearing mutation")
			}
			got, err := Decode(strings.NewReader(tc.input))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Decode() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Document{}) || !errors.Is(err, core.ErrAccessPermitContract) {
					t.Fatalf("refusal = (%v, %v), want zero and typed contract", got, err)
				}
				return
			}
			if got != document {
				t.Fatalf("Decode() = %v, want %v", got, document)
			}
			proof, err := Verify(got, document.Terms.Binding, keys)
			if err != nil || proof.Validate() != nil {
				t.Fatalf("Verify() = (%v, %v), want authentic proof", proof, err)
			}
		})
	}
}

type countedReader struct {
	source io.Reader
	count  int
}

func (r *countedReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.count += n
	return n, err
}

type refusedReader struct{ err error }

func (r refusedReader) Read([]byte) (int, error) { return 0, r.err }

type spaceReader struct{}

func (spaceReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestPermitStreamingRefusalPreservesCauseAndConsumption(t *testing.T) {
	t.Parallel()
	seed, _, _ := permitFixture(t)
	encoded, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name      string
		source    io.Reader
		wantErr   error
		wantBytes int
	}{
		{"cancelled before bytes", refusedReader{context.Canceled}, context.Canceled, 0},
		{"failure after complete document is not EOF", io.MultiReader(bytes.NewReader(encoded), refusedReader{io.ErrClosedPipe}), io.ErrClosedPipe, len(encoded)},
		{"partial transport retains unexpected EOF", io.MultiReader(bytes.NewReader(encoded[:10]), refusedReader{io.ErrUnexpectedEOF}), io.ErrUnexpectedEOF, 10},
		{"unbounded source stops at ceiling plus probe", spaceReader{}, core.ErrJSONContract, DocumentMaximumBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader := &countedReader{source: tc.source}
			got, err := Decode(reader)
			if got != (Document{}) || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrAccessPermitContract) || reader.count != tc.wantBytes {
				t.Fatalf("Decode() = (%v, %v, %d bytes), want zero, %v, %d bytes", got, err, reader.count, tc.wantErr, tc.wantBytes)
			}
		})
	}
}

func FuzzPermitDomainCanonicalText(f *testing.F) {
	canonical, err := DomainV1.MarshalText()
	if err != nil {
		f.Fatalf("MarshalText(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := DomainV1.ParseCanonicalText(data)
		if bytes.Equal(data, canonical) {
			if err != nil || got != DomainV1 {
				t.Fatalf("ParseCanonicalText() = (%v, %v), want DomainV1 and nil", got, err)
			}
		} else if !errors.Is(err, core.ErrAccessPermitContract) || got != 0 {
			t.Fatalf("ParseCanonicalText() = (%v, %v), want zero and typed refusal", got, err)
		}
	})
}
