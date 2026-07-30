package fuzzfinder

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestFindRealDirectoryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive real directory returns canonical bounded generated names", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		location := fuzzDirectoryLocation(t, rootDirectory)
		for _, position := range []uint64{9, 1, 7, 3} {
			writeGeneratedFile(t, rootDirectory, generatedNameForPosition(t, position))
		}
		got, gotErr := Find(crasherRequest(location, mustRetentionLimit(t, MaximumRetainedEntries)))
		if gotErr != nil || got.State() != ObservationComplete {
			t.Fatalf("Find(real directory) = (state %d, %v), want (%d, nil)", got.State(), gotErr, ObservationComplete)
		}
		if got.Kind() != ArtifactCrasher {
			t.Fatalf("Find(real directory).Kind() = %d, want %d", got.Kind(), ArtifactCrasher)
		}
		gotNames := got.Names()
		wantNames := []GeneratedName{
			generatedNameForPosition(t, 1),
			generatedNameForPosition(t, 3),
			generatedNameForPosition(t, 7),
			generatedNameForPosition(t, 9),
		}
		if !slices.Equal(gotNames, wantNames) || got.Retained().Uint64() != uint64(len(wantNames)) {
			t.Fatalf("Find(real directory) names/count = (%v, %d), want (%v, %d)", gotNames, got.Retained().Uint64(), wantNames, len(wantNames))
		}
	})
	t.Run("negative unknown regular file reports unsupported format without fake completeness", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		location := fuzzDirectoryLocation(t, rootDirectory)
		writeGeneratedFile(t, rootDirectory, generatedNameForPosition(t, 1))
		if err := os.WriteFile(filepath.Join(rootDirectory, cacheDirectoryComponent, "new-go-format"), []byte("unknown"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, gotErr := Find(crasherRequest(location, mustRetentionLimit(t, MaximumRetainedEntries)))
		if !errors.Is(gotErr, core.ErrFuzzFinderFormat) || got.State() != ObservationUnsupportedFormat {
			t.Fatalf("Find(format drift) = (state %d, %v), want (%d, %v)", got.State(), gotErr, ObservationUnsupportedFormat, core.ErrFuzzFinderFormat)
		}
		if got.Retained().Uint64() != 1 || got.UnsupportedRegular().Uint64() != 1 {
			t.Fatalf("Find(format drift) counts = retained:%d unsupported:%d, want 1/1", got.Retained().Uint64(), got.UnsupportedRegular().Uint64())
		}
	})
	t.Run("neutral empty directory reports complete zero observation", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		location := fuzzDirectoryLocation(t, rootDirectory)
		got, gotErr := Find(crasherRequest(location, mustRetentionLimit(t, 1)))
		if gotErr != nil || got.State() != ObservationComplete || len(got.Names()) != 0 || got.Retained().Uint64() != 0 {
			t.Fatalf("Find(empty directory) = (state %d, names %v, retained %d, error %v), want complete empty", got.State(), got.Names(), got.Retained().Uint64(), gotErr)
		}
		if got.Kind() != ArtifactCrasher {
			t.Fatalf("Find(empty directory).Kind() = %d, want %d", got.Kind(), ArtifactCrasher)
		}
	})
}

