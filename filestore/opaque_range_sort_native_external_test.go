package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestOpaqueRangeSortingUsesNativeFilesAcrossMergeBoundaries(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			var files [3]*os.File
			for slot, name := range [...]string{"source", "index", "merge"} {
				file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				files[slot] = file
				defer func() {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				}()
			}
			count := [...]int{1, 63, 64, 65, 4095, 4096, 4097, 8193}[index%8]
			length, err := core.NewByteLength(12)
			if err != nil {
				t.Fatal(err)
			}
			for arrival := range count {
				body := []byte(fmt.Sprintf("%08d\n", count-1-arrival))
				body = append(body, 255, 0, '\n')
				written, err := files[0].Write(body)
				if err != nil || written != 12 {
					t.Fatalf("native source write: %d/%v", written, err)
				}
				record := lineio.RecordRange{Offset: lineio.SourceByteOffset(arrival * 12), Bytes: length, Framing: lineio.RecordFramingOpaque}
				if err := filestore.WriteRecordRangeIndex(files[1], record); err != nil {
					t.Fatal(err)
				}
			}
			read := func(source lineio.RecordSource, record lineio.RecordRange, destination []byte) error {
				count, err := source.ReadAt(destination, int64(record.Offset))
				if err != nil || count != len(destination) {
					return errors.Join(err, io.ErrUnexpectedEOF)
				}
				return nil
			}
			visited := 0
			observation, err := filestore.SortRecordRanges(t.Context(), filestore.RecordRangeSortRequest{
				Source: files[0], Files: [2]*os.File{files[1], files[2]},
				Compare: func(_ context.Context, source lineio.RecordSource, left, right lineio.RecordRange) (core.Comparison, error) {
					var a, b [12]byte
					if err := errors.Join(read(source, left, a[:]), read(source, right, b[:])); err != nil {
						return core.ComparisonUnknown, err
					}
					switch bytes.Compare(a[:], b[:]) {
					case -1:
						return core.ComparisonLess, nil
					case 0:
						return core.ComparisonEqual, nil
					default:
						return core.ComparisonGreater, nil
					}
				},
				Visit: func(record lineio.RecordRange) error {
					var data [12]byte
					if err := read(files[0], record, data[:]); err != nil {
						return err
					}
					want := append([]byte(fmt.Sprintf("%08d\n", visited)), 255, 0, '\n')
					if !bytes.Equal(data[:], want) || record.Framing != lineio.RecordFramingOpaque {
						t.Fatalf("native sorted span %d changed bytes or framing", visited)
					}
					visited++
					return nil
				},
			})
			if err != nil || observation.Records != uint64(count) || visited != count {
				t.Fatalf("native merge count: %d/%d/%v", observation.Records, visited, err)
			}
		})
	}
}
