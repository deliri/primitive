package hostfacts

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCgroupAncestorDepthLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		depth     int
		rootValue string
		canceled  bool
		wantState WorkloadMemoryLimitState
		wantErr   error
	}{
		{name: "mount root is observed without ascent", rootValue: "400", wantState: WorkloadMemoryLimitLimited},
		{name: "255 levels retain the mount declaration", depth: 255, rootValue: "400", wantState: WorkloadMemoryLimitLimited},
		{name: "256 levels must include the final mount observation", depth: 256, rootValue: "400", wantState: WorkloadMemoryLimitLimited},
		{name: "257 levels cannot lose the mount declaration", depth: 257, rootValue: "400", wantState: WorkloadMemoryLimitLimited},
		{name: "257 empty levels remain unavailable", depth: 257, wantState: WorkloadMemoryLimitUnavailable},
		{name: "257 levels preserve an unlimited root", depth: 257, rootValue: cgroupV2MaxToken, wantState: WorkloadMemoryLimitUnlimited},
		{name: "cancellation cannot return the ancestor declaration", depth: 257, rootValue: "400", canceled: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			absolute := mustAbsolutePathForHostfactsTest(t, directory)
			root, err := filestore.OpenRoot(t.Context(), absolute)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			relativeText := "."
			membershipText := "/"
			if tc.depth > 0 {
				membershipText += strings.Repeat("d/", tc.depth-1) + "d"
				relativeText = filepath.FromSlash(membershipText[1:])
				relative, err := core.ParseRelativePath(relativeText)
				if err != nil {
					t.Fatal(err)
				}
				if err := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: relative}, Mode: 0700}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.rootValue != "" {
				target, err := core.ParseRelativePath(cgroupV2LimitName)
				if err != nil {
					t.Fatal(err)
				}
				temporary, err := core.ParseRelativePath("stage")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := filestore.Write(t.Context(), filestore.WriteRequest{Location: filestore.Location{Root: root, Path: target}, Temporary: temporary, Source: strings.NewReader(tc.rootValue), Mode: 0600, Install: filestore.InstallCreate}); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			got, err := foldCgroupLimits(ctx, cgroupMembership{path: membershipText, source: WorkloadMemoryLimitSourceCgroupV2}, cgroupMount{root: "/", mountPoint: absolute, source: WorkloadMemoryLimitSourceCgroupV2})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("fold depth %d = %+v/%v, want %v", tc.depth, got, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (WorkloadMemoryLimit{}) {
					t.Fatalf("refused fold = %+v, want zero", got)
				}
				return
			}
			limit, present := got.LimitBytes()
			path, pathPresent := got.InterfacePath()
			if got.Validate() != nil || got.State() != tc.wantState || present != (tc.wantState == WorkloadMemoryLimitLimited) || present && limit.Uint64() != 400 || pathPresent != (tc.rootValue != "") || pathPresent && path.String() != filepath.Join(directory, cgroupV2LimitName) {
				t.Fatalf("fold depth %d = %+v, limit=%v/%t, path=%v/%t; want exact mount state %v", tc.depth, got, limit, present, path, pathPresent, tc.wantState)
			}
		})
	}
}