func TestFindClassifiesEveryRealEntryKindUnderRetentionPressure(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	location := fuzzDirectoryLocation(t, rootDirectory)
	const generatedFiles = 12
	for position := range uint64(generatedFiles) {
		writeGeneratedFile(t, rootDirectory, generatedNameForPosition(t, position))
	}
	cacheDirectory := filepath.Join(rootDirectory, cacheDirectoryComponent)
	// The subdirectory, the dangling symlink, and the off-format regular file all
	// carry names the format would otherwise accept, so a classifier that read the
	// name before the entry type would miscount every one of them.
	if err := os.Mkdir(filepath.Join(cacheDirectory, generatedNameForPosition(t, 90).String()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("absent-target", filepath.Join(cacheDirectory, generatedNameForPosition(t, 91).String())); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDirectory, "0123456789abcdeF"), []byte("uppercase"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, gotErr := Find(crasherRequest(location, mustRetentionLimit(t, 5)))
	if !errors.Is(gotErr, core.ErrFuzzFinderFormat) || got.State() != ObservationUnsupportedFormat {
		t.Fatalf("Find(mixed directory) = (state %d, %v), want (%d, %v)", got.State(), gotErr, ObservationUnsupportedFormat, core.ErrFuzzFinderFormat)
	}
	wantNames := []GeneratedName{
		generatedNameForPosition(t, 0),
		generatedNameForPosition(t, 1),
		generatedNameForPosition(t, 2),
		generatedNameForPosition(t, 3),
		generatedNameForPosition(t, 4),
	}
	if !slices.Equal(got.Names(), wantNames) {
		t.Fatalf("Find(mixed directory) names = %v, want %v", got.Names(), wantNames)
	}
	if got.OverLimitObservations().Uint64() != generatedFiles-uint64(len(wantNames)) ||
		got.IgnoredDirectories().Uint64() != 1 ||
		got.NonRegular().Uint64() != 1 ||
		got.UnsupportedRegular().Uint64() != 1 {
		t.Fatalf(
			"Find(mixed directory) counts = over-limit:%d directories:%d non-regular:%d unsupported:%d, want %d/1/1/1",
			got.OverLimitObservations().Uint64(),
			got.IgnoredDirectories().Uint64(),
			got.NonRegular().Uint64(),
			got.UnsupportedRegular().Uint64(),
			generatedFiles-uint64(len(wantNames)),
		)
	}
}

func TestFindStreamsRealDirectoryAcrossManyReadBatches(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	location := fuzzDirectoryLocation(t, rootDirectory)
	// More entries than three full read batches, and more than the retention
	// ceiling, so the batch loop, the eviction branch, and the canonical-prefix
	// invariant are all proved against real getdirents ordering rather than an
	// order the test chose.
	const realEntries = uint64(directoryReadBatchEntries)*3 + 8
	for position := range realEntries {
		writeGeneratedFile(t, rootDirectory, generatedNameForPosition(t, position))
	}
	limit := mustRetentionLimit(t, MaximumRetainedEntries)
	got, gotErr := Find(corpusRequest(location, limit))
	if gotErr != nil || got.State() != ObservationComplete {
		t.Fatalf("Find(%d entries) = (state %d, %v), want (%d, nil)", realEntries, got.State(), gotErr, ObservationComplete)
	}
	wantNames := make([]GeneratedName, MaximumRetainedEntries)
	for position := range uint64(MaximumRetainedEntries) {
		wantNames[position] = generatedNameForPosition(t, position)
	}
	if !slices.Equal(got.Names(), wantNames) {
		t.Fatalf("Find(%d entries) retained the wrong canonical prefix: first = %q last = %q, want %q..%q",
			realEntries, got.Names()[0].String(), got.Names()[len(got.Names())-1].String(),
			wantNames[0].String(), wantNames[len(wantNames)-1].String())
	}
	if got.OverLimitObservations().Uint64() != realEntries-uint64(MaximumRetainedEntries) {
		t.Fatalf("Find(%d entries) over-limit = %d, want %d", realEntries, got.OverLimitObservations().Uint64(), realEntries-uint64(MaximumRetainedEntries))
	}
}

func TestFindIngressAndNativeFailurePressure(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootDirectory, cacheDirectoryComponent), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, "regular"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openRootForTest(t, rootDirectory)
	cachePath := relativePathForTest(t, cacheDirectoryComponent)
	missingPath := relativePathForTest(t, "missing")
	regularPath := relativePathForTest(t, "regular")
	validLimit := mustRetentionLimit(t, 1)
	cacheLocation := filestore.Location{Root: root, Path: cachePath}
	cases := []struct {
		wantErr   error
		name      string
		request   FindRequest
		wantState ObservationState
	}{
		{name: "zero request is rejected", wantErr: core.ErrFuzzFinderContract},
		{name: "nil root is rejected", request: FindRequest{Location: filestore.Location{Path: cachePath}, Kind: ArtifactCorpus, Format: CacheFormatGo1_26, Retention: validLimit}, wantErr: core.ErrFuzzFinderContract},
		{name: "zero path is rejected", request: FindRequest{Location: filestore.Location{Root: root}, Kind: ArtifactCorpus, Format: CacheFormatGo1_26, Retention: validLimit}, wantErr: core.ErrFuzzFinderContract},
		{name: "undeclared artifact kind is rejected", request: FindRequest{Location: cacheLocation, Format: CacheFormatGo1_26, Retention: validLimit}, wantErr: core.ErrFuzzFinderContract},
		{name: "future artifact kind is rejected", request: FindRequest{Location: cacheLocation, Kind: ArtifactKind(255), Format: CacheFormatGo1_26, Retention: validLimit}, wantErr: core.ErrFuzzFinderContract},
		{name: "unknown format is rejected", request: FindRequest{Location: cacheLocation, Kind: ArtifactCorpus, Retention: validLimit}, wantErr: core.ErrFuzzFinderFormat},
		{name: "future format is rejected", request: FindRequest{Location: cacheLocation, Kind: ArtifactCorpus, Format: CacheFormat(255), Retention: validLimit}, wantErr: core.ErrFuzzFinderFormat},
		{name: "zero retention is rejected", request: FindRequest{Location: cacheLocation, Kind: ArtifactCorpus, Format: CacheFormatGo1_26}, wantErr: core.ErrFuzzFinderContract},
		{name: "missing directory preserves native absence", request: FindRequest{Location: filestore.Location{Root: root, Path: missingPath}, Kind: ArtifactCorpus, Format: CacheFormatGo1_26, Retention: validLimit}, wantErr: core.ErrFuzzFinderObservation, wantState: ObservationFailed},
		{name: "regular file refuses directory contract", request: FindRequest{Location: filestore.Location{Root: root, Path: regularPath}, Kind: ArtifactCorpus, Format: CacheFormatGo1_26, Retention: validLimit}, wantErr: core.ErrFuzzFinderContract, wantState: ObservationFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := Find(tc.request)
			if !errors.Is(gotErr, tc.wantErr) || got.State() != tc.wantState {
				t.Fatalf("Find() = (state %d, %v), want (state %d, %v)", got.State(), gotErr, tc.wantState, tc.wantErr)
			}
			if tc.wantState != ObservationFailed {
				return
			}
			if got.Validate() != nil {
				t.Fatalf("Find() failed observation Validate() error = %v, want nil", got.Validate())
			}
			if got.Kind() != tc.request.Kind {
				t.Fatalf("Find() failed observation Kind() = %d, want the requested %d", got.Kind(), tc.request.Kind)
			}
		})
	}
	t.Run("missing directory preserves the native absence error", func(t *testing.T) {
		t.Parallel()

		_, gotErr := Find(FindRequest{
			Location:  filestore.Location{Root: root, Path: missingPath},
			Kind:      ArtifactCorpus,
			Format:    CacheFormatGo1_26,
			Retention: validLimit,
		})
		if !errors.Is(gotErr, fs.ErrNotExist) {
			t.Fatalf("Find(missing) error = %v, want native %v", gotErr, fs.ErrNotExist)
		}
	})
}

