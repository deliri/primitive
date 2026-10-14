package exchange_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type multipartPatternSource struct {
	left int64
	read int64
}

func (s *multipartPatternSource) Read(data []byte) (int, error) {
	if s.left == 0 {
		return 0, io.EOF
	}
	if int64(len(data)) > s.left {
		data = data[:s.left]
	}
	for index := range data {
		data[index] = byte((s.read + int64(index)) % 251)
	}
	s.left -= int64(len(data))
	s.read += int64(len(data))
	return len(data), nil
}

type multipartPatternSink struct{ written int64 }

func (s *multipartPatternSink) Write(data []byte) (int, error) {
	for index, value := range data {
		if value != byte((s.written+int64(index))%251) {
			return 0, core.ErrExchangeContract
		}
	}
	s.written += int64(len(data))
	return len(data), nil
}

type multipartTrackedFile struct {
	file    *os.File
	closed  atomic.Int64
	maximum atomic.Int64
}

func (s *multipartTrackedFile) Read(data []byte) (int, error) {
	for old := s.maximum.Load(); int64(len(data)) > old && !s.maximum.CompareAndSwap(old, int64(len(data))); old = s.maximum.Load() {
	}
	return s.file.Read(data)
}
func (s *multipartTrackedFile) Close() error { s.closed.Add(1); return s.file.Close() }

func multipartFieldName(t testing.TB, value string) exchange.MultipartName {
	t.Helper()
	got, err := exchange.ParseMultipartName(value)
	if err != nil {
		t.Fatalf("field name = %v, want admitted", err)
	}
	return got
}
func multipartFilename(t testing.TB, value string) exchange.MultipartFileName {
	t.Helper()
	got, err := exchange.ParseMultipartFileName(value)
	if err != nil {
		t.Fatalf("filename = %v, want admitted", err)
	}
	return got
}
func multipartFieldText(t testing.TB, value string) exchange.MultipartText {
	t.Helper()
	got, err := exchange.ParseMultipartText(value)
	if err != nil {
		t.Fatalf("field data = %v, want admitted", err)
	}
	return got
}

