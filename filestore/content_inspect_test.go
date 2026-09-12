package filestore

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestContentInspectionLayerTriadExactHeldBytes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                         string
		order                                        []uint64
		wrongDigest, wrongExtent, cancelled, missing bool
		wantErr                                      error
	}{
		{name: "empty canonical stream remains empty"},
		{name: "strict ordered records bind exact held bytes", order: []uint64{1, 2}},
		{name: "duplicate canonical records are refused", order: []uint64{1, 1}, wantErr: core.ErrFilestoreContract},
		{name: "reversed records are refused", order: []uint64{2, 1}, wantErr: core.ErrFilestoreContract},
		{name: "foreign digest cannot authenticate valid records", order: []uint64{1}, wrongDigest: true, wantErr: core.ErrFilestoreContract},
		{name: "wrong encoded extent cannot authenticate valid records", order: []uint64{1}, wrongExtent: true, wantErr: core.ErrFilestoreContract},
		{name: "cancelled inspection returns no summary", order: []uint64{1}, cancelled: true, wantErr: context.Canceled},
		{name: "missing file returns no summary", missing: true, wantErr: core.ErrFilestoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			files := contentScratchForTest(t, dir)
			var canonical bytes.Buffer
			for _, ordinal := range tc.order {
				if err := WriteContentIndexEntry(&canonical, contentEntryForTest(t, ordinal, 1)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := files[0].Write(canonical.Bytes()); err != nil {
				t.Fatal(err)
			}
			extent, err := core.NewByteLength(uint64(canonical.Len()))
			if err != nil {
				t.Fatal(err)
			}
			request := ContentIndexInspectionRequest{File: files[0], Content: ContentIndexEntry{Digest: core.SHA256Of(canonical.Bytes()), Extent: extent}}
			if tc.wrongDigest {
				request.Content.Digest = core.SHA256Of([]byte("foreign"))
			}
			if tc.wrongExtent {
				request.Content.Extent, err = core.NewByteLength(uint64(canonical.Len() + 1))
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.missing {
				request.File = nil
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			got, err := InspectContentIndex(ctx, request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("InspectContentIndex() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil && got != (ContentIndexSummary{}) {
				t.Fatalf("refused inspection = %+v, want zero", got)
			}
			if err == nil && (got.Entries != uint64(len(tc.order)) || got.Extent.Uint64() != uint64(len(tc.order))) {
				t.Fatalf("inspection = %+v, want %d one-byte entries", got, len(tc.order))
			}
			if _, err := files[0].Stat(); err != nil {
				t.Fatalf("borrowed file Stat() error = %v, want nil", err)
			}
		})
	}
}
