package exchange_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

type standardResponseFixtureDoor uint8

const (
	standardResponseError standardResponseFixtureDoor = iota
	standardResponseNotFound
	standardResponseRedirect
	standardResponseCookie
)

func TestServerStandardResponsesLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		door                standardResponseFixtureDoor
		method              string
		status              int
		message, location   string
		cookie              http.Cookie
		cancelled, zeroCall bool
		wantErr             error
	}{
		{name: "error body preserves Go escaping and text framing", door: standardResponseError, status: http.StatusBadRequest, message: "<invalid> & request"},
		{name: "server error retains embedded and trailing newline bytes", door: standardResponseError, status: http.StatusInternalServerError, message: "first\nsecond\n"},
		{name: "empty diagnostic cannot commit error status", door: standardResponseError, status: http.StatusBadRequest, wantErr: core.ErrExchangeResponse},
		{name: "success status cannot masquerade as error response", door: standardResponseError, status: http.StatusOK, message: "invalid", wantErr: core.ErrExchangeResponse},
		{name: "cancelled error cannot release diagnostic", door: standardResponseError, status: http.StatusBadRequest, message: "withheld", cancelled: true, wantErr: core.ErrExchangeCancelled},
		{name: "not found preserves exact Go body and framing", door: standardResponseNotFound},
		{name: "HEAD not found follows Go writer semantics", door: standardResponseNotFound, method: http.MethodHead},
		{name: "zero socket cannot release not found", door: standardResponseNotFound, zeroCall: true, wantErr: core.ErrExchangeResponse},
		{name: "cancelled not found cannot commit response", door: standardResponseNotFound, cancelled: true, wantErr: core.ErrExchangeCancelled},
		{name: "relative redirect uses Go path resolution and HTML escaping", door: standardResponseRedirect, status: http.StatusFound, location: "../destination?a=1&b=2"},
		{name: "HEAD redirect retains location without Go HTML body", door: standardResponseRedirect, method: http.MethodHead, status: http.StatusTemporaryRedirect, location: "/destination"},
		{name: "POST redirect follows Go method-specific body suppression", door: standardResponseRedirect, method: http.MethodPost, status: http.StatusSeeOther, location: "/destination"},
		{name: "empty location cannot commit redirect", door: standardResponseRedirect, status: http.StatusFound, wantErr: core.ErrExchangeResponse},
		{name: "ordinary success cannot masquerade as redirect", door: standardResponseRedirect, status: http.StatusOK, location: "/destination", wantErr: core.ErrExchangeResponse},
		{name: "malformed escape cannot cross redirect parsing", door: standardResponseRedirect, status: http.StatusFound, location: "/%zz", wantErr: core.ErrExchangeResponse},
		{name: "cancelled redirect withholds location", door: standardResponseRedirect, status: http.StatusFound, location: "/destination", cancelled: true, wantErr: core.ErrExchangeCancelled},
		{name: "cookie preserves Go quoting and security attributes", door: standardResponseCookie, cookie: http.Cookie{Name: "session", Value: "opaque value", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}},
		{name: "deletion cookie retains Go negative MaxAge semantics", door: standardResponseCookie, cookie: http.Cookie{Name: "session", Path: "/", MaxAge: -1}},
		{name: "empty cookie value stays a real field", door: standardResponseCookie, cookie: http.Cookie{Name: "session", Value: ""}},
		{name: "cookie name delimiter cannot add a second field", door: standardResponseCookie, cookie: http.Cookie{Name: "bad name", Value: "opaque"}, wantErr: core.ErrExchangeResponse},
		{name: "cookie value newline cannot inject response header", door: standardResponseCookie, cookie: http.Cookie{Name: "session", Value: "a\r\nb"}, wantErr: core.ErrExchangeResponse},
		{name: "cancelled cookie cannot append field", door: standardResponseCookie, cookie: http.Cookie{Name: "session", Value: "opaque"}, cancelled: true, wantErr: core.ErrExchangeCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			request := httptest.NewRequestWithContext(ctx, method, "https://example.test/source/start", nil)
			recorder, oracle := httptest.NewRecorder(), httptest.NewRecorder()
			call := socketServerCallFrom(t, recorder, request)
			if tc.zeroCall {
				call = exchange.SocketServerCall{}
			}
			var gotErr error
			switch tc.door {
			case standardResponseError:
				response := exchange.ServerErrorResponse{Message: tc.message, Status: mustHTTPStatus(t, tc.status)}
				gotErr = exchange.Error(call, response)
				if tc.wantErr == nil {
					http.Error(oracle, tc.message, tc.status)
				}
			case standardResponseNotFound:
				gotErr = exchange.NotFound(call)
				if tc.wantErr == nil {
					http.NotFound(oracle, request)
				}
			case standardResponseRedirect:
				response := exchange.ServerRedirectResponse{Location: tc.location, Status: mustHTTPStatus(t, tc.status)}
				gotErr = exchange.Redirect(call, response)
				if tc.wantErr == nil {
					http.Redirect(oracle, request, tc.location, tc.status)
				}
			case standardResponseCookie:
				gotErr = exchange.SetCookie(call, tc.cookie)
				if tc.wantErr == nil {
					http.SetCookie(oracle, &tc.cookie)
				}
			default:
				t.Fatalf("fixture door=%v, want a declared operation", tc.door)
			}
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, context.Canceled) != tc.cancelled {
				t.Fatalf("standard writer error=%v,want %v,cancelled=%t", gotErr, tc.wantErr, tc.cancelled)
			}
			if recorder.Code != oracle.Code || recorder.Body.String() != oracle.Body.String() || !maps.EqualFunc(recorder.Header(), oracle.Header(), slices.Equal[[]string]) || recorder.Flushed != oracle.Flushed {
				t.Fatalf("standard response=(%d,%q,%v),want exact Go response (%d,%q,%v)", recorder.Code, recorder.Body.String(), recorder.Header(), oracle.Code, oracle.Body.String(), oracle.Header())
			}
		})
	}
}
