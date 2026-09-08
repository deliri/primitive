package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type hostfactsReadStep struct {
	data       string
	err        error
	countDelta int
}
type hostfactsScriptReader struct {
	steps        []hostfactsReadStep
	calls, bytes int
	cancel       context.CancelFunc
}

func (r *hostfactsScriptReader) Read(p []byte) (int, error) {
	r.calls++
	if r.cancel != nil {
		r.cancel()
	}
	if len(r.steps) == 0 {
		return 0, io.EOF
	}
	step := r.steps[0]
	r.steps = r.steps[1:]
	n := copy(p, step.data)
	r.bytes += n
	return n + step.countDelta, step.err
}

func TestOOMReadAccountingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		steps                []hostfactsReadStep
		length               uint64
		cancelOnRead         bool
		wantState            GoOOMBannerState
		wantErr              error
		wantCalls, wantBytes int
	}{
		{name: "empty extent never reads even a failing source", steps: []hostfactsReadStep{{err: io.ErrClosedPipe}}, wantState: GoOOMBannerAbsent},
		{name: "exact bytes with EOF seal presence", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: io.EOF}}, length: uint64(len(GoOOMPlainBanner)), wantState: GoOOMBannerPresent, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "exact bytes with native error follow io.ReadFull completion", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: io.ErrClosedPipe}}, length: uint64(len(GoOOMPlainBanner)), wantState: GoOOMBannerPresent, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "early banner does not hide a short tail", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: io.EOF}}, length: uint64(len(GoOOMPlainBanner) + 1), wantErr: io.ErrUnexpectedEOF, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "early banner does not hide tail read failure", steps: []hostfactsReadStep{{data: GoOOMPlainBanner}, {err: io.ErrClosedPipe}}, length: uint64(len(GoOOMPlainBanner) + 1), wantErr: io.ErrClosedPipe, wantCalls: 2, wantBytes: len(GoOOMPlainBanner)},
		{name: "partial data with error returns zero evidence", steps: []hostfactsReadStep{{data: "x", err: io.ErrClosedPipe}}, length: 2, wantErr: io.ErrClosedPipe, wantCalls: 1, wantBytes: 1},
		{name: "negative count cannot escape slice validation", steps: []hostfactsReadStep{{countDelta: -1}}, length: 1, wantErr: core.ErrHostFactsObservation, wantCalls: 1},
		{name: "count beyond requested extent cannot consume tail", steps: []hostfactsReadStep{{data: "x", countDelta: 1}}, length: 1, wantErr: core.ErrHostFactsObservation, wantCalls: 1, wantBytes: 1},
		{name: "empty read is tolerated before progress", steps: []hostfactsReadStep{{}, {data: "x"}}, length: 1, wantState: GoOOMBannerAbsent, wantCalls: 2, wantBytes: 1},
		{name: "cancellation between chunks returns no observation", steps: []hostfactsReadStep{{data: "x"}, {data: "y"}}, length: 2, cancelOnRead: true, wantErr: context.Canceled, wantCalls: 1, wantBytes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			source := &hostfactsScriptReader{steps: tc.steps}
			if tc.cancelOnRead {
				source.cancel = cancel
			}
			got, err := ClassifyGoOOMBanner(ctx, GoOOMBannerRequest{Source: source, Length: mustByteLength(t, tc.length)})
			if !errors.Is(err, tc.wantErr) || source.calls != tc.wantCalls || source.bytes != tc.wantBytes {
				t.Fatalf("classified = %+v/%v calls=%d bytes=%d, want error=%v calls=%d bytes=%d", got, err, source.calls, source.bytes, tc.wantErr, tc.wantCalls, tc.wantBytes)
			}
			if tc.wantErr != nil {
				if got != (GoOOMBannerEvidence{}) {
					t.Fatalf("refused evidence = %+v, want zero", got)
				}
				return
			}
			if got.State() != tc.wantState || got.BytesExamined().Uint64() != tc.length {
				t.Fatalf("evidence = %+v, want state=%v extent=%d", got, tc.wantState, tc.length)
			}
		})
	}
}

func TestOOMEveryBufferBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, banner string
		mutate       bool
	}{
		{name: "plain exact bytes", banner: GoOOMPlainBanner},
		{name: "prefixed exact bytes", banner: GoOOMPrefixedBanner},
		{name: "plain last byte mutation", banner: GoOOMPlainBanner, mutate: true},
		{name: "prefixed last byte mutation", banner: GoOOMPrefixedBanner, mutate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for split := 1; split < len(tc.banner); split++ {
				data := bytes.Repeat([]byte{'x'}, goOOMBufferBytes+len(tc.banner))
				start := goOOMBufferBytes - split
				copy(data[start:], tc.banner)
				if tc.mutate {
					data[start+len(tc.banner)-1] = 'x'
				}
				reader := bytes.NewReader(data)
				got, err := ClassifyGoOOMBanner(t.Context(), GoOOMBannerRequest{Source: reader, Length: mustByteLength(t, uint64(len(data)))})
				want := GoOOMBannerPresent
				if tc.mutate {
					want = GoOOMBannerAbsent
				}
				if err != nil || got.State() != want || got.BytesExamined().Uint64() != uint64(len(data)) || reader.Len() != 0 {
					t.Fatalf("buffer split %d = %+v/%v unread=%d, want %v exact exhausted extent", split, got, err, reader.Len(), want)
				}
			}
		})
	}
}
