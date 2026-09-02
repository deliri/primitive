package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestProjectedJSONReceiveLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive request state completes the private decoded structure before validation", func(t *testing.T) {
		t.Parallel()

		request := projectedJSONRequest(
			t,
			[]byte(`{"message":"candidate"}`),
		)
		got, gotErr := exchange.ReceiveProjectedJSON[
			projectedTransportDocument,
			*projectedTransportDocument,
		](exchange.ProjectedJSONReceiveCall[
			projectedTransportDocument,
			*projectedTransportDocument,
		]{
			Call: socketServerCall(t, request),
			Project: func(
				_ context.Context,
				gotCall exchange.SocketServerCall,
				body *projectedTransportDocument,
			) error {
				if err := gotCall.Validate(); err != nil {
					return core.ErrExchangeContract
				}
				body.Method = exchange.MethodPost
				return nil
			},
			Policy: exchange.ServerPolicy{
				RequestBodyLimit: mustByteCount(t, 4*1024),
			},
			Route: exchange.RouteSemantics{
				Method: exchange.MethodPost,
				Replay: exchange.ReplaySingleAttempt,
			},
		})
		if gotErr != nil {
			t.Fatalf("ReceiveProjectedJSON() error = %v, want nil", gotErr)
		}
		if got.Body == nil ||
			got.Body.Message != "candidate" ||
			got.Body.Method != exchange.MethodPost {
			t.Fatalf(
				"ReceiveProjectedJSON() body = %+v, want candidate with %v",
				got.Body,
				exchange.MethodPost,
			)
		}
	})

	t.Run("negative projector failure returns no partially completed body", func(t *testing.T) {
		t.Parallel()

		request := projectedJSONRequest(
			t,
			[]byte(`{"message":"candidate"}`),
		)
		got, gotErr := exchange.ReceiveProjectedJSON[
			projectedTransportDocument,
			*projectedTransportDocument,
		](exchange.ProjectedJSONReceiveCall[
			projectedTransportDocument,
			*projectedTransportDocument,
		]{
			Call: socketServerCall(t, request),
			Project: func(
				context.Context,
				exchange.SocketServerCall,
				*projectedTransportDocument,
			) error {
				return core.ErrPrimitiveContract
			},
			Policy: exchange.ServerPolicy{
				RequestBodyLimit: mustByteCount(t, 4*1024),
			},
			Route: exchange.RouteSemantics{
				Method: exchange.MethodPost,
				Replay: exchange.ReplaySingleAttempt,
			},
		})
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf(
				"ReceiveProjectedJSON(projector failure) error = %v, want %v and %v",
				gotErr,
				core.ErrExchangeRequest,
				core.ErrPrimitiveContract,
			)
		}
		if got.Body != nil || !got.IdempotencyKey.IsZero() {
			t.Fatalf(
				"ReceiveProjectedJSON(projector failure) = %+v, want zero",
				got,
			)
		}
	})

	t.Run("negative call validation still closes the real request body", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "request.json")
		if gotErr := os.WriteFile(
			bodyPath,
			[]byte(`{"message":"candidate"}`),
			0o600,
		); gotErr != nil {
			t.Fatalf("os.WriteFile(%q) setup error = %v, want nil", bodyPath, gotErr)
		}
		body, gotErr := os.Open(bodyPath)
		if gotErr != nil {
			t.Fatalf("os.Open(%q) setup error = %v, want nil", bodyPath, gotErr)
		}
		target := mustEndpoint(t, "http://example.test/exchange")
		request, gotErr := http.NewRequestWithContext(
			context.Background(),
			exchange.MethodPost.String(),
			target.String(),
			body,
		)
		if gotErr != nil {
			t.Fatalf("http.NewRequestWithContext() setup error = %v, want nil", gotErr)
		}
		request.Header.Set(
			core.HTTPHeaderContentType().String(),
			mustHTTPMediaType(t, "application/json").String(),
		)

		got, gotErr := exchange.ReceiveProjectedJSON[
			projectedTransportDocument,
			*projectedTransportDocument,
		](exchange.ProjectedJSONReceiveCall[
			projectedTransportDocument,
			*projectedTransportDocument,
		]{
			Call: socketServerCall(t, request),
			Policy: exchange.ServerPolicy{
				RequestBodyLimit: mustByteCount(t, 4*1024),
			},
			Route: exchange.RouteSemantics{
				Method: exchange.MethodPost,
				Replay: exchange.ReplaySingleAttempt,
			},
		})
		if !errors.Is(gotErr, core.ErrExchangeRequest) {
			t.Fatalf(
				"ReceiveProjectedJSON(nil projector) error = %v, want %v",
				gotErr,
				core.ErrExchangeRequest,
			)
		}
		if got.Body != nil || !got.IdempotencyKey.IsZero() {
			t.Fatalf("ReceiveProjectedJSON(nil projector) = %+v, want zero", got)
		}
		if _, gotStatErr := body.Stat(); !errors.Is(gotStatErr, fs.ErrClosed) {
			t.Fatalf(
				"request body Stat() error after rejected call = %v, want %v",
				gotStatErr,
				fs.ErrClosed,
			)
		}
	})

	t.Run("neutral rejected wire document never reaches projection", func(t *testing.T) {
		t.Parallel()

		var projectCalls atomic.Uint64
		request := projectedJSONRequest(
			t,
			[]byte(`{"message":"candidate","unknown":true}`),
		)
		got, gotErr := exchange.ReceiveProjectedJSON[
			projectedTransportDocument,
			*projectedTransportDocument,
		](exchange.ProjectedJSONReceiveCall[
			projectedTransportDocument,
			*projectedTransportDocument,
		]{
			Call: socketServerCall(t, request),
			Project: func(
				context.Context,
				exchange.SocketServerCall,
				*projectedTransportDocument,
			) error {
				projectCalls.Add(1)
				return nil
			},
			Policy: exchange.ServerPolicy{
				RequestBodyLimit: mustByteCount(t, 4*1024),
			},
			Route: exchange.RouteSemantics{
				Method: exchange.MethodPost,
				Replay: exchange.ReplaySingleAttempt,
			},
		})
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf(
				"ReceiveProjectedJSON(rejected wire document) error = %v, want %v and %v",
				gotErr,
				core.ErrExchangeRequest,
				core.ErrJSONContract,
			)
		}
		if projectCalls.Load() != 0 {
			t.Fatalf(
				"projector calls after rejected wire document = %d, want 0",
				projectCalls.Load(),
			)
		}
		if got.Body != nil || !got.IdempotencyKey.IsZero() {
			t.Fatalf(
				"ReceiveProjectedJSON(rejected wire document) = %+v, want zero",
				got,
			)
		}
	})
}

func projectedJSONRequest(
	t *testing.T,
	body []byte,
) *http.Request {
	t.Helper()

	target := mustEndpoint(t, "http://example.test/exchange")
	request, gotErr := http.NewRequestWithContext(
		context.Background(),
		exchange.MethodPost.String(),
		target.String(),
		bytes.NewReader(body),
	)
	if gotErr != nil {
		t.Fatalf("http.NewRequestWithContext() setup error = %v, want nil", gotErr)
	}
	request.Header.Set(
		core.HTTPHeaderContentType().String(),
		mustHTTPMediaType(t, "application/json").String(),
	)
	return request
}
