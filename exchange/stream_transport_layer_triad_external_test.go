package exchange_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const testLargeTransferBytes = 10 * 1024 * 1024

type uploadServerObservation struct {
	receiveErr error
	writeErr   error
	bytes      uint64
	digest     [sha256.Size]byte
}

type downloadServerObservation struct {
	writeErr error
}

func TestUploadTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive ten MiB file arrives with the source SHA256 and exact byte count", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		sourcePath := filepath.Join(tempDir, "source.bin")
		writeDeterministicFile(t, sourcePath, testLargeTransferBytes)
		wantDigest := sha256File(t, sourcePath)

		created := mustHTTPStatus(t, http.StatusCreated)
		serverPolicy := exchange.ServerStreamPolicy{
			RequestBodyLimit: mustByteCount(t, testLargeTransferBytes),
		}
		observed := make(chan uploadServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			destination := sha256.New()
			received, receiveErr := exchange.ReceiveStream(
				exchange.StreamReceiveCall{
					Request:     request,
					Destination: destination,
					Route: exchange.RouteSemantics{
						Method: core.HTTPMethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					Policy:              serverPolicy,
					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			observation := uploadServerObservation{
				receiveErr: receiveErr,
				bytes:      received.Bytes.Uint64(),
			}
			copy(observation.digest[:], destination.Sum(nil))
			if receiveErr == nil {
				observation.writeErr = exchange.WriteNoBody(
					exchange.NoBodyWriteCall{
						Writer: writer,
						Response: exchange.ServerNoBodyResponse{
							Status: created,
						},
					},
				)
			}
			observed <- observation
		}))
		defer server.Close()

		source, gotOpenErr := os.Open(sourcePath)
		if gotOpenErr != nil {
			t.Fatalf("os.Open(%q) error = %v, want nil", sourcePath, gotOpenErr)
		}
		got, gotErr := exchange.Upload(
			exchange.UploadCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.UploadRequest{
					Target: mustEndpoint(t, server.URL),
					Source: source,
					Semantics: exchange.RequestSemantics{
						Method: core.HTTPMethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					ContentLength:  core.NewByteLength(testLargeTransferBytes),
					ContentType:    core.HTTPMediaTypeOctetStream(),
					ExpectedStatus: created,
				},
				Policy: singleAttemptStreamPolicy(t),
			},
		)
		gotCloseErr := source.Close()
		if gotErr != nil || gotCloseErr != nil {
			t.Fatalf("exchange.Upload()/source.Close() errors = (%v, %v), want (nil, nil)", gotErr, gotCloseErr)
		}
		if got.Metadata.Bytes.Uint64() != testLargeTransferBytes ||
			got.Metadata.Attempts != 1 ||
			got.Metadata.Status != created {
			t.Fatalf(
				"exchange.Upload() metadata = %+v, want %d bytes, one attempt, status %v",
				got.Metadata,
				testLargeTransferBytes,
				created,
			)
		}
		select {
		case serverGot := <-observed:
			if serverGot.receiveErr != nil || serverGot.writeErr != nil {
				t.Fatalf(
					"server receive/write errors = (%v, %v), want (nil, nil)",
					serverGot.receiveErr,
					serverGot.writeErr,
				)
			}
			if serverGot.bytes != testLargeTransferBytes {
				t.Fatalf("server received bytes = %d, want %d", serverGot.bytes, testLargeTransferBytes)
			}
			if serverGot.digest != wantDigest {
				t.Fatalf("server SHA256 = %x, want source SHA256 %x", serverGot.digest, wantDigest)
			}
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"upload server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})

	t.Run("negative truncated source cannot satisfy declared HTTP extent", func(t *testing.T) {
		t.Parallel()

		created := mustHTTPStatus(t, http.StatusCreated)
		serverPolicy := exchange.ServerStreamPolicy{
			RequestBodyLimit: mustByteCount(t, testLargeTransferBytes),
		}
		observed := make(chan uploadServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			received, receiveErr := exchange.ReceiveStream(
				exchange.StreamReceiveCall{
					Request:     request,
					Destination: io.Discard,
					Route: exchange.RouteSemantics{
						Method: core.HTTPMethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					Policy:              serverPolicy,
					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			observed <- uploadServerObservation{
				receiveErr: receiveErr,
				bytes:      received.Bytes.Uint64(),
			}
			if receiveErr == nil {
				_ = exchange.WriteNoBody(exchange.NoBodyWriteCall{
					Writer: writer,
					Response: exchange.ServerNoBodyResponse{
						Status: created,
					},
				})
			}
		}))
		defer server.Close()

		got, gotErr := exchange.Upload(
			exchange.UploadCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.UploadRequest{
					Target: mustEndpoint(t, server.URL),
					Source: bytes.NewReader(
						make([]byte, testLargeTransferBytes-1),
					),
					Semantics: exchange.RequestSemantics{
						Method: core.HTTPMethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					ContentLength:  core.NewByteLength(testLargeTransferBytes),
					ContentType:    core.HTTPMediaTypeOctetStream(),
					ExpectedStatus: created,
				},
				Policy: singleAttemptStreamPolicy(t),
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeTransport) {
			t.Fatalf("exchange.Upload(truncated source) error = %v, want %v", gotErr, core.ErrExchangeTransport)
		}
		if got.Metadata.Attempts != 0 ||
			got.Metadata.Bytes.Uint64() != 0 ||
			len(got.Metadata.Headers.Values) != 0 {
			t.Fatalf("exchange.Upload(truncated source) response = %+v, want zero", got)
		}
		select {
		case serverGot := <-observed:
			if !errors.Is(serverGot.receiveErr, core.ErrExchangeRequest) {
				t.Fatalf("server truncated receive error = %v, want %v", serverGot.receiveErr, core.ErrExchangeRequest)
			}
			if serverGot.bytes >= testLargeTransferBytes {
				t.Fatalf("server truncated receive bytes = %d, want below %d", serverGot.bytes, testLargeTransferBytes)
			}
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"truncated upload server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})

	t.Run("neutral zero extent sends no bytes and does not consume the next segment", func(t *testing.T) {
		t.Parallel()

		created := mustHTTPStatus(t, http.StatusCreated)
		serverPolicy := exchange.ServerStreamPolicy{
			RequestBodyLimit: mustByteCount(t, 1),
		}
		observed := make(chan uploadServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			received, receiveErr := exchange.ReceiveStream(
				exchange.StreamReceiveCall{
					Request:     request,
					Destination: io.Discard,
					Route: exchange.RouteSemantics{
						Method: core.HTTPMethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					Policy:              serverPolicy,
					ExpectedContentType: core.HTTPMediaTypeOctetStream(),
				},
			)
			writeErr := exchange.WriteNoBody(exchange.NoBodyWriteCall{
				Writer: writer,
				Response: exchange.ServerNoBodyResponse{
					Status: created,
				},
			})
			observed <- uploadServerObservation{
				receiveErr: receiveErr,
				writeErr:   writeErr,
				bytes:      received.Bytes.Uint64(),
			}
		}))
		defer server.Close()

		source := bytes.NewReader([]byte{0x7f})
		got, gotErr := exchange.Upload(
			exchange.UploadCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.UploadRequest{
					Target: mustEndpoint(t, server.URL),
					Source: source,
					Semantics: exchange.RequestSemantics{
						Method: core.HTTPMethodPut,
						Replay: exchange.ReplaySingleAttempt,
					},
					ContentLength:  core.NewByteLength(0),
					ContentType:    core.HTTPMediaTypeOctetStream(),
					ExpectedStatus: created,
				},
				Policy: singleAttemptStreamPolicy(t),
			},
		)
		if gotErr != nil {
			t.Fatalf("exchange.Upload(zero extent) error = %v, want nil", gotErr)
		}
		if got.Metadata.Bytes.Uint64() != 0 || source.Len() != 1 {
			t.Fatalf(
				"zero upload bytes/source remaining = (%d, %d), want (0, 1)",
				got.Metadata.Bytes.Uint64(),
				source.Len(),
			)
		}
		select {
		case serverGot := <-observed:
			if serverGot.receiveErr != nil || serverGot.writeErr != nil ||
				serverGot.bytes != 0 {
				t.Fatalf(
					"zero upload server = (%d, %v, %v), want (0, nil, nil)",
					serverGot.bytes,
					serverGot.receiveErr,
					serverGot.writeErr,
				)
			}
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"zero upload server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})
}

// TestUploadMetadataIsValidatedNotStamped pins the ordering invariant: upload
// byte accounting belongs to the metadata that finishUploadResponse validates,
// so a rejected operation can never carry a transferred-byte claim that the
// returned value would fail to validate.
func TestUploadMetadataIsValidatedNotStamped(t *testing.T) {
	t.Parallel()

	t.Run("positive an unexpected status still returns coherent validated metadata", func(t *testing.T) {
		t.Parallel()

		const extent = 4 * exchange.TransferBufferBytes
		created := mustHTTPStatus(t, http.StatusCreated)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			_, _ = io.Copy(io.Discard, request.Body)
			writer.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		got, gotErr := exchange.Upload(exchange.UploadCall{
			Context: context.Background(),
			Client:  mustExchangeClient(t, server.Client()),
			Request: exchange.UploadRequest{
				Target: mustEndpoint(t, server.URL),
				Source: bytes.NewReader(bytes.Repeat([]byte{0x6e}, extent)),
				Semantics: exchange.RequestSemantics{
					Method: core.HTTPMethodPut,
					Replay: exchange.ReplaySingleAttempt,
				},
				ContentLength:  core.NewByteLength(extent),
				ContentType:    core.HTTPMediaTypeOctetStream(),
				ExpectedStatus: created,
			},
			Policy: singleAttemptStreamPolicy(t),
		})
		var statusErr exchange.StatusError
		if !errors.As(gotErr, &statusErr) || statusErr.Expected() != created {
			t.Fatalf("exchange.Upload(unexpected status) error = %v, want a status error", gotErr)
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf("returned exchange.StreamResponse.Validate() = %v, want nil", gotValidateErr)
		}
		if got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != extent {
			t.Fatalf(
				"unexpected-status metadata attempts/bytes = (%d, %d), want (1, %d)",
				got.Metadata.Attempts,
				got.Metadata.Bytes.Uint64(),
				extent,
			)
		}
	})

	t.Run("negative an invalid observed status reports no transferred bytes", func(t *testing.T) {
		t.Parallel()

		created := mustHTTPStatus(t, http.StatusCreated)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			_, _ = io.Copy(io.Discard, request.Body)
			writer.WriteHeader(600)
		}))
		defer server.Close()

		got, gotErr := exchange.Upload(exchange.UploadCall{
			Context: context.Background(),
			Client:  mustExchangeClient(t, server.Client()),
			Request: exchange.UploadRequest{
				Target: mustEndpoint(t, server.URL),
				Source: bytes.NewReader([]byte{0x01}),
				Semantics: exchange.RequestSemantics{
					Method: core.HTTPMethodPut,
					Replay: exchange.ReplaySingleAttempt,
				},
				ContentLength:  core.NewByteLength(1),
				ContentType:    core.HTTPMediaTypeOctetStream(),
				ExpectedStatus: created,
			},
			Policy: singleAttemptStreamPolicy(t),
		})
		if !errors.Is(gotErr, core.ErrExchangeResponse) {
			t.Fatalf(
				"exchange.Upload(invalid status) error = %v, want %v",
				gotErr,
				core.ErrExchangeResponse,
			)
		}
		if got.Metadata.Attempts != 0 || got.Metadata.Bytes.Uint64() != 0 ||
			len(got.Metadata.Headers.Values) != 0 {
			t.Fatalf("invalid-status upload response = %+v, want zero", got)
		}
	})

	t.Run("negative invalid captured metadata is rejected before response draining", func(t *testing.T) {
		t.Parallel()

		const errorBodyBytes = 4 * 1024
		created := mustHTTPStatus(t, http.StatusCreated)
		capturedName, gotNameErr := core.ParseHTTPHeaderName("X-Exchange-Captured")
		if gotNameErr != nil {
			t.Fatalf("ParseHTTPHeaderName() setup error = %v, want nil", gotNameErr)
		}
		policy := singleAttemptStreamPolicy(t)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			_, _ = io.Copy(io.Discard, request.Body)
			writer.Header().Set(
				capturedName.String(),
				string(bytes.Repeat(
					[]byte{'v'},
					exchange.HeaderValueMaximumBytes+1,
				)),
			)
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write(bytes.Repeat(
				[]byte{0x01},
				errorBodyBytes+1,
			))
		}))
		defer server.Close()

		got, gotErr := exchange.Upload(exchange.UploadCall{
			Context: context.Background(),
			Client:  mustExchangeClient(t, server.Client()),
			Request: exchange.UploadRequest{
				Target: mustEndpoint(t, server.URL),
				Source: bytes.NewReader([]byte{0x01}),
				Semantics: exchange.RequestSemantics{
					Method: core.HTTPMethodPut,
					Replay: exchange.ReplaySingleAttempt,
				},
				ContentLength: core.NewByteLength(1),
				ContentType:   core.HTTPMediaTypeOctetStream(),
				CaptureHeaders: exchange.HeaderSelection{
					Names: []core.HTTPHeaderName{capturedName},
				},
				ExpectedStatus: created,
			},
			Policy: policy,
		})
		if !errors.Is(gotErr, core.ErrExchangeResponse) {
			t.Fatalf(
				"exchange.Upload(invalid metadata) error = %v, want %v",
				gotErr,
				core.ErrExchangeResponse,
			)
		}
		if got.Metadata.Attempts != 0 ||
			got.Metadata.Bytes.Uint64() != 0 ||
			len(got.Metadata.Headers.Values) != 0 ||
			got.Metadata.Status != (core.HTTPStatusCode{}) {
			t.Fatalf("invalid-metadata upload response = %+v, want zero", got)
		}
	})
}

func TestDownloadTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive ten MiB file lands with the source SHA256 and exact byte count", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		sourcePath := filepath.Join(tempDir, "source.bin")
		destinationPath := filepath.Join(tempDir, "destination.bin")
		writeDeterministicFile(t, sourcePath, testLargeTransferBytes)
		wantDigest := sha256File(t, sourcePath)

		ok := mustHTTPStatus(t, http.StatusOK)
		observed := make(chan downloadServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			source, openErr := os.Open(sourcePath)
			if openErr != nil {
				observed <- downloadServerObservation{writeErr: openErr}
				return
			}
			section := io.NewSectionReader(
				source,
				0,
				testLargeTransferBytes,
			)
			writeErr := exchange.WriteStream(exchange.StreamWriteCall{
				Context: request.Context(),
				Writer:  writer,
				Response: exchange.ServerStreamResponse{
					Source:        section,
					ContentLength: core.NewByteLength(testLargeTransferBytes),
					ContentType:   core.HTTPMediaTypeOctetStream(),
					Status:        ok,
				},
			})
			closeErr := source.Close()
			observed <- downloadServerObservation{
				writeErr: errors.Join(writeErr, closeErr),
			}
		}))
		defer server.Close()

		destination, gotCreateErr := os.Create(destinationPath)
		if gotCreateErr != nil {
			t.Fatalf("os.Create(%q) error = %v, want nil", destinationPath, gotCreateErr)
		}
		got, gotErr := exchange.Download(exchange.DownloadCall{
			Context: context.Background(),
			Client:  mustExchangeClient(t, server.Client()),
			Request: exchange.DownloadRequest{
				Target:      mustEndpoint(t, server.URL),
				Destination: destination,
				Semantics: exchange.RequestSemantics{
					Method: core.HTTPMethodGet,
					Replay: exchange.ReplaySingleAttempt,
				},
				ResponseBodyLimit:           mustByteCount(t, testLargeTransferBytes),
				ExpectedStatus:              ok,
				ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
			},
			Policy: singleAttemptStreamPolicy(t),
		})
		gotCloseErr := destination.Close()
		var serverGot downloadServerObservation
		select {
		case serverGot = <-observed:
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"download server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
		if gotErr != nil || gotCloseErr != nil || serverGot.writeErr != nil {
			t.Fatalf(
				"exchange.Download()/destination.Close()/server WriteStream errors = (%v, %v, %v), want (nil, nil, nil)",
				gotErr,
				gotCloseErr,
				serverGot.writeErr,
			)
		}
		if got.Metadata.Bytes.Uint64() != testLargeTransferBytes ||
			got.Metadata.Attempts != 1 ||
			got.Metadata.Status != ok {
			t.Fatalf(
				"exchange.Download() metadata = %+v, want %d bytes, one attempt, status %v",
				got.Metadata,
				testLargeTransferBytes,
				ok,
			)
		}
		gotDigest := sha256File(t, destinationPath)
		if gotDigest != wantDigest {
			t.Fatalf("downloaded SHA256 = %x, want source SHA256 %x", gotDigest, wantDigest)
		}
	})

	t.Run("negative chunked one-over response writes only the bound and rejects the excess byte", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		limit := uint64(exchange.TransferBufferBytes)
		body := bytes.Repeat([]byte{0xa5}, int(limit+1))
		observed := make(chan downloadServerObservation, 1)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				core.HTTPMediaTypeOctetStream().String(),
			)
			writer.WriteHeader(http.StatusOK)
			if flusher, available := writer.(http.Flusher); available {
				flusher.Flush()
			}
			_, writeErr := writer.Write(body)
			observed <- downloadServerObservation{writeErr: writeErr}
		}))
		defer server.Close()

		destination := bytes.NewBuffer(nil)
		got, gotErr := exchange.Download(exchange.DownloadCall{
			Context: context.Background(),
			Client:  mustExchangeClient(t, server.Client()),
			Request: exchange.DownloadRequest{
				Target:      mustEndpoint(t, server.URL),
				Destination: destination,
				Semantics: exchange.RequestSemantics{
					Method: core.HTTPMethodGet,
					Replay: exchange.ReplaySingleAttempt,
				},
				ResponseBodyLimit:           mustByteCount(t, limit),
				ExpectedStatus:              ok,
				ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
			},
			Policy: singleAttemptStreamPolicy(t),
		})
		if !errors.Is(gotErr, core.ErrExchangeResponse) ||
			!errors.Is(gotErr, core.ErrExchangeBodyLimit) {
			t.Fatalf(
				"exchange.Download(one over) error = %v, want %v and %v",
				gotErr,
				core.ErrExchangeResponse,
				core.ErrExchangeBodyLimit,
			)
		}
		if got.Metadata.Bytes.Uint64() != limit {
			t.Fatalf("exchange.Download(one over) bytes = %d, want %d", got.Metadata.Bytes.Uint64(), limit)
		}
		if uint64(destination.Len()) != limit ||
			!bytes.Equal(destination.Bytes(), body[:limit]) {
			t.Fatalf(
				"one-over destination bytes/prefix = (%d, %t), want (%d, true)",
				destination.Len(),
				bytes.Equal(destination.Bytes(), body[:limit]),
				limit,
			)
		}
		select {
		case serverGot := <-observed:
			if serverGot.writeErr != nil {
				t.Fatalf("one-over server write error = %v, want nil", serverGot.writeErr)
			}
		case <-time.After(testDeadlockBackstop):
			t.Fatalf(
				"one-over download server observation = absent after %v, want one completed observation",
				testDeadlockBackstop,
			)
		}
	})

	t.Run("neutral zero-byte response leaves destination unchanged", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			_ = exchange.WriteStream(exchange.StreamWriteCall{
				Context: request.Context(),
				Writer:  writer,
				Response: exchange.ServerStreamResponse{
					Source:        bytes.NewReader(nil),
					ContentLength: core.NewByteLength(0),
					ContentType:   core.HTTPMediaTypeOctetStream(),
					Status:        ok,
				},
			})
		}))
		defer server.Close()

		destination := bytes.NewBufferString("preserved")
		got, gotErr := exchange.Download(exchange.DownloadCall{
			Context: context.Background(),
			Client:  mustExchangeClient(t, server.Client()),
			Request: exchange.DownloadRequest{
				Target:      mustEndpoint(t, server.URL),
				Destination: destination,
				Semantics: exchange.RequestSemantics{
					Method: core.HTTPMethodGet,
					Replay: exchange.ReplaySingleAttempt,
				},
				ResponseBodyLimit:           mustByteCount(t, 1),
				ExpectedStatus:              ok,
				ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(),
			},
			Policy: singleAttemptStreamPolicy(t),
		})
		if gotErr != nil {
			t.Fatalf("exchange.Download(zero response) error = %v, want nil", gotErr)
		}
		if got.Metadata.Bytes.Uint64() != 0 || destination.String() != "preserved" {
			t.Fatalf(
				"zero download bytes/destination = (%d, %q), want (0, %q)",
				got.Metadata.Bytes.Uint64(),
				destination.String(),
				"preserved",
			)
		}
	})
}

