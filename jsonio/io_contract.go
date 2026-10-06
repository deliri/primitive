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
	return n, err
}

type checkedJSONDestination struct{ destination io.Writer }

func (w checkedJSONDestination) Write(data []byte) (int, error) {
	n, err := w.destination.Write(data)
	if n < 0 || n > len(data) {
		return 0, errors.Join(core.ErrJSONContract, err)
	}
	if err == nil && n != len(data) {
		return n, io.ErrShortWrite
	}
	return n, err
}
