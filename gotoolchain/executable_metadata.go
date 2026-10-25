package gotoolchain

import (
	"context"
	"debug/buildinfo"
	"errors"
	"io"
	"os"
	"runtime/debug"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// ExecutableMetadataHeader lends Go's fixed executable header. Main and its
// replacement remain native compiler data, not authenticated module authority.
type ExecutableMetadataHeader struct {
	GoVersion string
	Path      string
	Main      debug.Module
}

// ExecutableMetadataRequest borrows one regular file and synchronous visitors.
// The caller owns the descriptor and excludes writes throughout the call.
// Visitors must not mutate or retain borrowed native module pointers. They
// promote interpreted coordinates and settings into their owning policy types.
type ExecutableMetadataRequest struct {
	File       *os.File
	Header     func(ExecutableMetadataHeader) error
	Dependency func(debug.Module) error
	Setting    func(debug.BuildSetting) error
}

func (r ExecutableMetadataRequest) Validate() error {
	if r.File == nil || r.Header == nil || r.Dependency == nil || r.Setting == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}

// ObserveExecutableMetadata reads a held executable through Go's buildinfo
// parser, hashes its complete captured extent, then lends native facts in their
// original order. It neither closes the descriptor nor changes its offset or
// reopens its former path. No executable is run.
//
// The returned content entry binds the observed bytes only after every visitor
// and final extent check succeeds. Visitor prefixes remain provisional until
// then. This is an observation, not authentication or an atomic snapshot against
// concurrent writers. Go owns executable-parser and BuildInfo allocations; the
// additional hash window is fixed and no metadata collection is copied here.
// Cancellation is checked between native reads and visits; an in-flight regular
// file read or native parser call has no interruption guarantee.
func ObserveExecutableMetadata(ctx context.Context, request ExecutableMetadataRequest) (filestore.ContentIndexEntry, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return filestore.ContentIndexEntry{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	extent, err := executableMetadataExtent(ctx, request.File)
	if err != nil {
		return filestore.ContentIndexEntry{}, err
	}
	bound, err := extent.Int64()
	if err != nil {
		return filestore.ContentIndexEntry{}, outputError("executable metadata source extent is not representable", err)
	}
	source := executableMetadataSource{ctx: ctx, file: request.File}
	info, parseErr := buildinfo.Read(io.NewSectionReader(&source, 0, bound))
	if err := errors.Join(parseErr, source.readError, contextstate.Validate(ctx)); err != nil {
		return filestore.ContentIndexEntry{}, outputError("read executable Go metadata", err)
	}
	content, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{
		Source: io.NewSectionReader(&source, 0, bound), Destination: io.Discard,
	})
	if err != nil {
		return filestore.ContentIndexEntry{}, outputError("digest executable Go metadata source", err)
	}
	if content.Extent != extent {
		return filestore.ContentIndexEntry{}, outputError("executable metadata source shrank", io.ErrUnexpectedEOF)
	}
	if err := errors.Join(visitExecutableMetadata(ctx, request, info), contextstate.Validate(ctx)); err != nil {
		return filestore.ContentIndexEntry{}, outputError("visit executable Go metadata", err)
	}
	finalExtent, err := executableMetadataExtent(ctx, request.File)
	if err != nil {
		return filestore.ContentIndexEntry{}, err
	}
	if finalExtent != extent {
		return filestore.ContentIndexEntry{}, outputError("executable metadata source extent changed", nil)
	}
	var extra [1]byte
	count, readErr := source.ReadAt(extra[:], bound)
	if count != 0 || readErr != io.EOF {
		return filestore.ContentIndexEntry{}, outputError("executable metadata source exceeds captured extent", readErr)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return filestore.ContentIndexEntry{}, outputError("executable metadata observation cancelled", err)
	}
	return content, nil
}

func executableMetadataExtent(ctx context.Context, file *os.File) (core.ByteLength, error) {
	inspection, err := filestore.InspectOpenFile(ctx, filestore.HandleInspectionRequest{File: file})
	if err != nil {
		return core.ByteLength{}, outputError("inspect executable metadata source", err)
	}
	extent, err := inspection.SizeBytes()
	if err != nil {
		return core.ByteLength{}, outputError("executable metadata source is not regular", err)
	}
	return extent, nil
}

func visitExecutableMetadata(ctx context.Context, request ExecutableMetadataRequest, info *debug.BuildInfo) error {
	if err := contextstate.Validate(ctx); err != nil {
		return outputError("visit executable Go header", err)
	}
	if err := request.Header(ExecutableMetadataHeader{GoVersion: info.GoVersion, Path: info.Path, Main: info.Main}); err != nil {
		return outputError("visit executable Go header", err)
	}
	for _, module := range info.Deps {
		if err := contextstate.Validate(ctx); err != nil {
			return outputError("visit executable Go dependency", err)
		}
		if err := request.Dependency(*module); err != nil {
			return outputError("visit executable Go dependency", err)
		}
	}
	for _, setting := range info.Settings {
		if err := contextstate.Validate(ctx); err != nil {
			return outputError("visit executable Go setting", err)
		}
		if err := request.Setting(setting); err != nil {
			return outputError("visit executable Go setting", err)
		}
	}
	return nil
}

// executableMetadataSource preserves native read failures even when Go's
// executable-format probe replaces its diagnostic. Exact EOF is normal probing.
type executableMetadataSource struct {
	ctx       context.Context
	file      *os.File
	readError error
}

func (s *executableMetadataSource) ReadAt(data []byte, offset int64) (int, error) {
	if err := contextstate.Validate(s.ctx); err != nil {
		return 0, err
	}
	count, err := filestore.ReadAt(s.ctx, filestore.ReadAtRequest{File: s.file, Data: data, Offset: offset})
	if err != nil && err != io.EOF && s.readError == nil {
		s.readError = err
	}
	if contextErr := contextstate.Validate(s.ctx); contextErr != nil {
		return count, errors.Join(err, contextErr)
	}
	return count, err
}

func (ExecutableMetadataHeader) goToolchainProtocolFact()  {}
func (ExecutableMetadataRequest) goToolchainProtocolFact() {}
func (executableMetadataSource) goToolchainInternalFlow()  {}

var _ core.Validatable = ExecutableMetadataRequest{}
