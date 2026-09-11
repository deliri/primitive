package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
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
	lastError    error
	cancelCalls  int
	cancel       context.CancelFunc
}

func (r *hostfactsScriptReader) Read(p []byte) (int, error) {
	r.calls++
	if r.cancel != nil {
		r.cancel()
		r.cancelCalls++
	}
	if len(r.steps) == 0 {
		r.lastError = io.EOF
		return 0, io.EOF
	}
	step := r.steps[0]
	r.steps = r.steps[1:]
	n := copy(p, step.data)
	r.bytes += n
	r.lastError = step.err
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
		wantReadErr          error
		wantAlsoErr          error
		wantCancelCalls      int
	}{
		{name: "empty extent never reads even a failing source", steps: []hostfactsReadStep{{err: io.ErrClosedPipe}}, wantState: GoOOMBannerAbsent},
		{name: "exact bytes with EOF seal presence", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: io.EOF}}, length: uint64(len(GoOOMPlainBanner)), wantState: GoOOMBannerPresent, wantReadErr: io.EOF, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "exact bytes with native error refuse evidence", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: io.ErrClosedPipe}}, length: uint64(len(GoOOMPlainBanner)), wantErr: io.ErrClosedPipe, wantReadErr: io.ErrClosedPipe, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "final bytes with joined EOF and resource failure retain both causes", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: errors.Join(io.EOF, io.ErrClosedPipe)}}, length: uint64(len(GoOOMPlainBanner)), wantErr: io.ErrClosedPipe, wantAlsoErr: io.EOF, wantReadErr: io.ErrClosedPipe, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "cancellation during final bytes cannot seal presence", steps: []hostfactsReadStep{{data: GoOOMPlainBanner}}, length: uint64(len(GoOOMPlainBanner)), cancelOnRead: true, wantCancelCalls: 1, wantErr: context.Canceled, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "cancellation accompanying exact EOF cannot seal presence", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: io.EOF}}, length: uint64(len(GoOOMPlainBanner)), cancelOnRead: true, wantCancelCalls: 1, wantErr: context.Canceled, wantReadErr: io.EOF, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "early banner does not hide a short tail", steps: []hostfactsReadStep{{data: GoOOMPlainBanner, err: io.EOF}}, length: uint64(len(GoOOMPlainBanner) + 1), wantErr: io.ErrUnexpectedEOF, wantReadErr: io.EOF, wantCalls: 1, wantBytes: len(GoOOMPlainBanner)},
		{name: "early banner does not hide tail read failure", steps: []hostfactsReadStep{{data: GoOOMPlainBanner}, {err: io.ErrClosedPipe}}, length: uint64(len(GoOOMPlainBanner) + 1), wantErr: io.ErrClosedPipe, wantReadErr: io.ErrClosedPipe, wantCalls: 2, wantBytes: len(GoOOMPlainBanner)},
		{name: "partial data with error returns zero evidence", steps: []hostfactsReadStep{{data: "x", err: io.ErrClosedPipe}}, length: 2, wantErr: io.ErrClosedPipe, wantReadErr: io.ErrClosedPipe, wantCalls: 1, wantBytes: 1},
		{name: "negative count cannot escape slice validation", steps: []hostfactsReadStep{{countDelta: -1}}, length: 1, wantErr: core.ErrHostFactsObservation, wantCalls: 1},
		{name: "count beyond requested extent cannot consume tail", steps: []hostfactsReadStep{{data: "x", countDelta: 1}}, length: 1, wantErr: core.ErrHostFactsObservation, wantCalls: 1, wantBytes: 1},
		{name: "empty read is tolerated before progress", steps: []hostfactsReadStep{{}, {data: "x"}}, length: 1, wantState: GoOOMBannerAbsent, wantCalls: 2, wantBytes: 1},
		{name: "cancellation between chunks returns no observation", steps: []hostfactsReadStep{{data: "x"}, {data: "y"}}, length: 2, cancelOnRead: true, wantCancelCalls: 1, wantErr: context.Canceled, wantCalls: 1, wantBytes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			source := &hostfactsScriptReader{steps: slices.Clone(tc.steps)}
			if tc.cancelOnRead {
				source.cancel = cancel
			}
			got, err := ClassifyGoOOMBanner(ctx, GoOOMBannerRequest{Source: source, Length: mustByteLength(t, tc.length)})
			// Prove the actual reader/cancellation facts before accepting the
			// classifier projection. A fixture that skipped a read fails here.
			if source.calls != tc.wantCalls || source.bytes != tc.wantBytes || source.cancelCalls != tc.wantCancelCalls || !errors.Is(source.lastError, tc.wantReadErr) {
				t.Fatalf("source facts = calls %d bytes %d cancellations %d error %v, want %d/%d/%d/%v", source.calls, source.bytes, source.cancelCalls, source.lastError, tc.wantCalls, tc.wantBytes, tc.wantCancelCalls, tc.wantReadErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ClassifyGoOOMBanner() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantAlsoErr != nil && !errors.Is(err, tc.wantAlsoErr) {
				t.Fatalf("secondary read cause = %v, want %v", err, tc.wantAlsoErr)
			}
			if tc.wantErr != nil {
				var refusal Failure
				if got != (GoOOMBannerEvidence{}) || !errors.As(err, &refusal) || refusal.Operation != OperationGoOOMBanner || refusal.Identity != core.ErrHostFactsObservation {
					t.Fatalf("refused evidence = (%+v, %v), want zero and ErrHostFactsObservation", got, err)
				}
				encoded, encodeErr := got.MarshalJSON()
				if len(encoded) != 0 || !errors.Is(encodeErr, core.ErrHostFactsEvidence) {
					t.Fatalf("refusal.MarshalJSON() = (%q, %v), want no evidence bytes and ErrHostFactsEvidence", encoded, encodeErr)
				}
				return
			}
			if got.State() != tc.wantState || got.BytesExamined().Uint64() != tc.length {
				t.Fatalf("evidence = %+v, want state=%v extent=%d", got, tc.wantState, tc.length)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("evidence.Validate() error = %v, want nil", err)
			}
			encoded, encodeErr := got.MarshalJSON()
			if encodeErr != nil || len(encoded) == 0 {
				t.Fatalf("evidence.MarshalJSON() = (%q, %v), want nonempty canonical evidence", encoded, encodeErr)
			}
			var replay GoOOMBannerEvidence
			if replayErr := replay.UnmarshalJSON(encoded); replayErr != nil || replay != got {
				t.Fatalf("evidence replay = (%v, %v), want (%v, nil)", replay, replayErr, got)
			}
			second, secondErr := replay.MarshalJSON()
			if secondErr != nil || !bytes.Equal(second, encoded) {
				t.Fatalf("second evidence encoding = (%q, %v), want (%q, nil)", second, secondErr, encoded)
			}
		})
	}
}

