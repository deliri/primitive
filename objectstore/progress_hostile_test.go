package objectstore

import (
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type progressBoundaryCase struct {
	name      string
	completed uint64
	total     uint64
}

func TestTransferProgressSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive upload and download extents close exact boundaries", func(t *testing.T) {
		t.Parallel()

		cases := []progressBoundaryCase{
			{name: "zero of one byte", completed: 0, total: 1},
			{name: "one of one byte", completed: 1, total: 1},
			{name: "one of two bytes", completed: 1, total: 2},
			{name: "maximum signed extent", completed: math.MaxInt64, total: math.MaxInt64},
		}
		for _, direction := range []Direction{DirectionUpload, DirectionDownload} {
			t.Run(direction.String(), func(t *testing.T) {
				t.Parallel()

				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						t.Parallel()

						completed := progressLength(t, tc.completed)
						total := progressLength(t, tc.total)
						got, gotErr := newTransferProgress(direction, completed, total)
						if gotErr != nil || got.Direction() != direction ||
							got.Completed() != completed || got.Total() != total {
							t.Fatalf("newTransferProgress(%v, %d, %d) = (%v, %v), want exact typed progress",
								direction, tc.completed, tc.total, got, gotErr)
						}
					})
				}
			})
		}
	})

	t.Run("negative every non-domain direction refuses otherwise valid extents", func(t *testing.T) {
		t.Parallel()

		zero := progressLength(t, 0)
		one := progressLength(t, 1)
		for raw := range 256 {
			direction := Direction(raw)
			got, gotErr := newTransferProgress(direction, zero, one)
			if direction == DirectionUpload || direction == DirectionDownload {
				if gotErr != nil || got.Direction() != direction || got.Completed() != zero || got.Total() != one {
					t.Fatalf("newTransferProgress(%v, valid extent) = (%v, %v), want exact direction, zero completed, one total",
						direction, got, gotErr)
				}
				continue
			}
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (TransferProgress{}) {
				t.Fatalf("newTransferProgress(Direction(%d)) = (%v, %v), want zero and errors.Is %v",
					raw, got, gotErr, core.ErrObjectStoreContract)
			}
		}
	})

	t.Run("neutral zero total carries no fabricated completion", func(t *testing.T) {
		t.Parallel()

		zero := progressLength(t, 0)
		got, gotErr := newTransferProgress(DirectionUpload, zero, zero)
		if gotErr != nil || got.Completed().Uint64() != 0 || got.Total().Uint64() != 0 {
			t.Fatalf("newTransferProgress(upload, zero, zero) = (%v, %v), want exact neutral progress", got, gotErr)
		}
	})
}

// Sequences attack cumulative accounting, early refusal, and retry after a
// refused observation. No arbitrary buffer-size rows: this writer does not buffer.
func TestProgressWriterAcceptedAndRefusedTransitions(t *testing.T) {
	t.Parallel()
	for _, direction := range []Direction{DirectionUpload, DirectionDownload} {
		t.Run(direction.String(), func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name          string
				total         uint64
				writes        []int
				refuseCall    int
				wantCompleted []uint64
				wantCalls     []int
				wantErrors    []error
			}{
				{name: "partial writes accumulate exactly", total: 3, writes: []int{1, 2}, wantCompleted: []uint64{1, 3}, wantCalls: []int{1, 2}, wantErrors: []error{nil, nil}},
				{name: "empty observation fabricates no bytes", total: 1, writes: []int{0, 1, 0}, wantCompleted: []uint64{0, 1, 1}, wantCalls: []int{1, 2, 3}, wantErrors: []error{nil, nil, nil}},
				{name: "oversized first write preserves next valid write", total: 1, writes: []int{2, 1}, wantCompleted: []uint64{0, 1}, wantCalls: []int{0, 1}, wantErrors: []error{core.ErrObjectStoreSize, nil}},
				{name: "overrun after partial progress preserves accepted prefix", total: 2, writes: []int{1, 2, 1}, wantCompleted: []uint64{1, 1, 2}, wantCalls: []int{1, 1, 2}, wantErrors: []error{nil, core.ErrObjectStoreSize, nil}},
				{name: "observer refusal does not consume extent", total: 2, writes: []int{1, 1, 1}, refuseCall: 2, wantCompleted: []uint64{1, 1, 2}, wantCalls: []int{1, 2, 3}, wantErrors: []error{nil, io.ErrClosedPipe, nil}},
				{name: "empty transfer refuses first content byte", writes: []int{0, 1, 0}, wantCompleted: []uint64{0, 0, 0}, wantCalls: []int{1, 1, 2}, wantErrors: []error{nil, core.ErrObjectStoreSize, nil}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					calls := 0
					var offered TransferProgress
					writer := progressDestination(func(progress TransferProgress) error {
						calls++
						offered = progress
						if calls == tc.refuseCall {
							return io.ErrClosedPipe
						}
						return nil
					}, direction, progressLength(t, tc.total))
					before := uint64(0)
					for step, size := range tc.writes {
						priorCalls, priorOffer := calls, offered
						n, err := writer.Write(make([]byte, size))
						wantN := size
						if tc.wantErrors[step] != nil {
							wantN = 0
						}
						if n != wantN || !errors.Is(err, tc.wantErrors[step]) || calls != tc.wantCalls[step] {
							t.Fatalf("step %d: written=%d error=%v calls=%d, want %d/%v/%d", step, n, err, calls, wantN, tc.wantErrors[step], tc.wantCalls[step])
						}
						if err != nil {
							identity := core.ErrObjectStoreSource
							if direction == DirectionDownload {
								identity = core.ErrObjectStoreDestination
							}
							if !errors.Is(err, identity) {
								t.Fatalf("step %d: error=%v, want direction identity %v", step, err, identity)
							}
						}
						if calls == priorCalls {
							if offered != priorOffer {
								t.Fatalf("step %d: refused extent changed observation from %+v to %+v", step, priorOffer, offered)
							}
						} else if offered.Direction() != direction || offered.Total().Uint64() != tc.total || offered.Completed().Uint64() != before+uint64(size) || offered.Validate() != nil {
							t.Fatalf("step %d: offered=%+v, want direction=%v completed=%d total=%d", step, offered, direction, before+uint64(size), tc.total)
						}
						before = tc.wantCompleted[step]
					}
				})
			}
		})
	}
}

func progressLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()

	got, gotErr := core.NewByteLength(value)
	if gotErr != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, gotErr)
	}
	return got
}
