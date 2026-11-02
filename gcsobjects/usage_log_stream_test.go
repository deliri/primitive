package gcsobjects

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// Generate arbitrary stream extent from one immutable record. No fixture or
// result aggregate grows with the number of rows; the visitor owns backpressure.
func TestGCSUsageCSVWindowContinuation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		recordBytes int
		rows        uint64
		stopAt      uint64
		cancel      bool
	}{
		{name: "record immediately below CSV working window", recordBytes: 4095, rows: 1},
		{name: "record exactly at CSV working window", recordBytes: 4096, rows: 1},
		{name: "record above CSV working window remains valid", recordBytes: 4097, rows: 1},
		{name: "whole value record spans several windows", recordBytes: 4*4096 + 3, rows: 1},
		{name: "many records continue without total extent quota", rows: 8192},
		{name: "visitor refusal backpressures large source", rows: 8192, stopAt: 2},
		{name: "caller cancellation backpressures large source", rows: 8192, stopAt: 2, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			names := usageColumnNames()
			row := usageRecordFixture(t)
			header := usageCSVFixture(t, names[:])
			wire := usageCSVFixture(t, names[:], usageRecordValues(t, row))
			record := wire[len(header):]
			if tc.recordBytes > 0 {
				row.UserAgent += strings.Repeat("x", tc.recordBytes-len(record))
				wire = usageCSVFixture(t, names[:], usageRecordValues(t, row))
				record = wire[len(header):]
				if len(record) != tc.recordBytes {
					t.Fatalf("record extent=%d,want %d", len(record), tc.recordBytes)
				}
			}
			repeated := usageRepeatedReader{record: record, remaining: tc.rows}
			source := io.MultiReader(bytes.NewReader(header), &repeated)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			cause := errors.New("fixture visitor stops streaming")
			visits := uint64(0)
			responseBytes := uint64(0)
			count, err := visitGCSUsageCSV(ctx, source, func(got GCSUsageRecord) error {
				if got != row {
					t.Fatalf("stream row=%+v,want exact %+v", got, row)
				}
				visits++
				responseBytes += got.ResponseBytes.Uint64()
				if tc.stopAt == visits {
					if tc.cancel {
						cancel(cause)
						return nil
					}
					return cause
				}
				return nil
			})
			if tc.stopAt > 0 {
				if !errors.Is(err, cause) || count != 0 || visits != tc.stopAt {
					t.Fatalf("stopped stream=%d/%d/%v,want zero/%d/original cause", count, visits, err, tc.stopAt)
				}
				maximumRead := tc.stopAt*uint64(len(record)) + 4096
				if repeated.readBytes > maximumRead || repeated.remaining == 0 {
					t.Fatalf("stopped source consumed=%d remaining=%d,want bounded prefetch<=%d and remaining work", repeated.readBytes, repeated.remaining, maximumRead)
				}
				return
			}
			if err != nil || count != tc.rows || visits != tc.rows || responseBytes != tc.rows*row.ResponseBytes.Uint64() || repeated.readBytes != tc.rows*uint64(len(record)) {
				t.Fatalf("stream conservation=%d/%d/%d/%v bytes=%d,want %d rows/exact bytes/nil", count, visits, responseBytes, err, repeated.readBytes, tc.rows)
			}
		})
	}
}

type usageRepeatedReader struct {
	record    []byte
	remaining uint64
	offset    int
	readBytes uint64
}

func (r *usageRepeatedReader) Read(target []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	written := 0
	for written < len(target) && r.remaining > 0 {
		count := copy(target[written:], r.record[r.offset:])
		written += count
		r.offset += count
		r.readBytes += uint64(count)
		if r.offset == len(r.record) {
			r.remaining--
			r.offset = 0
		}
	}
	return written, nil
}
