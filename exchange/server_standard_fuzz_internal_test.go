package exchange

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The oracle observes Go's own returned write error. Standard helpers return
// no error, and ResponseRecorder records attempted bytes even when it refuses
// their status. Neither the Exchange observer nor its validators compute wants.
type standardGoWriteOracle struct {
	recorder *httptest.ResponseRecorder
	cause    error
}

func (w *standardGoWriteOracle) Header() http.Header    { return w.recorder.Header() }
func (w *standardGoWriteOracle) WriteHeader(status int) { w.recorder.WriteHeader(status) }
func (w *standardGoWriteOracle) Write(p []byte) (int, error) {
	n, err := w.recorder.Write(p)
	if err != nil {
		w.cause = err
	}
	return n, err
}

func FuzzErrorNotFoundRedirectAndSetCookieGoParity(f *testing.F) {
	seed := ServerRedirectResponse{Location: "/scope/target", Status: mustInternalHTTPStatus(f, http.StatusFound)}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	// The typed redirect and Go cookie are the public representation. Project
	// their validated fields rather than inventing a separate fixture protocol.
	cookie := http.Cookie{Name: sessionCookieName, Value: sessionCookieValue}
	if err := cookie.Valid(); err != nil {
		f.Fatal(err)
	}
	for _, status := range []int{http.StatusFound, http.StatusNotModified, http.StatusBadRequest, http.StatusInternalServerError, http.StatusOK, 0} {
		for method := range uint8(3) {
			f.Add(seed.Location, cookie.Name, cookie.Value, uint16(status), method, false)
		}
	}
	f.Add(seed.Location, cookie.Name, cookie.Value, uint16(http.StatusFound), uint8(0), true)
	for _, size := range []int{serverRedirectMaximumBytes - 1, serverRedirectMaximumBytes, serverRedirectMaximumBytes + 1, serverErrorMessageMaximumBytes - 1, serverErrorMessageMaximumBytes, serverErrorMessageMaximumBytes + 1} {
		f.Add("/"+strings.Repeat("p", size-1), cookie.Name, cookie.Value, uint16(http.StatusFound), uint8(0), false)
		f.Add(strings.Repeat("e", size), cookie.Name, cookie.Value, uint16(http.StatusBadRequest), uint8(0), false)
	}
	for _, hostile := range []string{"", "/%zz", "location\r\ninjection", "日本語?x=<&\""} {
		f.Add(hostile, "invalid name", hostile, uint16(http.StatusFound), uint8(0), false)
	}
	for _, size := range []int{serverSetCookieMaximumHeaderBytes - 1, serverSetCookieMaximumHeaderBytes, serverSetCookieMaximumHeaderBytes + 1} {
		f.Add(seed.Location, cookie.Name, strings.Repeat("v", size-len(cookie.Name)-1), uint16(http.StatusBadRequest), uint8(0), false)
	}
	f.Fuzz(func(t *testing.T, text, name, value string, statusInput uint16, methodInput uint8, cancelled bool) {
		if len(text) > serverErrorMessageMaximumBytes+1 || len(name) > serverSetCookieMaximumHeaderBytes+1 || len(value) > serverSetCookieMaximumHeaderBytes+1 || methodInput > 2 {
			return
		}
		methods := []string{http.MethodGet, http.MethodHead, http.MethodPost}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if cancelled {
			cancel()
		}
		request := httptest.NewRequestWithContext(ctx, methods[methodInput], "https://writer-oracle.invalid/parent/source", nil)
		var status core.HTTPStatusCode
		statusErr := status.AdmitInt(int(statusInput))
		cookie := http.Cookie{Name: name, Value: value}
		_, locationErr := url.Parse(text)
		for _, lane := range []writeAdmissionLane{writeAdmissionError, writeAdmissionNotFound, writeAdmissionRedirect, writeAdmissionCookie} {
			got, want := httptest.NewRecorder(), httptest.NewRecorder()
			oracle := &standardGoWriteOracle{recorder: want}
			call, err := NewSocketServerCall(got, request)
			if err != nil {
				t.Fatal(err)
			}
			var gotErr error
			wantAccepted := false
			switch lane {
			case writeAdmissionError:
				gotErr = Error(call, ServerErrorResponse{Message: text, Status: status})
				wantAccepted = statusErr == nil && statusInput >= http.StatusBadRequest && len(text) > 0 && len(text) <= serverErrorMessageMaximumBytes
				if wantAccepted {
					http.Error(oracle, text, int(statusInput))
				}
			case writeAdmissionNotFound:
				gotErr = NotFound(call)
				wantAccepted = true
				http.NotFound(oracle, request)
			case writeAdmissionRedirect:
				gotErr = Redirect(call, ServerRedirectResponse{Location: text, Status: status})
				wantAccepted = statusErr == nil && statusInput >= http.StatusMultipleChoices && statusInput < http.StatusBadRequest && len(text) > 0 && len(text) <= serverRedirectMaximumBytes && locationErr == nil
				if wantAccepted {
					http.Redirect(oracle, request, text, int(statusInput))
				}
			case writeAdmissionCookie:
				gotErr = SetCookie(call, cookie)
				wantAccepted = cookie.Valid() == nil && len(cookie.String()) > 0 && len(cookie.String()) <= serverSetCookieMaximumHeaderBytes
				if wantAccepted {
					http.SetCookie(oracle, &cookie)
				}
			default:
				t.Fatalf("unclassified writer %d", lane)
			}
			if cancelled {
				wantAccepted = false
			}
			if wantAccepted {
				if !errors.Is(gotErr, oracle.cause) || errors.Is(gotErr, core.ErrExchangeWrite) != (oracle.cause != nil) || errors.Is(gotErr, core.ErrExchangeResponse) != (oracle.cause != nil) || got.Code != want.Code || !bytes.Equal(got.Body.Bytes(), want.Body.Bytes()) || !maps.EqualFunc(got.Header(), want.Header(), slices.Equal[[]string]) || got.Flushed != want.Flushed {
					t.Fatalf("writer %d=(%d,%q,%v,%v), want exact Go response (%d,%q,%v)", lane, got.Code, got.Body.Bytes(), got.Header(), gotErr, want.Code, want.Body.Bytes(), want.Header())
				}
			} else {
				if !errors.Is(gotErr, core.ErrExchangeResponse) || got.Body.Len() != 0 || len(got.Header()) != 0 || got.Flushed {
					t.Fatalf("writer %d refusal=(%v,%q,%v), want typed refusal without output", lane, gotErr, got.Body.Bytes(), got.Header())
				}
				if cancelled && (!errors.Is(gotErr, context.Canceled) || !errors.Is(gotErr, core.ErrExchangeCancelled)) {
					t.Fatalf("writer %d lost cancelled context: %v", lane, gotErr)
				}
			}
		}
	})
}
