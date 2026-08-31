package runworkspace_test

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runworkspace"
)

func TestExpectedArtifactLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive signed path is hashed and streamed from the member output", func(t *testing.T) {
		t.Parallel()
		manager, _, experiment, root := captureFixture(t)
		defer closeScrubManager(t, manager)
		content := []byte("mode: atomic\nexample/file.go:1.1,1.2 1 1\n")
		expectation := artifactExpectationFixture(t, experiment, "coverage.out", true, uint64(len(content)))
		writeExpectedArtifact(t, root, expectation.Path, content)

		got, present, gotErr := manager.ObserveArtifact(t.Context(), experiment, expectation)
		if gotErr != nil || !present {
			t.Fatalf("Manager.ObserveArtifact(present coverage) = (%+v, %t, %v), want evidence, true, nil", got, present, gotErr)
		}
		if got.SHA256 != core.SHA256Of(content) || got.Bytes.Uint64() != uint64(len(content)) || got.Kind != runnercontrol.ArtifactCoverage {
			t.Fatalf("ArtifactEvidence = kind %s/digest %v/bytes %d, want coverage/%v/%d", got.Kind.String(), got.SHA256, got.Bytes.Uint64(), core.SHA256Of(content), len(content))
		}
		var streamed bytes.Buffer
		if err := manager.StreamArtifactEvidence(t.Context(), experiment, got, &streamed); err != nil {
			t.Fatalf("Manager.StreamArtifactEvidence(coverage) error = %v, want nil", err)
		}
		if !bytes.Equal(streamed.Bytes(), content) {
			t.Fatalf("streamed coverage bytes = %q, want %q", streamed.Bytes(), content)
		}
	})

	t.Run("negative required file absence remains a typed filesystem refusal", func(t *testing.T) {
		t.Parallel()
		manager, _, experiment, _ := captureFixture(t)
		defer closeScrubManager(t, manager)
		expectation := artifactExpectationFixture(t, experiment, "cpu.pprof", true, 1024)

		got, present, gotErr := manager.ObserveArtifact(t.Context(), experiment, expectation)
		if !errors.Is(gotErr, fs.ErrNotExist) || present || got != (runworkspace.ArtifactEvidence{}) {
			t.Fatalf("Manager.ObserveArtifact(missing required profile) = (%+v, %t, %v), want zero, false, errors.Is(..., %v)", got, present, gotErr, fs.ErrNotExist)
		}
	})

	t.Run("neutral optional file absence creates no evidence", func(t *testing.T) {
		t.Parallel()
		manager, _, experiment, _ := captureFixture(t)
		defer closeScrubManager(t, manager)
		expectation := artifactExpectationFixture(t, experiment, "optional.trace", false, 1024)

		got, present, gotErr := manager.ObserveArtifact(t.Context(), experiment, expectation)
		if gotErr != nil || present || got != (runworkspace.ArtifactEvidence{}) {
			t.Fatalf("Manager.ObserveArtifact(missing optional trace) = (%+v, %t, %v), want zero, false, nil", got, present, gotErr)
		}
	})

	t.Run("negative sibling path cannot escape the member output namespace", func(t *testing.T) {
		t.Parallel()
		manager, _, experiment, _ := captureFixture(t)
		defer closeScrubManager(t, manager)
		foreign, err := projectstandards.ParseSourcePath("foreign/coverage.out")
		if err != nil {
			t.Fatalf("projectstandards.ParseSourcePath(foreign artifact) setup error = %v, want nil", err)
		}
		expectation := artifactExpectationFixture(t, experiment, "coverage.out", false, 1024)
		expectation.Path = foreign

		got, present, gotErr := manager.ObserveArtifact(t.Context(), experiment, expectation)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || present || got != (runworkspace.ArtifactEvidence{}) {
			t.Fatalf("Manager.ObserveArtifact(foreign path) = (%+v, %t, %v), want zero, false, errors.Is(..., %v)", got, present, gotErr, core.ErrPrimitiveContract)
		}
	})
}

func artifactExpectationFixture(t testing.TB, experiment runworkspace.Experiment, name string, required bool, maximum uint64) runnercontrol.ArtifactExpectation {
	t.Helper()
	component, componentErr := core.ParsePathComponent(name)
	path, pathErr := experiment.Output.Join(component)
	protocolPath, protocolPathErr := projectstandards.ParseSourcePath(path.String())
	limit, limitErr := core.NewByteCount(maximum)
	if err := errors.Join(componentErr, pathErr, protocolPathErr, limitErr); err != nil {
		t.Fatalf("artifact expectation %q setup error = %v, want nil", name, err)
	}
	return runnercontrol.ArtifactExpectation{Kind: runnercontrol.ArtifactCoverage, Path: protocolPath, MediaType: core.HTTPMediaTypeOctetStream(), MaximumBytes: limit, Required: required}
}

func writeExpectedArtifact(t testing.TB, rootPath core.AbsolutePath, protocolPath projectstandards.SourcePath, content []byte) {
	t.Helper()
	path, pathErr := core.ParseRelativePath(protocolPath.String())
	root, rootErr := filestore.OpenRoot(t.Context(), rootPath)
	base, baseErr := core.ParseAbsolutePath("/" + path.String())
	component, componentErr := base.Base()
	temporaryComponent, temporaryComponentErr := core.ParsePathComponent("." + component.String() + ".stage")
	parent, parentErr := base.Parent()
	parentRelative, parentRelativeErr := parent.RelativeTo(mustArtifactRoot(t))
	temporary, temporaryErr := parentRelative.Join(temporaryComponent)
	maximum, maximumErr := core.NewByteCount(uint64(len(content)))
	if err := errors.Join(pathErr, rootErr, baseErr, componentErr, temporaryComponentErr, parentErr, parentRelativeErr, temporaryErr, maximumErr); err != nil {
		t.Fatalf("artifact write path setup error = %v, want nil", err)
	}
	_, writeErr := filestore.Write(t.Context(), filestore.WriteRequest{Source: bytes.NewReader(content), Location: filestore.Location{Root: root, Path: path}, Temporary: temporary, Mode: fs.FileMode(0o600), Install: filestore.InstallCreate, MaximumBytes: maximum})
	closeErr := root.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		t.Fatalf("filestore.Write(expected artifact) error = %v, want nil", err)
	}
}

func mustArtifactRoot(t testing.TB) core.AbsolutePath {
	t.Helper()
	root, err := core.ParseAbsolutePath("/")
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(/) setup error = %v, want nil", err)
	}
	return root
}
