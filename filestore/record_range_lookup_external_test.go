package filestore_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

type lookupNativeSource struct {
	*os.File
	StatFunc   func() (fs.FileInfo, error)
	ReadAtFunc func([]byte, int64) (int, error)
	Reads      int
}

func (s *lookupNativeSource) Stat() (fs.FileInfo, error) {
	if s.StatFunc != nil {
		return s.StatFunc()
	}
	return s.File.Stat()
}
func (s *lookupNativeSource) ReadAt(body []byte, offset int64) (int, error) {
	s.Reads++
	if s.ReadAtFunc != nil {
		return s.ReadAtFunc(body, offset)
	}
	return s.File.ReadAt(body, offset)
}

func lookupIndexFile(t testing.TB, keys []int64) *os.File {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "coordinates"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Error(err)
		}
	})
	extent, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if err := filestore.WriteRecordRangeIndex(file, lineio.RecordRange{Offset: lineio.SourceByteOffset(key), Bytes: extent, Framing: lineio.RecordFramingOpaque}); err != nil {
			t.Fatal(err)
		}
	}
	return file
}
func lookupComparison(value, target int64) core.Comparison {
	if value < target {
		return core.ComparisonLess
	}
	if value > target {
		return core.ComparisonGreater
	}
	return core.ComparisonEqual
}

func TestRecordRangeLookupLowerBoundMatchesNativeCoordinates(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		Name     string
		Keys     []int64
		Target   int64
		Position int
		Present  bool
	}{
		{Name: "empty"},
		{Name: "before first", Keys: []int64{5}, Target: -1, Present: true},
		{Name: "exact singleton", Keys: []int64{5}, Target: 5, Present: true},
		{Name: "after last", Keys: []int64{5}, Target: 6},
		{Name: "first equal of duplicates", Keys: []int64{2, 2, 2, 7}, Target: 2, Present: true},
		{Name: "greater candidate", Keys: []int64{2, 2, 2, 7}, Target: 3, Position: 3, Present: true},
		{Name: "interior exact", Keys: []int64{0, 2, 4, 6, 8}, Target: 4, Position: 2, Present: true},
		{Name: "last exact", Keys: []int64{0, 2, 4, 6, 8}, Target: 8, Position: 4, Present: true},
		{Name: "beyond entire index", Keys: []int64{0, 2, 4, 6, 8}, Target: math.MaxInt64},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			file := lookupIndexFile(t, tc.Keys)
			if _, err := file.Seek(3, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			source := lookupNativeSource{File: file}
			got, err := filestore.LowerBoundRecordRangeIndex(t.Context(), filestore.RecordRangeIndexSearchRequest{Source: &source, Compare: func(_ context.Context, record lineio.RecordRange) (core.Comparison, error) {
				return lookupComparison(int64(record.Offset), tc.Target), nil
			}})
			if err != nil || got.Present != tc.Present {
				t.Fatalf("lookup=%+v/%v want present=%v", got, err, tc.Present)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			if tc.Present {
				if got.Position != filestore.RecordRangeIndexPosition(tc.Position) || int64(got.Record.Offset) != tc.Keys[tc.Position] || got.Record.Bytes.Uint64() != 1 || got.Record.Framing != lineio.RecordFramingOpaque {
					t.Fatalf("lookup=%+v want position=%d key=%d", got, tc.Position, tc.Keys[tc.Position])
				}
			} else if got != (filestore.RecordRangeIndexSearchObservation{}) {
				t.Fatalf("missing result=%+v want zero", got)
			}
			position, err := file.Seek(0, io.SeekCurrent)
			if err != nil || position != 3 {
				t.Fatalf("sequential cursor=%d/%v want3", position, err)
			}
		})
	}
}

func TestRecordRangeLookupPositionalReadPreservesCursorAndEOF(t *testing.T) {
	t.Parallel()
	file := lookupIndexFile(t, []int64{7, 11, 19})
	if _, err := file.Seek(2, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		got, err := filestore.ReadRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: file, Position: filestore.RecordRangeIndexPosition(i)})
		if i >= 3 {
			if !errors.Is(err, io.EOF) || got != (lineio.RecordRange{}) {
				t.Fatalf("position%d=%+v/%v want zero/EOF", i, got, err)
			}
			continue
		}
		if err != nil || int64(got.Offset) != []int64{7, 11, 19}[i] {
			t.Fatalf("position%d=%+v/%v", i, got, err)
		}
	}
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil || position != 2 {
		t.Fatalf("cursor=%d/%v want2", position, err)
	}
}

func TestRecordRangeLookupRefusesMalformedAndUnavailableNativeObjects(t *testing.T) {
	t.Parallel()
	for _, kind := range [...]string{"closed", "directory", "partial tuple", "invalid framing", "oversize offset", "nil info"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			file := lookupIndexFile(t, []int64{7})
			source := lookupNativeSource{File: file}
			want := error(core.ErrFilestoreContract)
			switch kind {
			case "closed":
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				want = os.ErrClosed
			case "directory":
				directory, err := os.Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := directory.Close(); err != nil {
						t.Error(err)
					}
				}()
				source.File = directory
			case "partial tuple":
				if _, err := file.Write([]byte{0}); err != nil {
					t.Fatal(err)
				}
			case "invalid framing":
				if _, err := file.WriteAt([]byte{255}, 16); err != nil {
					t.Fatal(err)
				}
			case "oversize offset":
				if _, err := file.WriteAt([]byte{255}, 0); err != nil {
					t.Fatal(err)
				}
			case "nil info":
				source.StatFunc = func() (fs.FileInfo, error) { return nil, nil }
			}
			got, err := filestore.ReadRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: &source})
			if !errors.Is(err, want) || got != (lineio.RecordRange{}) {
				t.Fatalf("read=%+v/%v want %v", got, err, want)
			}
			found, err := filestore.LowerBoundRecordRangeIndex(t.Context(), filestore.RecordRangeIndexSearchRequest{Source: &source, Compare: func(context.Context, lineio.RecordRange) (core.Comparison, error) {
				t.Fatal("malformed native object reached comparison")
				return core.ComparisonEqual, nil
			}})
			if !errors.Is(err, want) || found != (filestore.RecordRangeIndexSearchObservation{}) {
				t.Fatalf("search=%+v/%v want %v", found, err, want)
			}
			if kind == "closed" {
				var native *os.PathError
				if !errors.As(err, &native) || !errors.Is(err, core.ErrFilestoreSource) || errors.Is(err, core.ErrFilestoreContract) {
					t.Fatalf("native close error lost identity: %v", err)
				}
			}
		})
	}
}

