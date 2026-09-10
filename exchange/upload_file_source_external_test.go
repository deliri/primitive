package exchange_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestUploadFileSourceCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	type sourceKind uint8
	const (
		regularFile sourceKind = iota
		pipeFile
	)
	cases := []struct {
		name           string
		kind           sourceKind
		payload        string
		offset         int64
		declared       uint64
		wantErr        error
		wantCalls      int
		wantBytes      uint64
		wantWire       string
		wantSourceOpen bool
	}{
		{name: "regular file exact extent crosses unchanged", payload: "a\x00z", declared: 3, wantCalls: 1, wantBytes: 3, wantWire: "a\x00z", wantSourceOpen: true},
		{name: "regular file current offset controls remaining extent", payload: "a\x00z", offset: 1, declared: 2, wantCalls: 1, wantBytes: 2, wantWire: "\x00z", wantSourceOpen: true},
		{name: "regular file understated declaration is refused before transport", payload: "abc", declared: 2, wantErr: core.ErrExchangeRequest, wantSourceOpen: true},
		{name: "regular file overstated declaration is refused before transport", payload: "abc", declared: 4, wantErr: core.ErrExchangeRequest, wantSourceOpen: true},
		{name: "pipe has unknown extent and remains a valid streaming source", kind: pipeFile, payload: "p\x00q", declared: 3, wantCalls: 1, wantBytes: 3, wantWire: "p\x00q", wantSourceOpen: true},
		{name: "empty pipe cannot acquire phantom payload bytes", kind: pipeFile, wantCalls: 1, wantSourceOpen: true},
		{name: "empty regular file stays empty without surrendering ownership", wantCalls: 1, wantSourceOpen: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var source *os.File
			var err error
			switch tc.kind {
			case regularFile:
				path := filepath.Join(t.TempDir(), "source")
				if err := writeExchangeFixtureFile(t, path, []byte(tc.payload)); err != nil {
					t.Fatalf("file fixture setup error = %v, want nil", err)
				}
				source, err = openExchangeFixtureFile(t, path)
			case pipeFile:
				var writer *os.File
				var pipe filestore.Pipe
				pipe, err = filestore.OpenPipe(t.Context())
				source, writer = pipe.Reader, pipe.Writer
				if err == nil {
					// Each payload is smaller than one OS pipe buffer. No worker
					// or scheduler timing is needed to present a non-seekable file.
					n, writeErr := io.WriteString(writer, tc.payload)
					err = errors.Join(writeErr, writer.Close())
					if n != len(tc.payload) {
						err = errors.Join(err, io.ErrShortWrite)
					}
				}
			default:
				t.Fatalf("source fixture kind = %v, want declared kind", tc.kind)
			}
			if source != nil {
				t.Cleanup(func() {
					if err := source.Close(); err != nil {
						t.Errorf("caller-owned source Close() = %v, want nil", err)
					}
				})
			}
			if err != nil {
				t.Fatalf("source fixture setup error = %v, want nil", err)
			}
			if tc.offset != 0 {
				if offset, err := source.Seek(tc.offset, io.SeekStart); err != nil || offset != tc.offset {
					t.Fatalf("source offset = (%d, %v), want (%d, nil)", offset, err, tc.offset)
				}
			}
			type observation struct {
				wire []byte
				err  error
			}
			observed := make(chan observation, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wire, readErr := io.ReadAll(io.LimitReader(r.Body, 16))
				observed <- observation{wire: wire, err: errors.Join(readErr, r.Body.Close())}
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)
			noContent := mustHTTPStatus(t, http.StatusNoContent)
			got, gotErr := exchange.Upload(exchange.UploadCall{
				Context: t.Context(), Client: mustExchangeClient(t, server.Client()), Policy: singleAttemptStreamPolicy(t),
				Request: exchange.UploadRequest{Target: mustEndpoint(t, server.URL), Source: source,
					Semantics:     exchange.RequestSemantics{Method: exchange.MethodPut, Replay: exchange.ReplaySingleAttempt},
					ContentLength: new(mustByteLength(t, tc.declared)), ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: noContent},
			})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Upload() error = %v, want %v", gotErr, tc.wantErr)
			}
			var calls int
			var received observation
			select {
			case received = <-observed:
				calls++
			default:
			}
			if calls != tc.wantCalls || string(received.wire) != tc.wantWire || received.err != nil || got.DeclaredRequestBytes.Uint64() != tc.wantBytes {
				t.Fatalf("server calls/wire/error/client bytes = (%d, %q, %v, %d), want (%d, %q, nil, %d)", calls, received.wire, received.err, got.DeclaredRequestBytes.Uint64(), tc.wantCalls, tc.wantWire, tc.wantBytes)
			}
			_, statErr := source.Stat()
			if (statErr == nil) != tc.wantSourceOpen {
				t.Fatalf("source remains caller-owned = %t (%v), want %t", statErr == nil, statErr, tc.wantSourceOpen)
			}
			if tc.wantErr != nil {
				if got.Metadata.Attempts != 0 || got.Metadata.Status != (core.HTTPStatusCode{}) || got.DeclaredRequestBytes != (core.ByteLength{}) || got.Metadata.Headers.Values != nil {
					t.Fatalf("refused upload metadata = %+v, want exact zero", got.Metadata)
				}
			} else if got.Metadata.Attempts != 1 || got.Metadata.Status != noContent || got.Metadata.Headers.Values != nil {
				t.Fatalf("upload metadata = %+v, want one no-content attempt and absent headers", got.Metadata)
			}
		})
	}
}
