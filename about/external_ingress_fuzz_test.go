package about

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
)

type aboutJSONDoor uint8

const (
	doorIdentifier aboutJSONDoor = iota
	doorName
	doorText
	doorSourcePath
	doorRepositoryIdentity
	doorProfileIdentity
	doorRequestIdentity
	doorRunID
	doorExperimentID
	doorObservationID
	doorMachineID
	doorMachineGenerationID
	doorMachineObservationID
	doorDeliveryState
	doorOutcome
	doorReportPlacement
	doorComplexityGrowth
	doorComplexityCase
	doorComplexityAssessment
	doorDispositionKind
	doorRefusalReason
	doorTerminalState
	doorObservationKind
	doorInfrastructureStage
	doorMachineToolchainKind
	doorMachineInstallMode
	doorMachineProvisioningModel
	doorMachineMaintenancePolicy
	doorMachineChangeField
	doorAssuranceStage
	doorAssuranceAuthority
	doorComponentKind
	doorProbeRole
	doorProbeKind
	doorProbeTargetKind
	doorSourceCompleteness
	doorFunctionPosture
	doorQueryKind
	aboutJSONDoorLimit
)

type comparableAboutJSON interface {
	comparable
	core.ValidatedJSONMarshaler
}

func FuzzAboutTextConstructorsSemanticClosure(f *testing.F) {
	f.Add(uint8(0), "identity")
	f.Add(uint8(1), "Package name")
	f.Add(uint8(2), "Exact engineering knowledge.")
	f.Add(uint8(3), "about/catalog.go")
	f.Add(uint8(4), "github.com/deliri/primitive")
	f.Add(uint8(0), "")
	f.Add(uint8(3), "../outside")

	f.Fuzz(func(t *testing.T, selector uint8, value string) {
		switch selector % 5 {
		case 0:
			proveTextConstructorClosure(t, value, NewIdentifier, (*Identifier).UnmarshalJSON)
		case 1:
			proveTextConstructorClosure(t, value, NewName, (*Name).UnmarshalJSON)
		case 2:
			proveTextConstructorClosure(t, value, NewText, (*Text).UnmarshalJSON)
		case 3:
			proveTextConstructorClosure(t, value, ParseSourcePath, (*SourcePath).UnmarshalJSON)
		case 4:
			proveTextConstructorClosure(t, value, NewRepositoryIdentity, (*RepositoryIdentity).UnmarshalJSON)
		default:
			t.Fatalf("About text constructor selector = %d, want closed 0..4", selector%5)
		}
	})
}

