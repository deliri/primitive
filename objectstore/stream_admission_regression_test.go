package objectstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type countedStreamReply struct {
	count int
	cause error
	calls int
}

func (r *countedStreamReply) Read(p []byte) (int, error) {
	r.calls++
	for i := range min(len(p), max(0, r.count)) {
		p[i] = byte(i + 1)
	}
	return r.count, r.cause
}

func TestExactReaderNativeReplyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		declared              uint64
		replyCount, wantCount int
		cause, wantErr        error
		wantDelivered         uint64
	}{
		{name: "one byte and EOF in one source call is complete", declared: 1, replyCount: 1, cause: io.EOF, wantCount: 1, wantErr: io.EOF, wantDelivered: 1},
		{name: "two bytes and EOF cannot lose the terminal source fact", declared: 2, replyCount: 2, cause: io.EOF, wantCount: 2, wantErr: io.EOF, wantDelivered: 2},
		{name: "unproven end withholds the final byte", declared: 1, replyCount: 1, wantErr: io.ErrNoProgress},
		{name: "short terminal source retains consumed prefix", declared: 2, replyCount: 1, cause: io.EOF, wantCount: 1, wantErr: core.ErrObjectStoreSource, wantDelivered: 1},
		{name: "final native failure retains delivered byte accounting", declared: 1, replyCount: 1, cause: io.ErrClosedPipe, wantCount: 1, wantErr: io.ErrClosedPipe, wantDelivered: 1},
		{name: "negative reader count refuses without panic", declared: 1, replyCount: -1, wantErr: core.ErrObjectStoreSource},
		{name: "count beyond destination refuses without panic", declared: 1, replyCount: math.MaxInt, wantErr: core.ErrObjectStoreSource},
		{name: "absent source bytes cannot create delivered evidence", declared: 1, replyCount: 0, cause: io.EOF, wantErr: core.ErrObjectStoreSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if got := recover(); got != nil {
					t.Errorf("ExactReader.Read panic = %v, want typed refusal", got)
				}
			}()
			length, err := core.NewByteLength(tc.declared)
			if err != nil {
				t.Fatalf("length error = %v, want nil", err)
			}
			source := &countedStreamReply{count: tc.replyCount, cause: tc.cause}
			reader, err := NewExactReader(source, length)
			if err != nil {
				t.Fatalf("NewExactReader error = %v, want nil", err)
			}
			var buffer [2]byte
			count, gotErr := reader.Read(buffer[:tc.declared])
			if count != tc.wantCount || !errors.Is(gotErr, tc.wantErr) || reader.delivered != tc.wantDelivered || source.calls != 1 {
				t.Fatalf("Read = (%d, %v), delivered=%d calls=%d; want (%d, %v), delivered=%d calls=1", count, gotErr, reader.delivered, source.calls, tc.wantCount, tc.wantErr, tc.wantDelivered)
			}
			for i := range count {
				if buffer[i] != byte(i+1) {
					t.Fatalf("delivered byte[%d] = %d, want %d", i, buffer[i], i+1)
				}
			}
			if errors.Is(tc.wantErr, io.EOF) {
				if reader.Failure() != nil || !reader.verified {
					t.Fatalf("terminal proof = (%t, %v), want verified and nil", reader.verified, reader.Failure())
				}
			} else if !errors.Is(reader.Failure(), core.ErrObjectStoreSource) || !errors.Is(reader.Failure(), core.ErrObjectStoreIntegrity) {
				t.Fatalf("Failure = %v, want source and integrity identities", reader.Failure())
			}
			again, againErr := reader.Read(buffer[:])
			if again != 0 || !errors.Is(againErr, tc.wantErr) || source.calls != 1 || reader.delivered != tc.wantDelivered {
				t.Fatalf("terminal replay = (%d, %v), calls=%d delivered=%d; want preserved terminal facts", again, againErr, source.calls, reader.delivered)
			}
		})
	}
}

