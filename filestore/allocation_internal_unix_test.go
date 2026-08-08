//go:build unix

package filestore

import (
	"errors"
	"io/fs"
	"math"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

// statInfo is a typed test input carrying the real platform structure the
// production leaf reads. It is injected data, not a fake substrate: the value
// under the Sys answer is exactly the *syscall.Stat_t the standard library
// hands back, with its block count set to the boundary under pressure.
type statInfo struct {
	sys any
}

func (statInfo) Name() string       { return "carrier" }
func (statInfo) Size() int64        { return 0 }
func (statInfo) Mode() fs.FileMode  { return 0o600 }
func (statInfo) ModTime() time.Time { return time.Time{} }
func (statInfo) IsDir() bool        { return false }
func (s statInfo) Sys() any         { return s.sys }

// TestObservedAllocationProjectsAndRefusesAtEveryBlockBoundary drives the x512
// projection and its guards directly, because the native proof cannot: a dense
// or sparse file proves direction, not the exact constant, and never executes
// the garbage arms. The oracle is arithmetic stated in the row, so an x1024
// defect or a guard that quietly turned garbage into "unreported" goes red.
func TestObservedAllocationProjectsAndRefusesAtEveryBlockBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		sys          any
		wantErr      error
		name         string
		wantBytes    uint64
		wantReported bool
	}{
		{
			name: "one block is exactly five hundred twelve bytes",
			sys:  &syscall.Stat_t{Blocks: 1}, wantReported: true, wantBytes: 512,
		},
		{
			name: "two blocks are exactly one thousand twenty four bytes",
			sys:  &syscall.Stat_t{Blocks: 2}, wantReported: true, wantBytes: 1024,
		},
		{
			name: "zero blocks are a reported hole, not an unreported answer",
			sys:  &syscall.Stat_t{Blocks: 0}, wantReported: true, wantBytes: 0,
		},
		{
			name: "the largest byte-length-expressible count projects exactly",
			sys:  &syscall.Stat_t{Blocks: math.MaxInt64 / 512}, wantReported: true,
			wantBytes: uint64(math.MaxInt64/512) * 512,
		},
		{
			name:    "one block above the byte-length domain refuses",
			sys:     &syscall.Stat_t{Blocks: math.MaxInt64/512 + 1},
			wantErr: core.ErrFilestoreContract,
		},
		{
			name:    "a count that would wrap the multiplication refuses as garbage",
			sys:     &syscall.Stat_t{Blocks: int64(math.MaxUint64/512) + 1},
			wantErr: core.ErrFilestoreSource,
		},
		{
			name:    "a negative count refuses as garbage",
			sys:     &syscall.Stat_t{Blocks: -1},
			wantErr: core.ErrFilestoreSource,
		},
		{
			name: "a host whose FileInfo carries no Stat_t stays honestly unreported",
			sys:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := observedAllocation(statInfo{sys: tc.sys})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("observedAllocation() error = %v, want errors.Is %v", err, tc.wantErr)
				}
				if got != (Allocation{}) {
					t.Fatalf("observedAllocation() = %+v alongside the refusal, want the zero allocation", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("observedAllocation() error = %v, want nil", err)
			}
			if got.Reported() != tc.wantReported {
				t.Fatalf("observedAllocation().Reported() = %t, want %t", got.Reported(), tc.wantReported)
			}
			if !tc.wantReported {
				return
			}
			bytes, err := got.Bytes()
			if err != nil {
				t.Fatalf("Allocation.Bytes() error = %v, want nil", err)
			}
			if bytes.Uint64() != tc.wantBytes {
				t.Fatalf("observedAllocation() = %d bytes, want %d", bytes.Uint64(), tc.wantBytes)
			}
		})
	}
}