func FuzzAboutAtomicJSONDoorsSemanticClosure(f *testing.F) {
	addAtomicDoorSeed(f, doorIdentifier, fixtureIdentifier(f, "identity"))
	addAtomicDoorSeed(f, doorName, fixtureName(f, "Package name"))
	addAtomicDoorSeed(f, doorText, fixtureText(f, "Exact engineering knowledge."))
	addAtomicDoorSeed(f, doorSourcePath, fixturePath(f, "about/catalog.go"))
	addAtomicDoorSeed(f, doorRepositoryIdentity, fixtureRepository(f, "github.com/deliri/primitive"))
	addAtomicDoorSeed(f, doorProfileIdentity, fixtureProfile(f, "acceptance"))
	uuid := fixtureAboutUUID(f)
	requestID, requestErr := NewRequestIdentity(uuid)
	if requestErr != nil {
		f.Fatalf("NewRequestIdentity(seed) error = %v, want nil", requestErr)
	}
	runID, runErr := NewRunID(uuid)
	if runErr != nil {
		f.Fatalf("NewRunID(seed) error = %v, want nil", runErr)
	}
	experimentID, experimentErr := NewExperimentID(uuid)
	if experimentErr != nil {
		f.Fatalf("NewExperimentID(seed) error = %v, want nil", experimentErr)
	}
	observationID, observationErr := NewObservationID(uuid)
	if observationErr != nil {
		f.Fatalf("NewObservationID(seed) error = %v, want nil", observationErr)
	}
	addAtomicDoorSeed(f, doorRequestIdentity, requestID)
	addAtomicDoorSeed(f, doorRunID, runID)
	addAtomicDoorSeed(f, doorExperimentID, experimentID)
	addAtomicDoorSeed(f, doorObservationID, observationID)
	machineID, machineErr := NewMachineID(uuid)
	if machineErr != nil {
		f.Fatalf("NewMachineID(seed) error = %v, want nil", machineErr)
	}
	generationID, generationErr := NewMachineGenerationID(uuid)
	if generationErr != nil {
		f.Fatalf("NewMachineGenerationID(seed) error = %v, want nil", generationErr)
	}
	machineObservationID, machineObservationErr := NewMachineObservationID(uuid)
	if machineObservationErr != nil {
		f.Fatalf("NewMachineObservationID(seed) error = %v, want nil", machineObservationErr)
	}
	addAtomicDoorSeed(f, doorMachineID, machineID)
	addAtomicDoorSeed(f, doorMachineGenerationID, generationID)
	addAtomicDoorSeed(f, doorMachineObservationID, machineObservationID)
	addEnumDoorSeeds(f, doorDeliveryState, uint8(deliveryLimit), func(value uint8) DeliveryState { return DeliveryState(value) })
	addEnumDoorSeeds(f, doorOutcome, uint8(outcomeLimit), func(value uint8) Outcome { return Outcome(value) })
	addEnumDoorSeeds(f, doorReportPlacement, uint8(reportPlacementLimit), func(value uint8) ReportPlacement { return ReportPlacement(value) })
	addEnumDoorSeeds(f, doorComplexityGrowth, uint8(complexityGrowthLimit), func(value uint8) ComplexityGrowth { return ComplexityGrowth(value) })
	addEnumDoorSeeds(f, doorComplexityCase, uint8(complexityCaseLimit), func(value uint8) ComplexityCase { return ComplexityCase(value) })
	addEnumDoorSeeds(f, doorComplexityAssessment, uint8(complexityAssessmentLimit), func(value uint8) ComplexityAssessmentStatus { return ComplexityAssessmentStatus(value) })
	addEnumDoorSeeds(f, doorDispositionKind, uint8(dispositionLimit), func(value uint8) DispositionKind { return DispositionKind(value) })
	addEnumDoorSeeds(f, doorRefusalReason, uint8(refusalLimit), func(value uint8) RefusalReason { return RefusalReason(value) })
	addEnumDoorSeeds(f, doorTerminalState, uint8(terminalLimit), func(value uint8) TerminalState { return TerminalState(value) })
	addEnumDoorSeeds(f, doorObservationKind, uint8(observationLimit), func(value uint8) ObservationKind { return ObservationKind(value) })
	addEnumDoorSeeds(f, doorInfrastructureStage, uint8(infrastructureStageLimit), func(value uint8) InfrastructureStage { return InfrastructureStage(value) })
	addEnumDoorSeeds(f, doorMachineToolchainKind, uint8(machineToolchainLimit), func(value uint8) MachineToolchainKind { return MachineToolchainKind(value) })
	addEnumDoorSeeds(f, doorMachineInstallMode, uint8(machineInstallModeLimit), func(value uint8) MachineInstallMode { return MachineInstallMode(value) })
	addEnumDoorSeeds(f, doorMachineProvisioningModel, uint8(machineProvisioningLimit), func(value uint8) MachineProvisioningModel { return MachineProvisioningModel(value) })
	addEnumDoorSeeds(f, doorMachineMaintenancePolicy, uint8(machineMaintenanceLimit), func(value uint8) MachineMaintenancePolicy { return MachineMaintenancePolicy(value) })
	addEnumDoorSeeds(f, doorMachineChangeField, uint8(machineChangeFieldLimit), func(value uint8) MachineChangeField { return MachineChangeField(value) })
	addEnumDoorSeeds(f, doorAssuranceStage, uint8(assuranceStageLimit), func(value uint8) AssuranceStage { return AssuranceStage(value) })
	addEnumDoorSeeds(f, doorAssuranceAuthority, uint8(assuranceAuthorityLimit), func(value uint8) AssuranceAuthority { return AssuranceAuthority(value) })
	addEnumDoorSeeds(f, doorComponentKind, uint8(componentKindLimit), func(value uint8) ComponentKind { return ComponentKind(value) })
	addEnumDoorSeeds(f, doorProbeRole, uint8(probeRoleLimit), func(value uint8) ProbeRole { return ProbeRole(value) })
	addEnumDoorSeeds(f, doorProbeKind, uint8(probeKindLimit), func(value uint8) ProbeKind { return ProbeKind(value) })
	addEnumDoorSeeds(f, doorProbeTargetKind, uint8(probeTargetLimit), func(value uint8) ProbeTargetKind { return ProbeTargetKind(value) })
	addEnumDoorSeeds(f, doorSourceCompleteness, uint8(sourceAnalysisLimit), func(value uint8) SourceAnalysisCompleteness { return SourceAnalysisCompleteness(value) })
	addEnumDoorSeeds(f, doorFunctionPosture, uint8(functionReferenceLimit), func(value uint8) FunctionReferencePosture { return FunctionReferencePosture(value) })
	addEnumDoorSeeds(f, doorQueryKind, uint8(queryKindLimit), func(value uint8) QueryKind { return QueryKind(value) })
	f.Add(uint8(doorIdentifier), []byte{})
	f.Add(uint8(doorIdentifier), []byte(`{}`))
	f.Add(uint8(doorIdentifier), []byte(`null`))
	// Regression: semantic scalar refusal must retain the JSON boundary identity.
	f.Add(uint8(doorName), []byte(`""`))

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		door := aboutJSONDoor(rawDoor % uint8(aboutJSONDoorLimit))
		switch door {
		case doorIdentifier:
			proveComparableJSONClosure(t, fixtureIdentifier(t, "identity"), data, (*Identifier).UnmarshalJSON)
		case doorName:
			proveComparableJSONClosure(t, fixtureName(t, "Package name"), data, (*Name).UnmarshalJSON)
		case doorText:
			proveComparableJSONClosure(t, fixtureText(t, "Exact engineering knowledge."), data, (*Text).UnmarshalJSON)
		case doorSourcePath:
			proveComparableJSONClosure(t, fixturePath(t, "about/catalog.go"), data, (*SourcePath).UnmarshalJSON)
		case doorRepositoryIdentity:
			proveComparableJSONClosure(t, fixtureRepository(t, "github.com/deliri/primitive"), data, (*RepositoryIdentity).UnmarshalJSON)
		case doorProfileIdentity:
			proveComparableJSONClosure(t, fixtureProfile(t, "acceptance"), data, (*ProfileIdentity).UnmarshalJSON)
		case doorRequestIdentity:
			seed, err := NewRequestIdentity(fixtureAboutUUID(t))
			proveConstructedDoor(t, seed, err, data, (*RequestIdentity).UnmarshalJSON)
		case doorRunID:
			seed, err := NewRunID(fixtureAboutUUID(t))
			proveConstructedDoor(t, seed, err, data, (*RunID).UnmarshalJSON)
		case doorExperimentID:
			seed, err := NewExperimentID(fixtureAboutUUID(t))
			proveConstructedDoor(t, seed, err, data, (*ExperimentID).UnmarshalJSON)
		case doorObservationID:
			seed, err := NewObservationID(fixtureAboutUUID(t))
			proveConstructedDoor(t, seed, err, data, (*ObservationID).UnmarshalJSON)
		case doorMachineID:
			seed, err := NewMachineID(fixtureAboutUUID(t))
			proveConstructedDoor(t, seed, err, data, (*MachineID).UnmarshalJSON)
		case doorMachineGenerationID:
			seed, err := NewMachineGenerationID(fixtureAboutUUID(t))
			proveConstructedDoor(t, seed, err, data, (*MachineGenerationID).UnmarshalJSON)
		case doorMachineObservationID:
			seed, err := NewMachineObservationID(fixtureAboutUUID(t))
			proveConstructedDoor(t, seed, err, data, (*MachineObservationID).UnmarshalJSON)
		case doorDeliveryState:
			proveComparableJSONClosure(t, DeliveryPlanned, data, (*DeliveryState).UnmarshalJSON)
		case doorOutcome:
			proveComparableJSONClosure(t, OutcomePassed, data, (*Outcome).UnmarshalJSON)
		case doorReportPlacement:
			proveComparableJSONClosure(t, ReportPlacementProject, data, (*ReportPlacement).UnmarshalJSON)
		case doorComplexityGrowth:
			proveComparableJSONClosure(t, ComplexityConstant, data, (*ComplexityGrowth).UnmarshalJSON)
		case doorComplexityCase:
			proveComparableJSONClosure(t, ComplexityBestCase, data, (*ComplexityCase).UnmarshalJSON)
		case doorComplexityAssessment:
			proveComparableJSONClosure(t, ComplexityAssessmentSupported, data, (*ComplexityAssessmentStatus).UnmarshalJSON)
		case doorDispositionKind:
			proveComparableJSONClosure(t, DispositionAdmitted, data, (*DispositionKind).UnmarshalJSON)
		case doorRefusalReason:
			proveComparableJSONClosure(t, RefusalUnauthorized, data, (*RefusalReason).UnmarshalJSON)
		case doorTerminalState:
			proveComparableJSONClosure(t, TerminalCompleted, data, (*TerminalState).UnmarshalJSON)
		case doorObservationKind:
			proveComparableJSONClosure(t, ObservationExperiment, data, (*ObservationKind).UnmarshalJSON)
		case doorInfrastructureStage:
			proveComparableJSONClosure(t, InfrastructureStageSource, data, (*InfrastructureStage).UnmarshalJSON)
		case doorMachineToolchainKind:
			proveComparableJSONClosure(t, MachineToolchainGo, data, (*MachineToolchainKind).UnmarshalJSON)
		case doorMachineInstallMode:
			proveComparableJSONClosure(t, MachineInstallModeInstalled, data, (*MachineInstallMode).UnmarshalJSON)
		case doorMachineProvisioningModel:
			proveComparableJSONClosure(t, MachineProvisioningStandard, data, (*MachineProvisioningModel).UnmarshalJSON)
		case doorMachineMaintenancePolicy:
			proveComparableJSONClosure(t, MachineMaintenanceMigrate, data, (*MachineMaintenancePolicy).UnmarshalJSON)
		case doorMachineChangeField:
			proveComparableJSONClosure(t, MachineChangeCompute, data, (*MachineChangeField).UnmarshalJSON)
		case doorAssuranceStage:
			proveComparableJSONClosure(t, AssuranceStagePolicy, data, (*AssuranceStage).UnmarshalJSON)
		case doorAssuranceAuthority:
			proveComparableJSONClosure(t, AssuranceAuthorityProduct, data, (*AssuranceAuthority).UnmarshalJSON)
		case doorComponentKind:
			proveComparableJSONClosure(t, ComponentKindSourceFile, data, (*ComponentKind).UnmarshalJSON)
		case doorProbeRole:
			proveComparableJSONClosure(t, ProbeRoleSelection, data, (*ProbeRole).UnmarshalJSON)
		case doorProbeKind:
			proveComparableJSONClosure(t, ProbeKindGoFileSelection, data, (*ProbeKind).UnmarshalJSON)
		case doorProbeTargetKind:
			proveComparableJSONClosure(t, ProbeTargetGoFile, data, (*ProbeTargetKind).UnmarshalJSON)
		case doorSourceCompleteness:
			proveComparableJSONClosure(t, SourceAnalysisComplete, data, (*SourceAnalysisCompleteness).UnmarshalJSON)
		case doorFunctionPosture:
			proveComparableJSONClosure(t, FunctionProductionReferenceObserved, data, (*FunctionReferencePosture).UnmarshalJSON)
		case doorQueryKind:
			proveComparableJSONClosure(t, QueryProject, data, (*QueryKind).UnmarshalJSON)
		default:
			t.Fatalf("About JSON door = %d, want a closed admitted door", door)
		}
	})
}

