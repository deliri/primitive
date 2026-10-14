package jsonio

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// These guards enforce Go's reader/writer count contract before the JSON
// engine indexes its buffer or acknowledges a complete encoded record.
type checkedJSONSource struct{ source io.Reader }

func (r checkedJSONSource) Read(data []byte) (int, error) {
	n, err := r.source.Read(data)
	if n < 0 || n > len(data) {
		return 0, errors.Join(core.ErrJSONContract, err)
	}
	// A decoder may consume accompanying bytes and subsequently observe only
	// EOF from a bounded native section. Refuse provisional bytes immediately
	// when their read also refused; do not let framing replace native failure.
	if err != nil && err != io.EOF {
		return 0, &nativeJSONError{cause: err}
	}
	return n, err
}

// nativeJSONError marks I/O provenance through Go's codec error wrapping.
// It introduces no Primitive failure identity; the native cause remains owner.
type nativeJSONError struct{ cause error }

func (e *nativeJSONError) Error() string { return e.cause.Error() }
func (e *nativeJSONError) Unwrap() error { return e.cause }

func nativeJSONCause(err error) error {
	if native, ok := errors.AsType[*nativeJSONError](err); ok {
		return native.cause
	}
	return err
}

func jsonOperationError(err error) error {
	if err == nil {
		return nil
	}
	if native, ok := errors.AsType[*nativeJSONError](err); ok {
		return native.cause
	}
	return errors.Join(core.ErrJSONContract, err)
}

type checkedJSONDestination struct{ destination io.Writer }

func (w checkedJSONDestination) Write(data []byte) (int, error) {
	n, err := w.destination.Write(data)
	if n < 0 || n > len(data) {
		return 0, errors.Join(core.ErrJSONContract, err)
	}
	if err == nil && n != len(data) {
		return n, &nativeJSONError{cause: io.ErrShortWrite}
	}
	if err != nil {
		return n, &nativeJSONError{cause: err}
	}
	return n, nil
}
