package projectstandards

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

type projectStandardsJSONDoorInventory struct {
	Identifier                 func(*Identifier, []byte) error
	Name                       func(*Name, []byte) error
	Text                       func(*Text, []byte) error
	SourcePath                 func(*SourcePath, []byte) error
	RepositoryIdentity         func(*RepositoryIdentity, []byte) error
	ProfileIdentity            func(*ProfileIdentity, []byte) error
	RequestIdentity            func(*RequestIdentity, []byte) error
	RequestNonce               func(*RequestNonce, []byte) error
	RunID                      func(*RunID, []byte) error
	ExperimentID               func(*ExperimentID, []byte) error
	ObservationID              func(*ObservationID, []byte) error
	MachineID                  func(*MachineID, []byte) error
	MachineGenerationID        func(*MachineGenerationID, []byte) error
	MachineObservationID       func(*MachineObservationID, []byte) error
	DeliveryState              func(*DeliveryState, []byte) error
	CachePosture               func(*CachePosture, []byte) error
	Outcome                    func(*Outcome, []byte) error
	ReportPlacement            func(*ReportPlacement, []byte) error
	ComplexityGrowth           func(*ComplexityGrowth, []byte) error
	ComplexityCase             func(*ComplexityCase, []byte) error
	ComplexityAssessmentStatus func(*ComplexityAssessmentStatus, []byte) error
	DispositionKind            func(*DispositionKind, []byte) error
	RefusalReason              func(*RefusalReason, []byte) error
	TerminalState              func(*TerminalState, []byte) error
	ObservationKind            func(*ObservationKind, []byte) error
	InfrastructureStage        func(*InfrastructureStage, []byte) error
	MachineToolchainKind       func(*MachineToolchainKind, []byte) error
	MachineInstallMode         func(*MachineInstallMode, []byte) error
	MachineProvisioningModel   func(*MachineProvisioningModel, []byte) error
	MachineMaintenancePolicy   func(*MachineMaintenancePolicy, []byte) error
	MachineChangeField         func(*MachineChangeField, []byte) error
	AssuranceStage             func(*AssuranceStage, []byte) error
	AssuranceAuthority         func(*AssuranceAuthority, []byte) error
	ComponentKind              func(*ComponentKind, []byte) error
	ProbeRole                  func(*ProbeRole, []byte) error
	ProbeKind                  func(*ProbeKind, []byte) error
	ProbeTargetKind            func(*ProbeTargetKind, []byte) error
	SourceAnalysisCompleteness func(*SourceAnalysisCompleteness, []byte) error
	FunctionReferencePosture   func(*FunctionReferencePosture, []byte) error
	QueryKind                  func(*QueryKind, []byte) error
	Query                      func(*Query, []byte) error
	Response                   func(*Response, []byte) error
	MachineProbeReport         func(*MachineProbeReport, []byte) error
	MachineQuery               func(*MachineQuery, []byte) error
	MachineResponse            func(*MachineResponse, []byte) error
}

