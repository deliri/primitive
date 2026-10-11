package filestore_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func recordRangeSortFiles(t testing.TB) (*os.File, [2]*os.File) {
	t.Helper()
	dir := t.TempDir()
	var files [3]*os.File
	for i, name := range [3]string{"source", "index", "scratch"} {
		file, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		files[i] = file
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Errorf("close native fixture%v", err)
			}
		})
	}
	return files[0], [2]*os.File{files[1], files[2]}
}
func recordRangeSortPrefixComparison(_ context.Context, source lineio.RecordSource, a, b lineio.RecordRange) (core.Comparison, error) {
	var left, right [8]byte
	if _, err := source.ReadAt(left[:], int64(a.Offset)); err != nil {
		return core.ComparisonUnknown, err
	}
	if _, err := source.ReadAt(right[:], int64(b.Offset)); err != nil {
		return core.ComparisonUnknown, err
	}
	switch bytes.Compare(left[:], right[:]) {
	case -1:
		return core.ComparisonLess, nil
	case 0:
		return core.ComparisonEqual, nil
	case 1:
		return core.ComparisonGreater, nil
	default:
		return core.ComparisonUnknown, core.ErrFilestoreContract
	}
}
func writeRecordRangeSortFixture(t testing.TB, source *os.File, index *os.File, count int, equal bool) {
	t.Helper()
	for i := range count {
		key := count - i
		if equal {
			key = 1
		}
		data := []byte(fmt.Sprintf("%08d\n", key))
		if _, err := source.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	buffer, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatal(err)
	}
	for record, err := range lineio.RecordRanges(context.Background(), lineio.RecordRangeRequest{Source: source, BufferBytes: buffer}) {
		if err != nil {
			t.Fatal(err)
		}
		if err := filestore.WriteRecordRangeIndex(index, record); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRecordRangeSortNativeAdmissionAndStableMergeMatrix(t *testing.T) {
	t.Parallel()
	for _, count := range []int{0, 1, 2, 4095, 4096, 4097, 8192, 8193} {
		for mode := range 8 {
			t.Run(fmt.Sprintf("count%d-mode%d", count, mode), func(t *testing.T) {
				t.Parallel()
				source, files := recordRangeSortFiles(t)
				writeRecordRangeSortFixture(t, source, files[0], count, mode == 1)
				ctx := t.Context()
				seen := 0
				request := filestore.RecordRangeSortRequest{Source: source, Files: files, Compare: recordRangeSortPrefixComparison, Visit: func(record lineio.RecordRange) error {
					expected := int64(count-seen-1) * 9
					if mode == 1 {
						expected = int64(seen) * 9
					}
					if int64(record.Offset) != expected || record.Bytes.Uint64() != 9 || record.Framing != lineio.RecordFramingLF {
						t.Fatalf("visited%d=%+v want offset%d", seen, record, expected)
					}
					seen++
					return nil
				}}
				switch mode {
				case 2:
					if _, err := files[0].Write([]byte{1}); err != nil {
						t.Fatal(err)
					}
				case 3:
					request.Compare = func(context.Context, lineio.RecordSource, lineio.RecordRange, lineio.RecordRange) (core.Comparison, error) {
						return core.ComparisonUnknown, io.ErrClosedPipe
					}
				case 4:
					request.Compare = func(context.Context, lineio.RecordSource, lineio.RecordRange, lineio.RecordRange) (core.Comparison, error) {
						return core.Comparison(255), nil
					}
				case 5:
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				case 6:
					request.Visit = func(lineio.RecordRange) error { seen++; return io.ErrClosedPipe }
				case 7:
					if count > 0 {
						if err := source.Truncate(int64(count*9 - 1)); err != nil {
							t.Fatal(err)
						}
					}
				}
				got, err := filestore.SortRecordRanges(ctx, request)
				switch {
				case mode == 2:
					if !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(err, io.ErrUnexpectedEOF) || seen != 0 || got.Records != 0 {
						t.Fatalf("truncated%+v/%v effects%d", got, err, seen)
					}
				case mode == 3 && count > 1:
					if !errors.Is(err, io.ErrClosedPipe) || seen != 0 || got.Records != 0 {
						t.Fatalf("comparison%+v/%v effects%d", got, err, seen)
					}
				case mode == 4 && count > 1:
					if !errors.Is(err, core.ErrFilestoreContract) || seen != 0 || got.Records != 0 {
						t.Fatalf("enum%+v/%v effects%d", got, err, seen)
					}
				case mode == 5:
					if !errors.Is(err, context.Canceled) || seen != 0 || got.Records != 0 {
						t.Fatalf("cancel%+v/%v effects%d", got, err, seen)
					}
				case mode == 6 && count > 0:
					if !errors.Is(err, io.ErrClosedPipe) || seen != 1 || got.Records != 0 {
						t.Fatalf("consumer%+v/%v effects%d", got, err, seen)
					}
				case mode == 7 && count > 0:
					if !errors.Is(err, core.ErrFilestoreContract) || seen != 0 || got.Records != 0 {
						t.Fatalf("source%+v/%v effects%d", got, err, seen)
					}
				default:
					if err != nil || got.Records != uint64(count) || seen != count {
						t.Fatalf("sort%+v/%v effects%d want%d", got, err, seen, count)
					}
				}
			})
		}
	}
}
func FuzzRecordRangeSortMatchesIndependentStableBytes(f *testing.F) {
	f.Add([]byte{9, 2, 4, 2, 0})
	f.Add([]byte{})
	f.Add([]byte{255, 0, 255, 0})
	f.Fuzz(func(t *testing.T, keys []byte) {
		source, files := recordRangeSortFiles(t)
		for _, key := range keys {
			if _, err := fmt.Fprintf(source, "%08d\n", key); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		buffer, err := core.NewByteCount(4096)
		if err != nil {
			t.Fatal(err)
		}
		var expected []lineio.RecordRange
		for record, err := range lineio.RecordRanges(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: buffer}) {
			if err != nil {
				t.Fatal(err)
			}
			expected = append(expected, record)
			if err := filestore.WriteRecordRangeIndex(files[0], record); err != nil {
				t.Fatal(err)
			}
		}
		// The oracle orders the original input bytes and pins ties to input ordinal.
		slices.SortStableFunc(expected, func(a, b lineio.RecordRange) int { return int(keys[int(a.Offset)/9]) - int(keys[int(b.Offset)/9]) })
		var got []lineio.RecordRange
		observation, err := filestore.SortRecordRanges(t.Context(), filestore.RecordRangeSortRequest{Source: source, Files: files, Compare: recordRangeSortPrefixComparison, Visit: func(record lineio.RecordRange) error { got = append(got, record); return nil }})
		if err != nil || observation.Records != uint64(len(keys)) || !slices.Equal(got, expected) {
			t.Fatalf("stable sort%+v/%v got%v want%v", observation, err, got, expected)
		}
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		parent, err := core.ParseAbsolutePath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var owned []lineio.RecordRange
		scoped, err := filestore.SortRecordStream(t.Context(), filestore.RecordRangeStreamRequest{Parent: parent, Source: source, Compare: recordRangeSortPrefixComparison, Visit: func(record lineio.RecordRange) error { owned = append(owned, record); return nil }})
		if err != nil || scoped != observation || !slices.Equal(owned, expected) {
			t.Fatalf("owned observation%+v/%v records%v want%+v/%v", scoped, err, owned, observation, expected)
		}

	})
}
func FuzzRecordRangeIndexCanonicalTuple(f *testing.F) {
	f.Add([]byte{})
	f.Add(make([]byte, filestore.RecordRangeIndexBytes))
	validSeed := make([]byte, filestore.RecordRangeIndexBytes)
	validSeed[15] = 1
	validSeed[16] = byte(lineio.RecordFramingEOF)
	f.Add(validSeed)
	f.Add([]byte{1, 2, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := filestore.ReadRecordRangeIndex(bytes.NewReader(data))
		valid := false
		if len(data) >= filestore.RecordRangeIndexBytes {
			offset := binary.BigEndian.Uint64(data[:8])
			extent := binary.BigEndian.Uint64(data[8:16])
			valid = offset <= math.MaxInt64 && extent > 0 && extent <= math.MaxInt64-offset && (data[16] == 1 || data[16] == 2)
		}
		if (err == nil) != valid {
			t.Fatalf("tuple admission%+v/%v independently valid%t bytes%v", got, err, valid, data)
		}
		if err != nil {
			if got != (lineio.RecordRange{}) || !errors.Is(err, io.EOF) && !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("rejection%+v/%v", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted tuple%+v invalid%v", got, err)
		}
		var encoded bytes.Buffer
		if err := filestore.WriteRecordRangeIndex(&encoded, got); err != nil {
			t.Fatal(err)
		}
		if len(data) < filestore.RecordRangeIndexBytes || !bytes.Equal(encoded.Bytes(), data[:filestore.RecordRangeIndexBytes]) {
			t.Fatalf("canonical bytes%v want%v", encoded.Bytes(), data)
		}
	})
}

type recordRangeAtRefusal struct{ *os.File }

func (recordRangeAtRefusal) ReadAt([]byte, int64) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestRecordRangeStreamOwnsNativeScratchWithoutOwningSource(t *testing.T) {
	t.Parallel()
	for mode := range 8 {
		t.Run(fmt.Sprintf("mode%d", mode), func(t *testing.T) {
			t.Parallel()
			source, files := recordRangeSortFiles(t)
			writeRecordRangeSortFixture(t, source, files[0], 2, false)
			if _, err := source.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			parent, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			ctx := t.Context()
			request := filestore.RecordRangeStreamRequest{Parent: parent, Source: source, Compare: recordRangeSortPrefixComparison, Visit: func(lineio.RecordRange) error { calls++; return nil }}
			switch mode {
			case 1:
				request.Visit = func(lineio.RecordRange) error { calls++; return io.ErrClosedPipe }
			case 2:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case 3:
				request.Compare = func(context.Context, lineio.RecordSource, lineio.RecordRange, lineio.RecordRange) (core.Comparison, error) {
					return core.ComparisonUnknown, io.ErrClosedPipe
				}
			case 4:
				request.Visit = nil
			case 5:
				request.Visit = func(lineio.RecordRange) error { calls++; panic(io.ErrClosedPipe) }
			case 6:
				request.Source = recordRangeAtRefusal{source}
			case 7:
				request.Source = nil
			}
			var got filestore.RecordRangeSortObservation
			var resultErr error
			var recovered error
			func() {
				defer func() {
					if value := recover(); value != nil {
						if refusal, ok := value.(error); ok {
							recovered = refusal
						} else {
							t.Fatalf("foreign panic%v", value)
						}
					}
				}()
				got, resultErr = filestore.SortRecordStream(ctx, request)
			}()
			switch mode {
			case 0:
				if resultErr != nil || got.Records != 2 || calls != 2 || recovered != nil {
					t.Fatalf("complete%+v/%v calls%d panic%v", got, resultErr, calls, recovered)
				}
			case 1:
				if !errors.Is(resultErr, io.ErrClosedPipe) || got.Records != 0 || calls != 1 {
					t.Fatalf("consumer%+v/%v calls%d", got, resultErr, calls)
				}
			case 2:
				if !errors.Is(resultErr, context.Canceled) || got.Records != 0 || calls != 0 {
					t.Fatalf("cancel%+v/%v calls%d", got, resultErr, calls)
				}
			case 3:
				if !errors.Is(resultErr, io.ErrClosedPipe) || got.Records != 0 || calls != 0 {
					t.Fatalf("compare%+v/%v calls%d", got, resultErr, calls)
				}
			case 4, 7:
				if !errors.Is(resultErr, core.ErrFilestoreContract) || got.Records != 0 || calls != 0 {
					t.Fatalf("contract%+v/%v calls%d", got, resultErr, calls)
				}
			case 5:
				if recovered != io.ErrClosedPipe || calls != 1 {
					t.Fatalf("panic%v calls%d", recovered, calls)
				}
			case 6:
				if !errors.Is(resultErr, io.ErrUnexpectedEOF) || !errors.Is(resultErr, core.ErrFilestoreContract) || got.Records != 0 || calls != 0 {
					t.Fatalf("native%+v/%v calls%d", got, resultErr, calls)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("scratch cleanup%d/%v", len(entries), err)
			}
			var byteProbe [1]byte
			if n, err := source.ReadAt(byteProbe[:], 0); n != 1 || err != nil {
				t.Fatalf("borrowed source closed%d/%v", n, err)
			}
		})
	}
}

type recordRangeIndexRefusalReader struct{ data []byte }

func (p *recordRangeIndexRefusalReader) Read(buffer []byte) (int, error) {
	n := copy(buffer, p.data)
	p.data = p.data[n:]
	return n, io.ErrClosedPipe
}
func TestRecordRangeIndexPreservesNativeRefusalAlongsideCompleteTuple(t *testing.T) {
	t.Parallel()
	data := make([]byte, filestore.RecordRangeIndexBytes)
	data[15] = 1
	data[16] = byte(lineio.RecordFramingEOF)
	for _, prefix := range []int{0, 1, 8, 16, 17} {
		t.Run(fmt.Sprintf("prefix%d", prefix), func(t *testing.T) {
			t.Parallel()
			got, err := filestore.ReadRecordRangeIndex(&recordRangeIndexRefusalReader{data: data[:prefix]})
			if got != (lineio.RecordRange{}) || !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("native tuple%+v/%v want zero and both refusals", got, err)
			}
		})
	}
}

func TestContentIndexPreservesNativeRefusalAlongsideCompleteTuple(t *testing.T) {
	t.Parallel()
	entry := filestore.ContentIndexEntry{Digest: core.SHA256Of(nil)}
	var encoded bytes.Buffer
	if err := filestore.WriteContentIndexEntry(&encoded, entry); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	for _, prefix := range []int{0, 1, 8, 16, 39, 40} {
		t.Run(fmt.Sprintf("prefix%d", prefix), func(t *testing.T) {
			t.Parallel()
			got, err := filestore.ReadContentIndexEntry(&recordRangeIndexRefusalReader{data: data[:prefix]})
			if got != (filestore.ContentIndexEntry{}) || !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("native tuple%+v/%v want zero and both refusals", got, err)
			}
		})
	}
}

type recordRangeIndexBadCountReader struct{ delta int }

func (p recordRangeIndexBadCountReader) Read(buffer []byte) (int, error) {
	if p.delta < 0 {
		return -1, io.ErrClosedPipe
	}
	return len(buffer) + p.delta, io.ErrClosedPipe
}
func TestRecordRangeIndexRefusesInvalidNativeReadCounts(t *testing.T) {
	t.Parallel()
	for _, delta := range []int{-1, 1, 1 << 30} {
		t.Run(fmt.Sprintf("delta%d", delta), func(t *testing.T) {
			t.Parallel()
			got, err := filestore.ReadRecordRangeIndex(recordRangeIndexBadCountReader{delta: delta})
			if got != (lineio.RecordRange{}) || !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(err, io.ErrShortBuffer) || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("read count%+v/%v want zero and count/native refusals", got, err)
			}
		})
	}
}

func BenchmarkRecordRangeSort1024NativeRecords(b *testing.B) {
	source, files := recordRangeSortFiles(b)
	writeRecordRangeSortFixture(b, source, files[0], 1024, false)
	seen := 0
	request := filestore.RecordRangeSortRequest{Source: source, Files: files, Compare: recordRangeSortPrefixComparison, Visit: func(lineio.RecordRange) error { seen++; return nil }}
	b.ReportAllocs()
	for b.Loop() {
		seen = 0
		observation, err := filestore.SortRecordRanges(b.Context(), request)
		if err != nil || observation.Records != 1024 || seen != 1024 {
			b.Fatalf("native sort%+v/%v visits%d", observation, err, seen)
		}
	}
}
