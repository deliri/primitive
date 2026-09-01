package hostfacts

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestMeasureTreeUsesHeldRootWithoutFollowingLinks(t *testing.T) {
	t.Parallel()

	t.Run("positive nested regular sparse and hard-linked entries are measured logically", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		nested := filepath.Join(root, "nested")
		if err := os.Mkdir(nested, 0o700); err != nil {
			t.Fatalf("os.Mkdir(%s) error = %v", nested, err)
		}
		if err := os.WriteFile(filepath.Join(root, "small"), []byte("abc"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(small) error = %v", err)
		}
		sparse := filepath.Join(nested, "sparse")
		if err := os.WriteFile(sparse, nil, 0o600); err != nil {
			t.Fatalf("os.WriteFile(sparse) error = %v", err)
		}
		if err := os.Truncate(sparse, 1<<20); err != nil {
			t.Fatalf("os.Truncate(sparse) error = %v", err)
		}
		if err := os.Link(filepath.Join(root, "small"), filepath.Join(nested, "hard-link")); err != nil {
			t.Fatalf("os.Link(hard-link) error = %v", err)
		}
		if err := os.Symlink(filepath.Join(root, "small"), filepath.Join(root, "ignored-link")); err != nil {
			t.Fatalf("os.Symlink(ignored-link) error = %v", err)
		}

		got, gotErr := MeasureTree(context.Background(), TreeUsageRequest{
			Root:          mustAbsolutePathForHostfactsTest(t, root),
			MissingPolicy: MissingPathReject,
		})
		if gotErr != nil {
			t.Fatalf("MeasureTree() error = %v, want nil", gotErr)
		}
		wantBytes := uint64(1<<20 + 6)
		if got.RegularFileBytes().Uint64() != wantBytes || got.RegularFileCount().Uint64() != 3 {
			t.Fatalf(
				"MeasureTree() = %d bytes/%d files, want %d bytes/3 files",
				got.RegularFileBytes().Uint64(),
				got.RegularFileCount().Uint64(),
				wantBytes,
			)
		}
		if got.Validate() != nil {
			t.Fatalf("MeasureTree().Validate() error = %v, want nil", got.Validate())
		}
	})

	t.Run("negative symlink root is refused without following it", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		target := filepath.Join(parent, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("os.Mkdir(target) error = %v", err)
		}
		link := filepath.Join(parent, "root-link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("os.Symlink(root-link) error = %v", err)
		}

		got, gotErr := MeasureTree(context.Background(), TreeUsageRequest{
			Root:          mustAbsolutePathForHostfactsTest(t, link),
			MissingPolicy: MissingPathReject,
		})
		if got != (TreeUsage{}) ||
			!errors.Is(gotErr, core.ErrHostFactsContract) ||
			errors.Is(gotErr, core.ErrHostFactsObservation) {
			t.Fatalf(
				"MeasureTree(symlink root) = (%v, %v), want zero contract failure without observation identity",
				got,
				gotErr,
			)
		}
	})

	t.Run("neutral empty and missing-as-empty roots produce valid empty facts", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		for _, candidate := range []core.AbsolutePath{
			mustAbsolutePathForHostfactsTest(t, root),
			mustAbsolutePathForHostfactsTest(t, filepath.Join(root, "absent")),
		} {
			policy := MissingPathReject
			if candidate.String() != root {
				policy = MissingPathIsEmpty
			}
			got, gotErr := MeasureTree(context.Background(), TreeUsageRequest{
				Root: candidate, MissingPolicy: policy,
			})
			if gotErr != nil || got.Validate() != nil ||
				got.RegularFileBytes().Uint64() != 0 ||
				got.RegularFileCount().Uint64() != 0 {
				t.Fatalf("MeasureTree(%s) = (%v, %v), want valid empty usage", candidate.String(), got, gotErr)
			}
		}
	})
}

func TestMeasureTreeIngressAndCancellationPressure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		ctx     context.Context
		wantErr error
		name    string
		request TreeUsageRequest
	}{
		{name: "nil context is refused", ctx: nil, request: TreeUsageRequest{Root: mustAbsolutePathForHostfactsTest(t, root), MissingPolicy: MissingPathReject}, wantErr: core.ErrNilContext},
		{name: "cancelled context is observed", ctx: cancelled, request: TreeUsageRequest{Root: mustAbsolutePathForHostfactsTest(t, root), MissingPolicy: MissingPathReject}, wantErr: context.Canceled},
		{name: "zero request is refused", ctx: context.Background(), request: TreeUsageRequest{}, wantErr: core.ErrHostFactsContract},
		{name: "unset root is refused", ctx: context.Background(), request: TreeUsageRequest{MissingPolicy: MissingPathReject}, wantErr: core.ErrHostFactsContract},
		{name: "unknown missing policy is refused", ctx: context.Background(), request: TreeUsageRequest{Root: mustAbsolutePathForHostfactsTest(t, root)}, wantErr: core.ErrHostFactsContract},
		{name: "future missing policy is refused", ctx: context.Background(), request: TreeUsageRequest{Root: mustAbsolutePathForHostfactsTest(t, root), MissingPolicy: MissingPathPolicy(255)}, wantErr: core.ErrHostFactsContract},
		{name: "missing reject preserves not-exist", ctx: context.Background(), request: TreeUsageRequest{Root: mustAbsolutePathForHostfactsTest(t, filepath.Join(root, "missing")), MissingPolicy: MissingPathReject}, wantErr: os.ErrNotExist},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := MeasureTree(tc.ctx, tc.request)
			if got != (TreeUsage{}) || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("MeasureTree() = (%v, %v), want (zero, %v)", got, gotErr, tc.wantErr)
			}
		})
	}
}

