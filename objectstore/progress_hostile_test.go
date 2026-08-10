package objectstore

import (
	"errors"
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
			{name: "one below first stream boundary", completed: 32<<10 - 1, total: 32 << 10},
			{name: "at first stream boundary", completed: 32 << 10, total: 32 << 10},
			{name: "one above first boundary within second", completed: 32<<10 + 1, total: 64 << 10},
			{name: "one below second stream boundary", completed: 64<<10 - 1, total: 64 << 10},
			{name: "at second stream boundary", completed: 64 << 10, total: 64 << 10},
			{name: "one above second boundary within third", completed: 64<<10 + 1, total: 96 << 10},
			{name: "maximum signed extent", completed: uint64(^uint64(0) >> 1), total: uint64(^uint64(0) >> 1)},
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

	t.Run("negative every non-domain direction and overrun reject", func(t *testing.T) {
		t.Parallel()

		zero := progressLength(t, 0)
		one := progressLength(t, 1)
		for raw := 0; raw <= int(^uint8(0)); raw++ {
			direction := Direction(raw)
			got, gotErr := newTransferProgress(direction, one, zero)
			if direction == DirectionUpload || direction == DirectionDownload {
				if !errors.Is(gotErr, core.ErrObjectStoreSize) || got != (TransferProgress{}) {
					t.Fatalf("newTransferProgress(%v, one above total) = (%v, %v), want zero and errors.Is %v",
						direction, got, gotErr, core.ErrObjectStoreSize)
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

func TestProgressWriterReportsOnlyAcceptedMonotonicBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		size int
	}{
		{name: "one byte", size: 1},
		{name: "two bytes", size: 2},
		{name: "one below first stream boundary", size: 32<<10 - 1},
		{name: "at first stream boundary", size: 32 << 10},
		{name: "one above first stream boundary", size: 32<<10 + 1},
		{name: "one below second stream boundary", size: 64<<10 - 1},
		{name: "at second stream boundary", size: 64 << 10},
		{name: "one above second stream boundary", size: 64<<10 + 1},
		{name: "at third stream boundary", size: 96 << 10},
		{name: "one above third stream boundary", size: 96<<10 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			total := progressLength(t, uint64(tc.size))
			var observed TransferProgress
			writer := progressDestination(func(progress TransferProgress) error {
				observed = progress
				return nil
			}, DirectionUpload, total)
			gotWritten, gotErr := writer.Write(make([]byte, tc.size))
			if gotErr != nil || gotWritten != tc.size || observed.Completed().Uint64() != uint64(tc.size) ||
				observed.Total() != total || observed.Direction() != DirectionUpload {
				t.Fatalf("progress write(%d) = (%d, %v, %v), want exact final upload progress",
					tc.size, gotWritten, gotErr, observed)
			}
		})
	}
}

func TestProgressWriterRefusesObserverAndExtentFailuresTransactionally(t *testing.T) {
	t.Parallel()

	total := progressLength(t, 1)
	for _, direction := range []Direction{DirectionUpload, DirectionDownload} {
		t.Run(direction.String(), func(t *testing.T) {
			t.Parallel()

			calls := 0
			writer := progressDestination(func(TransferProgress) error {
				calls++
				return core.ErrObjectStoreContract
			}, direction, total)
			gotWritten, gotErr := writer.Write([]byte{1})
			wantErr := core.ErrObjectStoreSource
			if direction == DirectionDownload {
				wantErr = core.ErrObjectStoreDestination
			}
			if gotWritten != 0 || calls != 1 || !errors.Is(gotErr, wantErr) {
				t.Fatalf("observer refusal = (%d bytes, %d calls, %v), want (0, 1, errors.Is %v)", gotWritten, calls, gotErr, wantErr)
			}

			calls = 0
			writer = progressDestination(func(TransferProgress) error {
				calls++
				return nil
			}, direction, total)
			gotWritten, gotErr = writer.Write([]byte{1, 2})
			if gotWritten != 0 || calls != 0 || !errors.Is(gotErr, core.ErrObjectStoreSize) {
				t.Fatalf("extent refusal = (%d bytes, %d calls, %v), want (0, 0, errors.Is %v)",
					gotWritten, calls, gotErr, core.ErrObjectStoreSize)
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
