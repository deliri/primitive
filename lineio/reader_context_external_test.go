package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

type cancelingFragmentSource struct {
	cancel context.CancelFunc
	data   string
	cause  error
	reads  int
}

func (s *cancelingFragmentSource) Read(destination []byte) (int, error) {
	s.reads++
	s.cancel()
	return copy(destination, s.data), s.cause
}

func TestFragmentContextAdmissionDoesNotReadOrPoisonTheSource(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		wantErr error
	}{
		{name: "nil context refuses native read", wantErr: core.ErrNilContext},
		{name: "canceled context refuses native read", ctx: canceled, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &observedReader{Reader: strings.NewReader("admitted\n")}
			reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: mustByteCount(t, 16)})
			if err != nil {
				t.Fatalf("New() error = %v, want nil", err)
			}
			fragment, err := reader.ReadFragment(tc.ctx)
			if !errors.Is(err, tc.wantErr) || len(fragment.Bytes) != 0 || fragment.More || source.calls != 0 {
				t.Fatalf("ReadFragment(refused) = (%q, more:%t, reads:%d, %v), want empty, zero reads and %v", fragment.Bytes, fragment.More, source.calls, err, tc.wantErr)
			}
			fragment, err = reader.ReadFragment(t.Context())
			if err != nil || string(fragment.Bytes) != "admitted\n" || source.calls != 1 {
				t.Fatalf("ReadFragment(live) = (%q, reads:%d, %v), want original line, one read and nil", fragment.Bytes, source.calls, err)
			}
		})
	}
}

func TestFragmentCancellationDuringNativeReadRetainsBytesAndRefusal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		data     string
		cause    error
		wantMore bool
	}{
		{name: "complete LF cannot hide cancellation", data: "line\n"},
		{name: "full native window cannot hide cancellation", data: strings.Repeat("x", 16), wantMore: true},
		{name: "EOF cannot impersonate successful completion", data: "tail", cause: io.EOF},
		{name: "source error retains its identity beside cancellation", data: "tail", cause: io.ErrClosedPipe},
		{name: "wrapped EOF retains source refusal", data: "tail", cause: errors.Join(io.EOF, io.ErrClosedPipe)},
		{name: "empty native EOF still observes cancellation", cause: io.EOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			source := &cancelingFragmentSource{cancel: cancel, data: tc.data, cause: tc.cause}
			reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: mustByteCount(t, 16)})
			if err != nil {
				t.Fatalf("New() error = %v, want nil", err)
			}
			fragment, err := reader.ReadFragment(ctx)
			if !errors.Is(err, context.Canceled) || !bytes.Equal(fragment.Bytes, []byte(tc.data)) || fragment.More != tc.wantMore || source.reads != 1 {
				t.Fatalf("ReadFragment(canceled during read) = (%q, more:%t, reads:%d, %v), want exact %q, more:%t, one read and cancellation", fragment.Bytes, fragment.More, source.reads, err, tc.data, tc.wantMore)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("ReadFragment() error = %v, want native identity %v", err, tc.cause)
			}
			if errors.Is(tc.cause, io.ErrClosedPipe) && !errors.Is(err, core.ErrLineIOScan) {
				t.Fatalf("ReadFragment() error = %v, want source scan refusal", err)
			}
			again, nextErr := reader.ReadFragment(t.Context())
			if len(again.Bytes) != 0 || again.More || !errors.Is(nextErr, context.Canceled) || source.reads != 1 {
				t.Fatalf("ReadFragment(after terminal) = (%q, more:%t, reads:%d, %v), want empty, original refusal and no retry", again.Bytes, again.More, source.reads, nextErr)
			}
		})
	}
}