func TestMultipartNativeHTTPModelAndFileStreamsConserveExactBytes(t *testing.T) {
	t.Parallel()
	for _, extent := range []int64{0, 32767, 32768, 32769, 8*32768 + 7, 4*1024*1024 + 1} {
		t.Run(fmt.Sprintf("native-upload-byte-extent-%d", extent), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			file, err := os.OpenFile(filepath.Join(directory, "workout.wav"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
			if err != nil {
				t.Fatalf("native file acquire = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
					t.Errorf("native fixture cleanup = %v, want nil/already closed", err)
				}
			})
			if count, err := io.CopyBuffer(file, &multipartPatternSource{left: extent}, make([]byte, 32768)); err != nil || count != extent {
				t.Fatalf("native file fixture = (%d,%v), want exact%d", count, err, extent)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatalf("native file rewind = %v, want nil", err)
			}
			owned := &multipartTrackedFile{file: file}
			observed := make(chan error, 1)
			fixture := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
				if err != nil {
					observed <- err
					writer.WriteHeader(400)
					return
				}
				decoder := multipart.NewReader(request.Body, parameters["boundary"])
				model, err := decoder.NextPart()
				if err != nil {
					observed <- err
					writer.WriteHeader(400)
					return
				}
				text, err := io.ReadAll(model)
				if err != nil || model.FormName() != "model" || model.FileName() != "" || string(text) != "gpt-4o-mini-transcribe" {
					observed <- errors.Join(core.ErrExchangeContract, err)
					writer.WriteHeader(400)
					return
				}
				upload, err := decoder.NextPart()
				if err != nil {
					observed <- err
					writer.WriteHeader(400)
					return
				}
				sink := &multipartPatternSink{}
				count, err := io.CopyBuffer(sink, upload, make([]byte, 32768))
				if err != nil || count != extent || sink.written != extent || upload.FormName() != "file" || upload.FileName() != "workout.wav" || upload.Header.Get("Content-Type") != core.HTTPMediaTypeOctetStream().String() {
					observed <- errors.Join(core.ErrExchangeContract, err)
					writer.WriteHeader(400)
					return
				}
				if extra, err := decoder.NextPart(); extra != nil || err != io.EOF {
					observed <- errors.Join(core.ErrExchangeContract, err)
					writer.WriteHeader(400)
					return
				}
				// Reading the HTTP body to EOF joins its chunked request producer rather
				// than treating MIME's terminal boundary as proof of native body EOF.
				_, err = io.Copy(io.Discard, request.Body)
				observed <- err
				writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			}))
			defer fixture.Close()
			client, err := exchange.NewClient(fixture.Client())
			if err != nil {
				t.Fatalf("native client = %v, want nil", err)
			}
			target, err := core.ParseHTTPEndpoint(fixture.URL)
			if err != nil {
				t.Fatalf("native target = %v, want nil", err)
			}
			modelName, fileName := multipartFieldName(t, "model"), multipartFieldName(t, "file")
			filename, text := multipartFilename(t, "workout.wav"), multipartFieldText(t, "gpt-4o-mini-transcribe")
			var captured io.Reader
			var response exchange.StreamRoundTripResponse
			err = exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{
				Parts: func(yield func(exchange.MultipartPart) bool) {
					if !yield(exchange.MultipartPart{Kind: exchange.MultipartPartField, Name: modelName, Text: text}) {
						return
					}
					yield(exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: fileName, FileName: filename, Source: owned})
				},
				Use: func(ctx context.Context, source exchange.MultipartSource) error {
					if err := source.Validate(); err != nil {
						return err
					}
					if _, closes := source.Reader.(io.Closer); closes {
						return core.ErrExchangeContract
					}
					captured = source.Reader
					var err error
					response, err = exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: client, Policy: exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}, Request: exchange.StreamRoundTripRequest{Target: target, Source: source.Reader, Destination: io.Discard, Buffer: make([]byte, 32768), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, RequestContentType: source.ContentType, ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}})
					return err
				},
			})
			if serverErr := <-observed; err != nil || serverErr != nil || response.Metadata.Attempts != 1 || owned.closed.Load() != 1 || owned.maximum.Load() > int64(exchange.TransferBufferBytes) {
				t.Fatalf("multipart native upload = (%v,server%v,attempts%d,closes%d,maxread%d), want exact stream/one attempt/one source close/fixed window", err, serverErr, response.Metadata.Attempts, owned.closed.Load(), owned.maximum.Load())
			}
			var probe [1]byte
			if count, err := captured.Read(probe[:]); count != 0 || err == nil {
				t.Fatalf("expired source read = (%d,%v), want closed native scope", count, err)
			}
			if _, err := file.Read(probe[:]); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("admitted file after scope = %v, want native closed identity", err)
			}
		})
	}
}

func TestMultipartNativeRepeatedPartsContinueWithoutRetainingAWorld(t *testing.T) {
	t.Parallel()
	for _, count := range []int{0, 1, 32767, 32768, 32769} {
		t.Run(fmt.Sprintf("native-part-cardinality-%d", count), func(t *testing.T) {
			t.Parallel()
			name, text := multipartFieldName(t, "repeat-\"quoted\\name"), multipartFieldText(t, "data\x00\r\nUnicode-雪")
			produced, consumed := 0, 0
			err := exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{
				Parts: func(yield func(exchange.MultipartPart) bool) {
					for range count {
						produced++
						if !yield(exchange.MultipartPart{Kind: exchange.MultipartPartField, Name: name, Text: text}) {
							return
						}
					}
				},
				Use: func(ctx context.Context, source exchange.MultipartSource) error {
					_, parameters, err := mime.ParseMediaType(source.ContentType.String())
					if err != nil {
						return err
					}
					decoder := multipart.NewReader(source.Reader, parameters["boundary"])
					for {
						part, err := decoder.NextPart()
						if err == io.EOF {
							break
						}
						if err != nil {
							return err
						}
						data, err := io.ReadAll(part)
						if err != nil {
							return err
						}
						if part.FormName() != name.String() || part.FileName() != "" || string(data) != text.String() {
							return core.ErrExchangeContract
						}
						consumed++
					}
					_, err = io.Copy(io.Discard, source.Reader)
					return err
				},
			})
			if err != nil || produced != count || consumed != count {
				t.Fatalf("native repeated parts = (%d produced,%d consumed,%v), want exact%d with no cardinality quota", produced, consumed, err, count)
			}
		})
	}
}
