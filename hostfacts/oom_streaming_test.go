package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// A regression workload, not an input limit.
const goOOMFixtureBytes = 1 << 20

type oomRepeatedSource struct {
	remaining      uint64
	maximumRequest int
}

func (s *oomRepeatedSource) Read(p []byte) (int, error) {
	s.maximumRequest = max(s.maximumRequest, len(p))
	count := int(min(uint64(len(p)), s.remaining))
	for index := range p[:count] {
		p[index] = 'x'
	}
	s.remaining -= uint64(count)
	if count == 0 {
		return 0, io.EOF
	}
	return count, nil
}

func TestGoOOMStreamingPastFormerCeiling(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		suffix string
		want   GoOOMBannerState
	}{
		{name: "late banner after former ceiling is observed", suffix: GoOOMPlainBanner, want: GoOOMBannerPresent},
		{name: "complete large stream without banner remains absent", want: GoOOMBannerAbsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prefix := &oomRepeatedSource{remaining: goOOMFixtureBytes + 1}
			length := uint64(goOOMFixtureBytes + 1 + len(tc.suffix))
			got, err := ClassifyGoOOMBanner(t.Context(), GoOOMBannerRequest{
				Source: io.MultiReader(prefix, strings.NewReader(tc.suffix)), Length: mustByteLength(t, length),
			})
			if err != nil || got.State() != tc.want || got.BytesExamined().Uint64() != length || prefix.remaining != 0 {
				t.Fatalf("ClassifyGoOOMBanner() = (%v, %v), unread %d, want state %v extent %d unread 0", got, err, prefix.remaining, tc.want, length)
			}
			if prefix.maximumRequest > goOOMBufferBytes {
				t.Fatalf("largest source read = %d, want <= fixed buffer %d", prefix.maximumRequest, goOOMBufferBytes)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var replay GoOOMBannerEvidence
			if err := replay.UnmarshalJSON(encoded); err != nil || replay != got {
				t.Fatalf("evidence round trip = (%v, %v), want (%v, nil)", replay, err, got)
			}
		})
	}
}

func TestGoOOMMaximumExtentPreservesSourceRefusal(t *testing.T) {
	t.Parallel()
	got, err := ClassifyGoOOMBanner(t.Context(), GoOOMBannerRequest{
		Source: strings.NewReader(""), Length: mustByteLength(t, math.MaxInt64),
	})
	if got != (GoOOMBannerEvidence{}) || !errors.Is(err, io.ErrUnexpectedEOF) || !errors.Is(err, core.ErrHostFactsObservation) {
		t.Fatalf("maximum extent with empty source = (%v, %v), want zero and typed unexpected EOF", got, err)
	}
}

func TestGoOOMRequestRejectsTypedNilReader(t *testing.T) {
	t.Parallel()
	var source *strings.Reader
	err := (GoOOMBannerRequest{Source: source}).Validate()
	if !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("typed nil request.Validate() error = %v, want ErrHostFactsContract", err)
	}
}

type oomReadEnding uint8

const (
	oomReadEndingOrdinary oomReadEnding = iota
	oomReadEndingEOF
	oomReadEndingFailure
	oomReadEndingMixedEOF
	oomReadEndingCancellation
	oomReadEndingLimit
)

// Fault injection at io.Reader, not a replacement classifier or a fabricated
// evidence record. Every byte still enters ClassifyGoOOMBanner.
type oomFaultSource struct {
	reader    *bytes.Reader
	cancel    context.CancelFunc
	ending    oomReadEnding
	calls     int
	consumed  int
	delivered error
	cancelled bool
}

func (s *oomFaultSource) Read(p []byte) (int, error) {
	s.calls++
	count, err := s.reader.Read(p)
	s.consumed += count
	if s.reader.Len() != 0 {
		return count, err
	}
	switch s.ending {
	case oomReadEndingOrdinary:
	case oomReadEndingEOF:
		err = io.EOF
	case oomReadEndingFailure:
		err = io.ErrClosedPipe
	case oomReadEndingMixedEOF:
		err = errors.Join(io.EOF, io.ErrClosedPipe)
	case oomReadEndingCancellation:
		s.cancel()
		s.cancelled = true
	default:
		panic("invalid test reader ending")
	}
	s.delivered = err
	return count, err
}

