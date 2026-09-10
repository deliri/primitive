package objectstore

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"testing"
)

type extentDestination struct {
	count          int
	err            error
	calls, offered int
}

func (w *extentDestination) Write(p []byte) (int, error) {
	w.calls++
	w.offered = len(p)
	return w.count, w.err
}

func TestExactDownloadWriterAcknowledgmentLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                              string
		remaining                         uint64
		payload                           string
		count                             int
		cause, wantErr                    error
		wantCount, wantOffered, wantCalls int
		wantRemaining                     uint64
	}{
		{name: "complete acknowledgment consumes exact remaining extent", remaining: 2, payload: "ab", count: 2, wantCount: 2, wantOffered: 2, wantCalls: 1},
		{name: "partial acknowledgment consumes only acknowledged bytes", remaining: 2, payload: "ab", count: 1, cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantCount: 1, wantOffered: 2, wantCalls: 1, wantRemaining: 1},
		{name: "native failure beside full acknowledgment remains reachable", remaining: 2, payload: "ab", count: 2, cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantCount: 2, wantOffered: 2, wantCalls: 1},
		{name: "excess source never reaches the destination beyond agreed bytes", remaining: 1, payload: "ab", count: 1, wantErr: core.ErrObjectStoreIntegrity, wantCount: 1, wantOffered: 1, wantCalls: 1},
		{name: "excess and native destination failure both remain reachable", remaining: 1, payload: "ab", count: 1, cause: io.ErrClosedPipe, wantErr: core.ErrObjectStoreIntegrity, wantCount: 1, wantOffered: 1, wantCalls: 1},
		{name: "zero remaining refuses excess without invoking destination", payload: "a", wantErr: core.ErrObjectStoreIntegrity},
		{name: "empty write changes no extent", remaining: 2, wantCalls: 1, wantRemaining: 2},
		{name: "negative destination count cannot increase remaining extent", remaining: 2, payload: "ab", count: -1, wantErr: core.ErrObjectStoreDestination, wantOffered: 2, wantCalls: 1, wantRemaining: 2},
		{name: "overreported destination count cannot consume invented bytes", remaining: 2, payload: "ab", count: 3, wantErr: core.ErrObjectStoreDestination, wantOffered: 2, wantCalls: 1, wantRemaining: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := &extentDestination{count: tc.count, err: tc.cause}
			writer := exactDownloadWriter{destination: sink, remaining: tc.remaining}
			got, err := writer.Write([]byte(tc.payload))
			if got != tc.wantCount || !errors.Is(err, tc.wantErr) || tc.cause != nil && !errors.Is(err, tc.cause) || sink.offered != tc.wantOffered || sink.calls != tc.wantCalls || writer.remaining != tc.wantRemaining {
				t.Fatalf("write=%d/%v, offered/calls/remaining=%d/%d/%d; want %d/%v and %d/%d/%d", got, err, sink.offered, sink.calls, writer.remaining, tc.wantCount, tc.wantErr, tc.wantOffered, tc.wantCalls, tc.wantRemaining)
			}
		})
	}
}