func FuzzAboutQueryJSONSemanticClosure(f *testing.F) {
	catalog := fixtureCatalog(f)
	project := fixtureProjectQuery(catalog)
	packagePath := catalog.Packages[0].Package.Knowledge.Path
	packageQuery := project
	packageQuery.Kind = QueryPackage
	packageQuery.Package = &packagePath
	addDocumentSeed(f, uint8(0), project)
	addDocumentSeed(f, uint8(1), packageQuery)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(0), []byte(`{}`))
	f.Add(uint8(0), []byte(`null`))

	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		seed := project
		if selector%2 == 1 {
			seed = packageQuery
		}
		proveDocumentJSONClosure(t, seed, data, (*Query).UnmarshalJSON)
	})
}

func FuzzAboutResponseJSONSemanticClosure(f *testing.F) {
	catalog := fixtureCatalog(f)
	projectQuery := fixtureProjectQuery(catalog)
	projectResponse := Response{SchemaVersion: SchemaVersion, Query: projectQuery, Project: &catalog.Project}
	packagePath := catalog.Packages[0].Package.Knowledge.Path
	packageQuery := projectQuery
	packageQuery.Kind = QueryPackage
	packageQuery.Package = &packagePath
	packageResponse := Response{SchemaVersion: SchemaVersion, Query: packageQuery, Package: &catalog.Packages[0]}
	addDocumentSeed(f, uint8(0), projectResponse)
	addDocumentSeed(f, uint8(1), packageResponse)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(0), []byte(`{}`))
	f.Add(uint8(0), []byte(`null`))

	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		seed := projectResponse
		if selector%2 == 1 {
			seed = packageResponse
		}
		proveDocumentJSONClosure(t, seed, data, (*Response).UnmarshalJSON)
	})
}

