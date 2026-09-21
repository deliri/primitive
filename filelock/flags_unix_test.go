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
		wantErr error
		name    string
		want    int
		ex      Exclusivity
		p       Patience
	}{
		{name: "exclusive_immediate", ex: Exclusive, p: Immediate, want: unix.LOCK_EX | unix.LOCK_NB, wantErr: nil},
		{name: "exclusive_blocking", ex: Exclusive, p: Blocking, want: unix.LOCK_EX, wantErr: nil},
		{name: "shared_immediate", ex: Shared, p: Immediate, want: unix.LOCK_SH | unix.LOCK_NB, wantErr: nil},
		{name: "shared_blocking", ex: Shared, p: Blocking, want: unix.LOCK_SH, wantErr: nil},
		{name: "unset_exclusivity", ex: ExclusivityUnknown, p: Immediate, want: 0, wantErr: core.ErrPrimitiveContract},
		{name: "unset_patience", ex: Exclusive, p: PatienceUnknown, want: 0, wantErr: core.ErrPrimitiveContract},
		{name: "future_exclusivity", ex: Exclusivity(3), p: Immediate, want: 0, wantErr: core.ErrPrimitiveContract},
		{name: "future_patience", ex: Exclusive, p: Patience(3), want: 0, wantErr: core.ErrPrimitiveContract},
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
		{name: "exclusive_immediate", ex: Exclusive, p: Immediate, unlock: false},
		{name: "shared_immediate", ex: Shared, p: Immediate, unlock: false},
		{name: "exclusive_blocking", ex: Exclusive, p: Blocking, unlock: false},
		{name: "shared_blocking", ex: Shared, p: Blocking, unlock: false},
		{name: "release_invalid", ex: ExclusivityUnknown, p: PatienceUnknown, unlock: true},
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
