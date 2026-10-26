package process_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestGroupMemoryRequestRefusesBeforeObservation(t *testing.T) {
	t.Parallel()
	directory, err := core.ParseAbsolutePath("/")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		request process.GroupMemoryRequest
	}{
		{name: "zero identity", request: process.GroupMemoryRequest{WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "all processes selector refused", request: process.GroupMemoryRequest{Group: 1, WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "native identity overflow", request: process.GroupMemoryRequest{Group: process.ProcessIdentity(core.ProcessPOSIXIdentityMaximum) + 1, WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "missing directory", request: process.GroupMemoryRequest{Group: 42, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}},
		{name: "missing environment mode", request: process.GroupMemoryRequest{Group: 42, WorkingDirectory: directory}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := process.ObserveGroupMemory(t.Context(), tc.request)
			if got.Uint64() != 0 || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("observation=(%d,%v), want zero contract refusal", got.Uint64(), err)
			}
			usage, err := process.ObserveGroupUsage(t.Context(), tc.request)
			if usage != (process.GroupUsage{}) || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("usage=(%v,%v), want zero contract refusal", usage, err)
			}
		})
	}
	request := process.GroupMemoryRequest{Group: 42, WorkingDirectory: directory, Environment: process.Environment{Mode: process.EnvironmentModeInherit}}
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
				t.Fatalf("context observation=(%d,%v), want zero %v", got.Uint64(), err, tc.wantErr)
			}
			usage, err := process.ObserveGroupUsage(tc.ctx, request)
			if usage != (process.GroupUsage{}) || !errors.Is(err, tc.wantErr) {
				t.Fatalf("context usage=(%v,%v), want zero %v", usage, err, tc.wantErr)
			}
		})
	}
}
