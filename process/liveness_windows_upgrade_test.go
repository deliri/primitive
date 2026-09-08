//go:build windows

package process

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// These rows exhaust the documented outcomes of a zero-timeout process wait;
// abandoned mutex and unknown statuses cannot fabricate a process observation.
func TestWindowsWaitLivenessLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		status  uint32
		cause   error
		want    Liveness
		wantErr error
	}{
		{name: "positive/nonsignaled process remains alive", status: syscall.WAIT_TIMEOUT, want: LivenessAlive},
		{name: "neutral/signaled process is gone regardless of exit code", status: syscall.WAIT_OBJECT_0, want: LivenessGone},
		{name: "negative/abandoned mutex is not a process outcome", status: syscall.WAIT_ABANDONED, wantErr: core.ErrProcessObservation},
		{name: "negative/unknown status cannot become alive", status: syscall.WAIT_OBJECT_0 + 1, wantErr: core.ErrProcessObservation},
		{name: "negative/failed wait preserves native error", status: syscall.WAIT_FAILED, cause: syscall.ERROR_BROKEN_PIPE, wantErr: core.ErrProcessObservation},
		{name: "negative/error overrides apparent completion", status: syscall.WAIT_OBJECT_0, cause: syscall.ERROR_ACCESS_DENIED, wantErr: core.ErrProcessObservation},
		{name: "negative/error overrides apparent liveness", status: syscall.WAIT_TIMEOUT, cause: syscall.ERROR_BROKEN_PIPE, wantErr: core.ErrProcessObservation},
		{name: "negative/failed wait without errno still refuses", status: syscall.WAIT_FAILED, wantErr: core.ErrProcessObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := livenessFromWait(tc.status, tc.cause)
			if got != tc.want || !errors.Is(err, tc.wantErr) || tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("wait observation=%v, %v; want %v, %v preserving %v", got, err, tc.want, tc.wantErr, tc.cause)
			}
		})
	}
}

func TestWindowsOpenLivenessLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		cause   error
		want    Liveness
		wantErr error
	}{
		{name: "positive/denied access still proves existence", cause: syscall.ERROR_ACCESS_DENIED, want: LivenessAlive},
		{name: "positive/wrapped denial retains existence", cause: fmt.Errorf("open: %w", syscall.ERROR_ACCESS_DENIED), want: LivenessAlive},
		{name: "neutral/absent identity is gone", cause: core.WindowsProcessInvalidParameter, want: LivenessGone},
		{name: "neutral/wrapped absence remains gone", cause: fmt.Errorf("open: %w", core.WindowsProcessInvalidParameter), want: LivenessGone},
		{name: "negative/broken pipe is no liveness fact", cause: syscall.ERROR_BROKEN_PIPE, wantErr: core.ErrProcessObservation},
		{name: "negative/unrelated missing file is not a gone process", cause: syscall.ERROR_FILE_NOT_FOUND, wantErr: core.ErrProcessObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := livenessFromOpenError(tc.cause)
			if got != tc.want || !errors.Is(err, tc.wantErr) || tc.wantErr != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("open observation=%v, %v; want %v, %v preserving %v", got, err, tc.want, tc.wantErr, tc.cause)
			}
		})
	}
}
