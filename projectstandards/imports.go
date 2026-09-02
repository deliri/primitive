package projectstandards

import (
	"errors"
	"strings"
)

const (
	// SourceFileImportMaximum is a resource ceiling for one file's exact import
	// declarations, not a claim about any current file.
	SourceFileImportMaximum = 1_024
)

// SourceImportKind classifies one compiler-observed import.
type SourceImportKind uint8

const (
	SourceImportKindUnknown SourceImportKind = iota
	SourceImportKindStandardLibrary
	SourceImportKindProject
	SourceImportKindExternal
	sourceImportKindLimit
)

func sourceImportKindLabels() []string {
	return []string{"", "standard_library", "project", "external"}
}

func (k SourceImportKind) Validate() error {
	return validateEnum(uint8(k), sourceImportKindLabels(), "project standards source import kind is invalid")
}

func (k SourceImportKind) IsValid() bool  { return k.Validate() == nil }
func (k SourceImportKind) String() string { return enumString(uint8(k), sourceImportKindLabels()) }
func (k SourceImportKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), sourceImportKindLabels(), "project standards source import kind is invalid")
}
func (k *SourceImportKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil project standards source import kind receiver"))
	}
	value, err := unmarshalEnum(data, sourceImportKindLabels(), "project standards source import kind is invalid")
	if err == nil {
		*k = SourceImportKind(value)
	}
	return err
}

// SourceImport is one exact compiler-observed import declaration. ProjectModule
// is present only when Path resolves inside the inspected Go module.
type SourceImport struct {
	ProjectModule *SourcePath      `json:"project_module,omitempty"`
	Path          SourcePath       `json:"path"`
	Kind          SourceImportKind `json:"kind"`
}

func (i SourceImport) Validate() error {
	if err := contractJoin(i.Path.Validate(), i.Kind.Validate()); err != nil {
		return err
	}
	if i.Kind == SourceImportKindProject {
		if i.ProjectModule == nil {
			return conflictError(errors.New("project standards project import omits its module identity"))
		}
		return i.validateProjectPath()
	}
	if i.ProjectModule != nil {
		return conflictError(errors.New("project standards non-project import carries a module identity"))
	}
	return nil
}

func (i SourceImport) validateProjectPath() error {
	if err := i.ProjectModule.Validate(); err != nil {
		return err
	}
	module := i.ProjectModule.String()
	importPath := i.Path.String()
	if importPath != module && !strings.HasPrefix(importPath, module+"/") {
		return conflictError(errors.New("project standards project import is outside its module"))
	}
	return nil
}

// SourceFileImports is an observed, bounded, canonical import catalog. A nil
// *SourceFileImports on SourceFile means imports were not observed; a present
// catalog with zero entries proves the file has no imports.
type SourceFileImports struct {
	Values []SourceImport `json:"values"`
}

func (i SourceFileImports) Validate() error {
	if len(i.Values) > SourceFileImportMaximum {
		return contractError(errors.New("project standards source import count exceeds its bound"))
	}
	for index := range i.Values {
		if err := i.Values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && i.Values[index-1].Path.String() >= i.Values[index].Path.String() {
			return conflictError(errors.New("project standards source imports are duplicated or not in canonical order"))
		}
	}
	return nil
}
