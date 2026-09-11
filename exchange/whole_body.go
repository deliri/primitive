package exchange

import (
	"bytes"
	"context"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// wholeBodyRead is used only by APIs whose contract returns an owned whole value.
// Streaming callers supply a destination to Download, RoundTripStream or ReceiveStream.
type wholeBodyRead struct {
	context  context.Context
	source   io.Reader
	declared declaredBodyLength
}

func readWholeBody(input wholeBodyRead) ([]byte, error) {
	if err := input.declared.Validate(); err != nil {
		return nil, err
	}
	if core.ReaderIsNil(input.source) {
		return nil, core.ErrExchangeContract
	}
	var destination wholeBodyDestination
	buffer := make([]byte, wholeBodyCopyWindow(input.declared))
	_, err := copyDownload(downloadCopyRequest{context: input.context, source: input.source, destination: &destination, buffer: buffer})
	if err != nil {
		return nil, err
	}
	if destination.buffer.Len() == 0 {
		return nil, nil
	}
	return destination.buffer.Bytes(), nil
}

// wholeBodyDestination makes Go grow the retained value only after a successful
// read supplies bytes. Hiding Buffer.ReadFrom avoids growing a full allocation
// merely to discover EOF. Go still owns both allocation and the copy loop.
type wholeBodyDestination struct{ buffer bytes.Buffer }

func (d *wholeBodyDestination) Write(data []byte) (int, error) {
	return d.buffer.Write(data)
}

// The declaration only sizes working scratch; it cannot limit admitted bytes.
func wholeBodyCopyWindow(declared declaredBodyLength) int {
	if !declared.present {
		return TransferBufferBytes
	}
	// #nosec G115 -- min caps this conversion at the compiler-owned TransferBufferBytes scratch size.
	return int(min(uint64(TransferBufferBytes), max(uint64(bytes.MinRead), declared.length.Uint64())))
}
