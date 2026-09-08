package core

import (
	"errors"
	"io"
	"testing"
)

type strictReadOutcome struct {
	count int
	err   error
	calls int
}

func (r *strictReadOutcome) Read(destination []byte) (int, error) {
	r.calls++
	if r.calls > 1 {
		return 0, io.EOF
	}
	copy(destination, []byte("7 "))
	return r.count, r.err
}

func TestStrictJSONReaderPreservesSimultaneousFailures(t *testing.T) {
	t.Parallel()
	// Count partitions exhaust the buffer and document thresholds (maximum 1,
	// proof buffer 2). EOF, success and a native error are independent facts.
	counts := []struct {
		name        string
		count       int
		wantReadErr bool
	}{
		{name: "negative count", count: -1, wantReadErr: true},
		{name: "empty count", count: 0},
		{name: "exact document", count: 1},
		{name: "proof byte crosses document bound", count: 2, wantReadErr: true},
		{name: "count exceeds supplied buffer", count: 3, wantReadErr: true},
	}
	outcomes := []struct {
		name            string
		err             error
		permitsDocument bool
	}{
		{name: "no native error", permitsDocument: true},
		{name: "terminal EOF", err: io.EOF, permitsDocument: true},
		{name: "native failure", err: strictJSONReaderTestError{}},
		{name: "EOF joined with failure", err: errors.Join(io.EOF, strictJSONReaderTestError{})},
	}
	for _, count := range counts {
		for _, outcome := range outcomes {
			t.Run(count.name+"/"+outcome.name, func(t *testing.T) {
				t.Parallel()
				reader := &strictReadOutcome{count: count.count, err: outcome.err}
				limits := DefaultStrictJSONLimits()
				limits.DocumentMaximumBytes = ByteCount{value: 1}
				got, err := DecodeStrictJSON[ByteCount](reader, limits)
				wantOK := count.count == 1 && outcome.permitsDocument
				if wantOK {
					if err != nil || got != (ByteCount{value: 7}) {
						t.Fatalf("decode=%v, %v; want exact count 7", got, err)
					}
				} else {
					if got != (ByteCount{}) || !errors.Is(err, ErrJSONContract) {
						t.Fatalf("decode=%v, %v; want zero and JSON refusal", got, err)
					}
					if count.wantReadErr && outcome.err != nil || errors.Is(outcome.err, strictJSONReaderTestError{}) {
						if !errors.Is(err, outcome.err) {
							t.Fatalf("decode error=%v; want simultaneous native cause %v", err, outcome.err)
						}
					}
				}
				wantCalls := 1
				if outcome.err == nil && !count.wantReadErr {
					wantCalls = 2
				}
				if reader.calls != wantCalls {
					t.Fatalf("read calls=%d; want %d, with no read after terminal outcome", reader.calls, wantCalls)
				}
			})
		}
	}
}
