package exchange

import (
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
)

const (
	serverErrorMessageMaximumBytes    = 64 << 10
	serverRedirectMaximumBytes        = 8192
	serverSetCookieMaximumHeaderBytes = 4096
)

// ServerErrorResponse is one bounded standard HTTP error response. Products
// choose the status and diagnostic; Exchange owns the wire mechanics.
type ServerErrorResponse struct {
	Message string
	Status  core.HTTPStatusCode
}

func (r ServerErrorResponse) Validate() error {
	if r.Message == "" || len(r.Message) > serverErrorMessageMaximumBytes {
		return responseError(core.ErrExchangeContract)
	}
	if err := r.Status.Validate(); err != nil || (!r.Status.IsClientError() && !r.Status.IsServerError()) {
		return responseError(errors.Join(core.ErrExchangeContract, err))
	}
	return nil
}

// Error writes the standard-library HTTP error shape through Exchange.
func Error(call SocketServerCall, response ServerErrorResponse) error {
	if err := call.validateWrite(); err != nil {
		return err
	}
	if err := response.Validate(); err != nil {
		return err
	}
	status, _ := response.Status.Int()
	return executeResponseWriterOperation(func() error {
		writer := &observedStandardResponseWriter{ResponseWriter: call.writer}
		http.Error(writer, response.Message, status)
		return writer.err
	})
}

// NotFound writes net/http's canonical 404 response through Exchange.
func NotFound(call SocketServerCall) error {
	if err := call.validateWrite(); err != nil {
		return err
	}
	return executeResponseWriterOperation(func() error {
		writer := &observedStandardResponseWriter{ResponseWriter: call.writer}
		http.NotFound(writer, call.request)
		return writer.err
	})
}

// ServerRedirectResponse is one bounded standard HTTP redirect effect.
type ServerRedirectResponse struct {
	Location string
	Status   core.HTTPStatusCode
}

func (r ServerRedirectResponse) Validate() error {
	if r.Location == "" || len(r.Location) > serverRedirectMaximumBytes {
		return responseError(core.ErrExchangeContract)
	}
	if _, err := url.Parse(r.Location); err != nil {
		return responseError(errors.Join(core.ErrExchangeContract, err))
	}
	if err := r.Status.Validate(); err != nil || !r.Status.IsRedirect() {
		return responseError(errors.Join(core.ErrExchangeContract, err))
	}
	return nil
}

// Redirect writes a standard-library redirect through Exchange.
func Redirect(call SocketServerCall, response ServerRedirectResponse) error {
	if err := call.validateWrite(); err != nil {
		return err
	}
	if err := response.Validate(); err != nil {
		return err
	}
	status, _ := response.Status.Int()
	return executeResponseWriterOperation(func() error {
		writer := &observedStandardResponseWriter{ResponseWriter: call.writer}
		http.Redirect(writer, call.request, response.Location, status)
		return writer.err
	})
}

// Go's standard response helpers return no error. Observe their writes without
// replacing their rendering or header mechanics.
type observedStandardResponseWriter struct {
	http.ResponseWriter
	err error
}

func (w *observedStandardResponseWriter) Write(payload []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	count, err := w.ResponseWriter.Write(payload)
	if count < 0 || count > len(payload) {
		count = 0
		err = errors.Join(core.ErrExchangeContract, err)
	}
	if count != len(payload) && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, err)
	}
	return count, w.err
}

// SetCookie appends one validated Set-Cookie response field. The Go cookie
// remains recognizable; Exchange owns only validation and the wire write.
func SetCookie(call SocketServerCall, cookie http.Cookie) error {
	if err := call.validateWrite(); err != nil {
		return err
	}
	if err := cookie.Valid(); err != nil {
		return responseError(errors.Join(core.ErrExchangeContract, err))
	}
	if value := cookie.String(); value == "" || len(value) > serverSetCookieMaximumHeaderBytes {
		return responseError(core.ErrExchangeContract)
	}
	return executeResponseWriterOperation(func() error {
		http.SetCookie(call.writer, &cookie)
		return nil
	})
}

var (
	_ core.Validatable = ServerErrorResponse{}
	_ core.Validatable = ServerRedirectResponse{}
)
