package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// PrefixReadRequest lends a fixed observation window. Its size selects prefix
// data, never a total source ceiling. The source and destination remain borrowed.
type PrefixReadRequest struct {
	Source      io.Reader
	Destination []byte
}

func (r PrefixReadRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrFilestoreContract
	}
	return nil
}

type PrefixReadObservation struct{ Bytes core.ByteLength }

func (r PrefixReadObservation) Validate() error { return r.Bytes.Validate() }

// ReadPrefix fills at most the caller's window through native io.ReadFull.
// Short clean EOF succeeds. It performs no read beyond the window. A source
// failure observed with the final bytes still refuses, clears provisional data,
// and preserves its cause. Invalid ingress leaves the destination untouched.
func ReadPrefix(ctx context.Context, request PrefixReadRequest) (result PrefixReadObservation, resultErr error) {
	defer func() {
		if resultErr != nil && ctx != nil {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return PrefixReadObservation{}, err
	}
	source := streamReader{ctx: ctx, source: request.Source}
	n, err := io.ReadFull(&source, request.Destination)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	if err := errors.Join(err, source.cause, contextstate.Validate(ctx)); err != nil {
		clear(request.Destination)
		return PrefixReadObservation{}, err
	}
	size, err := core.NewByteLength(uint64(n))
	if err != nil {
		clear(request.Destination)
		return PrefixReadObservation{}, err
	}
	return PrefixReadObservation{Bytes: size}, nil
}