func BenchmarkMeasureTree1000RegularFiles(b *testing.B) {
	root := b.TempDir()
	for index := range 1000 {
		path := filepath.Join(root, "entry-"+formatBenchmarkIndex(index))
		if err := os.WriteFile(path, []byte{byte(index)}, 0o600); err != nil {
			b.Fatalf("os.WriteFile(%s) error = %v", path, err)
		}
	}
	absolute, err := core.ParseAbsolutePath(root)
	if err != nil {
		b.Fatalf("core.ParseAbsolutePath(%s) error = %v", root, err)
	}
	request := TreeUsageRequest{Root: absolute, MissingPolicy: MissingPathReject}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		got, err := MeasureTree(context.Background(), request)
		if err != nil || got.RegularFileCount().Uint64() != 1000 {
			b.Fatalf("MeasureTree(1000 files) = (%v, %v), want 1000 files", got, err)
		}
	}
}

func mustAbsolutePathForHostfactsTest(t *testing.T, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%s) error = %v", value, err)
	}
	return path
}

func formatBenchmarkIndex(value int) string {
	const digits = "0123456789"
	var storage [4]byte
	for index := len(storage) - 1; index >= 0; index-- {
		storage[index] = digits[value%10]
		value /= 10
	}
	return string(storage[:])
}

func TestTreeEntryDecisionsExhaustTheClosedKindDomain(t *testing.T) {
	t.Parallel()

	// Only the three inspected kinds get a measurement decision. Any future kind
	// defaults to refusal, so adding one without deciding how it is measured
	// cannot silently undercount a tree.
	for raw := 0; raw <= math.MaxUint8; raw++ {
		kind := treeEntryKind(raw)
		want := treeEntryDecisionRefuse
		switch kind {
		case treeEntryRegular:
			want = treeEntryDecisionCount
		case treeEntryIgnored:
			want = treeEntryDecisionSkip
		case treeEntryDirectory:
			want = treeEntryDecisionDescend
		}
		if got := kind.decision(); got != want {
			t.Fatalf("treeEntryKind(%d).decision() = %d, want %d", raw, got, want)
		}
	}
}

func TestTreeWalkDepthRefusalClosesTheRejectedDirectory(t *testing.T) {
	t.Parallel()

	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatalf("os.Open(directory) error = %v, want nil", err)
	}
	walk := treeWalk{
		stack: make([]treeFrame, core.FilesystemPathMaximumComponents),
	}
	walk.stack[len(walk.stack)-1].relative = "."
	gotErr := walk.descend(
		len(walk.stack)-1,
		treeEntry{directory: directory, kind: treeEntryDirectory},
		"child",
	)
	if !errors.Is(gotErr, core.ErrHostFactsObservation) {
		t.Fatalf("treeWalk.descend(at depth ceiling) error = %v, want %v", gotErr, core.ErrHostFactsObservation)
	}
	if closeErr := directory.Close(); !errors.Is(closeErr, os.ErrClosed) {
		t.Fatalf("rejected directory second Close() error = %v, want %v proving deterministic first close", closeErr, os.ErrClosed)
	}

	gotErr = (&treeWalk{stack: []treeFrame{{relative: "."}}}).descend(
		0,
		treeEntry{kind: treeEntryDirectory},
		"child",
	)
	if !errors.Is(gotErr, core.ErrHostFactsObservation) {
		t.Fatalf("treeWalk.descend(nil directory) error = %v, want %v", gotErr, core.ErrHostFactsObservation)
	}
}

func TestTreeReadRetainsPartialErrorsAndBoundsEmptyProgress(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "visible-before-error"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v, want nil", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("os.ReadDir() = (%d entries, %v), want (1, nil)", len(entries), err)
	}
	partialErr := errors.New("directory became unreadable after a partial batch")
	frame := treeFrame{}
	retained, gotErr := retainTreeRead(&frame, entries, partialErr)
	if retained || !errors.Is(gotErr, partialErr) || len(frame.entries) != 0 {
		t.Fatalf("retainTreeRead(partial error) = (retained %t, error %v, entries %d), want (false, exact error, 0)", retained, gotErr, len(frame.entries))
	}

	for attempt := 1; attempt <= core.ReaderConsecutiveEmptyReadMaximum; attempt++ {
		retained, gotErr = retainTreeRead(&frame, nil, nil)
		if attempt < core.ReaderConsecutiveEmptyReadMaximum && (!retained || gotErr != nil) {
			t.Fatalf("retainTreeRead(empty attempt %d) = (%t, %v), want (true, nil)", attempt, retained, gotErr)
		}
	}
	if retained || !errors.Is(gotErr, io.ErrNoProgress) || !errors.Is(gotErr, core.ErrHostFactsObservation) {
		t.Fatalf("retainTreeRead(empty ceiling) = (%t, %v), want typed %v", retained, gotErr, io.ErrNoProgress)
	}
}
