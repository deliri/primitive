package gcsobjects

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGCSUsageShortUnknownHeadersKeepTheirValidation(t *testing.T) {
	t.Parallel()
	names := usageColumnNames()
	for _, tc := range []struct {
		name    string
		added   []string
		wantErr error
	}{
		{name: "one byte unknown header remains admissible", added: []string{"x"}},
		{name: "repeated unknown headers remain admissible", added: []string{"x", "x"}},
		{name: "empty unknown header still refuses", added: []string{""}, wantErr: core.ErrObjectStoreSource},
		{name: "short malformed UTF8 header still refuses", added: []string{string([]byte{0xff})}, wantErr: core.ErrObjectStoreSource},
		{name: "shortest known column duplicate still refuses", added: []string{names[usageIP]}, wantErr: core.ErrObjectStoreSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seed := usageRecordFixture(t)
			header := append([]string(nil), names[:]...)
			header = append(header, tc.added...)
			values := usageRecordValues(t, seed)
			for range tc.added {
				values = append(values, "opaque")
			}
			wire := usageCSVFixture(t, header, values)
			var got GCSUsageRecord
			visits := uint64(0)
			count, err := visitGCSUsageCSV(t.Context(), bytes.NewReader(wire), func(row GCSUsageRecord) error { got = row; visits++; return nil })
			wantCount := uint64(1)
			want := seed
			if tc.wantErr != nil {
				wantCount = 0
				want = GCSUsageRecord{}
			}
			if !errors.Is(err, tc.wantErr) || count != wantCount || visits != wantCount || got != want {
				t.Fatalf("short header result=%+v count=%d visits=%d error=%v, want %+v count=%d error=%v", got, count, visits, err, want, wantCount, tc.wantErr)
			}
		})
	}
}