// This exhausts banner split positions and single-byte corruption positions.
// It is a scanner regression, not a complete package or layer-triad claim.
func TestOOMBufferSplitsPreserveSingleByteMutation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, banner string
	}{
		{name: "plain banner spanning every buffer split", banner: GoOOMPlainBanner},
		{name: "prefixed banner spanning every buffer split", banner: GoOOMPrefixedBanner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for split := 1; split < len(tc.banner); split++ {
				baseline := bytes.Repeat([]byte{'x'}, goOOMBufferBytes+len(tc.banner))
				start := goOOMBufferBytes - split
				copy(baseline[start:], tc.banner)
				reader := bytes.NewReader(baseline)
				request := GoOOMBannerRequest{Source: reader, Length: mustByteLength(t, uint64(len(baseline)))}
				before, err := ClassifyGoOOMBanner(t.Context(), request)
				if err != nil || before.State() != GoOOMBannerPresent || before.BytesExamined() != request.Length || reader.Len() != 0 {
					t.Fatalf("baseline split %d = (%v, %v), unread %d, want present with exact exhausted extent", split, before, err, reader.Len())
				}
				for position := range len(tc.banner) {
					mutated := bytes.Clone(baseline)
					mutated[start+position] ^= 0x80
					changes := 0
					for i := range baseline {
						if baseline[i] != mutated[i] {
							changes++
						}
					}
					if changes != 1 || bytes.Contains(mutated, []byte(GoOOMPlainBanner)) || bytes.Contains(mutated, []byte(GoOOMPrefixedBanner)) {
						t.Fatalf("split %d position %d mutation = %d changed bytes, want exactly one and no canonical banner", split, position, changes)
					}
					reader.Reset(mutated)
					after, err := ClassifyGoOOMBanner(t.Context(), request)
					if err != nil || after.Validate() != nil || after.State() != GoOOMBannerAbsent || after.BytesExamined() != before.BytesExamined() || after == before || reader.Len() != 0 {
						t.Fatalf("split %d position %d mutation = (%v, %v), unread %d, want absent, changed classification, preserved extent", split, position, after, err, reader.Len())
					}
				}
			}
		})
	}
}

// Reader stalls have a finite, explicit progress boundary. These cases prove
// both sides and that actual progress resets the consecutive-read counter.
func TestOOMConsecutiveEmptyReadBoundary(t *testing.T) {
	t.Parallel()
	const limit = core.ReaderConsecutiveEmptyReadMaximum
	cases := []struct {
		name                    string
		emptyBefore, emptyAfter int
		progressBetween         bool
		wantCalls, wantBytes    int
		wantErr                 error
	}{
		{name: "one below stall limit permits subsequent byte", emptyBefore: limit - 1, wantCalls: limit, wantBytes: 1},
		{name: "exact stall limit refuses before pending byte", emptyBefore: limit, wantCalls: limit, wantErr: io.ErrNoProgress},
		{name: "one above stall limit cannot consume extra source step", emptyBefore: limit + 1, wantCalls: limit, wantErr: io.ErrNoProgress},
		{name: "progress resets counter before second near-limit stall", emptyBefore: limit - 1, emptyAfter: limit - 1, progressBetween: true, wantCalls: 2 * limit, wantBytes: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			steps := make([]hostfactsReadStep, tc.emptyBefore)
			length := uint64(1)
			if tc.progressBetween {
				steps = append(steps, hostfactsReadStep{data: "x"})
				steps = append(steps, make([]hostfactsReadStep, tc.emptyAfter)...)
				length++
			}
			steps = append(steps, hostfactsReadStep{data: "y"})
			source := &hostfactsScriptReader{steps: steps}
			got, gotErr := ClassifyGoOOMBanner(t.Context(), GoOOMBannerRequest{Source: source, Length: mustByteLength(t, length)})
			if source.calls != tc.wantCalls || source.bytes != tc.wantBytes || len(source.steps) != len(steps)-tc.wantCalls {
				t.Fatalf("source = calls %d bytes %d pending %d, want %d/%d/%d", source.calls, source.bytes, len(source.steps), tc.wantCalls, tc.wantBytes, len(steps)-tc.wantCalls)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("classification error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, core.ErrHostFactsObservation) {
					t.Fatalf("stalled result = (%v, %v), want zero and typed observation refusal", got, gotErr)
				}
			} else if got.Validate() != nil || got.State() != GoOOMBannerAbsent || got.BytesExamined().Uint64() != length {
				t.Fatalf("completed result = %v, want absent and extent %d", got, length)
			}
		})
	}
}