func writeDeterministicFile(t testing.TB, path string, size uint64) {
	t.Helper()

	file, gotCreateErr := os.Create(path)
	if gotCreateErr != nil {
		t.Fatalf("os.Create(%q) setup error = %v, want nil", path, gotCreateErr)
	}
	var pattern [exchange.TransferBufferBytes]byte
	for index := range pattern {
		pattern[index] = byte(index*31 + 17)
	}
	remaining := size
	var gotWriteErr error
	for remaining > 0 {
		chunk := min(uint64(len(pattern)), remaining)
		var written int
		written, gotWriteErr = file.Write(pattern[:chunk])
		if gotWriteErr != nil || written != int(chunk) {
			break
		}
		remaining -= chunk
	}
	gotCloseErr := file.Close()
	if gotWriteErr != nil || gotCloseErr != nil || remaining != 0 {
		t.Fatalf(
			"deterministic fixture write = (remaining %d, write %v, close %v), want (0, nil, nil)",
			remaining,
			gotWriteErr,
			gotCloseErr,
		)
	}
}

func sha256File(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()

	file, gotOpenErr := os.Open(path)
	if gotOpenErr != nil {
		t.Fatalf("os.Open(%q) digest setup error = %v, want nil", path, gotOpenErr)
	}
	digest := sha256.New()
	gotBytes, gotCopyErr := io.Copy(digest, file)
	gotCloseErr := file.Close()
	if gotCopyErr != nil || gotCloseErr != nil {
		t.Fatalf("SHA256 read/close errors = (%v, %v), want (nil, nil)", gotCopyErr, gotCloseErr)
	}
	if gotBytes < 0 {
		t.Fatalf("SHA256 read bytes = %d, want non-negative", gotBytes)
	}
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}
