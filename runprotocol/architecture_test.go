package runprotocol

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

//go:embed *.go
var aboutContractSources embed.FS

type runProtocolJSONDoorInventory struct {
	Identifier               func(*Identifier, []byte) error
	Name                     func(*Name, []byte) error
	Text                     func(*Text, []byte) error
	SourcePath               func(*SourcePath, []byte) error
	RepositoryIdentity       func(*RepositoryIdentity, []byte) error
	ProfileIdentity          func(*ProfileIdentity, []byte) error
	RequestIdentity          func(*RequestIdentity, []byte) error
	RequestNonce             func(*RequestNonce, []byte) error
	RunID                    func(*RunID, []byte) error
	ExperimentID             func(*ExperimentID, []byte) error
	ObservationID            func(*ObservationID, []byte) error
	MachineID                func(*MachineID, []byte) error
	MachineGenerationID      func(*MachineGenerationID, []byte) error
	MachineObservationID     func(*MachineObservationID, []byte) error
	CachePosture             func(*CachePosture, []byte) error
	Outcome                  func(*Outcome, []byte) error
	DispositionKind          func(*DispositionKind, []byte) error
	RefusalReason            func(*RefusalReason, []byte) error
	TerminalState            func(*TerminalState, []byte) error
	ObservationKind          func(*ObservationKind, []byte) error
	InfrastructureStage      func(*InfrastructureStage, []byte) error
	MachineToolchainKind     func(*MachineToolchainKind, []byte) error
	MachineInstallMode       func(*MachineInstallMode, []byte) error
	MachineProvisioningModel func(*MachineProvisioningModel, []byte) error
	MachineMaintenancePolicy func(*MachineMaintenancePolicy, []byte) error
	MachineChangeField       func(*MachineChangeField, []byte) error
	ProbeRole                func(*ProbeRole, []byte) error
	ProbeKind                func(*ProbeKind, []byte) error
	ProbeTargetKind          func(*ProbeTargetKind, []byte) error
	MachineProbeReport       func(*MachineProbeReport, []byte) error
}

var runProtocolJSONDoors = runProtocolJSONDoorInventory{
	Identifier: (*Identifier).UnmarshalJSON, Name: (*Name).UnmarshalJSON,
	Text: (*Text).UnmarshalJSON, SourcePath: (*SourcePath).UnmarshalJSON,
	RepositoryIdentity:       (*RepositoryIdentity).UnmarshalJSON,
	ProfileIdentity:          (*ProfileIdentity).UnmarshalJSON,
	RequestIdentity:          (*RequestIdentity).UnmarshalJSON,
	RequestNonce:             (*RequestNonce).UnmarshalJSON,
	RunID:                    (*RunID).UnmarshalJSON,
	ExperimentID:             (*ExperimentID).UnmarshalJSON,
	ObservationID:            (*ObservationID).UnmarshalJSON,
	MachineID:                (*MachineID).UnmarshalJSON,
	MachineGenerationID:      (*MachineGenerationID).UnmarshalJSON,
	MachineObservationID:     (*MachineObservationID).UnmarshalJSON,
	CachePosture:             (*CachePosture).UnmarshalJSON,
	Outcome:                  (*Outcome).UnmarshalJSON,
	DispositionKind:          (*DispositionKind).UnmarshalJSON,
	RefusalReason:            (*RefusalReason).UnmarshalJSON,
	TerminalState:            (*TerminalState).UnmarshalJSON,
	ObservationKind:          (*ObservationKind).UnmarshalJSON,
	InfrastructureStage:      (*InfrastructureStage).UnmarshalJSON,
	MachineToolchainKind:     (*MachineToolchainKind).UnmarshalJSON,
	MachineInstallMode:       (*MachineInstallMode).UnmarshalJSON,
	MachineProvisioningModel: (*MachineProvisioningModel).UnmarshalJSON,
	MachineMaintenancePolicy: (*MachineMaintenancePolicy).UnmarshalJSON,
	MachineChangeField:       (*MachineChangeField).UnmarshalJSON,
	ProbeRole:                (*ProbeRole).UnmarshalJSON,
	ProbeKind:                (*ProbeKind).UnmarshalJSON,
	ProbeTargetKind:          (*ProbeTargetKind).UnmarshalJSON,
	MachineProbeReport:       (*MachineProbeReport).UnmarshalJSON,
}

type aboutTextDoorInventory struct {
	NewIdentifier         func(string) (Identifier, error)
	NewName               func(string) (Name, error)
	NewText               func(string) (Text, error)
	ParseSourcePath       func(string) (SourcePath, error)
	NewRepositoryIdentity func(string) (RepositoryIdentity, error)
}

var aboutTextDoors = aboutTextDoorInventory{
	NewIdentifier: NewIdentifier, NewName: NewName, NewText: NewText,
	ParseSourcePath: ParseSourcePath, NewRepositoryIdentity: NewRepositoryIdentity,
}

func TestRunProtocolDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	gotStructs, gotMarkers, gotErr := scanRunProtocolDataFlowInventory()
	if gotErr != nil {
		t.Fatalf("scanRunProtocolDataFlowInventory() error = %v, want nil", gotErr)
	}
	if !slices.Equal(gotStructs, gotMarkers) {
		t.Fatalf("Run protocol production structs = %q, want exactly one classified marker each; markers = %q", gotStructs, gotMarkers)
	}
}

func TestRunProtocolExternalIngressInventoryMatchesSemanticFuzzDoors(t *testing.T) {
	t.Parallel()

	gotJSON, gotText, gotErr := scanRunProtocolExternalDoors()
	if gotErr != nil {
		t.Fatalf("scanRunProtocolExternalDoors() error = %v, want nil", gotErr)
	}
	wantJSON := inventoryFieldNames(runProtocolJSONDoors)
	wantText := inventoryFieldNames(aboutTextDoors)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("Run protocol production JSON doors = %q, want semantic fuzz inventory %q", gotJSON, wantJSON)
	}
	if !slices.Equal(gotText, wantText) {
		t.Fatalf("Run protocol production text doors = %q, want semantic fuzz inventory %q", gotText, wantText)
	}
	if len(wantJSON) != int(runProtocolJSONDoorLimit)+1 {
		t.Fatalf("Run protocol JSON fuzz selectors = %d atomic + 1 document, want %d inventoried doors", runProtocolJSONDoorLimit, len(wantJSON))
	}
}

func scanRunProtocolDataFlowInventory() ([]string, []string, error) {
	entries, err := aboutContractSources.ReadDir(".")
	if err != nil {
		return nil, nil, err
	}
	fileSet := token.NewFileSet()
	var structs []string
	var markers []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := aboutContractSources.ReadFile(entry.Name())
		if readErr != nil {
			return nil, nil, readErr
		}
		file, parseErr := parser.ParseFile(fileSet, entry.Name(), source, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		structs = append(structs, aboutStructNames(file)...)
		markers = append(markers, aboutMarkerReceivers(file)...)
	}
	sort.Strings(structs)
	sort.Strings(markers)
	return structs, markers, nil
}

func scanRunProtocolExternalDoors() ([]string, []string, error) {
	entries, err := aboutContractSources.ReadDir(".")
	if err != nil {
		return nil, nil, err
	}
	fileSet := token.NewFileSet()
	var jsonDoors []string
	var textDoors []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := aboutContractSources.ReadFile(entry.Name())
		if readErr != nil {
			return nil, nil, readErr
		}
		file, parseErr := parser.ParseFile(fileSet, entry.Name(), source, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if receiver, ok := aboutJSONReceiver(function); ok {
				jsonDoors = append(jsonDoors, receiver)
			}
			if aboutExportedTextDoor(function) {
				textDoors = append(textDoors, function.Name.Name)
			}
		}
	}
	sort.Strings(jsonDoors)
	sort.Strings(textDoors)
	return jsonDoors, textDoors, nil
}

func aboutJSONReceiver(function *ast.FuncDecl) (string, bool) {
	if function.Name.Name != "UnmarshalJSON" || function.Recv == nil || len(function.Recv.List) != 1 {
		return "", false
	}
	pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return "", false
	}
	receiver, ok := pointer.X.(*ast.Ident)
	return receiver.Name, ok
}

func aboutExportedTextDoor(function *ast.FuncDecl) bool {
	if function.Recv != nil || !ast.IsExported(function.Name.Name) || function.Type.Params == nil || len(function.Type.Params.List) == 0 {
		return false
	}
	first, ok := function.Type.Params.List[0].Type.(*ast.Ident)
	return ok && first.Name == "string"
}

func inventoryFieldNames[T any](inventory T) []string {
	typeOf := reflect.TypeOf(inventory)
	names := make([]string, 0, typeOf.NumField())
	for field := range typeOf.Fields() {
		names = append(names, field.Name)
	}
	sort.Strings(names)
	return names
}

func aboutStructNames(file *ast.File) []string {
	var names []string
	ast.Inspect(file, func(node ast.Node) bool {
		specification, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if _, ok := specification.Type.(*ast.StructType); ok {
			names = append(names, specification.Name.Name)
		}
		return true
	})
	return names
}

func aboutMarkerReceivers(file *ast.File) []string {
	var names []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || !aboutRoleMarker(function.Name.Name) {
			continue
		}
		identifier, ok := function.Recv.List[0].Type.(*ast.Ident)
		if ok {
			names = append(names, identifier.Name)
		}
	}
	return names
}

func aboutRoleMarker(name string) bool {
	switch name {
	case "runProtocolFact", "runProtocolSealedProjection", "runProtocolInternalFlowCarrier":
		return true
	default:
		return false
	}
}
