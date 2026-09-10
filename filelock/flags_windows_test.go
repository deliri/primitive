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
		{"exclusive_immediate", Exclusive, Immediate, windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY, nil},
		{"exclusive_blocking", Exclusive, Blocking, windows.LOCKFILE_EXCLUSIVE_LOCK, nil},
		{"shared_immediate", Shared, Immediate, windows.LOCKFILE_FAIL_IMMEDIATELY, nil},
		{"shared_blocking", Shared, Blocking, 0, nil},
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
