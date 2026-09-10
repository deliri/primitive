package exchange

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"net/http"
	"testing"
)

type nilOwnerDoor uint8

const (
	nilOwnerUpload nilOwnerDoor = iota
	nilOwnerDownload
	nilOwnerRoundTrip
)

func TestStreamTypedNilOwnerIngressLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		door      nilOwnerDoor
		nilSource bool
	}{
		{name: "upload typed nil source", door: nilOwnerUpload, nilSource: true},
		{name: "download typed nil destination", door: nilOwnerDownload},
		{name: "round trip typed nil source", door: nilOwnerRoundTrip, nilSource: true},
		{name: "round trip typed nil destination", door: nilOwnerRoundTrip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(*http.Request) (*http.Response, error) { calls++; return nil, io.ErrClosedPipe })})
			if err != nil {
				t.Fatal(err)
			}
			target, err := core.ParseHTTPEndpoint("https://nil-owner.invalid/body")
			if err != nil {
				t.Fatal(err)
			}
			timeout := runtimeAgreementPolicy(t).ReadTimeout
			policy := StreamPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Redirect: RedirectPolicy{Mode: RedirectReject}}
			semantics := RequestSemantics{Method: MethodPost, Replay: ReplaySingleAttempt}
			var source io.Reader = bytes.NewReader(nil)
			var destination io.Writer = io.Discard
			if tc.nilSource {
				source = (*bytes.Reader)(nil)
			} else {
				destination = (*bytes.Buffer)(nil)
			}
			var validationErr, executionErr error
			var count uint64
			switch tc.door {
			case nilOwnerUpload:
				request := UploadRequest{Target: target, Source: source, Semantics: semantics, ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK()}
				validationErr = request.Validate()
				got, err := Upload(UploadCall{Context: t.Context(), Client: client, Request: request, Policy: policy})
				executionErr = err
				count = got.Metadata.Bytes.Uint64()
			case nilOwnerDownload:
				request := DownloadRequest{Target: target, Destination: destination, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK()}
				validationErr = request.Validate()
				got, err := Download(DownloadCall{Context: t.Context(), Client: client, Request: request, Policy: policy})
				executionErr = err
				count = got.Metadata.Bytes.Uint64()
			case nilOwnerRoundTrip:
				request := StreamRoundTripRequest{Target: target, Source: source, Destination: destination, Semantics: semantics, RequestContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK()}
				validationErr = request.Validate()
				got, err := RoundTripStream(StreamRoundTripCall{Context: t.Context(), Client: client, Request: request, Policy: policy})
				executionErr = err
				count = got.Metadata.Bytes.Uint64()
			default:
				t.Fatalf("test door = %d, want declared streaming operation", tc.door)
			}
			for _, err := range []error{validationErr, executionErr} {
				if !errors.Is(err, core.ErrExchangeRequest) || !errors.Is(err, core.ErrExchangeContract) || errors.Is(err, core.ErrExchangeTransport) || errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("ingress error=%v; want request contract without transport", err)
				}
			}
			if calls != 0 || count != 0 {
				t.Fatalf("refused owner calls/bytes=%d/%d", calls, count)
			}
		})
	}
}
