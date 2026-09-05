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
	// cmd/go defines -overlay as a build flag whose JSON Replace map points an
	// absolute source path at an absolute backing path, or at an empty path to
	// make the source absent.
	goOverlayFlag           = "-overlay"
	goOverlayDocumentPrefix = `{"Replace":{`
	goOverlayDocumentSuffix = `}}`
	goOverlayDeletedValue   = `""`
	goOverlayFilename       = "overlay.json"
	goOverlayBufferBytes    = 64 << 10
	goOverlayActionInvalid  = "source overlay action is invalid"
)

// SourceOverlayAction is the closed cmd/go filesystem projection domain.
type SourceOverlayAction uint8

const (
	SourceOverlayActionUnknown SourceOverlayAction = iota
	SourceOverlayDelete
	SourceOverlayReplace
	sourceOverlayActionLimit
)

// Validate refuses unknown or future overlay actions.
func (a SourceOverlayAction) Validate() error {
	if a <= SourceOverlayActionUnknown || a >= sourceOverlayActionLimit {
		return contractError(goOverlayActionInvalid)
	}
	return nil
}

// IsValid reports whether a belongs to the closed overlay-action domain.
func (a SourceOverlayAction) IsValid() bool { return a.Validate() == nil }

// OffWireEnum marks SourceOverlayAction as a compiler-only enum.
func (SourceOverlayAction) OffWireEnum() {}

// String returns the stable execution identity of a valid overlay action.
func (a SourceOverlayAction) String() string {
	if !a.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{
		SourceOverlayDelete:  "delete",
		SourceOverlayReplace: "replace",
	}[a]
}

var _ core.OffWireEnum = SourceOverlayActionUnknown

// SourceOverlayMapping names one absolute cmd/go source path and its exact
// projection. Delete requires a zero Backing path; Replace requires a distinct
// absolute Backing path. Primitive owns the mechanics, not why the caller made
// the projection.
type SourceOverlayMapping struct {
	Path    core.AbsolutePath
	Backing core.AbsolutePath
	Action  SourceOverlayAction
}

// Validate refuses ambiguous, unset, relative, or no-op mappings.
func (m SourceOverlayMapping) Validate() error {
	if err := errors.Join(m.Path.Validate(), m.Action.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	switch m.Action {
	case SourceOverlayDelete:
		if m.Backing != (core.AbsolutePath{}) {
			return contractError("source overlay deletion has a backing path")
		}
	case SourceOverlayReplace:
		if err := m.Backing.Validate(); err != nil {
			return errors.Join(core.ErrGoToolchainContract, err)
		}
		if m.Backing == m.Path {
			return contractError("source overlay replacement is a no-op")
		}
	default:
		return contractError(goOverlayActionInvalid)
	}
	return nil
}

// EmitSourceOverlayMapping emits one mapping in canonical source-path order.
type EmitSourceOverlayMapping func(SourceOverlayMapping) error

// StreamSourceOverlayMappings streams a complete canonical mapping set.
type StreamSourceOverlayMappings func(EmitSourceOverlayMapping) error

// SourceOverlayRequest supplies a caller-owned directory and the complete
// canonical mapping stream. Primitive creates only its one owned file.
type SourceOverlayRequest struct {
	Mappings  StreamSourceOverlayMappings
	Directory core.AbsolutePath
}

// Validate refuses an unset directory or mapping stream.
func (r SourceOverlayRequest) Validate() error {
	if r.Mappings == nil {
		return contractError("source overlay mapping stream is nil")
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
// canonical mapping stream. It retains only the preceding path and count.
type sourceOverlayEncoder struct {
	ctx      context.Context
	err      error
	writer   *bufio.Writer
	previous string
	count    uint64
}

// OpenSourceOverlay streams a nonempty, unique, canonically ordered mapping
// set (deletions or replacements) to the caller-owned directory without
// materializing the set in memory.
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
	if err := overlay.write(ctx, request.Mappings); err != nil {
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

func (o SourceOverlay) write(ctx context.Context, stream StreamSourceOverlayMappings) error {
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

func writeSourceOverlay(ctx context.Context, writer *bufio.Writer, stream StreamSourceOverlayMappings) error {
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

func (e *sourceOverlayEncoder) emit(mapping SourceOverlayMapping) error {
	if e.err != nil {
		return e.err
	}
	e.err = e.writeMapping(mapping)
	return e.err
}

func (e *sourceOverlayEncoder) writeMapping(mapping SourceOverlayMapping) error {
	if err := e.ctx.Err(); err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	if err := mapping.Validate(); err != nil {
		return err
	}
	current := mapping.Path.String()
	if e.previous != "" && e.previous >= current {
		return contractError("source overlay mappings are duplicated or not canonical")
	}
	key, err := core.MarshalCanonicalJSONString(current)
	if err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	value, err := sourceOverlayMappingValue(mapping)
	if err != nil {
		return err
	}
	if err := e.writeMappingPair(key, value); err != nil {
		return err
	}
	e.previous = current
	e.count++
	return nil
}

func (e *sourceOverlayEncoder) writeMappingPair(key, value []byte) error {
	if err := e.writeSeparator(); err != nil {
		return err
	}
	if _, err := e.writer.Write(key); err != nil {
		return err
	}
	if err := e.writer.WriteByte(':'); err != nil {
		return err
	}
	if _, err := e.writer.Write(value); err != nil {
		return err
	}
	return nil
}

func sourceOverlayMappingValue(mapping SourceOverlayMapping) ([]byte, error) {
	if mapping.Action == SourceOverlayDelete {
		return []byte(goOverlayDeletedValue), nil
	}
	value, err := core.MarshalCanonicalJSONString(mapping.Backing.String())
	if err != nil {
		return nil, errors.Join(core.ErrGoToolchainContract, err)
	}
	return value, nil
}

func (e *sourceOverlayEncoder) writeSeparator() error {
	if e.count == 0 {
		return nil
	}
	return e.writer.WriteByte(',')
}

func (e *sourceOverlayEncoder) finish() error {
	if e.count == 0 {
		return contractError("source overlay mapping stream is empty")
	}
	if err := e.ctx.Err(); err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	_, err := e.writer.WriteString(goOverlayDocumentSuffix)
	return err
}
