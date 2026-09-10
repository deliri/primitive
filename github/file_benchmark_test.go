package github

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkReadFileCaptured1000000Bytes(b *testing.B) {
	const size = 1000000
	content := bytes.Repeat([]byte{0xa5}, size)
	wantDigest := core.SHA256Of(content)
	path := parsedPath(b, "source/data.bin")
	var calls atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set(core.HTTPHeaderContentType().String(), core.GitHubRawContentMediaType)
		if _, err := w.Write(content); err != nil {
			b.Errorf("provider Write=%v, want nil", err)
		}
	}))
	b.Cleanup(server.Close)
	client := clientFixture(b, server.URL)
	request := FileRequest{Repository: parsedRepository(b, "owner/repository"), Commit: parsedCommit(b), Path: path}
	b.ReportAllocs()
	b.SetBytes(size)
	var got FileObservation
	var captured *bytes.Buffer
	for b.Loop() {
		captured = new(bytes.Buffer)
		request.Destination = captured
		var err error
		got, err = client.ReadFile(b.Context(), request)
		if err != nil || got.Length.Uint64() != size || got.SHA256 != wantDigest {
			b.Fatalf("file=(%+v,%v), want exact %d-byte capture", got, err, size)
		}
	}
	if !bytes.Equal(captured.Bytes(), content) || calls.Load() != uint64(b.N) {
		b.Fatalf("captured bytes/calls=(%d,%d), want (%d,%d)", len(captured.Bytes()), calls.Load(), size, b.N)
	}
}

func BenchmarkReadFileStreaming(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name string
		size uint64
	}{
		{name: "32769_bytes", size: 32769},
		{name: "1000000_bytes", size: 1000000},
		{name: "16777217_bytes", size: (16 << 20) + 1},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			want := fileExpectedDigest(b, tc.size)
			var calls atomic.Uint64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set(core.HTTPHeaderContentType().String(), core.GitHubRawContentMediaType)
				w.Header().Set(core.HTTPHeaderContentLength().String(), strconv.FormatUint(tc.size, 10))
				n, err := io.Copy(w, &archiveExtentReader{remaining: tc.size})
				if err != nil || uint64(n) != tc.size {
					b.Errorf("provider count/error=%d/%v, want %d/nil", n, err, tc.size)
				}
			}))
			b.Cleanup(server.Close)
			client := clientFixture(b, server.URL)
			request := FileRequest{Repository: parsedRepository(b, "owner/repository"), Commit: parsedCommit(b), Path: parsedPath(b, "source/data.bin"), Destination: io.Discard, Buffer: make([]byte, 32768)}
			b.ReportAllocs()
			b.SetBytes(int64(tc.size))
			for b.Loop() {
				got, err := client.ReadFile(b.Context(), request)
				if err != nil || got.Length.Uint64() != tc.size || got.SHA256 != want {
					b.Fatalf("file=%+v/%v, want %d-byte complete digest", got, err, tc.size)
				}
			}
			if calls.Load() != uint64(b.N) {
				b.Fatalf("calls=%d, want %d", calls.Load(), b.N)
			}
		})
	}
}
