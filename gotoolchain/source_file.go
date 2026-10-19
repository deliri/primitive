package gotoolchain

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"go/ast"
	"go/parser"
	"go/token"
)

// FileParseRequest selects an actual source file and the caller's native
// position set. Parsing includes comments and Go's ordinary object resolution;
// this is a syntax observation, not compilation or build-file selection.
type FileParseRequest struct {
	FileSet *token.FileSet
	Path    core.AbsolutePath
}

func (r FileParseRequest) Validate() error {
	if r.FileSet == nil {
		return core.ErrGoToolchainContract
	}
	if err := r.Path.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

func (FileParseRequest) goToolchainProtocolFact() {}

var _ protocolFact = FileParseRequest{}

// ParseFile owns the native open/parse/close lifetime. There is no source-byte
// quota. Go's AST allocation belongs to this whole-file observation. Refusals
// preserve native causes and never return provisional syntax beside an error.
func ParseFile(ctx context.Context, request FileParseRequest) (*ast.File, error) {
	if err := errors.Join(request.Validate(), contextstate.Validate(ctx)); err != nil {
		if ctx != nil {
			err = errors.Join(err, context.Cause(ctx))
		}
		return nil, errors.Join(core.ErrGoToolchainContract, err)
	}
	source, err := openAnalysisInput(ctx, request.Path.String())
	if err != nil {
		return nil, errors.Join(core.ErrGoToolchainExecution, err)
	}
	file, parseErr := parser.ParseFile(request.FileSet, request.Path.String(), source, parser.ParseComments)
	err = errors.Join(parseErr, source.Close(), contextstate.Validate(ctx), context.Cause(ctx))
	if err != nil {
		return nil, errors.Join(core.ErrGoToolchainOutput, err)
	}
	return file, nil
}
