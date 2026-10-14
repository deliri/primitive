package jsonio_test

import (
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// A broken source has not supplied a document whose grammar can be judged.
func TestJSONNativeReadRefusalIsNotMalformedJSON(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{io.ErrClosedPipe, io.ErrUnexpectedEOF, errors.Join(io.EOF, io.ErrClosedPipe)} {
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			got := jsonio.ValidateDocument(t.Context(), iotest.ErrReader(cause))
			if !errors.Is(got, cause) || errors.Is(got, core.ErrJSONContract) {
				t.Fatalf("ValidateDocument(source refusal) = %v, want native %v without JSON rejection", got, cause)
			}
		})
	}
}
