package exchange

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type standardResponseOperation uint8

const (
	standardResponseError standardResponseOperation = iota
	standardResponseNotFound
	standardResponseRedirect
)

type standardResponseFixture struct {
	name      string
	text      string
	method    string
	status    int
	operation standardResponseOperation
	wantErr   error
}

func (f standardResponseFixture) goWrite(w http.ResponseWriter, r *http.Request) {
	switch f.operation {
	case standardResponseError:
		http.Error(w, f.text, f.status)
	case standardResponseNotFound:
		http.NotFound(w, r)
	case standardResponseRedirect:
		http.Redirect(w, r, f.text, f.status)
	default:
		panic(core.ErrExchangeContract)
	}
}

func (f standardResponseFixture) exchangeWrite(t *testing.T, w http.ResponseWriter, r *http.Request) error {
	t.Helper()
	call, err := NewSocketServerCall(w, r)
	if err != nil {
		t.Fatal(err)
	}
	var status core.HTTPStatusCode
	if err := status.AdmitInt(f.status); err != nil {
		t.Fatal(err)
	}
	switch f.operation {
	case standardResponseError:
		return Error(call, ServerErrorResponse{Message: f.text, Status: status})
	case standardResponseNotFound:
		return NotFound(call)
	case standardResponseRedirect:
		return Redirect(call, ServerRedirectResponse{Location: f.text, Status: status})
	default:
		t.Fatal("undeclared standard response operation")
		return core.ErrExchangeContract
	}
}

func TestStandardResponseGoParityLayerTriad(t *testing.T) {
	t.Parallel()
	fixtures := []standardResponseFixture{
		{name: "positive error overrides stale length", operation: standardResponseError, status: http.StatusBadRequest, text: "invalid boundary", method: http.MethodGet},
		{name: "positive error retains embedded newline", operation: standardResponseError, status: http.StatusInternalServerError, text: "first\nsecond\n", method: http.MethodGet},
		{name: "positive error unicode is unmodified", operation: standardResponseError, status: http.StatusUnprocessableEntity, text: "日本語 🧱", method: http.MethodGet},
		{name: "boundary error maximum diagnostic", operation: standardResponseError, status: http.StatusServiceUnavailable, text: strings.Repeat("x", serverErrorMessageMaximumBytes), method: http.MethodGet},
		{name: "positive canonical not found", operation: standardResponseNotFound, status: http.StatusNotFound, method: http.MethodGet},
		{name: "positive redirect resolves relative dot segments", operation: standardResponseRedirect, status: http.StatusFound, text: "../target/", method: http.MethodGet},
		{name: "positive redirect escapes html and unicode", operation: standardResponseRedirect, status: http.StatusSeeOther, text: "/日本語?q=\"<&", method: http.MethodGet},
		{name: "positive redirect retains escaped slash", operation: standardResponseRedirect, status: http.StatusTemporaryRedirect, text: "/a%2Fb?q=a%2Fb", method: http.MethodGet},
		{name: "positive redirect external authority", operation: standardResponseRedirect, status: http.StatusPermanentRedirect, text: "https://other.example.test/path", method: http.MethodGet},
		{name: "boundary redirect maximum location", operation: standardResponseRedirect, status: http.StatusMovedPermanently, text: "/" + strings.Repeat("x", serverRedirectMaximumBytes-1), method: http.MethodGet},
		{name: "neutral head redirect writes no body", operation: standardResponseRedirect, status: http.StatusFound, text: "/target", method: http.MethodHead},
		{name: "neutral post redirect writes no body", operation: standardResponseRedirect, status: http.StatusSeeOther, text: "/target", method: http.MethodPost},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(fixture.method, "https://example.test/parent/source", nil)
			got, want := httptest.NewRecorder(), httptest.NewRecorder()
			for _, recorder := range []*httptest.ResponseRecorder{got, want} {
				recorder.Header().Set(core.HTTPHeaderContentLength().String(), "999")
			}
			fixture.goWrite(want, request)
			if gotErr := fixture.exchangeWrite(t, got, request); !errors.Is(gotErr, fixture.wantErr) {
				t.Fatalf("standard response error = %v, want %v", gotErr, fixture.wantErr)
			}
			if got.Code != want.Code || !bytes.Equal(got.Body.Bytes(), want.Body.Bytes()) {
				t.Fatalf("status/body = %d/%q, want Go %d/%q", got.Code, got.Body.Bytes(), want.Code, want.Body.Bytes())
			}
			if len(got.Header()) != len(want.Header()) {
				t.Fatalf("header count = %d, want %d", len(got.Header()), len(want.Header()))
			}
			for name, values := range want.Header() {
				if !slices.Equal(got.Header()[name], values) {
					t.Fatalf("header %s = %q, want Go %q", name, got.Header()[name], values)
				}
			}
		})
	}

	t.Run("negative refused responses leave writer untouched", func(t *testing.T) {
		t.Parallel()
		cases := []standardResponseFixture{
			{name: "missing error diagnostic", operation: standardResponseError, status: http.StatusBadRequest, wantErr: core.ErrExchangeResponse},
			{name: "oversized diagnostic", operation: standardResponseError, status: http.StatusBadRequest, text: strings.Repeat("x", serverErrorMessageMaximumBytes+1), wantErr: core.ErrExchangeResponse},
			{name: "nonerror status", operation: standardResponseError, status: http.StatusOK, text: "message", wantErr: core.ErrExchangeResponse},
			{name: "missing location", operation: standardResponseRedirect, status: http.StatusFound, wantErr: core.ErrExchangeResponse},
			{name: "oversized location", operation: standardResponseRedirect, status: http.StatusFound, text: "/" + strings.Repeat("x", serverRedirectMaximumBytes), wantErr: core.ErrExchangeResponse},
			{name: "malformed escape", operation: standardResponseRedirect, status: http.StatusFound, text: "/%zz", wantErr: core.ErrExchangeResponse},
			{name: "nonredirect status", operation: standardResponseRedirect, status: http.StatusOK, text: "/target", wantErr: core.ErrExchangeResponse},
		}
		for _, fixture := range cases {
			t.Run(fixture.name, func(t *testing.T) {
				t.Parallel()
				writer := &standardFaultWriter{header: make(http.Header)}
				err := fixture.exchangeWrite(t, writer, httptest.NewRequest(http.MethodGet, "https://example.test", nil))
				if !errors.Is(err, fixture.wantErr) || !errors.Is(err, core.ErrExchangeContract) || writer.status != 0 || writer.calls != 0 || len(writer.header) != 0 {
					t.Fatalf("refusal = %v, status/writes/headers = %d/%d/%d, want response+contract and zero effects", err, writer.status, writer.calls, len(writer.header))
				}
			})
		}
	})
}

