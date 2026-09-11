package release_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
)

func TestBuildOutputPolicySurvivesPreparation(t *testing.T) {
	t.Parallel()
	maximum, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("NewByteCount(1) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		policy  process.OutputPolicy
		wantErr error
	}{
		{name: "streaming carries no total extent", policy: process.OutputPolicy{Mode: process.OutputModeStreaming}},
		{name: "explicit bounded caller retains its exact extent", policy: process.OutputPolicy{Mode: process.OutputModeBounded, Maximum: maximum}},
		{name: "unknown mode cannot prepare execution", wantErr: core.ErrReleaseContract},
		{name: "streaming cannot hide a finite extent", policy: process.OutputPolicy{Mode: process.OutputModeStreaming, Maximum: maximum}, wantErr: core.ErrReleaseContract},
		{name: "bounded mode requires its declared extent", policy: process.OutputPolicy{Mode: process.OutputModeBounded}, wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, home := t.TempDir(), t.TempDir()
			request := buildProcessRequestForHostileTest(t, root, home)
			request.OutputPolicy = tc.policy
			got, err := release.PrepareBuildProcess(request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("PrepareBuildProcess() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if !zeroProcessRequest(got) {
					t.Fatalf("refused process = %+v, want zero", got)
				}
				return
			}
			if got.OutputPolicy != tc.policy {
				t.Fatalf("prepared output policy = %+v, want %+v", got.OutputPolicy, tc.policy)
			}
		})
	}
}
