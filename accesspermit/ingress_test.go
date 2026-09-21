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
		wantErr     error
		name, input string
	}{
		{name: "canonical signed facts survive transport", input: seed, wantErr: nil},
		{name: "legal whitespace preserves signed facts", input: "\n" + seed + "\t", wantErr: nil},
		{name: "one below document ceiling", input: seed + strings.Repeat(" ", DocumentMaximumBytes-1-len(seed)), wantErr: nil},
		{name: "exact document ceiling", input: seed + strings.Repeat(" ", DocumentMaximumBytes-len(seed)), wantErr: nil},
		{name: "one above document ceiling", input: seed + strings.Repeat(" ", DocumentMaximumBytes+1-len(seed)), wantErr: core.ErrJSONContract},
		{name: "empty stream emits no agreement", input: "", wantErr: core.ErrJSONContract},
		{name: "explicit null cannot grant access", input: "null", wantErr: core.ErrJSONContract},
		{name: "truncated outer document", input: seed[:len(seed)-1], wantErr: core.ErrJSONContract},
		{name: "trailing second document", input: seed + seed, wantErr: core.ErrJSONContract},
		{name: "unknown outer member", input: strings.Replace(seed, "{", `{"extra":0,`, 1), wantErr: core.ErrJSONContract},
		{name: "duplicate allow member", input: strings.Replace(seed, `"decision":"allow"`, `"decision":"allow","decision":"allow"`, 1), wantErr: core.ErrJSONContract},
		{name: "conflicting decision members", input: strings.Replace(seed, `"decision":"allow"`, `"decision":"allow","decision":"refuse"`, 1), wantErr: core.ErrJSONContract},
		{name: "numeric decision cannot become allow", input: strings.Replace(seed, `"decision":"allow"`, `"decision":1`, 1), wantErr: core.ErrJSONContract},
		{name: "future decision is not refusal", input: strings.Replace(seed, `"decision":"allow"`, `"decision":"future"`, 1), wantErr: core.ErrJSONContract},
		{name: "missing decision cannot default", input: strings.Replace(seed, `,"decision":"allow"`, "", 1), wantErr: core.ErrJSONContract},
		{name: "future revision cannot borrow current signature", input: strings.Replace(seed, `"revision":1`, `"revision":2`, 1), wantErr: core.ErrJSONContract},
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
		source    io.Reader
		wantErr   error
		name      string
		wantBytes int
	}{
		{name: "cancelled before bytes", source: refusedReader{err: context.Canceled}, wantErr: context.Canceled, wantBytes: 0},
		{name: "failure after complete document is not EOF", source: io.MultiReader(bytes.NewReader(encoded), refusedReader{err: io.ErrClosedPipe}), wantErr: io.ErrClosedPipe, wantBytes: len(encoded)},
		{name: "partial transport retains unexpected EOF", source: io.MultiReader(bytes.NewReader(encoded[:10]), refusedReader{err: io.ErrUnexpectedEOF}), wantErr: io.ErrUnexpectedEOF, wantBytes: 10},
		{name: "unbounded source stops at ceiling plus probe", source: spaceReader{}, wantErr: core.ErrJSONContract, wantBytes: DocumentMaximumBytes + 1},
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
