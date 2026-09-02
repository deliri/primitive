package exchange_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestServerStandardResponsesLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive standard responses retain net http wire semantics", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodGet, "https://example.test/source", nil)
		errorResponse := exchange.ServerErrorResponse{Message: "invalid request", Status: mustHTTPStatus(t, http.StatusBadRequest)}
		if gotErr := errorResponse.Validate(); gotErr != nil {
			t.Fatalf("ServerErrorResponse.Validate() error = %v, want nil", gotErr)
		}
		errorRecorder := httptest.NewRecorder()
		if gotErr := exchange.Error(socketServerCallFrom(t, errorRecorder, request), errorResponse); gotErr != nil || errorRecorder.Code != http.StatusBadRequest || errorRecorder.Body.String() != "invalid request\n" {
			t.Fatalf("exchange.Error() = (status:%d, body:%q, error:%v), want (400, canonical body, nil)", errorRecorder.Code, errorRecorder.Body.String(), gotErr)
		}

		notFoundRecorder := httptest.NewRecorder()
		if gotErr := exchange.NotFound(socketServerCallFrom(t, notFoundRecorder, request)); gotErr != nil || notFoundRecorder.Code != http.StatusNotFound {
			t.Fatalf("exchange.NotFound() = (status:%d, error:%v), want (404, nil)", notFoundRecorder.Code, gotErr)
		}

		redirectResponse := exchange.ServerRedirectResponse{Location: "/destination", Status: mustHTTPStatus(t, http.StatusFound)}
		if gotErr := redirectResponse.Validate(); gotErr != nil {
			t.Fatalf("ServerRedirectResponse.Validate() error = %v, want nil", gotErr)
		}
		redirectRecorder := httptest.NewRecorder()
		if gotErr := exchange.Redirect(socketServerCallFrom(t, redirectRecorder, request), redirectResponse); gotErr != nil || redirectRecorder.Code != http.StatusFound || redirectRecorder.Header().Get("Location") != "/destination" {
			t.Fatalf("exchange.Redirect() = (status:%d, location:%q, error:%v), want (302, /destination, nil)", redirectRecorder.Code, redirectRecorder.Header().Get("Location"), gotErr)
		}

		cookieRecorder := httptest.NewRecorder()
		cookie := http.Cookie{Name: "session", Value: "opaque", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
		if gotErr := exchange.SetCookie(socketServerCallFrom(t, cookieRecorder, request), cookie); gotErr != nil || len(cookieRecorder.Header().Values("Set-Cookie")) != 1 {
			t.Fatalf("exchange.SetCookie() = (headers:%v, error:%v), want one canonical cookie and nil", cookieRecorder.Header().Values("Set-Cookie"), gotErr)
		}
	})

	t.Run("negative invalid standard responses write no status or headers", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodGet, "https://example.test/source", nil)
		recorder := httptest.NewRecorder()
		gotErr := exchange.Error(socketServerCallFrom(t, recorder, request), exchange.ServerErrorResponse{Status: core.HTTPStatusOK()})
		if !errors.Is(gotErr, core.ErrExchangeResponse) || recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
			t.Fatalf("exchange.Error(invalid) = (status:%d, bytes:%d, error:%v), want untouched recorder and typed refusal", recorder.Code, recorder.Body.Len(), gotErr)
		}
		redirectRecorder := httptest.NewRecorder()
		gotErr = exchange.Redirect(socketServerCallFrom(t, redirectRecorder, request), exchange.ServerRedirectResponse{Location: "/destination", Status: core.HTTPStatusOK()})
		if !errors.Is(gotErr, core.ErrExchangeResponse) || redirectRecorder.Header().Get("Location") != "" {
			t.Fatalf("exchange.Redirect(nonredirect) = (location:%q, error:%v), want no header and typed refusal", redirectRecorder.Header().Get("Location"), gotErr)
		}
		cookieRecorder := httptest.NewRecorder()
		gotErr = exchange.SetCookie(socketServerCallFrom(t, cookieRecorder, request), http.Cookie{Name: "bad name", Value: "opaque"})
		if !errors.Is(gotErr, core.ErrExchangeResponse) || len(cookieRecorder.Header().Values("Set-Cookie")) != 0 {
			t.Fatalf("exchange.SetCookie(invalid) = (headers:%v, error:%v), want none and typed refusal", cookieRecorder.Header().Values("Set-Cookie"), gotErr)
		}
	})
}
