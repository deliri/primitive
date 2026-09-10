package exchange_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
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
	source, err := openExchangeFixtureFile(b, sourcePath)
	if err != nil {
		b.Fatalf("Filestore fixture open(%q) setup error = %v, want nil", sourcePath, err)
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			b.Errorf("source.Close() error = %v, want nil", closeErr)
		}
	}()

	created := mustHTTPStatus(b, http.StatusCreated)
	observed := make(chan uploadBenchmarkObservation, 1)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		serverCall := socketServerCallFrom(b, writer, request)
		received, receiveErr := exchange.ReceiveStream(
			exchange.StreamReceiveCall{
				Call:        serverCall,
				Destination: io.Discard,
				Route: exchange.RouteSemantics{
					Method: exchange.MethodPut,
					Replay: exchange.ReplaySingleAttempt,
				},

				ExpectedContentType: core.HTTPMediaTypeOctetStream(),
			},
		)
		var writeErr error
		if receiveErr == nil {
			writeErr = exchange.WriteNoBody(
				exchange.NoBodyWriteCall{
					Call: serverCall,
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
	backstopDuration, err := temporal.NewDuration(testDeadlockBackstop)
	if err != nil {
		b.Fatal(err)
	}
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
				Context: b.Context(),
				Client:  client,
				Request: exchange.UploadRequest{
					Target: target,
					Source: source,
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					ContentLength:  new(mustByteLength(b, testLargeTransferBytes)),
					ContentType:    core.HTTPMediaTypeOctetStream(),
					ExpectedStatus: created,
				},
				Policy: policy,
			},
		)
		b.StopTimer()
		if gotErr != nil {
			b.Fatalf("client transfer failed before handler observation: %v", gotErr)
		}
		backstop, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: b.Context(), Duration: backstopDuration})
		if err != nil {
			b.Fatal(err)
		}
		var serverGot uploadBenchmarkObservation
		select {
		case serverGot = <-observed:
			cancel()
		case <-backstop.Done():
			cancel()
			b.Fatalf("handler observation backstop error=%v, want a completed observation", backstop.Err())
		}
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
		if got.DeclaredRequestBytes.Uint64() != testLargeTransferBytes ||
			serverGot.bytes != testLargeTransferBytes {
			b.Fatalf(
				"upload client/server bytes = (%d, %d), want (%d, %d)",
				got.DeclaredRequestBytes.Uint64(),
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
	source, err := openExchangeFixtureFile(b, sourcePath)
	if err != nil {
		b.Fatalf("Filestore fixture open(%q) setup error = %v, want nil", sourcePath, err)
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
		serverCall := socketServerCallFrom(b, writer, request)
		section := io.NewSectionReader(
			source,
			0,
			testLargeTransferBytes,
		)
		writeErr := exchange.WriteStream(
			exchange.StreamWriteCall{
				Call: serverCall,
				Response: exchange.ServerStreamResponse{
					Source:        section,
					ContentLength: new(mustByteLength(b, testLargeTransferBytes)),
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
	backstopDuration, err := temporal.NewDuration(testDeadlockBackstop)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(testLargeTransferBytes)
	b.ResetTimer()
	b.StopTimer()

	for range b.N {
		b.StartTimer()
		got, gotErr := exchange.Download(
			exchange.DownloadCall{
				Context: b.Context(),
				Client:  client,
				Request: exchange.DownloadRequest{
					Target:      target,
					Destination: io.Discard,
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},

					ExpectedStatus:              ok,
					ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
				},
				Policy: policy,
			},
		)
		b.StopTimer()
		if gotErr != nil {
			b.Fatalf("client transfer failed before handler observation: %v", gotErr)
		}
		backstop, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: b.Context(), Duration: backstopDuration})
		if err != nil {
			b.Fatal(err)
		}
		var serverGot downloadBenchmarkObservation
		select {
		case serverGot = <-observed:
			cancel()
		case <-backstop.Done():
			cancel()
			b.Fatalf("handler observation backstop error=%v, want a completed observation", backstop.Err())
		}
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