// The three tests below drive findDirectory through nonNativeDirectory, a
// stdlib-conforming fs.ReadDirFile that is not a real *os.File. They are named
// as such per test/production-path: a real rooted directory cannot be made to
// return a nil entry, a zero-progress read, a mid-stream getdirents failure, or
// two entries with the same name from inside this process, so these contracts
// have no native fixture. Every other behavior in this file is proved against a
// real directory.

func TestDirectoryReaderRefusalsARealFilesystemCannotProduce(t *testing.T) {
	t.Parallel()

	request := corpusRequest(filestore.Location{}, mustRetentionLimit(t, 2))
	cases := []struct {
		reader    fs.ReadDirFile
		wantErr   error
		name      string
		wantState ObservationState
	}{
		{
			name:    "nil reader refuses the directory contract",
			reader:  nil,
			wantErr: core.ErrFuzzFinderContract,
		},
		{
			name: "nil directory entry refuses rather than counting a phantom",
			reader: &nonNativeDirectory{steps: []directoryStep{{
				entries: []fs.DirEntry{nil},
			}}},
			wantErr:   core.ErrFuzzFinderObservation,
			wantState: ObservationPartial,
		},
		{
			name: "zero-progress read refuses instead of looping forever",
			reader: &nonNativeDirectory{steps: []directoryStep{{
				entries: nil,
			}}},
			wantErr:   io.ErrNoProgress,
			wantState: ObservationPartial,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := findDirectory(tc.reader, request)
			if !errors.Is(gotErr, tc.wantErr) || got.State() != tc.wantState {
				t.Fatalf("findDirectory() = (state %d, %v), want (state %d, %v)", got.State(), gotErr, tc.wantState, tc.wantErr)
			}
			if got.Retained().Uint64() != 0 {
				t.Fatalf("findDirectory() retained = %d, want 0", got.Retained().Uint64())
			}
		})
	}
}

