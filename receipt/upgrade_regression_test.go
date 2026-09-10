package receipt

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const receiptWhitespaceFixtureBytes = 16 << 10

type receiptJSONValue interface {
	comparable
	core.ValidatedJSONMarshaler
}
type receiptJSONReceiver[T any] interface {
	*T
	UnmarshalJSON([]byte) error
}

func TestReceiptJSONExtentLayerTriad(t *testing.T) {
	t.Parallel()
	f := receiptJSONFixturesForFuzz(t)
	for _, door := range []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "body", run: func(t *testing.T) { receiptJSONExtentCases(t, f.body) }},
		{name: "header", run: func(t *testing.T) { receiptJSONExtentCases(t, f.header) }},
		{name: "payload", run: func(t *testing.T) { receiptJSONExtentCases(t, f.payload) }},
		{name: "document", run: func(t *testing.T) { receiptJSONExtentCases(t, f.document) }},
		{name: "scope", run: func(t *testing.T) { receiptJSONExtentCases(t, f.scope) }},
		{name: "watermark", run: func(t *testing.T) { receiptJSONExtentCases(t, f.watermark) }},
	} {
		t.Run(door.name, func(t *testing.T) { t.Parallel(); door.run(t) })
	}
}
func receiptJSONExtentCases[T receiptJSONValue, P receiptJSONReceiver[T]](t *testing.T, seed T) {
	t.Helper()
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{name: "canonical", data: canonical},
		{name: "large_prefix", data: append(bytes.Repeat([]byte(" "), receiptWhitespaceFixtureBytes), canonical...)},
		{name: "large_suffix", data: append(append([]byte{}, canonical...), bytes.Repeat([]byte("\n"), receiptWhitespaceFixtureBytes)...)},
		{name: "large_inside_object", data: append(append([]byte{'{'}, bytes.Repeat([]byte(" "), receiptWhitespaceFixtureBytes)...), canonical[1:]...)},
		{name: "neutral_whitespace_has_no_document", data: bytes.Repeat([]byte(" "), receiptWhitespaceFixtureBytes), wantErr: core.ErrJSONContract},
		{name: "trailing_value_after_large_gap", data: append(append(append([]byte{}, canonical...), bytes.Repeat([]byte(" "), receiptWhitespaceFixtureBytes)...), []byte("{}")...), wantErr: core.ErrJSONContract},
		{name: "truncated_after_large_prefix", data: append(bytes.Repeat([]byte(" "), receiptWhitespaceFixtureBytes), canonical[:len(canonical)-1]...), wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := seed
			err := P(&got).UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) || got != seed || (tc.wantErr != nil && !errors.Is(err, core.ErrReceiptContract)) {
				t.Fatalf("decoded=%v error=%v, want exact retained seed and %v", got, err, tc.wantErr)
			}
		})
	}
}

type receiptPrefixWriter struct {
	prefix       bytes.Buffer
	remaining    int
	cause        error
	failed       bool
	afterFailure bool
}

func (w *receiptPrefixWriter) Write(p []byte) (int, error) {
	if w.failed {
		w.afterFailure = true
	}
	n := min(w.remaining, len(p))
	_, _ = w.prefix.Write(p[:n]) // bytes.Buffer.Write cannot fail.
	w.remaining -= n
	if n < len(p) || w.cause != nil {
		w.failed = true
		return n, w.cause
	}
	return n, nil
}
func TestReceiptCanonicalWriterLayerTriad(t *testing.T) {
	t.Parallel()
	payload := issueFixture(t, newReceiptFixture(t, 31)).Payload
	canonical, err := payload.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		cause error
	}{
		{name: "short_count"}, {name: "native_failure", cause: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for n := 0; n <= len(canonical); n++ {
				w := receiptPrefixWriter{remaining: n, cause: tc.cause}
				err := payload.WriteCanonical(&w)
				want := tc.cause
				if want == nil && n < len(canonical) {
					want = io.ErrShortWrite
				}
				if !errors.Is(err, want) || (want != nil && !errors.Is(err, core.ErrReceiptContract)) || !bytes.Equal(w.prefix.Bytes(), canonical[:n]) || w.afterFailure {
					t.Fatalf("prefix=%d output=%d error=%v writes-after-failure=%t, want exact prefix and %v", n, w.prefix.Len(), err, w.afterFailure, want)
				}
			}
		})
	}
	for _, tc := range []struct {
		name        string
		destination io.Writer
		invalid     bool
		want        error
	}{
		{name: "typed_nil", destination: (*bytes.Buffer)(nil), want: core.ErrReceiptContract},
		{name: "nil_interface", want: core.ErrReceiptContract},
		{name: "invalid_payload_has_no_effect", destination: &bytes.Buffer{}, invalid: true, want: core.ErrReceiptContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("writer panic=%v, want typed refusal", p)
				}
			}()
			value := payload
			if tc.invalid {
				value = EvidencePayload{}
			}
			err := value.WriteCanonical(tc.destination)
			if !errors.Is(err, tc.want) {
				t.Fatalf("writer=%v, want %v", err, tc.want)
			}
			if tc.invalid && tc.destination.(*bytes.Buffer).Len() != 0 {
				t.Fatalf("invalid payload wrote bytes, want zero")
			}
		})
	}
}
func TestGenerationWhitespaceLayerTriad(t *testing.T) {
	t.Parallel()
	seed := mustGeneration(t, 2)
	for _, tc := range []struct {
		name    string
		data    []byte
		want    uint64
		wantErr error
	}{
		{name: "positive_canonical", data: []byte("1"), want: 1},
		{name: "neutral_JSON_whitespace", data: []byte(" \t1\r\n"), want: 1},
		{name: "negative_non_JSON_space", data: []byte("\u00a01"), want: 2, wantErr: core.ErrJSONContract},
		{name: "negative_fraction", data: []byte("1.0"), want: 2, wantErr: core.ErrJSONContract},
		{name: "negative_exponent", data: []byte("1e0"), want: 2, wantErr: core.ErrJSONContract},
		{name: "negative_trailing_value", data: []byte("1 2"), want: 2, wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := seed
			err := got.UnmarshalJSON(tc.data)
			value, projectionErr := got.Uint64()
			if !errors.Is(err, tc.wantErr) || projectionErr != nil || value != tc.want {
				t.Fatalf("generation=%d/%v decode=%v, want %d/%v", value, projectionErr, err, tc.want, tc.wantErr)
			}
		})
	}
}
