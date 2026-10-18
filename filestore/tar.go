package filestore

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

type TARCompression uint8

const (
	TARCompressionUnknown TARCompression = iota
	TARPlain
	TARGzip
)

func (k TARCompression) IsValid() bool { return k == TARPlain || k == TARGzip }
func (k TARCompression) Validate() error {
	if !k.IsValid() {
		return core.ErrFilestoreContract
	}
	return nil
}
func (TARCompression) OffWireEnum() {}
func (k TARCompression) String() string {
	switch k {
	case TARPlain:
		return "plain"
	case TARGzip:
		return "gzip"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// TAREntry borrows one regular file. The declared native extent is exact;
// archive/tar owns framing and rejects excess bytes. Header clocks, ownership
// names and other optional metadata retain Go's deterministic zero values.
type TAREntry struct {
	Source io.Reader
	Path   core.RelativePath
	Bytes  core.ByteLength
	Mode   fs.FileMode
}

func (e TAREntry) Validate() error {
	if core.ReaderIsNil(e.Source) || e.Path.String() == "." {
		return core.ErrFilestoreContract
	}
	if _, err := e.Bytes.Int64(); err != nil {
		return err
	}
	return errors.Join(e.Path.Validate(), e.Bytes.Validate(), validatePermissionMode(e.Mode))
}

// TARWriteRequest lends a synchronous entry producer and one destination.
// Add consumes each source before returning its content observation. A refused
// entry remains a failure even if Produce ignores it. No entry inventory or
// volume quota is retained. Partial output remains caller-owned on failure.
type TARWriteRequest struct {
	Destination io.Writer
	Produce     func(context.Context, func(TAREntry) (ContentIndexEntry, error)) error
	Compression TARCompression
}

func (r TARWriteRequest) Validate() error {
	if core.WriterIsNil(r.Destination) || r.Produce == nil {
		return core.ErrFilestoreContract
	}
	return r.Compression.Validate()
}

// WriteTAR delegates TAR framing, optional gzip, copies and closure to Go.
// It retains fixed working windows and returns the compressed output's digest
// and extent only after both native encoders close successfully.
func WriteTAR(ctx context.Context, request TARWriteRequest) (content ContentIndexEntry, resultErr error) {
	defer func() {
		if resultErr != nil {
			content = ContentIndexEntry{}
			if ctx != nil {
				resultErr = errors.Join(resultErr, context.Cause(ctx))
			}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ContentIndexEntry{}, err
	}
	digest := core.NewDigestWriter()
	output := io.MultiWriter(request.Destination, digest)
	var encoded io.Writer = output
	var compression *gzip.Writer
	if request.Compression == TARGzip {
		compression = gzip.NewWriter(output)
		encoded = compression
	}
	archive := tar.NewWriter(encoded)
	var entryErr error
	defer func() {
		if recover() != nil {
			resultErr = errors.Join(resultErr, core.ErrFilestoreCallbackPanic)
		}
		resultErr = errors.Join(resultErr, entryErr, archive.Close())
		if compression != nil {
			resultErr = errors.Join(resultErr, compression.Close())
		}
		resultErr = errors.Join(resultErr, contextstate.Validate(ctx))
		if resultErr == nil {
			sum, extent, err := digest.Seal()
			resultErr = err
			if err == nil {
				content = ContentIndexEntry{Digest: sum, Extent: extent}
			}
		}
	}()
	resultErr = request.Produce(ctx, func(entry TAREntry) (ContentIndexEntry, error) {
		if entryErr != nil {
			return ContentIndexEntry{}, entryErr
		}
		observed, err := writeTAREntry(ctx, archive, entry)
		entryErr = err
		return observed, err
	})
	return content, resultErr
}
func writeTAREntry(ctx context.Context, archive *tar.Writer, entry TAREntry) (ContentIndexEntry, error) {
	if err := errors.Join(contextstate.Validate(ctx), entry.Validate()); err != nil {
		return ContentIndexEntry{}, err
	}
	size, err := entry.Bytes.Int64()
	if err != nil {
		return ContentIndexEntry{}, err
	}
	if err := archive.WriteHeader(&tar.Header{Name: filepath.ToSlash(entry.Path.String()), Mode: int64(entry.Mode.Perm()), Size: size, Typeflag: tar.TypeReg}); err != nil {
		return ContentIndexEntry{}, err
	}
	content, err := CopyContent(ctx, CopyContentRequest{Source: entry.Source, Destination: archive})
	if err != nil {
		return ContentIndexEntry{}, err
	}
	if content.Extent != entry.Bytes {
		return ContentIndexEntry{}, sizeError(io.ErrUnexpectedEOF)
	}
	return content, nil
}

var (
	_ core.Validatable = TARCompressionUnknown
	_ core.Validatable = TAREntry{}
	_ core.Validatable = TARWriteRequest{}
)
