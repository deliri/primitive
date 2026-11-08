package exchange

import (
	"bufio"
	"net"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
)

// ResponseForwarder carries the native writer wrapped by a caller-owned
// http.ResponseWriter decorator. After the decorator applies its own header or
// status policy, the forwarder hands the status, body bytes, flush and hijack
// to that writer unchanged. It adds no header, framing, status, buffering or
// allocation policy: native results, short writes and panics remain Go's.
// Typed responses use WriteJSON, WriteBounded, WriteNoBody or WriteStream.
type ResponseForwarder struct {
	writer http.ResponseWriter
}

// NewResponseForwarder admits the decorator's wrapped native writer.
func NewResponseForwarder(writer http.ResponseWriter) (ResponseForwarder, error) {
	forwarder := ResponseForwarder{writer: writer}
	if err := forwarder.Validate(); err != nil {
		return ResponseForwarder{}, err
	}
	return forwarder, nil
}

// Validate rejects an absent or typed-nil native writer.
func (f ResponseForwarder) Validate() error {
	if core.WriterIsNil(f.writer) {
		return core.ErrExchangeContract
	}
	return nil
}

// WriteHeader forwards the status the decorator itself received through Go's
// http.ResponseWriter.WriteHeader contract. Informational, repeated and
// invalid codes keep net/http's own handling, including its panic.
func (f ResponseForwarder) WriteHeader(status int) error {
	if err := f.Validate(); err != nil {
		return err
	}
	f.writer.WriteHeader(status)
	return nil
}

// Write forwards body bytes and returns the native count and error unchanged.
func (f ResponseForwarder) Write(body []byte) (int, error) {
	if err := f.Validate(); err != nil {
		return 0, err
	}
	return f.writer.Write(body)
}

// Flush flushes through Go's ResponseController, which follows Unwrap and
// reports a writer without flush support as http.ErrNotSupported.
func (f ResponseForwarder) Flush() error {
	if err := f.Validate(); err != nil {
		return err
	}
	return http.NewResponseController(f.writer).Flush()
}

// Hijack takes over the connection only when the wrapped writer itself is an
// http.Hijacker. It does not follow Unwrap past an intermediate decorator that
// declined hijacking; otherwise it returns http.ErrNotSupported.
func (f ResponseForwarder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if err := f.Validate(); err != nil {
		return nil, nil, err
	}
	hijacker, ok := f.writer.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}
