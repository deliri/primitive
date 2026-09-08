package exchange

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type writeAdmissionLane uint8

const (
	writeAdmissionJSON writeAdmissionLane = iota
	writeAdmissionNoBody
	writeAdmissionBounded
	writeAdmissionStream
	writeAdmissionError
	writeAdmissionNotFound
	writeAdmissionRedirect
	writeAdmissionCookie
)

type admissionResponseWriter struct {
	header      http.Header
	body        bytes.Buffer
	headerCalls int
	statusCalls int
	writes      int
	status      int
}

func (w *admissionResponseWriter) Header() http.Header         { w.headerCalls++; return w.header }
func (w *admissionResponseWriter) WriteHeader(status int)      { w.statusCalls++; w.status = status }
func (w *admissionResponseWriter) Write(p []byte) (int, error) { w.writes++; return w.body.Write(p) }

type admissionJSONDocument struct {
	Message  string `json:"message"`
	cancel   context.CancelFunc
	marshals *int
}

func (d admissionJSONDocument) Validate() error {
	if d.Message == "" {
		return core.ErrExchangeContract
	}
	return nil
}
func (d admissionJSONDocument) MarshalJSON() ([]byte, error) {
	if d.marshals != nil {
		*d.marshals++
	}
	if d.cancel != nil {
		d.cancel()
	}
	type wire admissionJSONDocument
	return json.Marshal(wire(d))
}

func mustInternalHTTPStatus(t testing.TB, value int) core.HTTPStatusCode {
	t.Helper()
	var status core.HTTPStatusCode
	if err := status.AdmitInt(value); err != nil {
		t.Fatalf("status fixture admission error = %v, want nil", err)
	}
	return status
}

func TestServerWriteAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	redirect := httptest.NewRecorder()
	http.Redirect(redirect, httptest.NewRequest(http.MethodGet, "/", nil), "/target", http.StatusFound)
	notFound := httptest.NewRecorder()
	http.NotFound(notFound, httptest.NewRequest(http.MethodGet, "/", nil))
	cookie := http.Cookie{Name: "boundary", Value: "owned", Path: "/", HttpOnly: true}
	cases := []struct {
		name         string
		lane         writeAdmissionLane
		cancelled    bool
		cancelInJSON bool
		wantErr      error
		wantNative   error
		wantBody     string
		wantStatus   int
		wantStatuses int
		wantWrites   int
		wantMarshals int
		wantHeaders  bool
		wantCookie   string
	}{
		{name: "positive JSON performs one exact encoded write", lane: writeAdmissionJSON, wantBody: `{"message":"abc"}`, wantStatus: http.StatusOK, wantStatuses: 1, wantWrites: 1, wantMarshals: 1, wantHeaders: true},
		{name: "neutral no-body response commits only status and framing", lane: writeAdmissionNoBody, wantStatus: http.StatusNoContent, wantStatuses: 1, wantHeaders: true},
		{name: "positive bounded bytes remain exact", lane: writeAdmissionBounded, wantBody: "abc", wantStatus: http.StatusOK, wantStatuses: 1, wantWrites: 1, wantHeaders: true},
		{name: "positive stream retains exact extent", lane: writeAdmissionStream, wantBody: "abc", wantStatus: http.StatusOK, wantStatuses: 1, wantWrites: 1, wantHeaders: true},
		{name: "positive error uses Go rendering", lane: writeAdmissionError, wantBody: "failed\n", wantStatus: http.StatusBadRequest, wantStatuses: 1, wantWrites: 1, wantHeaders: true},
		{name: "positive not found uses Go rendering", lane: writeAdmissionNotFound, wantBody: notFound.Body.String(), wantStatus: http.StatusNotFound, wantStatuses: 1, wantWrites: 1, wantHeaders: true},
		{name: "positive redirect uses Go rendering", lane: writeAdmissionRedirect, wantBody: redirect.Body.String(), wantStatus: http.StatusFound, wantStatuses: 1, wantWrites: 1, wantHeaders: true},
		{name: "positive cookie appends Go value without committing", lane: writeAdmissionCookie, wantHeaders: true, wantCookie: cookie.String()},
		{name: "negative cancelled JSON performs no encoding or write", lane: writeAdmissionJSON, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancelled no-body cannot commit status", lane: writeAdmissionNoBody, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancelled bounded response cannot touch headers", lane: writeAdmissionBounded, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancelled stream cannot read its source", lane: writeAdmissionStream, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancelled error cannot commit diagnostic", lane: writeAdmissionError, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancelled not found cannot commit diagnostic", lane: writeAdmissionNotFound, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancelled redirect cannot commit location", lane: writeAdmissionRedirect, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancelled cookie cannot mutate headers", lane: writeAdmissionCookie, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "negative cancellation during JSON encoding prevents commit", lane: writeAdmissionJSON, cancelInJSON: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled, wantMarshals: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			writer := &admissionResponseWriter{header: make(http.Header)}
			call, err := NewSocketServerCall(writer, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
			if err != nil {
				t.Fatalf("NewSocketServerCall() setup error = %v, want nil", err)
			}
			var marshals int
			document := admissionJSONDocument{Message: "abc", marshals: &marshals}
			if tc.cancelInJSON {
				document.cancel = cancel
			}
			source := bytes.NewReader([]byte("abc"))
			var gotErr error
			switch tc.lane {
			case writeAdmissionJSON:
				gotErr = WriteJSON(JSONWriteCall[admissionJSONDocument]{Call: call, Response: ServerJSONResponse[admissionJSONDocument]{Body: document, Status: core.HTTPStatusOK()}, Policy: JSONWritePolicy{ResponseBodyLimit: mustInternalByteCount(t, core.JSONDocumentMaximumBytes)}})
			case writeAdmissionNoBody:
				gotErr = WriteNoBody(NoBodyWriteCall{Call: call, Response: ServerNoBodyResponse{Status: mustInternalHTTPStatus(t, http.StatusNoContent)}})
			case writeAdmissionBounded:
				gotErr = WriteBounded(BoundedWriteCall{Call: call, Response: ServerBoundedResponse{Body: []byte("abc"), ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}})
			case writeAdmissionStream:
				length, err := core.NewByteLength(3)
				if err != nil {
					t.Fatalf("length setup error = %v, want nil", err)
				}
				gotErr = WriteStream(StreamWriteCall{Call: call, Response: ServerStreamResponse{Source: source, ContentLength: length, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}})
			case writeAdmissionError:
				gotErr = Error(call, ServerErrorResponse{Message: "failed", Status: mustInternalHTTPStatus(t, http.StatusBadRequest)})
			case writeAdmissionNotFound:
				gotErr = NotFound(call)
			case writeAdmissionRedirect:
				gotErr = Redirect(call, ServerRedirectResponse{Location: "/target", Status: mustInternalHTTPStatus(t, http.StatusFound)})
			case writeAdmissionCookie:
				gotErr = SetCookie(call, cookie)
			default:
				t.Fatalf("write lane = %v, want a declared lane", tc.lane)
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("write error = %v, want identities (%v, %v)", gotErr, tc.wantErr, tc.wantNative)
			}
			if writer.body.String() != tc.wantBody || writer.status != tc.wantStatus || writer.statusCalls != tc.wantStatuses || writer.writes != tc.wantWrites || marshals != tc.wantMarshals || (writer.headerCalls != 0) != tc.wantHeaders {
				t.Fatalf("body/status/commits/writes/encodes/header access = (%q, %d, %d, %d, %d, %d), want (%q, %d, %d, %d, %d, touched %t)", writer.body.String(), writer.status, writer.statusCalls, writer.writes, marshals, writer.headerCalls, tc.wantBody, tc.wantStatus, tc.wantStatuses, tc.wantWrites, tc.wantMarshals, tc.wantHeaders)
			}
			if tc.wantErr != nil && (len(writer.header) != 0 || source.Len() != len("abc")) {
				t.Fatalf("refused headers/source remaining = (%v, %d), want absent headers and untouched source", writer.header, source.Len())
			}
			if tc.lane == writeAdmissionCookie {
				cookies := (&http.Response{Header: writer.header}).Cookies()
				wantCookies := 0
				if tc.wantCookie != "" {
					wantCookies = 1
				}
				if len(cookies) != wantCookies {
					t.Fatalf("cookie count = %d, want %d", len(cookies), wantCookies)
				}
				if wantCookies == 1 && cookies[0].String() != tc.wantCookie {
					t.Fatalf("Go cookie projection = %q, want %q", cookies[0].String(), tc.wantCookie)
				}
			}
		})
	}
}
