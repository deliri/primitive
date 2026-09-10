package fuzzartifact

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestFindRealDirectoryLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                          string
		generated                                     int
		unsupported, child, cancelBefore, cancelVisit bool
		visitErr                                      error
		wantState                                     ObservationState
		wantErr                                       error
		wantMatched, wantDelivered                    uint64
	}{
		{name: "empty directory produces no visits", wantState: ObservationComplete},
		{name: "one corpus name reaches visitor", generated: 1, wantState: ObservationComplete, wantMatched: 1, wantDelivered: 1},
		{name: "unsupported file never reaches visitor", unsupported: true, wantState: ObservationUnsupportedFormat, wantErr: core.ErrFuzzArtifactFormat},
		{name: "unsupported sibling cannot erase valid observation", generated: 3, unsupported: true, wantState: ObservationUnsupportedFormat, wantErr: core.ErrFuzzArtifactFormat, wantMatched: 3, wantDelivered: 3},
		{name: "generated child directory is counted but not descended", child: true, wantState: ObservationComplete},
		{name: "nested corpus cannot leak into direct child scan", generated: 1, child: true, wantState: ObservationComplete, wantMatched: 1, wantDelivered: 1},
		{name: "canceled ingress cannot emit a directory fact", generated: 1, cancelBefore: true, wantState: ObservationFailed, wantErr: context.Canceled},
		{name: "final callback cancellation cannot seal completeness", generated: 1, cancelVisit: true, wantState: ObservationPartial, wantErr: context.Canceled, wantMatched: 1, wantDelivered: 1},
		{name: "callback failure counts attempt but not acknowledgment", generated: 3, visitErr: io.ErrClosedPipe, wantState: ObservationPartial, wantErr: io.ErrClosedPipe, wantMatched: 1},
		{name: "callback EOF is not successful enumeration", generated: 1, visitErr: io.EOF, wantState: ObservationPartial, wantErr: io.EOF, wantMatched: 1},
		{name: "joined callback cancellation preserves both causes", generated: 1, cancelVisit: true, visitErr: io.ErrClosedPipe, wantState: ObservationPartial, wantErr: io.ErrClosedPipe, wantMatched: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			location := fuzzDirectoryLocation(t, directory)
			for i := range tc.generated {
				writeNameAt(t, location, generatedNameForPosition(t, ArtifactCorpus, uint64(i)).String())
			}
			if tc.unsupported {
				writeNameAt(t, location, "not-a-generated-name")
			}
			if tc.child {
				child := location
				child.Path = relativePathForTest(t, cacheDirectoryComponent+"/"+generatedNameForPosition(t, ArtifactCorpus, 99).String())
				if err := filestore.EnsureScratchDirectory(t.Context(), filestore.DirectoryRequest{Location: child, Mode: 0700}); err != nil {
					t.Fatal(err)
				}
				writeNameAt(t, child, generatedNameForPosition(t, ArtifactCorpus, 100).String())
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelBefore {
				cancel()
			}
			var calls uint64
			got, err := Find(ctx, FindRequest{Location: location, Kind: ArtifactCorpus, Format: CacheFormatGo1_27, Visit: func(name GeneratedName) error {
				calls++
				if name.Validate() != nil || name.Kind() != ArtifactCorpus || name.Format() != CacheFormatGo1_27 {
					t.Fatalf("visitor name=%+v, want exact typed corpus name", name)
				}
				if tc.cancelVisit {
					cancel()
				}
				return tc.visitErr
			}})
			if tc.cancelVisit && !errors.Is(err, context.Canceled) {
				t.Fatalf("callback cancellation error=%v, want %v", err, context.Canceled)
			}
			if !errors.Is(err, tc.wantErr) || got.State() != tc.wantState || got.Validate() != nil {
				t.Fatalf("Find=%+v/%v, want state %v and %v", got, err, tc.wantState, tc.wantErr)
			}
			if got.Matched().Uint64() != tc.wantMatched || got.Delivered().Uint64() != tc.wantDelivered || calls != tc.wantMatched {
				t.Fatalf("Find matched/delivered/calls=%d/%d/%d, want %d/%d/%d", got.Matched().Uint64(), got.Delivered().Uint64(), calls, tc.wantMatched, tc.wantDelivered, tc.wantMatched)
			}
			if got.Kind() != ArtifactCorpus || got.Format() != CacheFormatGo1_27 || got.NonRegular().Uint64() != 0 ||
				got.UnsupportedRegular().Uint64() != boolCount(tc.unsupported) || got.IgnoredDirectories().Uint64() != boolCount(tc.child) {
				t.Fatalf("Find accounting=%+v, want exact declared class and direct-child counters", got)
			}
		})
	}
}

