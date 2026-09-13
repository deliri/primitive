package accesspermit

import (
	"bytes"
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"testing"
)

type faultWriter struct {
	limit, calls, bytes int
	err                 error
}

func (w *faultWriter) Write(p []byte) (int, error) {
	w.calls++
	n := min(len(p), w.limit)
	w.bytes += n
	return n, w.err
}

func TestPermitCanonicalTermsWriterLayerTriad(t *testing.T) {
	t.Parallel()
	seed, _, _ := permitFixture(t)
	for _, tc := range []struct {
		name                 string
		terms                Terms
		limit                int
		writeErr, wantErr    error
		wantCalls, wantBytes int
	}{
		{"zero terms do not touch output", Terms{}, 0, nil, core.ErrAccessPermitContract, 0, 0},
		{"refused before first byte", seed.Terms, 0, io.ErrClosedPipe, io.ErrClosedPipe, 1, 0},
		{"partial write preserves cancellation", seed.Terms, 7, context.Canceled, context.Canceled, 1, 7},
		{"silent short write is not success", seed.Terms, 0, nil, io.ErrShortWrite, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			writer := &faultWriter{limit: tc.limit, err: tc.writeErr}
			err := tc.terms.WriteCanonical(writer)
			if !errors.Is(err, tc.wantErr) || writer.calls != tc.wantCalls || writer.bytes != tc.wantBytes {
				t.Fatalf("WriteCanonical() = (%v,%d calls,%d bytes), want (%v,%d,%d)", err, writer.calls, writer.bytes, tc.wantErr, tc.wantCalls, tc.wantBytes)
			}
		})
	}
	var output bytes.Buffer
	if err := seed.Terms.WriteCanonical(&output); err != nil || output.Len() == 0 {
		t.Fatalf("WriteCanonical() = (%v,%d bytes), want nil and nonempty", err, output.Len())
	}
	got, err := core.DecodeStrictJSON[Terms](bytes.NewReader(output.Bytes()), limits())
	if err != nil || got != seed.Terms {
		t.Fatalf("written terms = (%v,%v), want (%v,nil)", got, err, seed.Terms)
	}
	var second bytes.Buffer
	if err := got.WriteCanonical(&second); err != nil || !bytes.Equal(output.Bytes(), second.Bytes()) {
		t.Fatalf("second write = (%v,equal %t), want nil and identical", err, bytes.Equal(output.Bytes(), second.Bytes()))
	}
}
