package objectstore

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type exactMetadataInfo struct {
	fs.FileInfo
	extent int64
}

func (i exactMetadataInfo) Size() int64 { return i.extent }

type exactMetadataSource struct {
	source           *bytes.Reader
	info             fs.FileInfo
	position         int64
	seekErr, statErr error
	statCalls        int
}

func (s *exactMetadataSource) Read(p []byte) (int, error) { return s.source.Read(p) }
func (s *exactMetadataSource) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != io.SeekCurrent {
		return 0, fs.ErrInvalid
	}
	return s.position, s.seekErr
}
func (s *exactMetadataSource) Stat() (fs.FileInfo, error) { s.statCalls++; return s.info, s.statErr }

func TestExactReaderMetadataReplyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                     string
		info                     fs.FileInfo
		position                 int64
		seekErr, statErr, want   error
		empty                    bool
		wantCount, wantStatCalls int
	}{
		{name: "finite file position proves exact final byte", info: exactMetadataInfo{extent: 1}, position: 1, wantCount: 1, wantStatCalls: 1},
		{name: "empty file metadata proves empty object", info: exactMetadataInfo{}, empty: true, wantStatCalls: 1},
		{name: "nil successful stat cannot certify a file", position: 1, want: core.ErrObjectStoreSource, wantStatCalls: 1},
		{name: "nil stat cannot certify empty source", empty: true, want: core.ErrObjectStoreSource, wantStatCalls: 1},
		{name: "seek failure survives without a stat call", seekErr: fs.ErrPermission, want: fs.ErrPermission},
		{name: "stat failure survives exact-byte read", statErr: fs.ErrPermission, position: 1, want: fs.ErrPermission, wantStatCalls: 1},
		{name: "negative position cannot become unsigned remaining bytes", info: exactMetadataInfo{extent: 1}, position: -1, want: core.ErrObjectStoreSource, wantStatCalls: 1},
		{name: "position beyond file cannot certify truncated data", info: exactMetadataInfo{extent: 1}, position: 2, want: core.ErrObjectStoreSource, wantStatCalls: 1},
		{name: "negative file size is refused", info: exactMetadataInfo{extent: -1}, want: core.ErrObjectStoreSource, wantStatCalls: 1},
		{name: "remaining file byte withholds final transfer byte", info: exactMetadataInfo{extent: 2}, position: 1, want: core.ErrObjectStoreSource, wantStatCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if caught := recover(); caught != nil {
					t.Errorf("metadata reply caused panic %v, want typed source refusal", caught)
				}
			}()
			data := []byte{0x81}
			if tc.empty {
				data = nil
			}
			source := &exactMetadataSource{source: bytes.NewReader(data), info: tc.info, position: tc.position, seekErr: tc.seekErr, statErr: tc.statErr}
			reader, err := NewExactReader(source, mustByteLength(t, uint64(len(data))))
			if err != nil {
				t.Fatal(err)
			}
			var output [1]byte
			count := 0
			if tc.empty {
				err = reader.ProveEmpty()
			} else {
				count, err = reader.Read(output[:])
			}
			if !errors.Is(err, tc.want) || count != tc.wantCount || source.statCalls != tc.wantStatCalls || reader.verified != (tc.want == nil) {
				t.Fatalf("metadata result=(%d,%v),stat calls=%d verified=%t; want (%d,%v),calls=%d verified=%t", count, err, source.statCalls, reader.verified, tc.wantCount, tc.want, tc.wantStatCalls, tc.want == nil)
			}
			if count == 1 && output[0] != 0x81 {
				t.Fatalf("delivered byte=%x,want 81", output[0])
			}
			if tc.want != nil && (!errors.Is(reader.Failure(), tc.want) || !errors.Is(reader.Failure(), core.ErrObjectStoreIntegrity)) {
				t.Fatalf("metadata failure=%v,want integrity and %v", reader.Failure(), tc.want)
			}
		})
	}
}
