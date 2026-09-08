package gotoolchain

import (
	"errors"
	"go/scanner"
	"go/token"
	"go/types"
	"os"

	"golang.org/x/tools/go/packages"
)

// Follow go/packages' parser/checker diagnostic projection while also keeping
// the original Go error identities available through errors.Is/errors.As.
type analysisDiagnostics struct {
	failures   []error
	records    []packages.Error
	typeErrors []types.Error
}

func (analysisDiagnostics) goToolchainInternalFlow() {}

func (d *analysisDiagnostics) failure() error {
	return errors.Join(d.failures...)
}

func (d *analysisDiagnostics) add(err error) {
	if err == nil {
		return
	}
	d.failures = append(d.failures, err)
	if diagnostic, ok := errors.AsType[packages.Error](err); ok {
		d.records = append(d.records, diagnostic)
	} else if diagnostics, ok := errors.AsType[scanner.ErrorList](err); ok {
		for _, diagnostic := range diagnostics {
			d.records = append(d.records, packages.Error{Pos: diagnostic.Pos.String(), Msg: diagnostic.Msg, Kind: packages.ParseError})
		}
	} else if diagnostic, ok := errors.AsType[types.Error](err); ok {
		d.typeErrors = append(d.typeErrors, diagnostic)
		d.records = append(d.records, packages.Error{Pos: diagnostic.Fset.Position(diagnostic.Pos).String(), Msg: diagnostic.Msg, Kind: packages.TypeError})
	} else if diagnostic, ok := errors.AsType[*os.PathError](err); ok {
		position := token.Position{Filename: diagnostic.Path, Line: 1}
		d.records = append(d.records, packages.Error{Pos: position.String(), Msg: diagnostic.Err.Error(), Kind: packages.ParseError})
	} else {
		d.records = append(d.records, packages.Error{Msg: err.Error(), Kind: packages.UnknownError})
	}
}
