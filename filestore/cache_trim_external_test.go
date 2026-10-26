package filestore_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// cacheTrimFile is one regular cache member: its path under the cache, its
// byte size, and how many seconds older than the fixture base its mtime is.
type cacheTrimFile struct {
	path string
	size int
	age  int
}

var cacheTrimBase = time.Unix(1_700_000_000, 0)

// cacheTrimFixture builds root/outside (old, never in scope) and root/cache
// with the given members and explicit mtimes.
func cacheTrimFixture(t *testing.T, files []cacheTrimFile) (string, *os.Root) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(directory, "outside")
	if err := os.WriteFile(outside, make([]byte, 1000), 0o600); err != nil {
		t.Fatal(err)
	}
	old := cacheTrimBase.Add(-time.Hour)
	if err := os.Chtimes(outside, old, old); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := filepath.Join(directory, "cache", file.path)
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, make([]byte, file.size), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := cacheTrimBase.Add(-time.Duration(file.age) * time.Second)
		if err := os.Chtimes(name, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return directory, root
}

func cacheTrimRequest(t *testing.T, root *os.Root, limit uint64) filestore.CacheTrimRequest {
	t.Helper()
	path, err := core.ParseRelativePath("cache")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	count, err := core.NewByteCount(limit)
	if err != nil {
		t.Fatal(err)
	}
	return filestore.CacheTrimRequest{Location: filestore.Location{Root: root, Path: path}, Parent: parent, Limit: count}
}

// assertCacheTrimSurvivors checks each member exists with its bytes exactly
// when it is not in removed, and that the out-of-scope file is untouched.
func assertCacheTrimSurvivors(t *testing.T, directory string, files []cacheTrimFile, removed []string) {
	t.Helper()
	for _, file := range files {
		info, err := os.Lstat(filepath.Join(directory, "cache", file.path))
		gone := slices.Contains(removed, file.path)
		switch {
		case gone && !errors.Is(err, fs.ErrNotExist):
			t.Errorf("%q survived (%v), want evicted", file.path, err)
		case !gone && err != nil:
			t.Errorf("%q = %v, want kept", file.path, err)
		case !gone && info.Size() != int64(file.size):
			t.Errorf("%q size = %d, want %d", file.path, info.Size(), file.size)
		}
	}
	info, err := os.Lstat(filepath.Join(directory, "outside"))
	if err != nil || info.Size() != 1000 {
		t.Errorf("outside file = %v/%v, want untouched 1000 bytes", info, err)
	}
}

func TestTrimCacheNativeLayerTriad(t *testing.T) {
	t.Parallel()
	many := make([]cacheTrimFile, 130)
	var manyRemoved []string
	for i := range many {
		// Names carry LF; older files have smaller indexes.
		many[i] = cacheTrimFile{path: fmt.Sprintf("line\n%03d", i), size: 1, age: 1000 - i}
		if i < 65 {
			manyRemoved = append(manyRemoved, many[i].path)
		}
	}
	two := []cacheTrimFile{{path: "a", size: 10, age: 3}, {path: "b", size: 10, age: 1}}
	for _, row := range []struct {
		name    string
		files   []cacheTrimFile
		limit   uint64
		link    bool
		removed []string
		want    filestore.CacheTrimObservation
	}{
		{name: "neutral empty cache removes nothing", limit: 1},
		{name: "neutral under limit removes nothing", files: two, limit: 100, want: filestore.CacheTrimObservation{BeforeBytes: 20, AfterBytes: 20}},
		{name: "boundary exact limit removes nothing", files: two, limit: 20, want: filestore.CacheTrimObservation{BeforeBytes: 20, AfterBytes: 20}},
		{name: "boundary one byte over evicts only the oldest", files: two, limit: 19, removed: []string{"a"}, want: filestore.CacheTrimObservation{BeforeBytes: 20, AfterBytes: 10, RemovedFiles: 1}},
		{name: "positive oldest first across directories", files: []cacheTrimFile{
			{path: "00/old", size: 5, age: 50}, {path: "ff/mid", size: 5, age: 30}, {path: "00/new", size: 5, age: 10}, {path: "7f/newest", size: 5},
		}, limit: 10, removed: []string{"00/old", "ff/mid"}, want: filestore.CacheTrimObservation{BeforeBytes: 20, AfterBytes: 10, RemovedFiles: 2}},
		{name: "positive equal mtimes break ties by path", files: []cacheTrimFile{
			{path: "b/x", size: 5, age: 5}, {path: "a/x", size: 5, age: 5}, {path: "c/x", size: 5},
		}, limit: 10, removed: []string{"a/x"}, want: filestore.CacheTrimObservation{BeforeBytes: 15, AfterBytes: 10, RemovedFiles: 1}},
		{name: "positive recency order is kept even when a newer file is larger", files: []cacheTrimFile{
			{path: "big", size: 100, age: 1}, {path: "small1", size: 1, age: 9}, {path: "small2", size: 1, age: 8},
		}, limit: 50, removed: []string{"small1", "small2", "big"}, want: filestore.CacheTrimObservation{BeforeBytes: 102, RemovedFiles: 3}},
		{name: "positive LF names across many walk batches", files: many, limit: 65, removed: manyRemoved, want: filestore.CacheTrimObservation{BeforeBytes: 130, AfterBytes: 65, RemovedFiles: 65}},
		{name: "neutral links and directories are neither counted nor removed", files: []cacheTrimFile{
			{path: "sub/a", size: 10, age: 2}, {path: "b", size: 10, age: 1},
		}, link: true, limit: 10, removed: []string{"sub/a"}, want: filestore.CacheTrimObservation{BeforeBytes: 20, AfterBytes: 10, RemovedFiles: 1}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory, root := cacheTrimFixture(t, row.files)
			if row.link {
				// An old link to a large outside file must stay invisible.
				if err := os.Symlink("../outside", filepath.Join(directory, "cache", "link")); err != nil {
					t.Fatal(err)
				}
			}
			request := cacheTrimRequest(t, root, row.limit)
			got, err := filestore.TrimCache(t.Context(), request)
			if err != nil {
				t.Fatalf("TrimCache = %v, want nil", err)
			}
			if got != row.want {
				t.Fatalf("TrimCache = %+v, want %+v", got, row.want)
			}
			assertCacheTrimSurvivors(t, directory, row.files, row.removed)
			if row.link {
				if info, err := os.Lstat(filepath.Join(directory, "cache", "link")); err != nil || info.Mode().Type() != fs.ModeSymlink {
					t.Fatalf("link = %v/%v, want kept", info, err)
				}
				if info, err := os.Lstat(filepath.Join(directory, "cache", "sub")); err != nil || !info.IsDir() {
					t.Fatalf("emptied directory = %v/%v, want kept", info, err)
				}
			}
			scratch, err := os.ReadDir(request.Parent.String())
			if err != nil || len(scratch) != 0 {
				t.Fatalf("scratch parent = %v/%v, want empty", scratch, err)
			}
		})
	}
}

