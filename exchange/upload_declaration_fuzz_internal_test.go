package exchange

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const uploadDeclarationFuzzMaximumBytes = 1 << 13

type declarationFuzzSource struct {
	reader *bytes.Reader
	closes int
}

func (s *declarationFuzzSource) Read(p []byte) (int, error) { return s.reader.Read(p) }
func (s *declarationFuzzSource) Close() error               { s.closes++; return nil }

// Both public upload doors admit hostile provider responses. Source reads are
// independent observed fixture facts; declarations must not impersonate them.
func FuzzUploadResponseDeclarationCustody(f *testing.F) {
	var binarySeed []byte
	for _, payload := range [][]byte{nil, {0}, {0, 0xff}, bytes.Repeat([]byte{0xff}, uploadDeclarationFuzzMaximumBytes)} {
		recorder := httptest.NewRecorder()
		call, err := NewSocketServerCall(recorder, httptest.NewRequest(http.MethodGet, "https://provider.example.test/seed", nil))
		if err != nil {
			f.Fatalf("typed seed call = %v, want nil", err)
		}
		err = WriteBounded(BoundedWriteCall{Call: call, Response: ServerBoundedResponse{Body: payload, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}})
		if err != nil {
			f.Fatalf("typed seed writer = %v, want nil", err)
		}
		f.Add(bytes.Clone(recorder.Body.Bytes()), uint16(0), false, false)
		if len(payload) > 0 {
			f.Add(bytes.Clone(recorder.Body.Bytes()), uint16(len(payload)), false, false)
		}
		if len(payload) == 2 {
			binarySeed = bytes.Clone(recorder.Body.Bytes())
		}
	}
	f.Add(binarySeed, uint16(1), false, false)
	f.Add(binarySeed, uint16(0), true, false)
	f.Add(binarySeed, uint16(1), false, true)
	f.Fuzz(func(t *testing.T, payload []byte, consume uint16, refuseStatus, transportFailure bool) {
		if len(payload) > uploadDeclarationFuzzMaximumBytes || int(consume) > len(payload) {
			return
		}
		length, err := core.NewByteLength(uint64(len(payload)))
		if err != nil {
			t.Fatal(err)
		}
		limit, err := core.NewByteCount(2)
		if err != nil {
			t.Fatal(err)
		}
		target, err := core.ParseHTTPEndpoint("https://provider.example.test/declaration")
		if err != nil {
			t.Fatal(err)
		}
		policy := StreamPolicy{OperationTimeout: runtimeAgreementPolicy(t).ReadTimeout, AttemptTimeout: runtimeAgreementPolicy(t).ReadTimeout, ErrorBodyLimit: limit, Redirect: RedirectPolicy{Mode: RedirectReject}}
		status := core.HTTPStatusOK()
		if refuseStatus {
			if err := status.AdmitInt(http.StatusServiceUnavailable); err != nil {
				t.Fatal(err)
			}
		}
		for _, roundTrip := range []bool{false, true} {
			source := &declarationFuzzSource{reader: bytes.NewReader(payload)}
			responseBody := &replayHandoffBody{reader: bytes.NewReader([]byte{0xff, 0})}
			calls := 0
			client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				n, err := io.CopyN(io.Discard, request.Body, int64(consume))
				closeErr := request.Body.Close()
				if err != nil || closeErr != nil || n != int64(consume) {
					return nil, errors.Join(core.ErrExchangeRequest, err, closeErr)
				}
				if transportFailure {
					return nil, io.ErrClosedPipe
				}
				code, err := status.Int()
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: code, Body: responseBody, ContentLength: 2, Request: request}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			request := UploadRequest{Target: target, Source: source, ContentLength: length, ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK(), Semantics: RequestSemantics{Method: MethodPut, Replay: ReplaySingleAttempt}}
			var got StreamResponse
			var destination bytes.Buffer
			if roundTrip {
				response, callErr := RoundTripStream(StreamRoundTripCall{Context: t.Context(), Client: client, Policy: policy, Request: StreamRoundTripRequest{Target: target, Source: source, Destination: &destination, RequestContentLength: length, RequestContentType: request.ContentType, ExpectedStatus: request.ExpectedStatus, Semantics: request.Semantics, ResponseBodyLimit: limit}})
				got = StreamResponse(response)
				err = callErr
			} else {
				got, err = Upload(UploadCall{Context: t.Context(), Client: client, Policy: policy, Request: request})
			}
			if calls != 1 || source.reader.Len() != len(payload)-int(consume) || source.closes != 0 {
				t.Fatalf("door roundTrip=%t provider/source remaining/close = %d/%d/%d, want 1/%d/0", roundTrip, calls, source.reader.Len(), source.closes, len(payload)-int(consume))
			}
			if transportFailure {
				if !errors.Is(err, core.ErrExchangeTransport) || !errors.Is(err, io.ErrClosedPipe) || got.Metadata.Attempts != 0 || got.Metadata.Status != (core.HTTPStatusCode{}) || got.Metadata.Bytes != (core.ByteLength{}) || got.DeclaredRequestBytes != (core.ByteLength{}) || got.Metadata.Headers.Values != nil || destination.Len() != 0 || responseBody.closes != 0 {
					t.Fatalf("transport refusal = (%+v,%v), want zero response and native cause", got, err)
				}
				continue
			}
			if (err == nil) == refuseStatus || refuseStatus && !errors.Is(err, core.ErrExchangeResponse) {
				t.Fatalf("provider status error = %v, want refusal=%t", err, refuseStatus)
			}
			if got.DeclaredRequestBytes != length || got.Metadata.Status != status || got.Metadata.Attempts != 1 || got.Metadata.Headers.Values != nil {
				t.Fatalf("response = %+v, want declaration %v, status %v, one attempt, absent headers", got, length, status)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("response validation = %v, want nil", err)
			}
			var wantBody []byte
			if roundTrip && !refuseStatus {
				wantBody = []byte{0xff, 0}
			}
			if !bytes.Equal(destination.Bytes(), wantBody) || got.Metadata.Bytes.Uint64() != uint64(len(wantBody)) || responseBody.readBytes != 2 || responseBody.closes != 1 {
				t.Fatalf("response byte custody = %x/%d read %d close %d, want %x/%d read 2 close 1", destination.Bytes(), got.Metadata.Bytes.Uint64(), responseBody.readBytes, responseBody.closes, wantBody, len(wantBody))
			}
		}
	})
}
