package exchange_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type uploadBenchmarkObservation struct {
	receiveErr error
	writeErr   error
	bytes      uint64
}

type downloadBenchmarkObservation struct {
	writeErr error
}

func BenchmarkUpload10MiBFileOverLoopback(b *testing.B) {
	tempDir := b.TempDir()
	sourcePath := filepath.Join(tempDir, "source.bin")
	writeDeterministicFile(b, sourcePath, testLargeTransferBytes)
	source, err := os.Open(sourcePath)
	if err != nil {
		b.Fatalf("os.Open(%q) setup error = %v, want nil", sourcePath, err)
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			b.Errorf("source.Close() error = %v, want nil", closeErr)
		}
	}()

	created := mustHTTPStatus(b, http.StatusCreated)
	serverPolicy := exchange.ServerStreamPolicy{
		RequestBodyLimit: mustByteCount(b, testLargeTransferBytes),
	}
	observed := make(chan uploadBenchmarkObservation, 1)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		received, receiveErr := exchange.ReceiveStream(
			exchange.StreamReceiveCall{
				Request:     request,
				Destination: io.Discard,
				Route: exchange.RouteSemantics{
					Method: exchange.MethodPut,
					Replay: exchange.ReplaySingleAttempt,
				},
				Policy:              serverPolicy,
				ExpectedContentType: core.HTTPMediaTypeOctetStream(),
			},
		)
		var writeErr error
		if receiveErr == nil {
			writeErr = exchange.WriteNoBody(
				exchange.NoBodyWriteCall{
					Writer: writer,
					Response: exchange.ServerNoBodyResponse{
						Status: created,
					},
				},
			)
		}
		observed <- uploadBenchmarkObservation{
			receiveErr: receiveErr,
			writeErr:   writeErr,
			bytes:      received.Bytes.Uint64(),
		}
	}))
	defer server.Close()

	client := mustExchangeClient(b, server.Client())
	target := mustEndpoint(b, server.URL)
	policy := singleAttemptStreamPolicy(b)
	b.ReportAllocs()
	b.SetBytes(testLargeTransferBytes)
	b.ResetTimer()
	b.StopTimer()

	for range b.N {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			b.Fatalf("source.Seek(0) error = %v, want nil", err)
		}
		b.StartTimer()
		got, gotErr := exchange.Upload(
			exchange.UploadCall{
				Context: context.Background(),
				Client:  client,
				Request: exchange.UploadRequest{
					Target: target,
					Source: source,
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					ContentLength:  mustByteLength(b, testLargeTransferBytes),
					ContentType:    core.HTTPMediaTypeOctetStream(),
					ExpectedStatus: created,
				},
				Policy: policy,
			},
		)
		b.StopTimer()
		serverGot := <-observed
		if gotErr != nil ||
			serverGot.receiveErr != nil ||
			serverGot.writeErr != nil {
			b.Fatalf(
				"upload/client receive/write errors = (%v, %v, %v), want (nil, nil, nil)",
				gotErr,
				serverGot.receiveErr,
				serverGot.writeErr,
			)
		}
		if got.Metadata.Bytes.Uint64() != testLargeTransferBytes ||
			serverGot.bytes != testLargeTransferBytes {
			b.Fatalf(
				"upload client/server bytes = (%d, %d), want (%d, %d)",
				got.Metadata.Bytes.Uint64(),
				serverGot.bytes,
				testLargeTransferBytes,
				testLargeTransferBytes,
			)
		}
	}
}

func BenchmarkDownload10MiBFileOverLoopback(b *testing.B) {
	tempDir := b.TempDir()
	sourcePath := filepath.Join(tempDir, "source.bin")
	writeDeterministicFile(b, sourcePath, testLargeTransferBytes)
	source, err := os.Open(sourcePath)
	if err != nil {
		b.Fatalf("os.Open(%q) setup error = %v, want nil", sourcePath, err)
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			b.Errorf("source.Close() error = %v, want nil", closeErr)
		}
	}()

	ok := mustHTTPStatus(b, http.StatusOK)
	observed := make(chan downloadBenchmarkObservation, 1)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		section := io.NewSectionReader(
			source,
			0,
			testLargeTransferBytes,
		)
		writeErr := exchange.WriteStream(
			exchange.StreamWriteCall{
				Context: request.Context(),
				Writer:  writer,
				Response: exchange.ServerStreamResponse{
					Source:        section,
					ContentLength: mustByteLength(b, testLargeTransferBytes),
					ContentType:   core.HTTPMediaTypeOctetStream(),
					Status:        ok,
				},
			},
		)
		observed <- downloadBenchmarkObservation{writeErr: writeErr}
	}))
	defer server.Close()

	client := mustExchangeClient(b, server.Client())
	target := mustEndpoint(b, server.URL)
	policy := singleAttemptStreamPolicy(b)
	responseLimit := mustByteCount(b, testLargeTransferBytes)
	b.ReportAllocs()
	b.SetBytes(testLargeTransferBytes)
	b.ResetTimer()
	b.StopTimer()

	for range b.N {
		b.StartTimer()
		got, gotErr := exchange.Download(
			exchange.DownloadCall{
				Context: context.Background(),
				Client:  client,
				Request: exchange.DownloadRequest{
					Target:      target,
					Destination: io.Discard,
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ResponseBodyLimit:           responseLimit,
					ExpectedStatus:              ok,
					ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
				},
				Policy: policy,
			},
		)
		b.StopTimer()
		serverGot := <-observed
		if gotErr != nil || serverGot.writeErr != nil {
			b.Fatalf(
				"download/client write errors = (%v, %v), want (nil, nil)",
				gotErr,
				serverGot.writeErr,
			)
		}
		if got.Metadata.Bytes.Uint64() != testLargeTransferBytes {
			b.Fatalf(
				"download client bytes = %d, want %d",
				got.Metadata.Bytes.Uint64(),
				testLargeTransferBytes,
			)
		}
	}
}
