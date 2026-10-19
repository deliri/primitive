package objectstore_test

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

func BenchmarkDownloadStreaming1KiB(b *testing.B) {
	benchmarkDownloadStreaming(b, 1024)
}

func BenchmarkDownloadStreaming10MiB(b *testing.B) {
	benchmarkDownloadStreaming(b, 10*1024*1024)
}

// benchmarkDownloadStreaming measures one complete verified GCS download from
// an in-memory provider response into a discarding destination. Response
// construction is part of each round trip; no socket or TLS work is measured.
func benchmarkDownloadStreaming(b *testing.B, size int) {
	payload := bytes.Repeat([]byte("d"), size)
	client := newObjectstoreClient(b, &http.Client{Transport: benchmarkPayloadTransport{payload: payload}})
	destination := &benchmarkCountingDestination{}
	request := downloadRequest(
		b,
		objectstore.ProviderGoogleCloudStorage,
		signedProviderURL(
			"https://storage.googleapis.com/bucket/object",
			objectstore.ProviderGoogleCloudStorage,
			objectstore.DirectionDownload,
		),
		destination,
		payload,
	)
	if request.Integrity.Length.Uint64() != uint64(size) {
		b.Fatalf("fixture length=%d, want %d before timing", request.Integrity.Length.Uint64(), size)
	}
	b.ReportAllocs()
	b.SetBytes(int64(size))
	for b.Loop() {
		destination.bytes = 0
		got, gotErr := objectstore.DownloadGCS(b.Context(), client, request)
		if gotErr != nil || got.Commitment() != objectstore.CommitmentConfirmed ||
			got.Bytes() != request.Integrity.Length || got.SHA256() != request.Integrity.SHA256 ||
			destination.bytes != size {
			b.Fatalf("DownloadGCS() = (commitment %v, bytes %d, %v), want confirmed %d bytes", got.Commitment(), destination.bytes, gotErr, size)
		}
	}
}

type benchmarkCountingDestination struct{ bytes int }

func (d *benchmarkCountingDestination) Write(data []byte) (int, error) {
	d.bytes += len(data)
	return len(data), nil
}

type benchmarkPayloadTransport struct{ payload []byte }

func (t benchmarkPayloadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body != nil {
		if _, err := io.Copy(io.Discard, request.Body); err != nil {
			return nil, err
		}
		if err := request.Body.Close(); err != nil {
			return nil, err
		}
	}
	headers := make(http.Header, 3)
	headers.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
	headers.Set(core.HTTPHeaderContentLength().String(), strconv.Itoa(len(t.payload)))
	headers.Set("X-Goog-Generation", "1")
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        headers,
		Body:          io.NopCloser(bytes.NewReader(t.payload)),
		ContentLength: int64(len(t.payload)),
		Request:       request,
	}, nil
}
