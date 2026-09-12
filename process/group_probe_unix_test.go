//go:build unix

package process

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGroupProbeLayerTriadDistinguishesAbsenceFromDeniedSignal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      error
		want    Liveness
		wantErr error
	}{
		{name: "signalable group exists", want: LivenessAlive},
		{name: "only ESRCH proves absent group", in: syscall.ESRCH, want: LivenessGone},
		{name: "EPERM group remains present including Darwin zombies", in: syscall.EPERM, want: LivenessAlive},
		{name: "wrapped absence preserves absence", in: &os.SyscallError{Syscall: "kill", Err: syscall.ESRCH}, want: LivenessGone},
		{name: "wrapped denial never means absent", in: &os.SyscallError{Syscall: "kill", Err: syscall.EPERM}, want: LivenessAlive},
		{name: "unexpected native refusal exposes no liveness fact", in: syscall.EINVAL, want: LivenessUnknown, wantErr: core.ErrProcessObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := groupProbeLiveness(tc.in)
			if got != tc.want || !errors.Is(gotErr, tc.wantErr) || tc.wantErr != nil && !errors.Is(gotErr, tc.in) {
				t.Fatalf("group probe = (%v, %v), want (%v, %v) retaining native cause %v", got, gotErr, tc.want, tc.wantErr, tc.in)
			}
		})
	}
}
