package release

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
)

// Cancellation is synchronized with real file reads, never a sleep or a fake
// build-info response. The owned adapter changes only the context state.
func TestBuildToolReadLayerTriadPreservesCancellation(t *testing.T) {
	t.Parallel()
	name, err := core.ParsePathComponent("go")
	if err != nil {
		t.Fatalf("ParsePathComponent(go) error = %v, want nil", err)
	}
	path, err := process.Resolve(t.Context(), name)
	if err != nil {
		t.Fatalf("Resolve(go) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name           string
		cancelReadSize int
		before         bool
		wantErr        error
	}{
		{name: "live context retains complete compiler proof"},
		{name: "canceled before parsing cannot produce proof", before: true, wantErr: context.Canceled},
		{name: "cancel during build-info reading cannot produce proof", cancelReadSize: -1, wantErr: context.Canceled},
		{name: "cancel during full-file hashing cannot produce proof", cancelReadSize: 64 << 10, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			location, err := filestore.OpenParent(t.Context(), path)
			if err != nil {
				t.Fatalf("OpenParent(go) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := location.Root.Close(); err != nil {
					t.Errorf("parent Close error = %v, want nil", err)
				}
			})
			file, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
			if err != nil {
				t.Fatalf("OpenRead(go) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Errorf("file Close error = %v, want nil", err)
				}
			})
			info, err := file.Stat()
			if err != nil {
				t.Fatalf("compiler Stat error = %v, want nil", err)
			}
			wantBuild, wantDigest, err := inspectBuildToolExtent(t.Context(), file, info.Size())
			if err != nil || wantBuild == nil {
				t.Fatalf("live fixture proof = (%v, %v), want compiler and nil", wantBuild, err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.before {
				cancel()
			}
			reader := &cancelInspectionRead{source: file, cancel: cancel, size: tc.cancelReadSize}
			build, digest, err := inspectBuildToolExtent(ctx, reader, info.Size())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("inspection error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !tc.before && !reader.fired {
					t.Fatalf("cancellation fixture fired = %t, want true during held-file reading", reader.fired)
				}
				if !errors.Is(err, core.ErrReleaseContract) || build != nil || digest != (core.SHA256Digest{}) {
					t.Fatalf("canceled inspection = (%v, %v, %v), want no proof and typed refusal", build, digest, err)
				}
				return
			}
			if build == nil || build.Path != wantBuild.Path || build.GoVersion != wantBuild.GoVersion || digest != wantDigest {
				t.Fatalf("live inspection = (%v, %v), want compiler %v and digest %v", build, digest, wantBuild, wantDigest)
			}
		})
	}
}

type cancelInspectionRead struct {
	source io.ReaderAt
	cancel context.CancelFunc
	size   int
	fired  bool
}

func (r *cancelInspectionRead) ReadAt(p []byte, offset int64) (int, error) {
	n, err := r.source.ReadAt(p, offset)
	if !r.fired && (r.size == -1 || r.size == len(p)) {
		r.cancel()
		r.fired = true
	}
	return n, err
}
