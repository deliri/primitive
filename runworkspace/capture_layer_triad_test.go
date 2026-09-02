package runworkspace_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runworkspace"
	"github.com/deliri/primitive/v2026/standard"
)

func TestCaptureLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive streamed bytes seal the exact path digest and extent", func(t *testing.T) {
		t.Parallel()
		manager, _, experiment, root := captureFixture(t)
		defer closeScrubManager(t, manager)
		capture, err := manager.OpenCapture(context.Background(), experiment, runworkspace.CaptureStdout)
		if err != nil {
			t.Fatalf("Manager.OpenCapture(stdout) error = %v, want nil", err)
		}
		content := []byte("complete execution evidence\n")
		if got, gotErr := capture.Write(content); gotErr != nil || got != len(content) {
			t.Fatalf("Capture.Write() = (%d, %v), want (%d, nil)", got, gotErr, len(content))
		}
		evidence, err := capture.Seal()
		if err != nil {
			t.Fatalf("Capture.Seal() error = %v, want nil", err)
		}
		if evidence.SHA256 != core.SHA256Of(content) || evidence.Bytes.Uint64() != uint64(len(content)) {
			t.Fatalf("CaptureEvidence digest/bytes = %v/%d, want %v/%d", evidence.SHA256, evidence.Bytes.Uint64(), core.SHA256Of(content), len(content))
		}
		var streamed bytes.Buffer
		if streamErr := manager.StreamCaptureEvidence(t.Context(), experiment, evidence, &streamed); streamErr != nil || !bytes.Equal(streamed.Bytes(), content) {
			t.Fatalf("Manager.StreamCaptureEvidence(stdout) = (%q, %v), want (%q, nil)", streamed.Bytes(), streamErr, content)
		}
		got := readCaptureEvidence(t, root, evidence)
		if string(got) != string(content) {
			t.Fatalf("captured disk bytes = %q, want %q", got, content)
		}
	})

	t.Run("negative duplicate capture identity refuses overwrite and preserves first bytes", func(t *testing.T) {
		t.Parallel()
		manager, _, experiment, root := captureFixture(t)
		defer closeScrubManager(t, manager)
		first, err := manager.OpenCapture(context.Background(), experiment, runworkspace.CaptureStderr)
		if err != nil {
			t.Fatalf("Manager.OpenCapture(first stderr) error = %v, want nil", err)
		}
		if _, err := first.Write([]byte("first")); err != nil {
			t.Fatalf("Capture.Write(first stderr) error = %v, want nil", err)
		}
		evidence, err := first.Seal()
		if err != nil {
			t.Fatalf("Capture.Seal(first stderr) error = %v, want nil", err)
		}

		second, gotErr := manager.OpenCapture(context.Background(), experiment, runworkspace.CaptureStderr)
		if gotErr == nil || second != nil {
			t.Fatalf("Manager.OpenCapture(duplicate stderr) = (%v, %v), want nil and typed refusal", second, gotErr)
		}
		if got := string(readCaptureEvidence(t, root, evidence)); got != "first" {
			t.Fatalf("first capture after duplicate refusal = %q, want %q", got, "first")
		}
	})

	t.Run("neutral empty stream seals the real empty digest without invented bytes", func(t *testing.T) {
		t.Parallel()
		manager, _, experiment, root := captureFixture(t)
		defer closeScrubManager(t, manager)
		capture, err := manager.OpenCapture(context.Background(), experiment, runworkspace.CaptureStdout)
		if err != nil {
			t.Fatalf("Manager.OpenCapture(empty stdout) error = %v, want nil", err)
		}
		evidence, err := capture.Seal()
		if err != nil {
			t.Fatalf("Capture.Seal(empty stdout) error = %v, want nil", err)
		}
		if evidence.Bytes.Uint64() != 0 || evidence.SHA256 != core.SHA256Of(nil) {
			t.Fatalf("empty CaptureEvidence digest/bytes = %v/%d, want %v/0", evidence.SHA256, evidence.Bytes.Uint64(), core.SHA256Of(nil))
		}
		var streamed bytes.Buffer
		if streamErr := manager.StreamCaptureEvidence(t.Context(), experiment, evidence, &streamed); streamErr != nil || streamed.Len() != 0 {
			t.Fatalf("Manager.StreamCaptureEvidence(empty stdout) = (%q, %v), want empty and nil", streamed.Bytes(), streamErr)
		}
		if got := readCaptureEvidence(t, root, evidence); len(got) != 0 {
			t.Fatalf("empty capture disk bytes = %q, want empty", got)
		}
	})
}

func captureFixture(t *testing.T) (runworkspace.Manager, runworkspace.Member, runworkspace.Experiment, core.AbsolutePath) {
	t.Helper()
	root, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(t.TempDir()) setup error = %v, want nil", err)
	}
	manager, err := runworkspace.Open(context.Background(), runworkspace.Configuration{RunParent: root})
	if err != nil {
		t.Fatalf("runworkspace.Open() setup error = %v, want nil", err)
	}
	uuid, err := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	if err != nil {
		t.Fatalf("id.ParseUUIDv7() setup error = %v, want nil", err)
	}
	unit, err := manager.CreateUnit(context.Background(), runnercontrol.SchedulingUnitIdentity{Kind: runnercontrol.SchedulingUnitRunPlan, Identity: uuid})
	run, runErr := standard.NewRunID(uuid)
	experimentID, experimentErr := standard.NewExperimentID(uuid)
	if err := errors.Join(err, runErr, experimentErr); err != nil {
		t.Fatalf("capture identity and unit setup error = %v, want nil", err)
	}
	member, err := manager.CreateMember(context.Background(), unit, run)
	if err != nil {
		t.Fatalf("Manager.CreateMember() setup error = %v, want nil", err)
	}
	experiment, err := manager.CreateExperiment(context.Background(), member, experimentID)
	if err != nil {
		t.Fatalf("Manager.CreateExperiment() setup error = %v, want nil", err)
	}
	return manager, member, experiment, root
}

func readCaptureEvidence(t *testing.T, rootPath core.AbsolutePath, evidence runworkspace.CaptureEvidence) []byte {
	t.Helper()
	root, err := filestore.OpenRoot(context.Background(), rootPath)
	if err != nil {
		t.Fatalf("filestore.OpenRoot(capture root) error = %v, want nil", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Fatalf("capture read root close error = %v, want nil", closeErr)
		}
	}()
	file, err := filestore.OpenRead(context.Background(), filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: evidence.Path}})
	if err != nil {
		t.Fatalf("filestore.OpenRead(%s) error = %v, want nil", evidence.Path, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, int64(evidence.Bytes.Uint64())+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("bounded capture read/close error = %v, want nil", err)
	}
	return data
}
