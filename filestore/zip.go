package filestore

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type ZIPCompression uint8

const (
	ZIPCompressionUnknown ZIPCompression = iota
	ZIPStore
	ZIPDeflate
)

func (c ZIPCompression) IsValid() bool { return c == ZIPStore || c == ZIPDeflate }

func (c ZIPCompression) Validate() error {
	if !c.IsValid() {
		return core.ErrFilestoreContract
	}
	return nil
}

func (c ZIPCompression) String() string {
	switch c {
	case ZIPStore:
		return "store"
	case ZIPDeflate:
		return "deflate"
	default:
		return core.UnknownEnumDiagnostic
	}
}

func (ZIPCompression) OffWireEnum() {}

// ZIPEntry lends one regular file's content. The producer owns its source
// lifetime and metadata. WriteZIP consumes the entry before Add returns.
type ZIPEntry struct {
	Source      io.Reader
	Path        core.RelativePath
	Modified    temporal.Instant
	Mode        fs.FileMode
	Compression ZIPCompression
}

func (e ZIPEntry) Validate() error {
	if core.ReaderIsNil(e.Source) || e.Path.String() == "." {
		return core.ErrFilestoreContract
	}
	return errors.Join(e.Path.Validate(), e.Modified.Validate(), validatePermissionMode(e.Mode), e.Compression.Validate())
}

// ZIPWriteRequest supplies entries synchronously, in archive order. Produce
// must not retain Add or borrowed sources. The first entry refusal survives
// even when the producer ignores it or subsequently panics.
// The destination remains caller-owned, including disposal of partial output.
type ZIPWriteRequest struct {
	Destination io.Writer
	Produce     func(context.Context, func(ZIPEntry) error) error
}

func (r ZIPWriteRequest) Validate() error {
	if core.WriterIsNil(r.Destination) || r.Produce == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// WriteZIP delegates headers, compression, central directory and closure to
// archive/zip. Content copies use fixed native windows without extent quotas.
// Go's ZIP writer retains central-directory metadata per entry; this function
// does not claim constant memory with respect to the number of archive entries.
func WriteZIP(ctx context.Context, request ZIPWriteRequest) (resultErr error) {
	defer func() {
		if resultErr != nil && ctx != nil {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	archive := zip.NewWriter(request.Destination)
	var window []byte
	var entryErr error
	defer func() {
		if recover() != nil {
			resultErr = errors.Join(resultErr, core.ErrFilestoreCallbackPanic)
		}
		resultErr = errors.Join(resultErr, entryErr, archive.Close(), contextstate.Validate(ctx))
	}()
	produceErr := request.Produce(ctx, func(entry ZIPEntry) error {
		if entryErr == nil {
			entryErr = writeZIPEntry(ctx, archive, entry, &window)
		}
		return entryErr
	})
	return errors.Join(produceErr, entryErr)
}

// writeZIPEntry copies through one archive-owned window, allocated once by the
// first entry and reused for every later entry instead of one Go window each.
func writeZIPEntry(ctx context.Context, archive *zip.Writer, entry ZIPEntry, window *[]byte) error {
	if err := errors.Join(contextstate.Validate(ctx), entry.Validate()); err != nil {
		return err
	}
	modified, err := entry.Modified.Time()
	if err != nil {
		return err
	}
	method := uint16(zip.Store)
	if entry.Compression == ZIPDeflate {
		method = zip.Deflate
	}
	header := &zip.FileHeader{Name: filepath.ToSlash(entry.Path.String()), Method: method, Modified: modified}
	header.SetMode(entry.Mode)
	destination, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	if *window == nil {
		*window = make([]byte, goCopyWindowBytes)
	}
	_, err = copyStream(streamCopyRequest{ctx: ctx, source: entry.Source, destination: destination, buffer: *window, kind: streamDestinationCaller})
	return err
}

var (
	_ core.Validatable = ZIPCompressionUnknown
	_ core.Validatable = ZIPEntry{}
	_ core.Validatable = ZIPWriteRequest{}
)
