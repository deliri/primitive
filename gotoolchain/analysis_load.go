package gotoolchain

import (
	"bytes"
	"context"
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"go/types"
	"io"
	"path/filepath"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/tools/go/packages"
)

// These are fields of cmd/go's documented PackagePublic JSON contract.
// The stream is bounded by Limits.OutputBytes and Limits.PackageMaximum;
// one package's dependency closure is the only aggregate admitted here.
const analysisJSONFields = "-json=Dir,ImportPath,Name,ForTest,Export,Module,GoFiles,CompiledGoFiles,Imports,ImportMap,Error,Incomplete"

type analysisPackageWire struct {
	Dir             string             `json:"Dir"`
	ImportPath      string             `json:"ImportPath"`
	Name            string             `json:"Name"`
	ForTest         string             `json:"ForTest"`
	Export          string             `json:"Export"`
	Module          *packages.Module   `json:"Module"`
	GoFiles         []string           `json:"GoFiles"`
	CompiledGoFiles []string           `json:"CompiledGoFiles"`
	Imports         []string           `json:"Imports"`
	ImportMap       map[string]string  `json:"ImportMap"`
	Error           *analysisErrorWire `json:"Error"`
	Incomplete      bool               `json:"Incomplete"`
}

type analysisErrorWire struct {
	Err string `json:"Err"`
	Pos string `json:"Pos"`
}

func (analysisPackageWire) goToolchainInternalFlow() {}
func (analysisErrorWire) goToolchainInternalFlow()   {}

func (c Capability) loadAnalysisMetadata(ctx context.Context, request AnalysisRequest) ([]*packages.Package, error) {
	observed, err := c.ObserveBuildContext(ctx, ObservationRequest{WorkingDirectory: request.WorkingDirectory})
	if err != nil {
		return nil, err
	}
	sizes := types.SizesFor("gc", observed.Platform.Architecture.String())
	if sizes == nil {
		return nil, outputError("compiler target has no type sizes", nil)
	}
	arguments := []string{"list", "-e", goDependenciesArgument, "-export", "-compiled", goModuleReadOnly, analysisJSONFields}
	if request.IncludeTests {
		arguments = append(arguments, "-test")
	}
	arguments = append(arguments, "--", request.Package.String())
	encoded, _, err := c.execute(ctx, request.WorkingDirectory, arguments...)
	if err != nil {
		return nil, err
	}
	return decodeAnalysisMetadata(encoded, c.configuration.Limits, sizes)
}

func decodeAnalysisMetadata(data []byte, limits Limits, sizes types.Sizes) ([]*packages.Package, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	maximum, err := limits.OutputBytes.Uint64()
	if err != nil || uint64(len(data)) > maximum || sizes == nil {
		return nil, outputError("analysis metadata exceeds its bound or has no target sizes", err)
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	wires := make([]analysisPackageWire, 0, min(len(data)/2, int(limits.PackageMaximum)))
	for {
		var wire analysisPackageWire
		err = json.UnmarshalDecode(decoder, &wire)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, outputError("analysis metadata JSON is invalid", err)
		}
		if uint32(len(wires)) >= limits.PackageMaximum {
			return nil, outputError("analysis package count exceeds its bound", nil)
		}
		wires = append(wires, wire)
	}
	return linkAnalysisMetadata(wires, sizes)
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
	if wire.Error != nil {
		unit.Errors = []packages.Error{{Pos: wire.Error.Pos, Msg: wire.Error.Err, Kind: packages.ListError}}
	}
	if wire.Incomplete && wire.Error == nil {
		unit.Errors = []packages.Error{{Msg: "compiler dependency closure is incomplete", Kind: packages.ListError}}
	}
	var err error
	unit.GoFiles, err = absoluteAnalysisFiles(wire.Dir, wire.GoFiles)
	if err != nil {
		return nil, err
	}
	unit.CompiledGoFiles, err = absoluteAnalysisFiles(wire.Dir, wire.CompiledGoFiles)
	if err != nil {
		return nil, err
	}
	return unit, nil
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