func FuzzGoOOMFinalReadEvidence(f *testing.F) {
	// Go owns these canonical diagnostics; the boundary consumes their raw
	// bytes rather than a JSON document. No wire spelling is duplicated here.
	for ending := oomReadEndingOrdinary; ending < oomReadEndingLimit; ending++ {
		f.Add([]byte(GoOOMPlainBanner), uint8(ending))
	}
	f.Add([]byte{}, uint8(oomReadEndingFailure))
	f.Add([]byte(GoOOMPrefixedBanner), uint8(oomReadEndingMixedEOF))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		ending := oomReadEnding(selector % uint8(oomReadEndingLimit))
		source := &oomFaultSource{reader: bytes.NewReader(data), cancel: cancel, ending: ending}
		request := GoOOMBannerRequest{Source: source, Length: mustByteLength(t, uint64(len(data)))}
		if err := request.Validate(); err != nil {
			t.Fatalf("request.Validate() error = %v, want nil", err)
		}
		got, gotErr := ClassifyGoOOMBanner(ctx, request)
		wantCalls := 0
		if len(data) != 0 {
			wantCalls = 1 + (len(data)-1)/goOOMBufferBytes
		}
		if source.calls != wantCalls || source.consumed != len(data) || source.reader.Len() != 0 {
			t.Fatalf("source = calls %d bytes %d unread %d, want %d/%d/0", source.calls, source.consumed, source.reader.Len(), wantCalls, len(data))
		}
		// A zero extent never invokes Read: no terminal fault was delivered.
		var wantErr error
		if len(data) != 0 {
			switch ending {
			case oomReadEndingOrdinary, oomReadEndingEOF:
			case oomReadEndingFailure, oomReadEndingMixedEOF:
				wantErr = io.ErrClosedPipe
				if !errors.Is(source.delivered, wantErr) {
					t.Fatalf("injected source error = %v, want %v", source.delivered, wantErr)
				}
			case oomReadEndingCancellation:
				wantErr = context.Canceled
				if !source.cancelled || !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatalf("source cancellation = %v/%v, want true/Canceled", source.cancelled, ctx.Err())
				}
			default:
				t.Fatalf("test ending = %v, want an admitted ending", ending)
			}
		}
		if wantErr != nil {
			var refusal Failure
			if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, wantErr) || !errors.As(gotErr, &refusal) || refusal.Operation != OperationGoOOMBanner || refusal.Identity != core.ErrHostFactsObservation {
				t.Fatalf("refused observation = (%v, %v), want zero and operation-bound %v", got, gotErr, wantErr)
			}
			encoded, err := got.MarshalJSON()
			if len(encoded) != 0 || !errors.Is(err, core.ErrHostFactsEvidence) {
				t.Fatalf("refusal encoding = (%q, %v), want no bytes and ErrHostFactsEvidence", encoded, err)
			}
			return
		}
		wantPresent := bytes.Contains(data, []byte(GoOOMPlainBanner)) || bytes.Contains(data, []byte(GoOOMPrefixedBanner))
		if gotErr != nil || got.Validate() != nil || (got.State() == GoOOMBannerPresent) != wantPresent || got.BytesExamined().Uint64() != uint64(len(data)) {
			t.Fatalf("accepted observation = (%v, %v), want presence %v and extent %d", got, gotErr, wantPresent, len(data))
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) == 0 {
			t.Fatalf("accepted encoding = (%q, %v), want nonempty and nil", encoded, err)
		}
		var replay GoOOMBannerEvidence
		if err := replay.UnmarshalJSON(encoded); err != nil || replay != got {
			t.Fatalf("canonical replay = (%v, %v), want (%v, nil)", replay, err, got)
		}
		second, err := replay.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second encoding = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}
