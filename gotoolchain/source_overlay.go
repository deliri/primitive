package gotoolchain

import (
	"bufio"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const (
	// Source: https://go.dev/cmd/go/#hdr-Compile_packages_and_dependencies
	// cmd/go defines -overlay as a build flag whose JSON Replace map uses an
	// empty backing path to make the named disk path absent.
	goOverlayFlag           = "-overlay"
	goOverlayDocumentPrefix = `{"Replace":{`
	goOverlayDocumentSuffix = `}}`
	goOverlayDeletedValue   = `""`
	goOverlayFilename       = "overlay.json"
	goOverlayBufferBytes    = 64 << 10
)

// SourceOverlayDeletion names one absolute disk path that cmd/go must observe
// as absent. Primitive owns only this filesystem projection, not why a caller
// rejected the path.
type SourceOverlayDeletion struct {
	Path core.AbsolutePath
}

// Validate refuses an unset or relative deletion path.
func (d SourceOverlayDeletion) Validate() error {
	if err := d.Path.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

// EmitSourceOverlayDeletion emits one deletion in canonical path order.
type EmitSourceOverlayDeletion func(SourceOverlayDeletion) error

// StreamSourceOverlayDeletions streams a complete canonical deletion set.
type StreamSourceOverlayDeletions func(EmitSourceOverlayDeletion) error

// SourceOverlayRequest supplies a caller-owned directory and the complete
// canonical deletion stream. Primitive creates only its one owned file.
type SourceOverlayRequest struct {
	Directory core.AbsolutePath
	Deletions StreamSourceOverlayDeletions
}

// Validate refuses an unset directory or deletion stream.
func (r SourceOverlayRequest) Validate() error {
	if r.Deletions == nil {
		return contractError("source overlay deletion stream is nil")
	}
	if err := r.Directory.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

// SourceOverlay is one materialized cmd/go overlay capability. Close removes
// its exact file after every command using it waits.
type SourceOverlay struct {
	path      core.AbsolutePath
	directory core.AbsolutePath
}

// sourceOverlayEncoder is the bounded internal flow state needed to project a
// canonical deletion stream. It retains only the preceding path and count.
type sourceOverlayEncoder struct {
	ctx      context.Context
	writer   *bufio.Writer
	previous string
	count    uint64
	err      error
}

// OpenSourceOverlay streams a nonempty, unique, canonically ordered deletion
// set to the caller-owned directory without materializing the set in memory.
func OpenSourceOverlay(ctx context.Context, request SourceOverlayRequest) (SourceOverlay, error) {
	if ctx == nil {
		return SourceOverlay{}, contractError("source overlay context is nil")
	}
	if err := request.Validate(); err != nil {
		return SourceOverlay{}, err
	}
	if err := ctx.Err(); err != nil {
		return SourceOverlay{}, errors.Join(core.ErrGoToolchainExecution, err)
	}
	path, err := request.Directory.Resolve(goOverlayFilename)
	if err != nil {
		return SourceOverlay{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	overlay := SourceOverlay{path: path, directory: request.Directory}
	if err := overlay.write(ctx, request.Deletions); err != nil {
		return SourceOverlay{}, err
	}
	return overlay, nil
}

// Validate proves the overlay path is the one file directly beneath its
// caller-owned directory.
func (o SourceOverlay) Validate() error {
	if err := errors.Join(o.path.Validate(), o.directory.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if filepath.Dir(o.path.String()) != o.directory.String() || filepath.Base(o.path.String()) != goOverlayFilename {
		return contractError("source overlay path escapes its owned directory")
	}
	return nil
}

// Argument returns the exact typed cmd/go build argument for this overlay.
func (o SourceOverlay) Argument() (process.Argument, error) {
	if err := o.Validate(); err != nil {
		return process.Argument{}, err
	}
	argument, err := process.NewArgument(goOverlayFlag + "=" + o.path.String())
	if err != nil {
		return process.Argument{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	return argument, nil
}

// Close removes only the exact overlay file from the caller-owned directory.
// Repeated closes are neutral.
func (o SourceOverlay) Close() error {
	if err := o.Validate(); err != nil {
		return err
	}
	if err := removeOverlayPath(o.path.String()); err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	return nil
}

func removeOverlayPath(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (o SourceOverlay) write(ctx context.Context, stream StreamSourceOverlayDeletions) error {
	file, err := os.OpenFile(o.path.String(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	defer func() { _ = file.Close() }()
	writer := bufio.NewWriterSize(file, goOverlayBufferBytes)
	writeErr := writeSourceOverlay(ctx, writer, stream)
	flushErr := writer.Flush()
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, flushErr, syncErr, closeErr); err != nil {
		cleanupErr := removeOverlayPath(o.path.String())
		return errors.Join(core.ErrGoToolchainExecution, err, cleanupErr)
	}
	return nil
}

func writeSourceOverlay(ctx context.Context, writer *bufio.Writer, stream StreamSourceOverlayDeletions) error {
	if _, err := writer.WriteString(goOverlayDocumentPrefix); err != nil {
		return err
	}
	encoder := sourceOverlayEncoder{ctx: ctx, writer: writer}
	streamErr := stream(encoder.emit)
	if err := errors.Join(encoder.err, streamErr); err != nil {
		return err
	}
	return encoder.finish()
}

func (e *sourceOverlayEncoder) emit(deletion SourceOverlayDeletion) error {
	if e.err != nil {
		return e.err
	}
	e.err = e.writeDeletion(deletion)
	return e.err
}

func (e *sourceOverlayEncoder) writeDeletion(deletion SourceOverlayDeletion) error {
	if err := e.ctx.Err(); err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	if err := deletion.Validate(); err != nil {
		return err
	}
	current := deletion.Path.String()
	if e.previous != "" && e.previous >= current {
		return contractError("source overlay deletions are duplicated or not canonical")
	}
	key, err := core.MarshalCanonicalJSONString(current)
	if err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if err := e.writeSeparator(); err != nil {
		return err
	}
	if _, err := e.writer.Write(key); err != nil {
		return err
	}
	if _, err := e.writer.WriteString(":" + goOverlayDeletedValue); err != nil {
		return err
	}
	e.previous = current
	e.count++
	return nil
}

func (e *sourceOverlayEncoder) writeSeparator() error {
	if e.count == 0 {
		return nil
	}
	return e.writer.WriteByte(',')
}

func (e *sourceOverlayEncoder) finish() error {
	if e.count == 0 {
		return contractError("source overlay deletion stream is empty")
	}
	if err := e.ctx.Err(); err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	_, err := e.writer.WriteString(goOverlayDocumentSuffix)
	return err
}