func TestDirectoryReaderPartialFailurePreservesObservedFactsAndNativeError(t *testing.T) {
	t.Parallel()

	nativeErr := errors.New("native directory failure")
	reader := &nonNativeDirectory{steps: []directoryStep{{
		entries: []fs.DirEntry{fixedDirectoryEntry{name: generatedNameForPosition(t, 3).String()}},
		err:     nativeErr,
	}}}
	got, gotErr := findDirectory(reader, corpusRequest(filestore.Location{}, mustRetentionLimit(t, 4)))
	if !errors.Is(gotErr, core.ErrFuzzFinderObservation) || !errors.Is(gotErr, nativeErr) {
		t.Fatalf("findDirectory(partial failure) error = %v, want %v and native error", gotErr, core.ErrFuzzFinderObservation)
	}
	if got.State() != ObservationPartial || got.Retained().Uint64() != 1 {
		t.Fatalf("findDirectory(partial failure) = state:%d retained:%d, want partial/1", got.State(), got.Retained().Uint64())
	}
	if got.Validate() != nil {
		t.Fatalf("findDirectory(partial failure) observation Validate() error = %v, want nil", got.Validate())
	}
}

func TestDirectoryReaderHoldsCanonicalPrefixUnderDescendingOrderAndDuplicates(t *testing.T) {
	t.Parallel()

	zero := generatedNameForPosition(t, 0)
	one := generatedNameForPosition(t, 1)
	three := generatedNameForPosition(t, 3)
	// Descending arrival forces the eviction branch after the retained prefix is
	// full, and the repeats prove a retained name is counted only once.
	reader := &nonNativeDirectory{steps: []directoryStep{{
		entries: []fs.DirEntry{
			fixedDirectoryEntry{name: three.String()},
			fixedDirectoryEntry{name: three.String()},
			fixedDirectoryEntry{name: one.String()},
			fixedDirectoryEntry{name: one.String()},
			fixedDirectoryEntry{name: zero.String()},
		},
		err: io.EOF,
	}}}
	got, gotErr := findDirectory(reader, corpusRequest(filestore.Location{}, mustRetentionLimit(t, 2)))
	if gotErr != nil || got.State() != ObservationComplete {
		t.Fatalf("findDirectory(descending duplicates) = (state %d, %v), want (%d, nil)", got.State(), gotErr, ObservationComplete)
	}
	if !slices.Equal(got.Names(), []GeneratedName{zero, one}) {
		t.Fatalf("findDirectory(descending duplicates) names = %v, want %v", got.Names(), []GeneratedName{zero, one})
	}
	if got.OverLimitObservations().Uint64() != 1 {
		t.Fatalf("findDirectory(descending duplicates) over-limit = %d, want 1", got.OverLimitObservations().Uint64())
	}
}

func TestDirectoryReaderCountsEachRepeatedOmittedObservation(t *testing.T) {
	t.Parallel()

	zero := generatedNameForPosition(t, 0)
	one := generatedNameForPosition(t, 1)
	two := generatedNameForPosition(t, 2)
	reader := &nonNativeDirectory{steps: []directoryStep{{
		entries: []fs.DirEntry{
			fixedDirectoryEntry{name: zero.String()},
			fixedDirectoryEntry{name: one.String()},
			fixedDirectoryEntry{name: two.String()},
			fixedDirectoryEntry{name: two.String()},
		},
		err: io.EOF,
	}}}
	got, gotErr := findDirectory(reader, corpusRequest(filestore.Location{}, mustRetentionLimit(t, 2)))
	if gotErr != nil || got.State() != ObservationComplete {
		t.Fatalf("findDirectory(repeated omitted name) = (state %d, %v), want (%d, nil)", got.State(), gotErr, ObservationComplete)
	}
	if !slices.Equal(got.Names(), []GeneratedName{zero, one}) {
		t.Fatalf("findDirectory(repeated omitted name) names = %v, want %v", got.Names(), []GeneratedName{zero, one})
	}
	if got.OverLimitObservations().Uint64() != 2 {
		t.Fatalf("findDirectory(repeated omitted name) over-limit = %d, want 2 observations", got.OverLimitObservations().Uint64())
	}
}

