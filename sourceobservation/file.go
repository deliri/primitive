package sourceobservation

import (
	"cmp"
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

type BuildSelection struct {
	Context ContextID      `json:"context"`
	State   SelectionState `json:"state"`
}

func (s BuildSelection) Validate() error {
	return contractJoin(s.Context.Validate(), s.State.Validate())
}

type Declaration struct {
	Name     Symbol          `json:"name"`
	Kind     DeclarationKind `json:"kind"`
	Line     uint32          `json:"line"`
	Column   uint32          `json:"column"`
	Exported bool            `json:"exported"`
}

func (d Declaration) Validate() error {
	if d.Line == 0 || d.Column == 0 {
		return contractError(errors.New("source observation declaration position is unset"))
	}
	return contractJoin(d.Name.Validate(), d.Kind.Validate())
}

type Import struct {
	Path ImportPath `json:"path"`
}

func (i Import) Validate() error { return i.Path.Validate() }

type Effect struct {
	Name   EffectName `json:"name"`
	Symbol Symbol     `json:"symbol"`
	Line   uint32     `json:"line"`
	Column uint32     `json:"column"`
}

func (e Effect) Validate() error {
	if e.Line == 0 || e.Column == 0 {
		return contractError(errors.New("source observation effect position is unset"))
	}
	return contractJoin(e.Name.Validate(), e.Symbol.Validate())
}

type Reference struct {
	From   Symbol        `json:"from"`
	To     Symbol        `json:"to"`
	Import *ImportPath   `json:"import,omitempty"`
	Kind   ReferenceKind `json:"kind"`
	Line   uint32        `json:"line"`
	Column uint32        `json:"column"`
}

func (r Reference) Validate() error {
	if r.Line == 0 || r.Column == 0 {
		return contractError(errors.New("source observation reference position is unset"))
	}
	if err := contractJoin(r.From.Validate(), r.To.Validate(), r.Kind.Validate()); err != nil {
		return err
	}
	if r.Import != nil {
		return r.Import.Validate()
	}
	return nil
}

// File records one exact source file without retaining its content bytes.
type File struct {
	Repository   core.RepositoryIdentity `json:"repository"`
	Path         core.SourcePath         `json:"path"`
	Package      *core.SourcePath        `json:"package,omitempty"`
	Snapshot     core.SourceSnapshot     `json:"snapshot"`
	SourceDigest core.SHA256Digest       `json:"source_digest"`
	Bytes        core.ByteLength         `json:"bytes"`
	Language     Language                `json:"language"`
	Generated    GeneratedState          `json:"generated"`
	Selections   []BuildSelection        `json:"selections"`
	Declarations []Declaration           `json:"declarations"`
	Imports      []Import                `json:"imports"`
	Effects      []Effect                `json:"effects"`
	References   []Reference             `json:"references"`
}

func (f File) Validate() error {
	if err := contractJoin(f.Repository.Validate(), f.Path.Validate(), f.Snapshot.Validate(), f.SourceDigest.Validate(), f.Bytes.Validate(), f.Language.Validate(), f.Generated.Validate()); err != nil {
		return err
	}
	if f.Package != nil {
		if err := f.Package.Validate(); err != nil {
			return err
		}
		if !fileOwnedByPackage(*f.Package, f.Path) {
			return conflictError(errors.New("source observation file is outside its package"))
		}
	}
	if len(f.Selections) == 0 {
		return contractError(errors.New("source observation file has no declared build context"))
	}
	if err := validateSelections(f.Selections); err != nil {
		return err
	}
	return contractJoin(validateDeclarations(f.Declarations), validateImports(f.Imports), validateEffects(f.Effects), validateReferences(f.References))
}

func validateSelections(values []BuildSelection) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && values[index-1].Context.String() >= values[index].Context.String() {
			return conflictError(errors.New("source observation build contexts are duplicated or not canonical"))
		}
	}
	return nil
}

func validateDeclarations(values []Declaration) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && compareDeclarations(values[index-1], values[index]) >= 0 {
			return conflictError(errors.New("source observation declarations are duplicated or not canonical"))
		}
	}
	return nil
}

func compareDeclarations(left, right Declaration) int {
	return cmp.Or(
		cmp.Compare(left.Line, right.Line),
		cmp.Compare(left.Column, right.Column),
		cmp.Compare(uint8(left.Kind), uint8(right.Kind)),
		strings.Compare(left.Name.String(), right.Name.String()),
		compareBool(left.Exported, right.Exported),
	)
}

func validateImports(values []Import) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && values[index-1].Path.String() >= values[index].Path.String() {
			return conflictError(errors.New("source observation imports are duplicated or not canonical"))
		}
	}
	return nil
}

func validateEffects(values []Effect) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && compareEffects(values[index-1], values[index]) >= 0 {
			return conflictError(errors.New("source observation effects are duplicated or not canonical"))
		}
	}
	return nil
}

func compareEffects(left, right Effect) int {
	return cmp.Or(
		cmp.Compare(left.Line, right.Line),
		cmp.Compare(left.Column, right.Column),
		strings.Compare(left.Name.String(), right.Name.String()),
		strings.Compare(left.Symbol.String(), right.Symbol.String()),
	)
}

func validateReferences(values []Reference) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && compareReferences(values[index-1], values[index]) >= 0 {
			return conflictError(errors.New("source observation references are duplicated or not canonical"))
		}
	}
	return nil
}

func compareReferences(left, right Reference) int {
	return cmp.Or(
		cmp.Compare(left.Line, right.Line),
		cmp.Compare(left.Column, right.Column),
		cmp.Compare(uint8(left.Kind), uint8(right.Kind)),
		strings.Compare(left.From.String(), right.From.String()),
		strings.Compare(left.To.String(), right.To.String()),
		strings.Compare(importCoordinate(left.Import), importCoordinate(right.Import)),
	)
}

func importCoordinate(value *ImportPath) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func compareBool(left, right bool) int {
	if left == right {
		return 0
	}
	if !left {
		return -1
	}
	return 1
}
