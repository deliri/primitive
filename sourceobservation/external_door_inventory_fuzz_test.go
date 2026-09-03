package sourceobservation_test

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceobservation"
)

type observationJSONDoor uint8

const (
	observationJSONDoorUnknown observationJSONDoor = iota
	observationJSONDoorContextID
	observationJSONDoorLanguage
	observationJSONDoorSymbol
	observationJSONDoorImportPath
	observationJSONDoorEffectName
	observationJSONDoorToolchain
	observationJSONDoorGeneratedState
	observationJSONDoorSelectionState
	observationJSONDoorDeclarationKind
	observationJSONDoorReferenceKind
	observationJSONDoorFileReference
	observationJSONDoorPackageReference
	observationJSONDoorFileMembership
	observationJSONDoorPackageMembership
	observationJSONDoorFile
	observationJSONDoorPackage
	observationJSONDoorProject
	observationJSONDoorSummary
	observationJSONDoorLimit
)

func (d observationJSONDoor) receiverName() string {
	switch d {
	case observationJSONDoorContextID:
		return "ContextID"
	case observationJSONDoorLanguage:
		return "Language"
	case observationJSONDoorSymbol:
		return "Symbol"
	case observationJSONDoorImportPath:
		return "ImportPath"
	case observationJSONDoorEffectName:
		return "EffectName"
	case observationJSONDoorToolchain:
		return "Toolchain"
	case observationJSONDoorGeneratedState:
		return "GeneratedState"
	case observationJSONDoorSelectionState:
		return "SelectionState"
	case observationJSONDoorDeclarationKind:
		return "DeclarationKind"
	case observationJSONDoorReferenceKind:
		return "ReferenceKind"
	case observationJSONDoorFileReference:
		return "FileReference"
	case observationJSONDoorPackageReference:
		return "PackageReference"
	case observationJSONDoorFileMembership:
		return "FileMembership"
	case observationJSONDoorPackageMembership:
		return "PackageMembership"
	case observationJSONDoorFile:
		return "File"
	case observationJSONDoorPackage:
		return "Package"
	case observationJSONDoorProject:
		return "Project"
	case observationJSONDoorSummary:
		return "Summary"
	case observationJSONDoorUnknown, observationJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type observationJSONFixtures struct {
	contextID         sourceobservation.ContextID
	language          sourceobservation.Language
	symbol            sourceobservation.Symbol
	importPath        sourceobservation.ImportPath
	effectName        sourceobservation.EffectName
	toolchain         sourceobservation.Toolchain
	fileReference     sourceobservation.FileReference
	packageReference  sourceobservation.PackageReference
	fileMembership    sourceobservation.FileMembership
	packageMembership sourceobservation.PackageMembership
	file              sourceobservation.File
	packageValue      sourceobservation.Package
	project           sourceobservation.Project
	summary           sourceobservation.Summary
}

type observationJSONSeed struct {
	document []byte
	door     observationJSONDoor
}

func FuzzSourceObservationExternalJSONDoorInventory(f *testing.F) {
	fixtures := observationFixturesForFuzz(f)
	for _, seed := range observationJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(observationJSONDoorProject), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch observationJSONDoor(rawDoor) {
		case observationJSONDoorContextID:
			fuzzObservationJSONDoor(t, data, fixtures.contextID)
		case observationJSONDoorLanguage:
			fuzzObservationJSONDoor(t, data, fixtures.language)
		case observationJSONDoorSymbol:
			fuzzObservationJSONDoor(t, data, fixtures.symbol)
		case observationJSONDoorImportPath:
			fuzzObservationJSONDoor(t, data, fixtures.importPath)
		case observationJSONDoorEffectName:
			fuzzObservationJSONDoor(t, data, fixtures.effectName)
		case observationJSONDoorToolchain:
			fuzzObservationJSONDoor(t, data, fixtures.toolchain)
		case observationJSONDoorGeneratedState:
			fuzzObservationJSONDoor(t, data, sourceobservation.GeneratedAuthored)
		case observationJSONDoorSelectionState:
			fuzzObservationJSONDoor(t, data, sourceobservation.SelectionIncluded)
		case observationJSONDoorDeclarationKind:
			fuzzObservationJSONDoor(t, data, sourceobservation.DeclarationFunction)
		case observationJSONDoorReferenceKind:
			fuzzObservationJSONDoor(t, data, sourceobservation.ReferencePackage)
		case observationJSONDoorFileReference:
			fuzzObservationJSONDoor(t, data, fixtures.fileReference)
		case observationJSONDoorPackageReference:
			fuzzObservationJSONDoor(t, data, fixtures.packageReference)
		case observationJSONDoorFileMembership:
			fuzzObservationJSONDoor(t, data, fixtures.fileMembership)
		case observationJSONDoorPackageMembership:
			fuzzObservationJSONDoor(t, data, fixtures.packageMembership)
		case observationJSONDoorFile:
			fuzzObservationJSONDoor(t, data, fixtures.file)
		case observationJSONDoorPackage:
			fuzzObservationJSONDoor(t, data, fixtures.packageValue)
		case observationJSONDoorProject:
			fuzzObservationJSONDoor(t, data, fixtures.project)
		case observationJSONDoorSummary:
			fuzzObservationJSONDoor(t, data, fixtures.summary)
		case observationJSONDoorUnknown, observationJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type observationJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzObservationJSONDoor[T observationJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("source observation seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("source observation JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrSourceObservationContract) {
			t.Fatalf("source observation JSON door error = %v, want %v and %v", decodeErr, core.ErrJSONContract, core.ErrSourceObservationContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected source observation JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted source observation JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("source observation canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("source observation round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("source observation canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("source observation JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func TestSourceObservationExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, err := observationExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("observationExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var want []string
	for door := observationJSONDoorUnknown + 1; door < observationJSONDoorLimit; door++ {
		want = append(want, door.receiverName())
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", got, want)
	}
}

func observationExportedJSONReceiverNames() ([]string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, file.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "UnmarshalJSON" || function.Recv == nil || len(function.Recv.List) != 1 {
				continue
			}
			pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			receiver, ok := pointer.X.(*ast.Ident)
			if ok && receiver.IsExported() {
				names = append(names, receiver.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func observationFixturesForFuzz(t testing.TB) observationJSONFixtures {
	t.Helper()
	project, resolver := packagedObservationFixture(t, true)
	summary, err := sourceobservation.VerifyProject(context.Background(), project, resolver)
	if err != nil {
		t.Fatalf("sourceobservation.VerifyProject(seed) error = %v, want nil", err)
	}
	packagePath := observedPath(t, "exchange")
	packageValue := resolver.packages[packagePath]
	filePath := observedPath(t, "exchange/client_test.go")
	file := resolver.files[filePath]
	fileReference := observedFileReference(t, file)
	packageDigest, err := packageValue.ObservationDigest()
	if err != nil {
		t.Fatalf("Package.ObservationDigest(seed) error = %v, want nil", err)
	}
	packageReference := sourceobservation.PackageReference{Path: packagePath, ObservationDigest: packageDigest}
	importPath, err := sourceobservation.NewImportPath("github.com/deliri/primitive/v2026/core")
	if err != nil {
		t.Fatalf("sourceobservation.NewImportPath(seed) error = %v, want nil", err)
	}
	effectName, err := sourceobservation.NewEffectName("filesystem-read")
	if err != nil {
		t.Fatalf("sourceobservation.NewEffectName(seed) error = %v, want nil", err)
	}
	toolchain, err := sourceobservation.NewToolchain("go1.27.1")
	if err != nil {
		t.Fatalf("sourceobservation.NewToolchain(seed) error = %v, want nil", err)
	}
	language, err := sourceobservation.NewLanguage("go")
	if err != nil {
		t.Fatalf("sourceobservation.NewLanguage(seed) error = %v, want nil", err)
	}
	symbol, err := sourceobservation.NewSymbol("RoundTrip")
	if err != nil {
		t.Fatalf("sourceobservation.NewSymbol(seed) error = %v, want nil", err)
	}
	return observationJSONFixtures{
		contextID: observationContext(t).ID, language: language, symbol: symbol,
		importPath: importPath, effectName: effectName, toolchain: toolchain,
		fileReference: fileReference, packageReference: packageReference,
		fileMembership: packageValue.Files, packageMembership: project.Packages,
		file: file, packageValue: packageValue, project: project, summary: summary,
	}
}

func observationJSONSeedsForFuzz(t testing.TB, fixtures observationJSONFixtures) []observationJSONSeed {
	t.Helper()
	return []observationJSONSeed{
		observationJSONSeedForFuzz(t, observationJSONDoorContextID, fixtures.contextID),
		observationJSONSeedForFuzz(t, observationJSONDoorLanguage, fixtures.language),
		observationJSONSeedForFuzz(t, observationJSONDoorSymbol, fixtures.symbol),
		observationJSONSeedForFuzz(t, observationJSONDoorImportPath, fixtures.importPath),
		observationJSONSeedForFuzz(t, observationJSONDoorEffectName, fixtures.effectName),
		observationJSONSeedForFuzz(t, observationJSONDoorToolchain, fixtures.toolchain),
		observationJSONSeedForFuzz(t, observationJSONDoorGeneratedState, sourceobservation.GeneratedAuthored),
		observationJSONSeedForFuzz(t, observationJSONDoorSelectionState, sourceobservation.SelectionIncluded),
		observationJSONSeedForFuzz(t, observationJSONDoorDeclarationKind, sourceobservation.DeclarationFunction),
		observationJSONSeedForFuzz(t, observationJSONDoorReferenceKind, sourceobservation.ReferencePackage),
		observationJSONSeedForFuzz(t, observationJSONDoorFileReference, fixtures.fileReference),
		observationJSONSeedForFuzz(t, observationJSONDoorPackageReference, fixtures.packageReference),
		observationJSONSeedForFuzz(t, observationJSONDoorFileMembership, fixtures.fileMembership),
		observationJSONSeedForFuzz(t, observationJSONDoorPackageMembership, fixtures.packageMembership),
		observationJSONSeedForFuzz(t, observationJSONDoorFile, fixtures.file),
		observationJSONSeedForFuzz(t, observationJSONDoorPackage, fixtures.packageValue),
		observationJSONSeedForFuzz(t, observationJSONDoorProject, fixtures.project),
		observationJSONSeedForFuzz(t, observationJSONDoorSummary, fixtures.summary),
	}
}

func observationJSONSeedForFuzz(t testing.TB, door observationJSONDoor, value observationJSONValue) observationJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("source observation fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return observationJSONSeed{door: door, document: document}
}
