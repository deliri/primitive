package runprotocol

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
)

type runProtocolJSONDoor uint8

const (
	doorIdentifier runProtocolJSONDoor = iota
	doorName
	doorText
	doorSourcePath
	doorRepositoryIdentity
	doorProfileIdentity
	doorRequestIdentity
	doorRequestNonce
	doorRunID
	doorExperimentID
	doorObservationID
	doorMachineID
	doorMachineGenerationID
	doorMachineObservationID
	doorCachePosture
	doorOutcome
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
	doorProbeRole
	doorProbeKind
	doorProbeTargetKind
	runProtocolJSONDoorLimit
)

type comparableRunProtocolJSON interface {
	comparable
	core.ValidatedJSONMarshaler
}

func FuzzRunProtocolTextConstructorsSemanticClosure(f *testing.F) {
	f.Add(uint8(0), "identity")
	f.Add(uint8(1), "Package name")
	f.Add(uint8(2), "Exact engineering knowledge.")
	f.Add(uint8(3), "runprotocol/scalar.go")
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
			t.Fatalf("Run protocol text constructor selector = %d, want closed 0..4", selector%5)
		}
	})
}

func FuzzRunProtocolAtomicJSONDoorsSemanticClosure(f *testing.F) {
	addAtomicDoorSeed(f, doorIdentifier, fixtureIdentifier(f, "identity"))
	addAtomicDoorSeed(f, doorName, fixtureName(f, "Package name"))
	addAtomicDoorSeed(f, doorText, fixtureText(f, "Exact engineering knowledge."))
	addAtomicDoorSeed(f, doorSourcePath, fixturePath(f, "runprotocol/scalar.go"))
	addAtomicDoorSeed(f, doorRepositoryIdentity, fixtureRepository(f, "github.com/deliri/primitive"))
	addAtomicDoorSeed(f, doorProfileIdentity, fixtureProfile(f, "acceptance"))
	uuid := fixtureRunProtocolUUID(f)
	requestID, requestErr := NewRequestIdentity(uuid)
	if requestErr != nil {
		f.Fatalf("NewRequestIdentity(seed) error = %v, want nil", requestErr)
	}
	requestNonce, requestNonceErr := NewRequestNonce(uuid)
	if requestNonceErr != nil {
		f.Fatalf("NewRequestNonce(seed) error = %v, want nil", requestNonceErr)
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
	addAtomicDoorSeed(f, doorRequestNonce, requestNonce)
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
	addEnumDoorSeeds(f, doorCachePosture, uint8(cachePostureLimit), func(value uint8) CachePosture { return CachePosture(value) })
	addEnumDoorSeeds(f, doorOutcome, uint8(outcomeLimit), func(value uint8) Outcome { return Outcome(value) })
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
	addEnumDoorSeeds(f, doorProbeRole, uint8(probeRoleLimit), func(value uint8) ProbeRole { return ProbeRole(value) })
	addEnumDoorSeeds(f, doorProbeKind, uint8(probeKindLimit), func(value uint8) ProbeKind { return ProbeKind(value) })
	addEnumDoorSeeds(f, doorProbeTargetKind, uint8(probeTargetLimit), func(value uint8) ProbeTargetKind { return ProbeTargetKind(value) })
	f.Add(uint8(doorIdentifier), []byte{})
	f.Add(uint8(doorIdentifier), []byte(`{}`))
	f.Add(uint8(doorIdentifier), []byte(`null`))
	// Regression: semantic scalar refusal must retain the JSON boundary identity.
	f.Add(uint8(doorName), []byte(`""`))

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		door := runProtocolJSONDoor(rawDoor % uint8(runProtocolJSONDoorLimit))
		switch door {
		case doorIdentifier:
			proveComparableJSONClosure(t, fixtureIdentifier(t, "identity"), data, (*Identifier).UnmarshalJSON)
		case doorName:
			proveComparableJSONClosure(t, fixtureName(t, "Package name"), data, (*Name).UnmarshalJSON)
		case doorText:
			proveComparableJSONClosure(t, fixtureText(t, "Exact engineering knowledge."), data, (*Text).UnmarshalJSON)
		case doorSourcePath:
			proveComparableJSONClosure(t, fixturePath(t, "runprotocol/scalar.go"), data, (*SourcePath).UnmarshalJSON)
		case doorRepositoryIdentity:
			proveComparableJSONClosure(t, fixtureRepository(t, "github.com/deliri/primitive"), data, (*RepositoryIdentity).UnmarshalJSON)
		case doorProfileIdentity:
			proveComparableJSONClosure(t, fixtureProfile(t, "acceptance"), data, (*ProfileIdentity).UnmarshalJSON)
		case doorRequestIdentity:
			seed, err := NewRequestIdentity(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*RequestIdentity).UnmarshalJSON)
		case doorRequestNonce:
			seed, err := NewRequestNonce(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*RequestNonce).UnmarshalJSON)
		case doorRunID:
			seed, err := NewRunID(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*RunID).UnmarshalJSON)
		case doorExperimentID:
			seed, err := NewExperimentID(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*ExperimentID).UnmarshalJSON)
		case doorObservationID:
			seed, err := NewObservationID(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*ObservationID).UnmarshalJSON)
		case doorMachineID:
			seed, err := NewMachineID(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*MachineID).UnmarshalJSON)
		case doorMachineGenerationID:
			seed, err := NewMachineGenerationID(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*MachineGenerationID).UnmarshalJSON)
		case doorMachineObservationID:
			seed, err := NewMachineObservationID(fixtureRunProtocolUUID(t))
			proveConstructedDoor(t, seed, err, data, (*MachineObservationID).UnmarshalJSON)
		case doorCachePosture:
			proveComparableJSONClosure(t, CacheDisabled, data, (*CachePosture).UnmarshalJSON)
		case doorOutcome:
			proveComparableJSONClosure(t, OutcomePassed, data, (*Outcome).UnmarshalJSON)
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
		case doorProbeRole:
			proveComparableJSONClosure(t, ProbeRoleSelection, data, (*ProbeRole).UnmarshalJSON)
		case doorProbeKind:
			proveComparableJSONClosure(t, ProbeKindGoFileSelection, data, (*ProbeKind).UnmarshalJSON)
		case doorProbeTargetKind:
			proveComparableJSONClosure(t, ProbeTargetGoFile, data, (*ProbeTargetKind).UnmarshalJSON)
		default:
			t.Fatalf("Run protocol JSON door = %d, want a closed admitted door", door)
		}
	})
}