func TestRecordRangeLookupPreservesNativeReaderAtOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		Name string
		N    int
		Err  error
		Want error
	}{
		{Name: "negative count", N: -1, Err: io.ErrClosedPipe, Want: core.ErrFilestoreContract},
		{Name: "impossible count", N: 18, Err: io.ErrClosedPipe, Want: core.ErrFilestoreContract},
		{Name: "empty without error", N: 0, Want: io.ErrUnexpectedEOF},
		{Name: "short without error", N: 8, Want: io.ErrUnexpectedEOF},
		{Name: "partial native refusal", N: 8, Err: io.ErrClosedPipe, Want: io.ErrClosedPipe},
		{Name: "full native refusal", N: 17, Err: io.ErrClosedPipe, Want: io.ErrClosedPipe},
		{Name: "full plus EOF is a complete native read", N: 17, Err: io.EOF},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			file := lookupIndexFile(t, []int64{7})
			source := lookupNativeSource{File: file, ReadAtFunc: func(body []byte, offset int64) (int, error) {
				if _, err := file.ReadAt(body, offset); err != nil {
					return 0, err
				}
				return tc.N, tc.Err
			}}
			got, err := filestore.ReadRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: &source})
			if tc.Want == nil {
				if err != nil || got.Offset != 7 {
					t.Fatalf("read=%+v/%v want complete row", got, err)
				}
				return
			}
			if !errors.Is(err, tc.Want) || got != (lineio.RecordRange{}) {
				t.Fatalf("read=%+v/%v want %v", got, err, tc.Want)
			}
			if tc.Err != nil && !errors.Is(err, tc.Err) {
				t.Fatalf("native cause lost: %v want %v", err, tc.Err)
			}
		})
	}
}

func TestRecordRangeLookupCallbackAndCancellationKeepNativeCauses(t *testing.T) {
	t.Parallel()
	for _, duringStat := range [...]bool{false, true} {
		t.Run(fmt.Sprint(duringStat), func(t *testing.T) {
			t.Parallel()
			file := lookupIndexFile(t, []int64{7})
			closed := lookupIndexFile(t, nil)
			if err := closed.Close(); err != nil {
				t.Fatal(err)
			}
			_, cause := closed.Stat()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			source := lookupNativeSource{File: file}
			if duringStat {
				source.StatFunc = func() (fs.FileInfo, error) { info, err := file.Stat(); cancel(cause); return info, err }
			}
			got, err := filestore.LowerBoundRecordRangeIndex(ctx, filestore.RecordRangeIndexSearchRequest{Source: &source, Compare: func(context.Context, lineio.RecordRange) (core.Comparison, error) {
				cancel(cause)
				return core.ComparisonEqual, cause
			}})
			var native *os.PathError
			if got != (filestore.RecordRangeIndexSearchObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, cause) || !errors.As(err, &native) {
				t.Fatalf("search=%+v/%v want canceled and original native cause", got, err)
			}
		})
	}
}

func TestRecordRangeLookupValidatesAdmissionBeforeEffects(t *testing.T) {
	t.Parallel()
	file := lookupIndexFile(t, []int64{7})
	var typedNil *lookupNativeSource
	for _, source := range []filestore.RecordRangeIndexSource{nil, typedNil} {
		if _, err := filestore.ReadRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: source}); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("nil source=%v", err)
		}
	}
	if _, err := filestore.LowerBoundRecordRangeIndex(t.Context(), filestore.RecordRangeIndexSearchRequest{Source: file}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("nil compare=%v", err)
	}
	if _, err := filestore.ReadRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: file, Position: filestore.RecordRangeIndexPosition(math.MaxUint64)}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("unrepresentable position=%v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	source := lookupNativeSource{File: file, StatFunc: func() (fs.FileInfo, error) { t.Fatal("canceled request observed source"); return nil, nil }}
	if _, err := filestore.LowerBoundRecordRangeIndex(ctx, filestore.RecordRangeIndexSearchRequest{Source: &source, Compare: func(context.Context, lineio.RecordRange) (core.Comparison, error) {
		t.Fatal("canceled comparison ran")
		return core.ComparisonEqual, nil
	}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled admission=%v", err)
	}
	got, err := filestore.LowerBoundRecordRangeIndex(t.Context(), filestore.RecordRangeIndexSearchRequest{Source: file, Compare: func(context.Context, lineio.RecordRange) (core.Comparison, error) { return core.ComparisonUnknown, nil }})
	if !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(err, core.ErrPrimitiveContract) || got != (filestore.RecordRangeIndexSearchObservation{}) {
		t.Fatalf("invalid comparison=%+v/%v", got, err)
	}
}
