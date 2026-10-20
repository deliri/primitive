package filestore

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TARReadEntry lends Go's current header and its exact member body for one
// synchronous callback. Native names and entry kinds are observations, not
// permission to extract a path. The consumer owns that policy. Neither value
// may be retained or used after the callback returns.
type TARReadEntry struct {
	Header *tar.Header
	Source io.Reader
}

func (e TARReadEntry) Validate() error {
	if e.Header == nil || core.ReaderIsNil(e.Source) {
		return core.ErrFilestoreContract
	}
	return nil
}

type TARReadRequest struct {
	Source      io.Reader
	Visit       func(context.Context, TARReadEntry) error
	Compression TARCompression
}

func (r TARReadRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.Visit == nil {
		return core.ErrFilestoreContract
	}
	return r.Compression.Validate()
}

// ReadTAR delegates framing, member extents and optional gzip to Go. Unread
// member bytes are discarded by Go before the next loan. Success requires the
// native gzip checksum and footer, even when TAR's end blocks precede them.
// A callback refusal stops immediately; the caller retains its source. No
// member catalog, extraction namespace or transfer quota is constructed.
func ReadTAR(ctx context.Context, request TARReadRequest) (resultErr error) {
	defer func() {
		if recover() != nil {
			resultErr = errors.Join(resultErr, core.ErrFilestoreCallbackPanic)
		}
		if ctx != nil {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	var decoded io.Reader = &streamReader{ctx: ctx, source: request.Source}
	var compression *gzip.Reader
	if request.Compression == TARGzip {
		var err error
		compression, err = gzip.NewReader(decoded)
		if err != nil {
			return sourceError(err)
		}
		defer func() { resultErr = errors.Join(resultErr, compression.Close()) }()
		decoded = compression
	}
	archive := tar.NewReader(decoded)
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		header, err := archive.Next()
		if err == io.EOF {
			if compression != nil {
				_, err := copyStream(streamCopyRequest{ctx: ctx, source: compression, destination: io.Discard, kind: streamDestinationCaller})
				return err
			}
			return nil
		}
		if err != nil {
			return sourceError(err)
		}
		entry := TARReadEntry{Header: header, Source: &streamReader{ctx: ctx, source: archive}}
		if err := request.Visit(ctx, entry); err != nil {
			return err
		}
	}
}

var (
	_ core.Validatable = TARReadEntry{}
	_ core.Validatable = TARReadRequest{}
)
