//go:build darwin || linux

package filelock

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
	"testing"
)

func TestNativeLockFlagAgreementLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		ex      Exclusivity
		p       Patience
		want    int
		wantErr error
	}{
		{"exclusive_immediate", Exclusive, Immediate, unix.LOCK_EX | unix.LOCK_NB, nil},
		{"exclusive_blocking", Exclusive, Blocking, unix.LOCK_EX, nil},
		{"shared_immediate", Shared, Immediate, unix.LOCK_SH | unix.LOCK_NB, nil},
		{"shared_blocking", Shared, Blocking, unix.LOCK_SH, nil},
		{"unset_exclusivity", ExclusivityUnknown, Immediate, 0, core.ErrPrimitiveContract},
		{"unset_patience", Exclusive, PatienceUnknown, 0, core.ErrPrimitiveContract},
		{"future_exclusivity", Exclusivity(3), Immediate, 0, core.ErrPrimitiveContract},
		{"future_patience", Exclusive, Patience(3), 0, core.ErrPrimitiveContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := lockFlags(tc.ex, tc.p)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("flags=%#x error=%v, want %#x and %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
func TestNativeInvalidDescriptorCannotBecomeContention(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		ex     Exclusivity
		p      Patience
		unlock bool
	}{
		{"exclusive_immediate", Exclusive, Immediate, false},
		{"shared_immediate", Shared, Immediate, false},
		{"exclusive_blocking", Exclusive, Blocking, false},
		{"shared_blocking", Shared, Blocking, false},
		{"release_invalid", ExclusivityUnknown, PatienceUnknown, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			held := false
			var err error
			if tc.unlock {
				err = release(^uintptr(0))
			} else {
				held, err = acquire(^uintptr(0), tc.ex, tc.p)
			}
			if !errors.Is(err, unix.EBADF) || held {
				t.Fatalf("held=%v error=%v, want false and native EBADF", held, err)
			}
		})
	}
}
