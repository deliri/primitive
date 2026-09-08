package gotoolchain

import (
	"cmp"
	"context"
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"go/types"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"golang.org/x/tools/go/packages"
)

// These are fields of cmd/go's documented PackagePublic JSON contract.
// The stream is bounded by Limits.OutputBytes and Limits.PackageMaximum;
// one package's dependency closure is the only aggregate admitted here.
const analysisJSONFields = "-json=Dir,ImportPath,Name,ForTest,Export,Module,GoFiles,CgoFiles,CompiledGoFiles,Imports,ImportMap,Error,DepsErrors,Incomplete"

type analysisPackageWire struct {
	Dir             string              `json:"Dir"`
	ImportPath      string              `json:"ImportPath"`
	Name            string              `json:"Name"`
	ForTest         string              `json:"ForTest"`
	Export          string              `json:"Export"`
	Module          *packages.Module    `json:"Module"`
	GoFiles         []string            `json:"GoFiles"`
	CgoFiles        []string            `json:"CgoFiles"`
	CompiledGoFiles []string            `json:"CompiledGoFiles"`
	Imports         []string            `json:"Imports"`
	ImportMap       map[string]string   `json:"ImportMap"`
	Error           *analysisErrorWire  `json:"Error"`
	DepsErrors      []analysisErrorWire `json:"DepsErrors"`
	Incomplete      bool                `json:"Incomplete"`
}

type analysisErrorWire struct {
	Err string `json:"Err"`
	Pos string `json:"Pos"`
}

func (analysisPackageWire) goToolchainInternalFlow() {}
func (analysisErrorWire) goToolchainInternalFlow()   {}

func (c Capability) loadAnalysisMetadata(ctx context.Context, directory core.AbsolutePath, requested []gomodule.ImportPath, includeTests bool) ([]*packages.Package, error) {
	observed, err := c.ObserveBuildContext(ctx, ObservationRequest{WorkingDirectory: directory})
	if err != nil {
		return nil, err
	}
	sizes := types.SizesFor("gc", observed.Platform.Architecture.String())
	if sizes == nil {
		return nil, outputError("compiler target has no type sizes", nil)
	}
	arguments := []string{"list", "-e", goDependenciesArgument, "-export", "-compiled", analysisJSONFields}
	if includeTests {
		arguments = append(arguments, "-test")
	}
	arguments = append(arguments, "--")
	for _, pkg := range requested {
		arguments = append(arguments, pkg.String())
	}
	return c.streamAnalysisMetadata(ctx, directory, arguments, sizes)
}

func (c Capability) streamAnalysisMetadata(ctx context.Context, directory core.AbsolutePath, arguments []string, sizes types.Sizes) ([]*packages.Package, error) {
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, writer := io.Pipe()
	completed := make(chan error, 1)
	go func() {
		_, err := c.executeTo(work, directory, writer, arguments...)
		completed <- errors.Join(err, writer.CloseWithError(err))
	}()
	units, decodeErr := decodeAnalysisMetadata(reader, c.configuration.Limits, sizes)
	closeErr := reader.Close()
	if decodeErr != nil {
		cancel()
	}
	if err := errors.Join(decodeErr, closeErr, <-completed); err != nil {
		return nil, err
	}
	return units, nil
}

func decodeAnalysisMetadata(input io.Reader, limits Limits, sizes types.Sizes) ([]*packages.Package, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	maximum, err := limits.OutputBytes.Int64()
	if err != nil || input == nil || sizes == nil {
		return nil, outputError("analysis metadata exceeds its bound or has no target sizes", err)
	}
	bounded := &io.LimitedReader{R: input, N: maximum}
	decoder := jsontext.NewDecoder(bounded)
	// Encoded byte length does not predict package count. Reserving the
	// maximum graph here wastes nearly a megabyte for ordinary small loads.
	var wires []analysisPackageWire
	for {
		var wire analysisPackageWire
		err = json.UnmarshalDecode(decoder, &wire)
		// witness:waiver doctrine/error/sentinel_compare -- Only unwrapped EOF is clean completion; joined EOF must retain the accompanying reader failure.
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, outputError("analysis metadata JSON is invalid", err)
		}
		if uint64(len(wires)) >= uint64(limits.PackageMaximum) {
			return nil, outputError("analysis package count exceeds its bound", nil)
		}
		// witness:waiver doctrine/quality/append_in_loop -- The preceding package-count admission and LimitedReader enforce caller-owned bounds without allocating the maximum graph up front.
		wires = append(wires, wire)
	}
	if err := analysisMetadataEOF(bounded); err != nil {
		return nil, err
	}
	return linkAnalysisMetadata(wires, sizes)
}

