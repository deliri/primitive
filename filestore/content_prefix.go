package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// CopyPrefixRequest selects an exact prefix of a borrowed stream. Bytes is an
// operation extent, not an input quota. The next source byte remains unread.
// Buffer is optional fixed working storage; neither stream is closed.
type CopyPrefixRequest struct {
	Source      io.Reader
	Destination io.Writer
	Bytes       core.ByteLength
	Buffer      []byte
}

func (r CopyPrefixRequest) Validate() error {
	return errors.Join(r.Bytes.Validate(), (CopyContentRequest{Source: r.Source, Destination: r.Destination, Buffer: r.Buffer}).Validate())
}

// CopyPrefix observes only a completely copied prefix. Native read/write and
// cancellation failures withdraw the content observation while preserving their
// causes; provisional destination bytes remain caller-owned.
func CopyPrefix(ctx context.Context, request CopyPrefixRequest) (content ContentIndexEntry, resultErr error) {
	defer func() {
		if ctx != nil && (errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded)) {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := request.Validate(); err != nil {
		return ContentIndexEntry{}, err
	}
	extent, err := request.Bytes.Int64()
	if err != nil {
		return ContentIndexEntry{}, err
	}
	content, err = CopyContent(ctx, CopyContentRequest{Source: &io.LimitedReader{R: request.Source, N: extent}, Destination: request.Destination, Buffer: request.Buffer})
	if err != nil {
		return ContentIndexEntry{}, err
	}
	if content.Extent != request.Bytes {
		return ContentIndexEntry{}, sourceError(io.ErrUnexpectedEOF)
	}
	return content, nil
}

var _ core.Validatable = CopyPrefixRequest{}