type standardWriterFault uint8

const (
	standardWriteClosed standardWriterFault = iota
	standardWritePartial
	standardWriteShort
	standardWriteNegative
	standardWriteExcess
	standardWritePanic
)

type standardFaultWriter struct {
	header http.Header
	status int
	calls  int
	fault  standardWriterFault
}

func (w *standardFaultWriter) Header() http.Header    { return w.header }
func (w *standardFaultWriter) WriteHeader(status int) { w.status = status }
func (w *standardFaultWriter) Write(p []byte) (int, error) {
	w.calls++
	switch w.fault {
	case standardWriteClosed:
		return 0, io.ErrClosedPipe
	case standardWritePartial:
		return len(p) / 2, io.ErrClosedPipe
	case standardWriteShort:
		return len(p) - 1, nil
	case standardWriteNegative:
		return -1, nil
	case standardWriteExcess:
		return len(p) + 1, nil
	case standardWritePanic:
		panic(core.ErrExchangeContract)
	default:
		panic(core.ErrExchangeContract)
	}
}

func TestStandardResponseWriteFailureIsObserved(t *testing.T) {
	t.Parallel()
	operations := []standardResponseFixture{
		{name: "error", operation: standardResponseError, status: http.StatusBadRequest, text: "rejected"},
		{name: "not found", operation: standardResponseNotFound, status: http.StatusNotFound},
		{name: "redirect", operation: standardResponseRedirect, status: http.StatusFound, text: "/target"},
	}
	faults := []struct {
		name  string
		fault standardWriterFault
		want  error
	}{
		{name: "closed writer", fault: standardWriteClosed, want: io.ErrClosedPipe},
		{name: "partial failed write", fault: standardWritePartial, want: io.ErrClosedPipe},
		{name: "short nil write", fault: standardWriteShort, want: io.ErrShortWrite},
		{name: "negative write count", fault: standardWriteNegative, want: core.ErrExchangeContract},
		{name: "excess write count", fault: standardWriteExcess, want: core.ErrExchangeContract},
		{name: "writer panic", fault: standardWritePanic, want: core.ErrExchangeContract},
	}
	for _, operation := range operations {
		for _, fault := range faults {
			t.Run(operation.name+"/"+fault.name, func(t *testing.T) {
				t.Parallel()
				writer := &standardFaultWriter{header: make(http.Header), fault: fault.fault}
				err := operation.exchangeWrite(t, writer, httptest.NewRequest(http.MethodGet, "https://example.test", nil))
				if !errors.Is(err, core.ErrExchangeResponse) || !errors.Is(err, core.ErrExchangeWrite) || !errors.Is(err, fault.want) {
					t.Fatalf("write error = %v, want response, write, and %v", err, fault.want)
				}
				if writer.calls != 1 || writer.status != operation.status {
					t.Fatalf("write effects = %d calls/status %d, want 1/%d", writer.calls, writer.status, operation.status)
				}
			})
		}
	}
}