func BenchmarkFindRealDirectory128(b *testing.B) {
	b.ReportAllocs()
	benchmarkFindRealDirectory(b, uint64(MaximumRetainedEntries))
}

func BenchmarkFindRealDirectory8192(b *testing.B) {
	b.ReportAllocs()
	benchmarkFindRealDirectory(b, 8192)
}

// benchmarkFindRealDirectory measures the shipped path: one os.Root open, one
// stat, and batched getdirents over a real directory. Measuring a non-native
// in-memory reader instead would report the accumulator's cost with the
// operating system removed, which is not the cost a caller pays.
func benchmarkFindRealDirectory(b *testing.B, entries uint64) {
	rootDirectory := b.TempDir()
	location := fuzzDirectoryLocation(b, rootDirectory)
	for position := range entries {
		writeGeneratedFile(b, rootDirectory, generatedNameForPosition(b, position))
	}
	request := corpusRequest(location, mustRetentionLimit(b, MaximumRetainedEntries))
	b.ResetTimer()
	for b.Loop() {
		got, err := Find(request)
		if err != nil || got.Retained().Uint64() != uint64(MaximumRetainedEntries) {
			b.Fatalf("Find(%d entries) = (retained %d, %v), want (%d, nil)", entries, got.Retained().Uint64(), err, MaximumRetainedEntries)
		}
	}
}

const cacheDirectoryComponent = "cache"

func corpusRequest(location filestore.Location, limit RetentionLimit) FindRequest {
	return FindRequest{Location: location, Kind: ArtifactCorpus, Format: CacheFormatGo1_26, Retention: limit}
}

func crasherRequest(location filestore.Location, limit RetentionLimit) FindRequest {
	return FindRequest{Location: location, Kind: ArtifactCrasher, Format: CacheFormatGo1_26, Retention: limit}
}

func writeGeneratedFile(t testing.TB, rootDirectory string, name GeneratedName) {
	t.Helper()
	path := filepath.Join(rootDirectory, cacheDirectoryComponent, name.String())
	if err := os.WriteFile(path, []byte(name.String()), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func fuzzDirectoryLocation(t testing.TB, rootDirectory string) filestore.Location {
	t.Helper()
	if err := os.Mkdir(filepath.Join(rootDirectory, cacheDirectoryComponent), 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", cacheDirectoryComponent, err)
	}
	return filestore.Location{
		Root: openRootForTest(t, rootDirectory),
		Path: relativePathForTest(t, cacheDirectoryComponent),
	}
}

func openRootForTest(t testing.TB, rootDirectory string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v, want nil", rootDirectory, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("root.Close() error = %v, want nil", closeErr)
		}
	})
	return root
}

func relativePathForTest(t testing.TB, value string) core.RelativePath {
	t.Helper()
	path, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("core.ParseRelativePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func mustRetentionLimit(t testing.TB, value uint16) RetentionLimit {
	t.Helper()
	got, err := NewRetentionLimit(value)
	if err != nil {
		t.Fatalf("NewRetentionLimit(%d) error = %v, want nil", value, err)
	}
	return got
}

type directoryStep struct {
	err     error
	entries []fs.DirEntry
}

// nonNativeDirectory is a stdlib-conforming fs.ReadDirFile used only for the
// directory-reader contracts a real rooted directory cannot produce in-process.
type nonNativeDirectory struct {
	steps []directoryStep
	next  int
}

func (r *nonNativeDirectory) ReadDir(int) ([]fs.DirEntry, error) {
	if r.next >= len(r.steps) {
		return nil, io.EOF
	}
	step := r.steps[r.next]
	r.next++
	return step.entries, step.err
}

func (r *nonNativeDirectory) Read([]byte) (int, error)   { return 0, io.EOF }
func (r *nonNativeDirectory) Close() error               { return nil }
func (r *nonNativeDirectory) Stat() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

type fixedDirectoryEntry struct {
	name string
	mode fs.FileMode
}

func (e fixedDirectoryEntry) Name() string               { return e.name }
func (e fixedDirectoryEntry) IsDir() bool                { return e.mode.IsDir() }
func (e fixedDirectoryEntry) Type() fs.FileMode          { return e.mode.Type() }
func (e fixedDirectoryEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }
