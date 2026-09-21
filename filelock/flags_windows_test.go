//go:build windows

package filelock

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/windows"
	"testing"
)

func TestNativeLockFlagAgreementLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		ex      Exclusivity
		p       Patience
		want    uint32
		wantErr error
	}{
		{name: "exclusive_immediate", ex: Exclusive, p: Immediate, want: windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY, wantErr: nil},
		{name: "exclusive_blocking", ex: Exclusive, p: Blocking, want: windows.LOCKFILE_EXCLUSIVE_LOCK, wantErr: nil},
		{name: "shared_immediate", ex: Shared, p: Immediate, want: windows.LOCKFILE_FAIL_IMMEDIATELY, wantErr: nil},
		{name: "shared_blocking", ex: Shared, p: Blocking, want: 0, wantErr: nil},
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
