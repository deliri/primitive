package filestore

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
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

// TAREntry borrows one regular file or describes one header-only directory or
// symbolic link. Regular sources have exact extents and no link target. Other
// entries have no source or extent; only symbolic links carry a native opaque
// target. Targets are encoded verbatim, never interpreted or followed. Mode
// carries only permission bits, including 0000. Optional header metadata keeps
// Go's deterministic zero values.
type TAREntry struct {
	Source     io.Reader
	Path       core.RelativePath
	Bytes      core.ByteLength
	LinkTarget SymbolicLinkTarget
	Mode       fs.FileMode
	Kind       PathKind
}

func (e TAREntry) Validate() error {
	if e.Path.String() == "." || e.Mode != e.Mode.Perm() {
		return core.ErrFilestoreContract
	}
	if _, err := e.Bytes.Int64(); err != nil {
		return err
	}
	if err := errors.Join(e.Path.Validate(), e.Bytes.Validate()); err != nil {
		return err
	}
	switch e.Kind {
	case PathKindRegularFile:
		if core.ReaderIsNil(e.Source) || e.LinkTarget != (SymbolicLinkTarget{}) {
			return core.ErrFilestoreContract
		}
	case PathKindDirectory:
		if e.Source != nil || e.Bytes != (core.ByteLength{}) || e.LinkTarget != (SymbolicLinkTarget{}) {
			return core.ErrFilestoreContract
		}
	case PathKindSymbolicLink:
		if e.Source != nil || e.Bytes != (core.ByteLength{}) {
			return core.ErrFilestoreContract
		}
		return e.LinkTarget.Validate()
	default:
		return core.ErrFilestoreContract
	}
	return nil
}

// TARWriteRequest lends a synchronous entry producer and one destination.
// Add consumes each regular source before returning its content observation.
// Header-only entries return the empty-body digest and zero extent; the final
// archive observation covers their encoded headers. A refused
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
func WriteTAR(ctx context.Context, request TARWriteRequest) (ContentIndexEntry, error) {
	content, _, err := writeTAR(ctx, request, core.NewDigestWriter())
	return content, err
}

// StreamTAR writes the same native archive and per-entry content observations
// as WriteTAR, without hashing the whole encoded archive. It returns its exact
// encoded extent only after the TAR and optional gzip encoder close. Failure
// returns zero; borrowed streams and provisional output remain caller-owned.
func StreamTAR(ctx context.Context, request TARWriteRequest) (core.ByteLength, error) {
	_, extent, err := writeTAR(ctx, request, nil)
	return extent, err
}

func writeTAR(ctx context.Context, request TARWriteRequest, digest *core.DigestWriter) (content ContentIndexEntry, extent core.ByteLength, resultErr error) {
	defer func() {
		if resultErr != nil {
			content = ContentIndexEntry{}
			extent = core.ByteLength{}
			if ctx != nil {
				resultErr = errors.Join(resultErr, context.Cause(ctx))
			}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ContentIndexEntry{}, core.ByteLength{}, err
	}
	var output io.Writer = request.Destination
	var counted *tarStreamDestination
	if digest != nil {
		output = io.MultiWriter(request.Destination, digest)
	} else {
		counted = &tarStreamDestination{destination: request.Destination}
		output = counted
	}
	var encoded io.Writer = output
	var compression *gzip.Writer
	if request.Compression == TARGzip {
		compression = gzip.NewWriter(output)
		encoded = compression
	}
	archive := tar.NewWriter(encoded)
	var window []byte
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
		if resultErr == nil && digest == nil {
			extent, resultErr = core.NewByteLength(counted.count)
		} else if resultErr == nil {
			sum, measured, err := digest.Seal()
			resultErr = err
			if err == nil {
				content = ContentIndexEntry{Digest: sum, Extent: measured}
				extent = measured
			}
		}
	}()
	resultErr = request.Produce(ctx, func(entry TAREntry) (ContentIndexEntry, error) {
		if entryErr != nil {
			return ContentIndexEntry{}, entryErr
		}
		observed, err := writeTAREntry(ctx, archive, entry, &window)
		entryErr = err
		return observed, err
	})
	return content, extent, resultErr
}

// writeTAREntry copies through one archive-owned window, allocated once by the
// first entry and reused for every later entry instead of one Go window each.
func writeTAREntry(ctx context.Context, archive *tar.Writer, entry TAREntry, window *[]byte) (ContentIndexEntry, error) {
	if err := errors.Join(contextstate.Validate(ctx), entry.Validate()); err != nil {
		return ContentIndexEntry{}, err
	}
	size, err := entry.Bytes.Int64()
	if err != nil {
		return ContentIndexEntry{}, err
	}
	header := tar.Header{Name: filepath.ToSlash(entry.Path.String()), Mode: int64(entry.Mode.Perm()), Size: size}
	switch entry.Kind {
	case PathKindRegularFile:
		header.Typeflag = tar.TypeReg
	case PathKindDirectory:
		header.Typeflag = tar.TypeDir
	case PathKindSymbolicLink:
		header.Typeflag = tar.TypeSymlink
		header.Linkname = entry.LinkTarget.String()
	default:
		return ContentIndexEntry{}, core.ErrFilestoreContract
	}
	if err := archive.WriteHeader(&header); err != nil {
		return ContentIndexEntry{}, err
	}
	if entry.Kind != PathKindRegularFile {
		return ContentIndexEntry{Digest: core.SHA256Of(nil)}, nil
	}
	if *window == nil {
		*window = make([]byte, goCopyWindowBytes)
	}
	content, err := CopyContent(ctx, CopyContentRequest{Source: entry.Source, Destination: archive, Buffer: *window})
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

// tarStreamDestination keeps only the encoded extent. It preserves the signed
// output domain formerly enforced by the archive digest, without hashing.
type tarStreamDestination struct {
	destination io.Writer
	count       uint64
	err         error
}

func (w *tarStreamDestination) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.count > math.MaxInt64 || uint64(len(data)) > math.MaxInt64-w.count {
		w.err = core.ErrNumericOverflow
		return 0, w.err
	}
	writer := streamWriter{destination: w.destination, kind: streamDestinationCaller}
	n, err := writer.Write(data)
	w.count += uint64(n)
	w.err = err
	return n, err
}
