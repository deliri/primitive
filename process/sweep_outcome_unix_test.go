//go:build unix

package process

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// Kill has a small outcome domain: success, absence, denied permission, and
// invalid signal. Wrapping must preserve that identity without changing the
// meaning of a refused effect. Native group execution is exercised separately.
func TestGroupSweepOutcomeLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input error
		want  error
	}{
		{name: "positive/delivered signal remains successful"},
		{name: "neutral/absent group remains a successful no-op", input: unix.ESRCH},
		{name: "negative/permission denial cannot prove cleanup", input: unix.EPERM, want: unix.EPERM},
		{name: "negative/invalid signal is preserved", input: unix.EINVAL, want: unix.EINVAL},
		{name: "boundary/wrapped absence stays absence", input: &os.SyscallError{Syscall: "kill", Err: unix.ESRCH}},
		{name: "boundary/wrapped permission denial stays denied", input: &os.SyscallError{Syscall: "kill", Err: unix.EPERM}, want: unix.EPERM},
		{name: "boundary/wrapped invalid signal stays invalid", input: &os.SyscallError{Syscall: "kill", Err: unix.EINVAL}, want: unix.EINVAL},
		{name: "boundary/other permission identity is not absence", input: os.ErrPermission, want: os.ErrPermission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := groupSweepError(tc.input)
			if !errors.Is(got, tc.want) {
				t.Fatalf("group sweep outcome = %v, want identity %v", got, tc.want)
			}
			if tc.want != nil && got != tc.input {
				t.Fatalf("group sweep replaced its native failure: got %v, original %v", got, tc.input)
			}
		})
	}
}