func FuzzAboutMachineProbeReportJSONSemanticClosure(f *testing.F) {
	seed := fixtureMachineProbeReport(f)
	addDocumentSeed(f, uint8(0), seed)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(0), []byte(`{}`))
	f.Add(uint8(0), []byte(`null`))

	f.Fuzz(func(t *testing.T, _ uint8, data []byte) {
		proveDocumentJSONClosure(t, seed, data, (*MachineProbeReport).UnmarshalJSON)
	})
}

func FuzzAboutMachineQueryJSONSemanticClosure(f *testing.F) {
	current := fixtureCurrentMachine(f)
	seed := MachineQuery{SchemaVersion: SchemaVersion, Machine: current.Generation.Configuration.Identity.ID}
	addDocumentSeed(f, uint8(0), seed)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(0), []byte(`{}`))
	f.Add(uint8(0), []byte(`null`))

	f.Fuzz(func(t *testing.T, _ uint8, data []byte) {
		proveDocumentJSONClosure(t, seed, data, (*MachineQuery).UnmarshalJSON)
	})
}

func FuzzAboutMachineResponseJSONSemanticClosure(f *testing.F) {
	current := fixtureCurrentMachine(f)
	query := MachineQuery{SchemaVersion: SchemaVersion, Machine: current.Generation.Configuration.Identity.ID}
	seed := MachineResponse{SchemaVersion: SchemaVersion, Query: query, Machine: current}
	addDocumentSeed(f, uint8(0), seed)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(0), []byte(`{}`))
	f.Add(uint8(0), []byte(`null`))

	f.Fuzz(func(t *testing.T, _ uint8, data []byte) {
		proveDocumentJSONClosure(t, seed, data, (*MachineResponse).UnmarshalJSON)
	})
}

