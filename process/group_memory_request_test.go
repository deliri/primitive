package process_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestGroupObservationRequestRefusesBeforeObservation(t *testing.T) {
	t.Parallel()
	directory, err := core.ParseAbsolutePath("/")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		request process.GroupObservationRequest
	}{
		{name: "zero identity", request: process.GroupObservationRequest{WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "all processes selector refused", request: process.GroupObservationRequest{Group: 1, WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "native identity overflow", request: process.GroupObservationRequest{Group: process.ProcessIdentity(core.ProcessPOSIXIdentityMaximum) + 1, WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "missing directory", request: process.GroupObservationRequest{Group: 42, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "missing environment mode", request: process.GroupObservationRequest{Group: 42, WorkingDirectory: directory}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := process.ObserveGroupMemory(t.Context(), tc.request)
			if got.Uint64() != 0 || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("memory observation=(%d,%v), want zero contract refusal", got.Uint64(), err)
			}
			cpu, err := process.ObserveGroupCPU(t.Context(), tc.request)
			if cpu != 0 || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("CPU observation=(%d,%v), want zero contract refusal", cpu, err)
			}
		})
	}
	request := process.GroupObservationRequest{Group: 42, WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		wantErr error
	}{
		{name: "absent context", wantErr: core.ErrNilContext},
		{name: "cancelled context", ctx: ctx, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := process.ObserveGroupMemory(tc.ctx, request)
			if got.Uint64() != 0 || !errors.Is(err, tc.wantErr) {
				t.Fatalf("memory context observation=(%d,%v), want zero %v", got.Uint64(), err, tc.wantErr)
			}
			cpu, err := process.ObserveGroupCPU(tc.ctx, request)
			if cpu != 0 || !errors.Is(err, tc.wantErr) {
				t.Fatalf("CPU context observation=(%d,%v), want zero %v", cpu, err, tc.wantErr)
			}
		})
	}
}