// Probe beyond an exhausted caller budget without adding one to a potentially
// maximal int64. Exact EOF is admitted; any extra byte or reader failure is not.
func analysisMetadataEOF(input *io.LimitedReader) error {
	if input.N != 0 {
		return nil
	}
	var extra [1]byte
	count, err := io.ReadFull(input.R, extra[:])
	// witness:waiver doctrine/error/sentinel_compare -- The exact-budget probe must refuse joined EOF and reader failure, preserving the native cause.
	if count == 0 && err == io.EOF {
		return nil
	}
	return outputError("analysis metadata exceeds its byte bound", err)
}

func linkAnalysisMetadata(wires []analysisPackageWire, sizes types.Sizes) ([]*packages.Package, error) {
	units := make([]*packages.Package, 0, len(wires))
	byID := make(map[string]*packages.Package, len(wires))
	for _, wire := range wires {
		unit, err := analysisMetadataFromWire(wire, sizes)
		if err != nil {
			return nil, err
		}
		if _, exists := byID[unit.ID]; exists {
			return nil, outputError("duplicate compiler package identity", nil)
		}
		units = append(units, unit)
		byID[unit.ID] = unit
	}
	for i, wire := range wires {
		if err := linkAnalysisImports(units[i], wire, byID); err != nil {
			return nil, err
		}
	}
	return units, nil
}

func analysisMetadataFromWire(wire analysisPackageWire, sizes types.Sizes) (*packages.Package, error) {
	if wire.ImportPath == "" {
		return nil, outputError("analysis package identity is missing", nil)
	}
	path, _, _ := strings.Cut(wire.ImportPath, " [")
	unit := &packages.Package{ID: wire.ImportPath, PkgPath: path, Name: wire.Name, Dir: wire.Dir, ForTest: wire.ForTest, ExportFile: wire.Export, Module: wire.Module, TypesSizes: sizes, Imports: make(map[string]*packages.Package)}
	unit.Errors = analysisMetadataErrors(wire)
	var err error
	unit.GoFiles, err = absoluteAnalysisFiles(wire.Dir, append(wire.GoFiles, wire.CgoFiles...))
	if err != nil {
		return nil, err
	}
	unit.CompiledGoFiles, err = absoluteAnalysisFiles(wire.Dir, wire.CompiledGoFiles)
	if err != nil {
		return nil, err
	}
	if len(wire.CgoFiles) > 0 && len(wire.CompiledGoFiles) > len(wire.GoFiles) {
		// Same cmd/go selection used by go/packages NeedCgo: retain its
		// generated type declarations and type-check the original Go files.
		support := unit.CompiledGoFiles[len(wire.GoFiles)]
		unit.CompiledGoFiles = append([]string{support}, unit.GoFiles...)
	}
	return unit, nil
}

// Cmd/go's dependency failures can arrive in different traversal orders.
// Canonicalize their typed records without dropping duplicates or changing
// their positions/messages. The caller's wire observation remains untouched.
func analysisMetadataErrors(wire analysisPackageWire) []packages.Error {
	var diagnostics []packages.Error
	if wire.Error != nil {
		diagnostics = append(diagnostics, packages.Error{Pos: wire.Error.Pos, Msg: wire.Error.Err, Kind: packages.ListError})
	}
	for _, diagnostic := range wire.DepsErrors {
		diagnostics = append(diagnostics, packages.Error{Pos: diagnostic.Pos, Msg: diagnostic.Err, Kind: packages.ListError})
	}
	if wire.Incomplete && len(diagnostics) == 0 {
		diagnostics = []packages.Error{{Msg: "compiler dependency closure is incomplete", Kind: packages.ListError}}
	}
	slices.SortFunc(diagnostics, func(left, right packages.Error) int {
		return cmp.Or(cmp.Compare(left.Pos, right.Pos), cmp.Compare(left.Msg, right.Msg))
	})
	return diagnostics
}

func absoluteAnalysisFiles(directory string, files []string) ([]string, error) {
	result := make([]string, 0, len(files))
	for _, file := range files {
		if file == "" {
			return nil, outputError("compiler input path is empty", nil)
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(directory, file)
		}
		if _, err := core.ParseAbsolutePath(file); err != nil {
			return nil, outputError("compiler input path is invalid", err)
		}
		result = append(result, file)
	}
	return result, nil
}

func linkAnalysisImports(unit *packages.Package, wire analysisPackageWire, byID map[string]*packages.Package) error {
	for _, identity := range wire.Imports {
		if identity == core.GoCgoImportPath {
			continue
		}
		target := byID[identity]
		if target == nil {
			if wire.Incomplete {
				continue
			}
			return outputError("compiler import is absent from dependency closure", nil)
		}
		if prior := unit.Imports[target.PkgPath]; prior != nil && prior != target {
			return outputError("compiler imports conflict for one logical package", nil)
		}
		unit.Imports[target.PkgPath] = target
	}
	for original, identity := range wire.ImportMap {
		target := byID[identity]
		if target == nil {
			return outputError("compiler import map target is absent", nil)
		}
		unit.Imports[original] = target
	}
	return nil
}
