//go:build unix

package process

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestUnixProcessIDLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		id      ProcessIdentity
		want    int
		wantErr error
	}{
		{name: "neutral/zero has no process identity", wantErr: core.ErrProcessContract},
		{name: "positive/minimum PID remains positive", id: 1, want: 1},
		{name: "boundary/below signed PID maximum", id: math.MaxInt32 - 1, want: math.MaxInt32 - 1},
		{name: "boundary/signed PID maximum remains exact", id: math.MaxInt32, want: math.MaxInt32},
		{name: "negative/sign bit cannot become a process group", id: ProcessIdentity(math.MaxInt32) + 1, wantErr: core.ErrProcessContract},
		{name: "negative/unsigned tail cannot become group two", id: math.MaxUint32 - 1, wantErr: core.ErrProcessContract},
		{name: "negative/unsigned maximum cannot become every process", id: math.MaxUint32, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := unixProcessID(tc.id)
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("native PID=%d, %v; want %d, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// Alive performs only a signal-zero observation. These invalid identities may
// never be narrowed into a group-wide probe; no test delivers a real signal.
func TestUnixAliveIdentityBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	self, err := Self()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		id      ProcessIdentity
		want    Liveness
		wantErr error
	}{
		{name: "positive/self is a real live process", id: self, want: LivenessAlive},
		{name: "neutral/zero cannot probe the caller group", wantErr: core.ErrProcessContract},
		{name: "negative/signed overflow cannot probe a group", id: ProcessIdentity(core.ProcessPOSIXIdentityMaximum) + 1, wantErr: core.ErrProcessContract},
		{name: "negative/unsigned tail cannot probe group two", id: math.MaxUint32 - 1, wantErr: core.ErrProcessContract},
		{name: "negative/unsigned maximum cannot probe every process", id: math.MaxUint32, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Alive(tc.id)
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("Alive=%v, %v; want %v, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
