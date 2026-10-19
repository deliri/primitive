package filestore_test

import (
	"context"
	"encoding/binary"
	"errors"
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

func TestRecordRangeObservationDistinguishesNativeEndAndKeepsCursor(t *testing.T) {
	t.Parallel()
	for _, keys := range [][]int64{nil, {4}, {4, 9, 17}} {
		file := lookupIndexFile(t, keys)
		if _, err := file.Seek(3, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		for position := 0; position <= len(keys)+1; position++ {
			got, err := filestore.ObserveRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: file, Position: filestore.RecordRangeIndexPosition(position)})
			if err != nil || got.Present != (position < len(keys)) {
				t.Fatalf("position=%d observation=%+v/%v", position, got, err)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			if got.Present && (int64(got.Record.Offset) != keys[position] || got.Record.Bytes.Uint64() != 1 || got.Record.Framing != lineio.RecordFramingOpaque) {
				t.Fatalf("native tuple substituted: %+v", got)
			}
			if !got.Present && got != (filestore.RecordRangeIndexReadObservation{}) {
				t.Fatalf("native end leaked record: %+v", got)
			}
			cursor, err := file.Seek(0, io.SeekCurrent)
			if err != nil || cursor != 3 {
				t.Fatalf("sequential cursor moved: %d/%v", cursor, err)
			}
		}
	}
}

func TestRecordRangeObservationRefusesPhysicalTruncationDespiteEOFCause(t *testing.T) {
	t.Parallel()
	file := lookupIndexFile(t, []int64{7})
	source := lookupNativeSource{File: file}
	source.StatFunc = func() (fs.FileInfo, error) {
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if err := file.Truncate(0); err != nil {
			return nil, err
		}
		return info, nil
	}
	got, err := filestore.ObserveRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: &source})
	if got != (filestore.RecordRangeIndexReadObservation{}) || !errors.Is(err, core.ErrFilestoreSource) || !errors.Is(err, io.EOF) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("physical EOF misclassified as observed end: %+v/%v", got, err)
	}
}

func TestRecordRangeObservationWithdrawsAfterNativeReadCancellation(t *testing.T) {
	t.Parallel()
	file := lookupIndexFile(t, []int64{9})
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	cause := errors.New("caller stopped after actual pread")
	source := lookupNativeSource{File: file}
	source.ReadAtFunc = func(data []byte, offset int64) (int, error) {
		n, err := file.ReadAt(data, offset)
		cancel(cause)
		return n, err
	}
	got, err := filestore.ObserveRecordRangeIndexAt(ctx, filestore.RecordRangeIndexReadRequest{Source: &source})
	if got != (filestore.RecordRangeIndexReadObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, cause) {
		t.Fatalf("canceled native result escaped: %+v/%v", got, err)
	}
}

func TestRecordRangeReadObservationRejectsImpossibleShapes(t *testing.T) {
	t.Parallel()
	bytes, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	record := lineio.RecordRange{Bytes: bytes, Framing: lineio.RecordFramingOpaque}
	if err := (filestore.RecordRangeIndexReadObservation{Present: true, Record: record}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (filestore.RecordRangeIndexReadObservation{}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []filestore.RecordRangeIndexReadObservation{{Present: true}, {Record: record}} {
		if value.Validate() == nil {
			t.Fatalf("impossible native observation admitted: %+v", value)
		}
	}
	var absent *os.File
	if value, err := filestore.ObserveRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: absent}); err == nil || value != (filestore.RecordRangeIndexReadObservation{}) {
		t.Fatalf("typed nil admitted: %+v/%v", value, err)
	}
}

func FuzzRecordRangeReadObservationMatchesNativeTupleOracle(f *testing.F) {
	valid := make([]byte, 17)
	binary.BigEndian.PutUint64(valid[:8], 4)
	binary.BigEndian.PutUint64(valid[8:16], 1)
	valid[16] = 3
	f.Add([]byte{}, uint64(0))
	f.Add(valid, uint64(0))
	f.Add(valid, uint64(1))
	f.Add(valid[:16], uint64(0))
	f.Add(valid, uint64(math.MaxUint64))
	f.Fuzz(func(t *testing.T, data []byte, position uint64) {
		path := filepath.Join(t.TempDir(), "native-tuples")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		}()
		got, err := filestore.ObserveRecordRangeIndexAt(t.Context(), filestore.RecordRangeIndexReadRequest{Source: file, Position: filestore.RecordRangeIndexPosition(position)})
		refused := position > uint64(math.MaxInt64/17) || len(data)%17 != 0
		present := !refused && position < uint64(len(data)/17)
		var offset, extent uint64
		var framing byte
		if present {
			body := data[int(position)*17:][:17]
			offset = binary.BigEndian.Uint64(body[:8])
			extent = binary.BigEndian.Uint64(body[8:16])
			framing = body[16]
			refused = offset > math.MaxInt64 || extent == 0 || extent > math.MaxInt64 || offset > math.MaxInt64-extent || framing < 1 || framing > 3
		}
		if refused {
			if err == nil || got != (filestore.RecordRangeIndexReadObservation{}) {
				t.Fatalf("malformed native tuple admitted: %+v/%v", got, err)
			}
			return
		}
		if err != nil || got.Present != present {
			t.Fatalf("native observation=%+v/%v, want present=%v", got, err, present)
		}
		if present && (uint64(got.Record.Offset) != offset || got.Record.Bytes.Uint64() != extent || byte(got.Record.Framing) != framing) {
			t.Fatalf("native tuple substituted: %+v", got)
		}
		if !present && got != (filestore.RecordRangeIndexReadObservation{}) {
			t.Fatalf("native end leaked tuple: %+v", got)
		}
	})
}