func FuzzRunProtocolMachineProbeReportJSONSemanticClosure(f *testing.F) {
	seed := fixtureMachineProbeReport(f)
	addDocumentSeed(f, uint8(0), seed)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(0), []byte(`{}`))
	f.Add(uint8(0), []byte(`null`))

	f.Fuzz(func(t *testing.T, _ uint8, data []byte) {
		proveDocumentJSONClosure(t, seed, data, (*MachineProbeReport).UnmarshalJSON)
	})
}

func addAtomicDoorSeed[T comparableRunProtocolJSON](f *testing.F, door runProtocolJSONDoor, seed T) {
	f.Helper()
	encoded, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("Run protocol atomic JSON seed MarshalJSON() error = %v, want nil", err)
	}
	f.Add(uint8(door), encoded)
}

func addEnumDoorSeeds[T comparableRunProtocolJSON](f *testing.F, door runProtocolJSONDoor, limit uint8, construct func(uint8) T) {
	f.Helper()
	for value := uint8(1); value < limit; value++ {
		addAtomicDoorSeed(f, door, construct(value))
	}
}

func addDocumentSeed[T core.ValidatedJSONMarshaler](f *testing.F, selector uint8, seed T) {
	f.Helper()
	encoded, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("Run protocol document seed MarshalJSON() error = %v, want nil", err)
	}
	f.Add(selector, encoded)
}

func proveConstructedDoor[T comparableRunProtocolJSON](t *testing.T, seed T, constructErr error, data []byte, unmarshal func(*T, []byte) error) {
	t.Helper()
	if constructErr != nil {
		t.Fatalf("Run protocol identity seed construction error = %v, want nil", constructErr)
	}
	proveComparableJSONClosure(t, seed, data, unmarshal)
}

func proveTextConstructorClosure[T comparableRunProtocolJSON](t *testing.T, value string, construct func(string) (T, error), unmarshal func(*T, []byte) error) {
	t.Helper()
	got, gotErr := construct(value)
	if gotErr != nil {
		var zero T
		if !errors.Is(gotErr, core.ErrRunProtocolContract) || got != zero {
			t.Fatalf("Run protocol text constructor(rejected) = (%v, %v), want (zero, errors.Is(..., %v))", got, gotErr, core.ErrRunProtocolContract)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Run protocol text constructor(accepted).Validate() error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		t.Fatalf("Run protocol text constructor(accepted).MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), err)
	}
	var roundTrip T
	if err := unmarshal(&roundTrip, encoded); err != nil || roundTrip != got {
		t.Fatalf("Run protocol text constructor JSON round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func proveComparableJSONClosure[T comparableRunProtocolJSON](t *testing.T, seed T, data []byte, unmarshal func(*T, []byte) error) {
	t.Helper()
	got := seed
	gotErr := unmarshal(&got, data)
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrRunProtocolContract) || got != seed {
			t.Fatalf("Run protocol atomic UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/run protocol rejection", got, gotErr)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Run protocol atomic UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		t.Fatalf("Run protocol atomic MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
	}
	var roundTrip T
	if err := unmarshal(&roundTrip, encoded); err != nil || roundTrip != got {
		t.Fatalf("Run protocol atomic canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("Run protocol atomic second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
	}
}

func proveDocumentJSONClosure[T core.ValidatedJSONMarshaler](t *testing.T, seed T, data []byte, unmarshal func(*T, []byte) error) {
	t.Helper()
	wantPreserved, wantErr := seed.MarshalJSON()
	if wantErr != nil {
		t.Fatalf("Run protocol document seed MarshalJSON() setup error = %v, want nil", wantErr)
	}
	got := seed
	gotErr := unmarshal(&got, data)
	if gotErr != nil {
		preserved, preservedErr := got.MarshalJSON()
		if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrRunProtocolContract) || preservedErr != nil || !bytes.Equal(preserved, wantPreserved) {
			t.Fatalf("Run protocol document UnmarshalJSON(rejected) = (%d preserved bytes, %v, %v), want exact preserved seed and typed JSON/run protocol rejection", len(preserved), preservedErr, gotErr)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Run protocol document UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		t.Fatalf("Run protocol document MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
	}
	var roundTrip T
	if err := unmarshal(&roundTrip, encoded); err != nil {
		t.Fatalf("Run protocol document canonical round trip error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("Run protocol document second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
	}
}

func fixtureRunProtocolUUID(t testing.TB) primitiveid.UUIDv7 {
	t.Helper()
	got, gotErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	if gotErr != nil {
		t.Fatalf("id.ParseUUIDv7(seed) error = %v, want nil", gotErr)
	}
	return got
}