func TestFindInvalidIngressEmitsNoCallback(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*FindRequest)
		wantErr error
		failed  bool
	}{
		{name: "unset root", mutate: func(r *FindRequest) { r.Location.Root = nil }, wantErr: core.ErrFuzzArtifactContract},
		{name: "unset path", mutate: func(r *FindRequest) { r.Location.Path = core.RelativePath{} }, wantErr: core.ErrFuzzArtifactContract},
		{name: "unset kind", mutate: func(r *FindRequest) { r.Kind = ArtifactUnknown }, wantErr: core.ErrFuzzArtifactContract},
		{name: "future kind", mutate: func(r *FindRequest) { r.Kind = ArtifactKind(255) }, wantErr: core.ErrFuzzArtifactContract},
		{name: "unset format", mutate: func(r *FindRequest) { r.Format = CacheFormatUnknown }, wantErr: core.ErrFuzzArtifactFormat},
		{name: "future format", mutate: func(r *FindRequest) { r.Format = CacheFormat(255) }, wantErr: core.ErrFuzzArtifactFormat},
		{name: "unset visitor", mutate: func(r *FindRequest) { r.Visit = nil }, wantErr: core.ErrFuzzArtifactContract},
		{name: "missing directory", mutate: func(r *FindRequest) { r.Location.Path, _ = core.ParseRelativePath("missing") }, wantErr: fs.ErrNotExist, failed: true},
		{name: "regular file is not a directory", mutate: func(r *FindRequest) {
			r.Location.Path, _ = core.ParseRelativePath(cacheDirectoryComponent + "/regular")
		}, wantErr: core.ErrFilestoreSource, failed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			location := fuzzDirectoryLocation(t, directory)
			writeNameAt(t, location, "regular")
			calls := 0
			request := FindRequest{Location: location, Kind: ArtifactCorpus, Format: CacheFormatGo1_27, Visit: func(GeneratedName) error { calls++; return nil }}
			tc.mutate(&request)
			got, err := Find(t.Context(), request)
			if !errors.Is(err, tc.wantErr) || calls != 0 {
				t.Fatalf("Find=%+v/%v calls=%d, want %v and no callback", got, err, calls, tc.wantErr)
			}
			if tc.failed {
				if got.Validate() != nil || got.State() != ObservationFailed || !errors.Is(err, core.ErrFuzzArtifactObservation) {
					t.Fatalf("failed observation=%+v/%v, want validated failed result and observation identity", got, err)
				}
			} else if got != (Observation{}) {
				t.Fatalf("refused observation=%+v, want zero", got)
			}
		})
	}
}

func BenchmarkFindRealDirectory128(b *testing.B) {
	b.ReportAllocs()
	benchmarkFindRealDirectory(b, 128)
}
func BenchmarkFindRealDirectory8192(b *testing.B) {
	b.ReportAllocs()
	benchmarkFindRealDirectory(b, 8192)
}

// Fixture creation is outside timing. Before retained a canonical prefix;
// after delivers every name. Both traverse the same native directory extent.
func benchmarkFindRealDirectory(b *testing.B, entries uint64) {
	directory := b.TempDir()
	location := fuzzDirectoryLocation(b, directory)
	for i := range entries {
		writeNameAt(b, location, generatedNameForPosition(b, ArtifactCorpus, i).String())
	}
	var calls uint64
	request := FindRequest{Location: location, Kind: ArtifactCorpus, Format: CacheFormatGo1_27, Visit: func(GeneratedName) error { calls++; return nil }}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		calls = 0
		got, err := Find(b.Context(), request)
		if err != nil || got.Delivered().Uint64() != entries || calls != entries {
			b.Fatalf("Find count/calls/error=%d/%d/%v, want %d/%d/nil", got.Delivered().Uint64(), calls, err, entries, entries)
		}
	}
}

