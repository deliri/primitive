package release_test

import (
	"bytes"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type releaseFileFixture struct {
	Path core.AbsolutePath
	Data []byte
	Mode fs.FileMode
}

func releaseFixtureLocation(t testing.TB, path core.AbsolutePath) filestore.Location {
	t.Helper()
	location, err := filestore.OpenParent(t.Context(), path)
	if err != nil {
		t.Fatalf("filestore.OpenParent(fixture) error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := location.Root.Close(); err != nil {
			t.Errorf("fixture parent Close() error = %v, want nil", err)
		}
	})
	return location
}

// Acquire the oracle's handle before removing permissions. The subject still
// opens the path independently, including mode-zero refusals. Tests can inspect
// the held handle afterward without repairing or reopening the refused path.
func writeReleaseFileFixture(t testing.TB, request releaseFileFixture) *os.File {
	t.Helper()
	location := releaseFixtureLocation(t, request.Path)
	temporary, err := core.ParseRelativePath("release-fixture-stage")
	if err != nil {
		t.Fatalf("ParseRelativePath(fixture temporary) error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(uint64(max(1, len(request.Data))))
	if err != nil {
		t.Fatalf("NewByteCount(fixture) error = %v, want nil", err)
	}
	recovery, err := filestore.Write(t.Context(), filestore.WriteRequest{
		Location: location, Temporary: temporary, Source: bytes.NewReader(request.Data),
		MaximumBytes: maximum, Mode: 0o600, Install: filestore.InstallCreate,
	})
	if err != nil {
		t.Fatalf("filestore.Write(fixture) error = %v, recovery = %v, want nil", err, recovery)
	}
	held, err := filestore.OpenUpdate(t.Context(), filestore.UpdateHandleRequest{Location: location})
	if err != nil {
		t.Fatalf("filestore.OpenUpdate(fixture oracle) error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := held.Close(); err != nil {
			t.Errorf("fixture oracle Close() error = %v, want nil", err)
		}
	})
	if err := held.Chmod(request.Mode); err != nil {
		t.Fatalf("fixture handle Chmod() error = %v, want nil", err)
	}
	return held
}

func readReleaseFileFixture(t testing.TB, path core.AbsolutePath, maximumBytes uint64) []byte {
	t.Helper()
	location := releaseFixtureLocation(t, path)
	maximum, err := core.NewByteCount(maximumBytes)
	if err != nil {
		t.Fatalf("NewByteCount(fixture read) error = %v, want nil", err)
	}
	var content bytes.Buffer
	if _, err := filestore.Read(t.Context(), filestore.ReadRequest{
		Location: location, Destination: &content, MaximumBytes: maximum,
	}); err != nil {
		t.Fatalf("filestore.Read(fixture) error = %v, want nil", err)
	}
	return content.Bytes()
}