var projectStandardsJSONDoors = projectStandardsJSONDoorInventory{
	Identifier: (*Identifier).UnmarshalJSON, Name: (*Name).UnmarshalJSON,
	Text: (*Text).UnmarshalJSON, SourcePath: (*SourcePath).UnmarshalJSON,
	RepositoryIdentity:         (*RepositoryIdentity).UnmarshalJSON,
	ProfileIdentity:            (*ProfileIdentity).UnmarshalJSON,
	RequestIdentity:            (*RequestIdentity).UnmarshalJSON,
	RequestNonce:               (*RequestNonce).UnmarshalJSON,
	RunID:                      (*RunID).UnmarshalJSON,
	ExperimentID:               (*ExperimentID).UnmarshalJSON,
	ObservationID:              (*ObservationID).UnmarshalJSON,
	MachineID:                  (*MachineID).UnmarshalJSON,
	MachineGenerationID:        (*MachineGenerationID).UnmarshalJSON,
	MachineObservationID:       (*MachineObservationID).UnmarshalJSON,
	DeliveryState:              (*DeliveryState).UnmarshalJSON,
	CachePosture:               (*CachePosture).UnmarshalJSON,
	Outcome:                    (*Outcome).UnmarshalJSON,
	ReportPlacement:            (*ReportPlacement).UnmarshalJSON,
	ComplexityGrowth:           (*ComplexityGrowth).UnmarshalJSON,
	ComplexityCase:             (*ComplexityCase).UnmarshalJSON,
	ComplexityAssessmentStatus: (*ComplexityAssessmentStatus).UnmarshalJSON,
	DispositionKind:            (*DispositionKind).UnmarshalJSON,
	RefusalReason:              (*RefusalReason).UnmarshalJSON,
	TerminalState:              (*TerminalState).UnmarshalJSON,
	ObservationKind:            (*ObservationKind).UnmarshalJSON,
	InfrastructureStage:        (*InfrastructureStage).UnmarshalJSON,
	MachineToolchainKind:       (*MachineToolchainKind).UnmarshalJSON,
	MachineInstallMode:         (*MachineInstallMode).UnmarshalJSON,
	MachineProvisioningModel:   (*MachineProvisioningModel).UnmarshalJSON,
	MachineMaintenancePolicy:   (*MachineMaintenancePolicy).UnmarshalJSON,
	MachineChangeField:         (*MachineChangeField).UnmarshalJSON,
	AssuranceStage:             (*AssuranceStage).UnmarshalJSON,
	AssuranceAuthority:         (*AssuranceAuthority).UnmarshalJSON,
	ComponentKind:              (*ComponentKind).UnmarshalJSON,
	ProbeRole:                  (*ProbeRole).UnmarshalJSON,
	ProbeKind:                  (*ProbeKind).UnmarshalJSON,
	ProbeTargetKind:            (*ProbeTargetKind).UnmarshalJSON,
	SourceAnalysisCompleteness: (*SourceAnalysisCompleteness).UnmarshalJSON,
	FunctionReferencePosture:   (*FunctionReferencePosture).UnmarshalJSON,
	QueryKind:                  (*QueryKind).UnmarshalJSON,
	Query:                      (*Query).UnmarshalJSON,
	Response:                   (*Response).UnmarshalJSON,
	MachineProbeReport:         (*MachineProbeReport).UnmarshalJSON,
	MachineQuery:               (*MachineQuery).UnmarshalJSON,
	MachineResponse:            (*MachineResponse).UnmarshalJSON,
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

func TestProjectStandardsDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	gotStructs, gotMarkers, gotErr := scanProjectStandardsDataFlowInventory()
	if gotErr != nil {
		t.Fatalf("scanProjectStandardsDataFlowInventory() error = %v, want nil", gotErr)
	}
	if !slices.Equal(gotStructs, gotMarkers) {
		t.Fatalf("Project standards production structs = %q, want exactly one classified marker each; markers = %q", gotStructs, gotMarkers)
	}
}

func TestProjectStandardsExternalIngressInventoryMatchesSemanticFuzzDoors(t *testing.T) {
	t.Parallel()

	gotJSON, gotText, gotErr := scanProjectStandardsExternalDoors()
	if gotErr != nil {
		t.Fatalf("scanProjectStandardsExternalDoors() error = %v, want nil", gotErr)
	}
	wantJSON := inventoryFieldNames(projectStandardsJSONDoors)
	wantText := inventoryFieldNames(aboutTextDoors)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("Project standards production JSON doors = %q, want semantic fuzz inventory %q", gotJSON, wantJSON)
	}
	if !slices.Equal(gotText, wantText) {
		t.Fatalf("Project standards production text doors = %q, want semantic fuzz inventory %q", gotText, wantText)
	}
	if len(wantJSON) != int(projectStandardsJSONDoorLimit)+5 {
		t.Fatalf("Project standards JSON fuzz selectors = %d atomic + 5 documents, want %d inventoried doors", projectStandardsJSONDoorLimit, len(wantJSON))
	}
}

func scanProjectStandardsDataFlowInventory() ([]string, []string, error) {
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

func scanProjectStandardsExternalDoors() ([]string, []string, error) {
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
	case "projectStandardsProtocolFact", "projectStandardsSealedProjection", "projectStandardsInternalFlowCarrier", "projectStandardsCapabilityWrapper":
		return true
	default:
		return false
	}
}