const cacheDirectoryComponent = "cache"

func fuzzDirectoryLocation(t testing.TB, directory string) filestore.Location {
	t.Helper()
	root := openRootForTest(t, directory)
	location := filestore.Location{Root: root, Path: relativePathForTest(t, cacheDirectoryComponent)}
	if err := filestore.EnsureScratchDirectory(t.Context(), filestore.DirectoryRequest{Location: location, Mode: 0700}); err != nil {
		t.Fatal(err)
	}
	return location
}

func openRootForTest(t testing.TB, directory string) *os.Root {
	t.Helper()
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("root.Close=%v, want nil", err)
		}
	})
	return root
}

func relativePathForTest(t testing.TB, value string) core.RelativePath {
	t.Helper()
	path, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeNameAt(t testing.TB, location filestore.Location, value string) {
	t.Helper()
	path, err := location.Path.Resolve(value)
	if err != nil {
		t.Fatal(err)
	}
	file, err := filestore.OpenScratch(t.Context(), filestore.ScratchRequest{Location: filestore.Location{Root: location.Root, Path: path}, Mode: 0600})
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := io.Copy(file, bytes.NewBufferString(value))
	if err := errors.Join(writeErr, file.Close()); err != nil || n != int64(len(value)) {
		t.Fatalf("fixture write=%d/%v, want %d/nil", n, err, len(value))
	}
}

func boolCount(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

func TestFindCallbackFailureStopsAtExactPrefix(t *testing.T) {
	t.Parallel()
	for _, stop := range []uint64{1, 63, 64, 65, 128, 129, 200} {
		t.Run(strconv.FormatUint(stop, 10)+"th callback refuses", func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			location := fuzzDirectoryLocation(t, directory)
			const total = 200
			for i := range total {
				writeNameAt(t, location, generatedNameForPosition(t, ArtifactCrasher, uint64(i)).String())
			}
			var calls uint64
			got, err := Find(t.Context(), FindRequest{Location: location, Kind: ArtifactCrasher, Format: CacheFormatGo1_27, Visit: func(name GeneratedName) error {
				calls++
				if calls == stop {
					return io.ErrClosedPipe
				}
				return nil
			}})
			if !errors.Is(err, io.ErrClosedPipe) || got.State() != ObservationPartial || got.Matched().Uint64() != stop || got.Delivered().Uint64() != stop-1 || calls != stop || got.Validate() != nil {
				t.Fatalf("Find=%+v/%v calls=%d, want matched %d delivered %d and native callback failure", got, err, calls, stop, stop-1)
			}
		})
	}
}

func TestFindNativeLinksNeverBecomeCorpusFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, target string }{
		{name: "dangling generated link remains a nonregular entry", target: "absent"},
		{name: "link to ordinary bytes remains a nonregular entry", target: "ordinary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			location := fuzzDirectoryLocation(t, directory)
			writeNameAt(t, location, "ordinary")
			name := generatedNameForPosition(t, ArtifactCorpus, 1)
			path, err := location.Path.Resolve(name.String())
			if err != nil {
				t.Fatal(err)
			}
			// The test owns the real os.Root returned by Filestore. Its native Symlink
			// supplies a filesystem fact that Filestore has no creation request for.
			if err := location.Root.Symlink(tc.target, path.String()); err != nil {
				t.Fatal(err)
			}
			calls := 0
			got, err := Find(t.Context(), FindRequest{Location: location, Kind: ArtifactCorpus, Format: CacheFormatGo1_27, Visit: func(GeneratedName) error { calls++; return nil }})
			if !errors.Is(err, core.ErrFuzzArtifactFormat) || got.State() != ObservationUnsupportedFormat || calls != 0 || got.NonRegular().Uint64() != 1 || got.UnsupportedRegular().Uint64() != 1 || got.Matched().Uint64() != 0 || got.Validate() != nil {
				t.Fatalf("link scan=%+v/%v calls=%d, want zero generated names, one link, one foreign regular file", got, err, calls)
			}
		})
	}
}