func addAtomicDoorSeed[T comparableAboutJSON](f *testing.F, door aboutJSONDoor, seed T) {
	f.Helper()
	encoded, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("About atomic JSON seed MarshalJSON() error = %v, want nil", err)
	}
	f.Add(uint8(door), encoded)
}

func addEnumDoorSeeds[T comparableAboutJSON](f *testing.F, door aboutJSONDoor, limit uint8, construct func(uint8) T) {
	f.Helper()
	for value := uint8(1); value < limit; value++ {
		addAtomicDoorSeed(f, door, construct(value))
	}
}

func addDocumentSeed[T core.ValidatedJSONMarshaler](f *testing.F, selector uint8, seed T) {
	f.Helper()
	encoded, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("About document seed MarshalJSON() error = %v, want nil", err)
	}
	f.Add(selector, encoded)
}

func proveConstructedDoor[T comparableAboutJSON](t *testing.T, seed T, constructErr error, data []byte, unmarshal func(*T, []byte) error) {
	t.Helper()
	if constructErr != nil {
		t.Fatalf("About identity seed construction error = %v, want nil", constructErr)
	}
	proveComparableJSONClosure(t, seed, data, unmarshal)
}

func proveTextConstructorClosure[T comparableAboutJSON](t *testing.T, value string, construct func(string) (T, error), unmarshal func(*T, []byte) error) {
	t.Helper()
	got, gotErr := construct(value)
	if gotErr != nil {
		var zero T
		if !errors.Is(gotErr, core.ErrAboutContract) || got != zero {
			t.Fatalf("About text constructor(rejected) = (%v, %v), want (zero, errors.Is(..., %v))", got, gotErr, core.ErrAboutContract)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("About text constructor(accepted).Validate() error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		t.Fatalf("About text constructor(accepted).MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), err)
	}
	var roundTrip T
	if err := unmarshal(&roundTrip, encoded); err != nil || roundTrip != got {
		t.Fatalf("About text constructor JSON round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func proveComparableJSONClosure[T comparableAboutJSON](t *testing.T, seed T, data []byte, unmarshal func(*T, []byte) error) {
	t.Helper()
	got := seed
	gotErr := unmarshal(&got, data)
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrAboutContract) || got != seed {
			t.Fatalf("About atomic UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/About rejection", got, gotErr)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("About atomic UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		t.Fatalf("About atomic MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
	}
	var roundTrip T
	if err := unmarshal(&roundTrip, encoded); err != nil || roundTrip != got {
		t.Fatalf("About atomic canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("About atomic second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
	}
}

func proveDocumentJSONClosure[T core.ValidatedJSONMarshaler](t *testing.T, seed T, data []byte, unmarshal func(*T, []byte) error) {
	t.Helper()
	wantPreserved, wantErr := seed.MarshalJSON()
	if wantErr != nil {
		t.Fatalf("About document seed MarshalJSON() setup error = %v, want nil", wantErr)
	}
	got := seed
	gotErr := unmarshal(&got, data)
	if gotErr != nil {
		preserved, preservedErr := got.MarshalJSON()
		if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrAboutContract) || preservedErr != nil || !bytes.Equal(preserved, wantPreserved) {
			t.Fatalf("About document UnmarshalJSON(rejected) = (%d preserved bytes, %v, %v), want exact preserved seed and typed JSON/About rejection", len(preserved), preservedErr, gotErr)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("About document UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		t.Fatalf("About document MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
	}
	var roundTrip T
	if err := unmarshal(&roundTrip, encoded); err != nil {
		t.Fatalf("About document canonical round trip error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("About document second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
	}
}

func fixtureAboutUUID(t testing.TB) primitiveid.UUIDv7 {
	t.Helper()
	got, gotErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	if gotErr != nil {
		t.Fatalf("id.ParseUUIDv7(seed) error = %v, want nil", gotErr)
	}
	return got
}