func TestExactReaderZeroDestinationDoesNotSpendProgressBudget(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		probes int
	}{
		{name: "one below empty-read ceiling preserves source", probes: core.ReaderConsecutiveEmptyReadMaximum - 1},
		{name: "exact empty-read ceiling preserves source", probes: core.ReaderConsecutiveEmptyReadMaximum},
		{name: "one above empty-read ceiling preserves source", probes: core.ReaderConsecutiveEmptyReadMaximum + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := bytes.NewReader([]byte{0x7a})
			length, err := core.NewByteLength(1)
			if err != nil {
				t.Fatalf("length error = %v, want nil", err)
			}
			reader, err := NewExactReader(source, length)
			if err != nil {
				t.Fatalf("NewExactReader error = %v, want nil", err)
			}
			for range tc.probes {
				n, err := reader.Read(nil)
				if n != 0 || err != nil || reader.delivered != 0 || source.Len() != 1 {
					t.Fatalf("zero destination = (%d, %v), delivered=%d remaining=%d; want no progress or source access", n, err, reader.delivered, source.Len())
				}
			}
			var got [1]byte
			n, err := reader.Read(got[:])
			if n != 1 || err != nil || got[0] != 0x7a || !reader.verified || reader.Failure() != nil {
				t.Fatalf("real read after empty destinations = (%d, %v, %x), want exact byte and completed proof", n, err, got)
			}
		})
	}
}

type cancelingInspectionReader struct {
	data     *bytes.Reader
	cancel   context.CancelFunc
	terminal bool
}

func (r *cancelingInspectionReader) Read(p []byte) (int, error) {
	n, err := r.data.Read(p)
	r.cancel()
	if r.terminal {
		err = io.EOF
	}
	return n, err
}
func (r *cancelingInspectionReader) Len() int { return r.data.Len() }

func TestInspectionFinalReadCancellationCannotSealProof(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		terminal bool
		maximum  uint64
	}{
		{name: "final EOF cancellation refuses exact extent", terminal: true, maximum: 1},
		{name: "final EOF cancellation refuses below ceiling", terminal: true, maximum: 2},
		{name: "exact length observation cannot erase cancellation", maximum: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			source := &cancelingInspectionReader{data: bytes.NewReader([]byte{0x7a}), cancel: cancel, terminal: tc.terminal}
			maximum, err := core.NewByteCount(tc.maximum)
			if err != nil {
				t.Fatalf("maximum error = %v, want nil", err)
			}
			got, err := Inspect(ctx, InspectionRequest{Source: source, MaximumBytes: maximum})
			if got != (Inspection{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrObjectStoreSource) || source.Len() != 0 {
				t.Fatalf("final read cancellation = (%v, %v), remaining=%d; want zero proof, typed cancellation and consumed byte", got, err, source.Len())
			}
		})
	}
}

func TestStreamEOFCannotHideAnotherFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		cause       error
		wantFailure bool
	}{
		{name: "native EOF is graceful completion", cause: io.EOF},
		{name: "closed pipe is retained", cause: io.ErrClosedPipe, wantFailure: true},
		{name: "EOF joined with closed pipe cannot certify completion", cause: errors.Join(io.EOF, io.ErrClosedPipe), wantFailure: true},
		{name: "EOF joined with cancellation cannot certify completion", cause: errors.Join(io.EOF, context.Canceled), wantFailure: true},
		{name: "wrapped EOF follows Go Reader failure semantics", cause: errors.Join(io.EOF), wantFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			length, err := core.NewByteLength(1)
			if err != nil {
				t.Fatal(err)
			}
			exact, err := NewExactReader(&countedStreamReply{count: 1, cause: tc.cause}, length)
			if err != nil {
				t.Fatal(err)
			}
			var output [1]byte
			n, err := exact.Read(output[:])
			if n != 1 || output[0] != 1 || !errors.Is(err, tc.cause) || exact.verified == tc.wantFailure {
				t.Errorf("exact reply = (%d, %v), byte=%d verified=%t; want (1, %v), byte=1 verified=%t", n, err, output[0], exact.verified, tc.cause, !tc.wantFailure)
			}
			if tc.wantFailure && (!errors.Is(exact.Failure(), tc.cause) || !errors.Is(exact.Failure(), core.ErrObjectStoreSource)) {
				t.Errorf("Failure = %v, want source and original %v", exact.Failure(), tc.cause)
			}
			maximum, err := core.NewByteCount(1)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Inspect(t.Context(), InspectionRequest{Source: &countedStreamReply{count: 1, cause: tc.cause}, MaximumBytes: maximum})
			if tc.wantFailure {
				if got != (Inspection{}) || !errors.Is(err, tc.cause) || !errors.Is(err, core.ErrObjectStoreSource) {
					t.Errorf("inspection = (%v, %v), want zero and source/%v", got, err, tc.cause)
				}
				return
			}
			want := providerIntegrity(t, []byte{1})
			if err != nil || got.Validate() != nil || got.Integrity != want {
				t.Errorf("native EOF inspection = (%v, %v), want exact integrity %v", got, err, want)
			}
		})
	}
}
