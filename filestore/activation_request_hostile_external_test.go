package filestore_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestActivationRequestClosesEveryPreEffectBoundary(t *testing.T) {
	t.Parallel()

	root := requireTestRoot(t, t.TempDir())
	for _, size := range []uint64{0, 1, 2, 32<<10 - 1, 32 << 10, 32<<10 + 1, 64<<10 - 1, 64 << 10, 64<<10 + 1, uint64(^uint64(0) >> 1)} {
		expected, err := core.NewByteLength(size)
		if err != nil {
			t.Fatalf("core.NewByteLength(%d) error = %v, want nil", size, err)
		}
		request := filestore.ActivationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Target:    mustRelativePath(t, "target"), ExpectedBytes: expected,
			Mode: 0o600, Install: filestore.InstallReplace,
		}
		if err := request.Validate(); err != nil {
			t.Fatalf("ActivationRequest.Validate(%d) error = %v, want nil", size, err)
		}
		stage := request.StageDestination()
		if err := stage.Validate(); err != nil || stage.Temporary != request.Temporary ||
			stage.ExpectedBytes != expected || stage.Mode != request.Mode {
			t.Fatalf("ActivationRequest.StageDestination(%d) = (%v, %v), want exact plan projection", size, stage, err)
		}
	}

	valid := filestore.ActivationRequest{
		Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
		Target:    mustRelativePath(t, "target"), ExpectedBytes: mustActivationLength(t, 1),
		Mode: 0o600, Install: filestore.InstallReplace,
	}
	cases := []struct {
		mutate func(*filestore.ActivationRequest)
		name   string
	}{
		{name: "zero request", mutate: func(request *filestore.ActivationRequest) { *request = filestore.ActivationRequest{} }},
		{name: "missing root", mutate: func(request *filestore.ActivationRequest) { request.Temporary.Root = nil }},
		{name: "zero temporary", mutate: func(request *filestore.ActivationRequest) { request.Temporary.Path = core.RelativePath{} }},
		{name: "root temporary", mutate: func(request *filestore.ActivationRequest) { request.Temporary.Path = mustRelativePath(t, ".") }},
		{name: "zero target", mutate: func(request *filestore.ActivationRequest) { request.Target = core.RelativePath{} }},
		{name: "root target", mutate: func(request *filestore.ActivationRequest) { request.Target = mustRelativePath(t, ".") }},
		{name: "target equals temporary", mutate: func(request *filestore.ActivationRequest) { request.Target = request.Temporary.Path }},
		{name: "zero mode", mutate: func(request *filestore.ActivationRequest) { request.Mode = 0 }},
		{name: "typed mode bits", mutate: func(request *filestore.ActivationRequest) { request.Mode = fs.ModeDir | 0o700 }},
		{name: "zero install", mutate: func(request *filestore.ActivationRequest) { request.Install = filestore.InstallUnknown }},
		{name: "install above domain", mutate: func(request *filestore.ActivationRequest) { request.Install = filestore.InstallMode(255) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := valid
			tc.mutate(&request)
			if err := request.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("ActivationRequest.Validate(%s) error = %v, want errors.Is %v", tc.name, err, core.ErrFilestoreContract)
			}
		})
	}
}

func TestActivationRequestBindsOnlyItsExactCompletedStage(t *testing.T) {
	t.Parallel()

	root := requireTestRoot(t, t.TempDir())
	plan := filestore.ActivationRequest{
		Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".planned")},
		Target:    mustRelativePath(t, "target"), ExpectedBytes: mustActivationLength(t, 1),
		Mode: 0o600, Install: filestore.InstallCreate,
	}
	destination, err := filestore.OpenStageDestination(t.Context(), plan.StageDestination())
	if err != nil {
		t.Fatalf("OpenStageDestination() error = %v, want nil", err)
	}
	file, err := destination.File()
	if err != nil {
		t.Fatalf("StageDestination.File() error = %v, want nil", err)
	}
	if _, err := file.Write([]byte{1}); err != nil {
		t.Fatalf("stage file Write() error = %v, want nil", err)
	}
	staged, err := filestore.FinishStageDestination(t.Context(), destination)
	if err != nil {
		t.Fatalf("FinishStageDestination() error = %v, want nil", err)
	}
	commit, err := plan.CommitRequest(staged)
	if err != nil || commit.Validate() != nil {
		t.Fatalf("ActivationRequest.CommitRequest(exact) = (%v, %v), want valid and nil", commit, err)
	}

	otherPlan := plan
	otherPlan.Temporary.Path = mustRelativePath(t, ".other")
	if got, gotErr := otherPlan.CommitRequest(staged); !errors.Is(gotErr, core.ErrFilestoreContract) || got != (filestore.CommitRequest{}) {
		t.Fatalf("ActivationRequest.CommitRequest(other stage) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrFilestoreContract)
	}
	if err := filestore.Discard(t.Context(), staged); err != nil {
		t.Fatalf("filestore.Discard() error = %v, want nil", err)
	}
}

func mustActivationLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}