func TestTrimCacheRefusalsRemoveNothing(t *testing.T) {
	t.Parallel()
	files := []cacheTrimFile{{path: "a", size: 10, age: 2}, {path: "b", size: 10, age: 1}}
	for _, row := range []struct {
		name    string
		mutate  func(*testing.T, *filestore.CacheTrimRequest, *os.Root) context.Context
		want    error
		wantErr error
	}{
		{name: "negative nil context", want: core.ErrNilContext, mutate: func(*testing.T, *filestore.CacheTrimRequest, *os.Root) context.Context { return nil }},
		{name: "negative cancelled context", want: context.Canceled, mutate: func(t *testing.T, _ *filestore.CacheTrimRequest, _ *os.Root) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}},
		{name: "negative zero limit", want: core.ErrFilestoreContract, mutate: func(t *testing.T, r *filestore.CacheTrimRequest, _ *os.Root) context.Context {
			r.Limit = core.ByteCount{}
			return t.Context()
		}},
		{name: "negative unset scratch parent", want: core.ErrFilestoreContract, mutate: func(t *testing.T, r *filestore.CacheTrimRequest, _ *os.Root) context.Context {
			r.Parent = core.AbsolutePath{}
			return t.Context()
		}},
		{name: "negative unset root", want: core.ErrFilestoreContract, mutate: func(t *testing.T, r *filestore.CacheTrimRequest, _ *os.Root) context.Context {
			r.Location.Root = nil
			return t.Context()
		}},
		{name: "negative missing cache preserves native absence", want: core.ErrFilestoreSource, wantErr: fs.ErrNotExist, mutate: func(t *testing.T, r *filestore.CacheTrimRequest, _ *os.Root) context.Context {
			path, err := core.ParseRelativePath("absent")
			if err != nil {
				t.Fatal(err)
			}
			r.Location.Path = path
			return t.Context()
		}},
		{name: "negative missing scratch parent preserves native absence", want: fs.ErrNotExist, mutate: func(t *testing.T, r *filestore.CacheTrimRequest, _ *os.Root) context.Context {
			parent, err := core.ParseAbsolutePath(filepath.Join(r.Parent.String(), "absent"))
			if err != nil {
				t.Fatal(err)
			}
			r.Parent = parent
			return t.Context()
		}},
		{name: "negative closed root preserves closed cause", want: os.ErrClosed, mutate: func(t *testing.T, _ *filestore.CacheTrimRequest, root *os.Root) context.Context {
			if err := root.Close(); err != nil {
				t.Fatal(err)
			}
			return t.Context()
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory, root := cacheTrimFixture(t, files)
			request := cacheTrimRequest(t, root, 1)
			ctx := row.mutate(t, &request, root)
			got, err := filestore.TrimCache(ctx, request)
			if !errors.Is(err, row.want) || (row.wantErr != nil && !errors.Is(err, row.wantErr)) {
				t.Fatalf("TrimCache error = %v, want %v and %v", err, row.want, row.wantErr)
			}
			if got != (filestore.CacheTrimObservation{}) {
				t.Fatalf("refused TrimCache = %+v, want zero observation", got)
			}
			assertCacheTrimSurvivors(t, directory, files, nil)
		})
	}
}

// trimCacheNativeProof binds the ingress inventory to the native tests.
func trimCacheNativeProof(t *testing.T) {
	t.Run("triad", TestTrimCacheNativeLayerTriad)
	t.Run("refusals", TestTrimCacheRefusalsRemoveNothing)
}
