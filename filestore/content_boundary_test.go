package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type contentRejectingWriter struct {
	err   error
	short bool
}

func (w contentRejectingWriter) Write(data []byte) (int, error) {
	if w.short {
		return len(data) - 1, nil
	}
	return 0, w.err
}

func TestContentIndexWriterRefusalPreservesIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		writer  io.Writer
		wantErr error
	}{
		{name: "nil destination", wantErr: core.ErrFilestoreContract},
		{name: "typed nil destination", writer: (*bytes.Buffer)(nil), wantErr: core.ErrFilestoreContract},
		{name: "short write retains io identity", writer: contentRejectingWriter{short: true}, wantErr: io.ErrShortWrite},
		{name: "destination failure retains permission identity", writer: contentRejectingWriter{err: os.ErrPermission}, wantErr: os.ErrPermission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := WriteContentIndexEntry(tc.writer, contentEntryForTest(t, 1, 1))
			if !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("WriteContentIndexEntry() error = %v, want %v and filestore contract", err, tc.wantErr)
			}
		})
	}
}

func TestContentSortRefusalReturnsNoSummary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		configure func(*testing.T, *ContentSortRequest) context.Context
		wantErr   error
	}{
		{name: "cancelled before scratch mutation", configure: func(t *testing.T, r *ContentSortRequest) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, wantErr: context.Canceled},
		{name: "same scratch handle is refused", configure: func(t *testing.T, r *ContentSortRequest) context.Context { r.Files[1] = r.Files[0]; return t.Context() }, wantErr: core.ErrFilestoreContract},
		{name: "destination aliases input inode", configure: func(t *testing.T, r *ContentSortRequest) context.Context {
			r.Destination = r.Files[0]
			return t.Context()
		}, wantErr: core.ErrFilestoreContract},
		{name: "missing scratch is refused", configure: func(t *testing.T, r *ContentSortRequest) context.Context { r.Files[1] = nil; return t.Context() }, wantErr: core.ErrFilestoreContract},
		{name: "typed nil destination is refused", configure: func(t *testing.T, r *ContentSortRequest) context.Context {
			r.Destination = (*bytes.Buffer)(nil)
			return t.Context()
		}, wantErr: core.ErrFilestoreContract},
		{name: "destination failure retains permission identity", configure: func(t *testing.T, r *ContentSortRequest) context.Context {
			r.Destination = contentRejectingWriter{err: os.ErrPermission}
			return t.Context()
		}, wantErr: os.ErrPermission},
		{name: "short destination does not issue summary", configure: func(t *testing.T, r *ContentSortRequest) context.Context {
			r.Destination = contentRejectingWriter{short: true}
			return t.Context()
		}, wantErr: io.ErrShortWrite},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			files := contentScratchForTest(t, dir)
			if err := WriteContentIndexEntry(files[0], contentEntryForTest(t, 1, 1)); err != nil {
				t.Fatal(err)
			}
			var destination bytes.Buffer
			request := ContentSortRequest{Files: files, Destination: &destination}
			ctx := tc.configure(t, &request)
			got, err := SortContentIndex(ctx, request)
			if !errors.Is(err, tc.wantErr) || got != (ContentIndexSummary{}) || destination.Len() != 0 {
				t.Fatalf("SortContentIndex() = (%+v,%v,%d bytes), want zero summary, %v and no captured output", got, err, destination.Len(), tc.wantErr)
			}
		})
	}
}
