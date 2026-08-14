package wiring

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type wiringTestIdentity uint16

const (
	wiringTestIdentityUnknown wiringTestIdentity = iota
	wiringTestIdentityRoot
	wiringTestIdentityFirst
	wiringTestIdentitySecond
	wiringTestIdentityThird
	wiringTestIdentityMaximum = 256
)

func (i wiringTestIdentity) Validate() error {
	if i <= wiringTestIdentityUnknown || i >= wiringTestIdentityMaximum {
		return core.ErrPrimitiveContract
	}
	return nil
}

type wiringTestComponent struct {
	definition Definition[wiringTestIdentity]
}

func (c wiringTestComponent) WiringDefinition() Definition[wiringTestIdentity] {
	return c.definition
}

type wiringNilTestComponent struct{}

func (*wiringNilTestComponent) WiringDefinition() Definition[wiringTestIdentity] {
	panic("typed nil component reached its method")
}

func TestDeriveRejectsEveryBrokenRuntimeGraph(t *testing.T) {
	t.Parallel()

	typedNil := (*wiringNilTestComponent)(nil)
	tests := []struct {
		name      string
		request   Request[wiringTestIdentity]
		wantKind  ErrorKind
		wantOwner wiringTestIdentity
		wantPeer  wiringTestIdentity
		wantDoor  core.PackageIdentity
	}{
		{name: "zero root", request: wiringRequest(wiringTestIdentityUnknown,
			wiringDefinition(wiringTestIdentityRoot)), wantKind: ErrorKindRequest},
		{name: "empty component set", request: wiringRequest(wiringTestIdentityRoot),
			wantKind: ErrorKindRequest, wantOwner: wiringTestIdentityRoot},
		{name: "nil component", request: Request[wiringTestIdentity]{Root: wiringTestIdentityRoot,
			Components: []Describer[wiringTestIdentity]{nil}}, wantKind: ErrorKindComponent},
		{name: "typed nil component", request: Request[wiringTestIdentity]{Root: wiringTestIdentityRoot,
			Components: []Describer[wiringTestIdentity]{typedNil}}, wantKind: ErrorKindComponent},
		{name: "invalid component identity", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityUnknown)), wantKind: ErrorKindComponent},
		{name: "invalid dependency identity", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityUnknown)),
			wantKind: ErrorKindDependency, wantOwner: wiringTestIdentityRoot},
		{name: "invalid Primitive door", request: wiringRequest(wiringTestIdentityRoot,
			wiringDoorDefinition(wiringTestIdentityRoot, core.PackageUnknown)),
			wantKind: ErrorKindPrimitiveDoor, wantOwner: wiringTestIdentityRoot, wantDoor: core.PackageUnknown},
		{name: "test-support Primitive door", request: wiringRequest(wiringTestIdentityRoot,
			wiringDoorDefinition(wiringTestIdentityRoot, core.PackageTestSerial)),
			wantKind: ErrorKindPrimitiveDoor, wantOwner: wiringTestIdentityRoot, wantDoor: core.PackageTestSerial},
		{name: "duplicate component identity", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot), wiringDefinition(wiringTestIdentityRoot)),
			wantKind: ErrorKindDuplicateComponent, wantOwner: wiringTestIdentityRoot},
		{name: "duplicate dependency identity", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst, wiringTestIdentityFirst),
			wiringDefinition(wiringTestIdentityFirst)),
			wantKind: ErrorKindDuplicateDependency, wantOwner: wiringTestIdentityRoot, wantPeer: wiringTestIdentityFirst},
		{name: "duplicate Primitive door", request: wiringRequest(wiringTestIdentityRoot,
			wiringDoorDefinition(wiringTestIdentityRoot, core.PackageFilestore, core.PackageFilestore)),
			wantKind: ErrorKindDuplicatePrimitiveDoor, wantOwner: wiringTestIdentityRoot, wantDoor: core.PackageFilestore},
		{name: "root absent", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityFirst)), wantKind: ErrorKindMissingRoot, wantOwner: wiringTestIdentityRoot},
		{name: "callee absent", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst)),
			wantKind: ErrorKindMissingDependency, wantOwner: wiringTestIdentityRoot, wantPeer: wiringTestIdentityFirst},
		{name: "self cycle", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityRoot)),
			wantKind: ErrorKindCycle, wantOwner: wiringTestIdentityRoot, wantPeer: wiringTestIdentityRoot},
		{name: "two component cycle", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst),
			wiringDefinition(wiringTestIdentityFirst, wiringTestIdentityRoot)),
			wantKind: ErrorKindCycle, wantOwner: wiringTestIdentityRoot, wantPeer: wiringTestIdentityFirst},
		{name: "disconnected component", request: wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst),
			wiringDefinition(wiringTestIdentityFirst), wiringDefinition(wiringTestIdentitySecond)),
			wantKind: ErrorKindDisconnected, wantOwner: wiringTestIdentitySecond, wantPeer: wiringTestIdentityRoot},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			manifest, err := Derive(testCase.request)
			if !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("Derive() error = %v, want errors.Is(..., %v)", err, core.ErrPrimitiveContract)
			}
			var typed *ContractError[wiringTestIdentity]
			if !errors.As(err, &typed) {
				t.Fatalf("Derive() error = %T, want *ContractError", err)
			}
			if typed.Kind != testCase.wantKind || typed.Owner != testCase.wantOwner ||
				typed.Peer != testCase.wantPeer || typed.PrimitiveDoor != testCase.wantDoor {
				t.Fatalf("Derive() typed error = %+v, want kind=%v owner=%v peer=%v door=%v",
					typed, testCase.wantKind, testCase.wantOwner, testCase.wantPeer, testCase.wantDoor)
			}
			if manifest.Count() != 0 || manifest.Root() != wiringTestIdentityUnknown {
				t.Fatalf("Derive() manifest = count %d root %v, want zero on refusal", manifest.Count(), manifest.Root())
			}
		})
	}
}

func TestDeriveAdmitsExactExtentEdgesAndPreservesOneCanonicalSnapshot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		componentSize  int
		dependencySize int
		doorSize       int
		wantKind       ErrorKind
	}{
		{name: "one below component maximum", componentSize: ComponentMaximum - 1},
		{name: "component maximum", componentSize: ComponentMaximum},
		{name: "one above component maximum", componentSize: ComponentMaximum + 1, wantKind: ErrorKindRequest},
		{name: "one below dependency maximum", dependencySize: DependencyMaximum - 1},
		{name: "dependency maximum", dependencySize: DependencyMaximum},
		{name: "one above dependency maximum", dependencySize: DependencyMaximum + 1, wantKind: ErrorKindComponent},
		{name: "one below Primitive door maximum", doorSize: PrimitiveDoorMaximum - 1},
		{name: "Primitive door maximum", doorSize: PrimitiveDoorMaximum},
		{name: "one above Primitive door maximum", doorSize: PrimitiveDoorMaximum + 1, wantKind: ErrorKindPrimitiveDoor},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request, want := wiringBoundaryRequest(testCase.componentSize, testCase.dependencySize, testCase.doorSize)
			manifest, err := Derive(request)
			if testCase.wantKind != ErrorKindUnknown {
				var typed *ContractError[wiringTestIdentity]
				if !errors.As(err, &typed) || typed.Kind != testCase.wantKind || !errors.Is(err, core.ErrPrimitiveContract) {
					t.Fatalf("Derive() error = %v, want kind=%v and errors.Is(..., %v)",
						err, testCase.wantKind, core.ErrPrimitiveContract)
				}
				return
			}
			if err != nil {
				t.Fatalf("Derive() error = %v, want nil", err)
			}
			if err := manifest.Validate(); err != nil {
				t.Fatalf("Manifest.Validate() error = %v, want nil", err)
			}
			if got := manifest.Root(); got != wiringTestIdentityRoot {
				t.Fatalf("Manifest.Root() = %v, want %v", got, wiringTestIdentityRoot)
			}
			if got := manifest.Count(); got != len(want) {
				t.Fatalf("Manifest.Count() = %d, want %d", got, len(want))
			}
			got := slices.Collect(manifest.Components())
			if !slices.EqualFunc(got, want, equalWiringDefinition) {
				t.Fatalf("Manifest.Components() = %v, want %v", got, want)
			}
			got[0].Dependencies[0] = wiringTestIdentityThird
			if len(got[0].PrimitiveDoors) > 0 {
				got[0].PrimitiveDoors[0] = core.PackageUnknown
			}
			again := slices.Collect(manifest.Components())
			if !slices.EqualFunc(again, want, equalWiringDefinition) {
				t.Fatalf("mutating projected components changed manifest = %v, want %v", again, want)
			}
		})
	}
}

func TestDeriveAdmitsSharedDependenciesWithoutInventingTreeOwnership(t *testing.T) {
	t.Parallel()

	request := wiringRequest(wiringTestIdentityRoot,
		wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst, wiringTestIdentitySecond),
		wiringDefinition(wiringTestIdentityFirst, wiringTestIdentityThird),
		wiringDefinition(wiringTestIdentitySecond, wiringTestIdentityThird),
		wiringDefinition(wiringTestIdentityThird),
	)
	manifest, err := Derive(request)
	if err != nil {
		t.Fatalf("Derive(shared dependency DAG) error = %v, want nil", err)
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Manifest.Validate(shared dependency DAG) error = %v, want nil", err)
	}
}

func TestDeriveAdmitsCombinedMaximumRuntimeGraph(t *testing.T) {
	t.Parallel()

	request := wiringMaximumRequest()
	manifest, err := Derive(request)
	if err != nil {
		t.Fatalf("Derive(combined maximum) error = %v, want nil", err)
	}
	if manifest.Count() != ComponentMaximum {
		t.Fatalf("Derive(combined maximum) count = %d, want %d", manifest.Count(), ComponentMaximum)
	}
	for definition := range manifest.Components() {
		if len(definition.Dependencies) > DependencyMaximum || len(definition.PrimitiveDoors) != PrimitiveDoorMaximum {
			t.Fatalf("component %v extents = dependencies %d doors %d, want <= %d and %d",
				definition.Identity, len(definition.Dependencies), len(definition.PrimitiveDoors),
				DependencyMaximum, PrimitiveDoorMaximum)
		}
	}
}

func TestErrorKindExhaustsUint8AndNilContractErrorPreservesIdentity(t *testing.T) {
	t.Parallel()

	for raw := range uint16(math.MaxUint8) + 1 {
		kind := ErrorKind(raw)
		wantValid := kind > ErrorKindUnknown && kind < errorKindLimit
		gotErr := kind.Validate()
		if (gotErr == nil) != wantValid {
			t.Fatalf("ErrorKind(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if !wantValid && !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("ErrorKind(%d).Validate() error = %v, want errors.Is(..., %v)", raw, gotErr, core.ErrPrimitiveContract)
		}
		encoded, marshalErr := kind.MarshalJSON()
		if !wantValid {
			if !errors.Is(marshalErr, core.ErrJSONContract) || encoded != nil {
				t.Fatalf("ErrorKind(%d).MarshalJSON() = (%v, %v), want nil and errors.Is(..., %v)",
					raw, encoded, marshalErr, core.ErrJSONContract)
			}
			continue
		}
		if marshalErr != nil {
			t.Fatalf("ErrorKind(%d).MarshalJSON() error = %v, want nil", raw, marshalErr)
		}
		var decoded ErrorKind
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != kind {
			t.Fatalf("ErrorKind(%d) JSON round trip = (%v, %v), want (%v, nil)", raw, decoded, err, kind)
		}
	}
	var nilContract *ContractError[wiringTestIdentity]
	if !errors.Is(nilContract, core.ErrPrimitiveContract) {
		t.Fatalf("errors.Is(nil *ContractError, %v) = false, want true", core.ErrPrimitiveContract)
	}
}

func BenchmarkDeriveCombinedMaximumRuntimeGraph(b *testing.B) {
	request := wiringMaximumRequest()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		manifest, err := Derive(request)
		if err != nil || manifest.Count() != ComponentMaximum {
			b.Fatalf("Derive(combined maximum) = count %d error %v", manifest.Count(), err)
		}
	}
}

func wiringRequest(root wiringTestIdentity, definitions ...Definition[wiringTestIdentity]) Request[wiringTestIdentity] {
	components := make([]Describer[wiringTestIdentity], len(definitions))
	for index := range definitions {
		components[index] = wiringTestComponent{definition: definitions[index]}
	}
	return Request[wiringTestIdentity]{Root: root, Components: components}
}

func wiringDefinition(identity wiringTestIdentity, dependencies ...wiringTestIdentity) Definition[wiringTestIdentity] {
	return Definition[wiringTestIdentity]{Identity: identity, Dependencies: dependencies}
}

func wiringDoorDefinition(identity wiringTestIdentity, doors ...core.PackageIdentity) Definition[wiringTestIdentity] {
	return Definition[wiringTestIdentity]{Identity: identity, PrimitiveDoors: doors}
}

func wiringBoundaryRequest(componentSize, dependencySize, doorSize int) (Request[wiringTestIdentity], []Definition[wiringTestIdentity]) {
	if componentSize == 0 {
		componentSize = max(dependencySize+1, 2)
	}
	definitions := make([]Definition[wiringTestIdentity], componentSize)
	for index := range definitions {
		identity := wiringTestIdentity(index + 1)
		definitions[index] = wiringDefinition(identity)
		if index > 0 {
			definitions[index-1].Dependencies = []wiringTestIdentity{identity}
		}
	}
	if dependencySize > 0 {
		definitions[0].Dependencies = make([]wiringTestIdentity, dependencySize)
		for index := range dependencySize {
			definitions[0].Dependencies[index] = wiringTestIdentity(index + 2)
		}
	}
	if doorSize > 0 {
		definitions[0].PrimitiveDoors = wiringProductionDoors(doorSize)
	}
	return wiringRequest(wiringTestIdentityRoot, definitions...), definitions
}

func wiringProductionDoors(count int) []core.PackageIdentity {
	doors := make([]core.PackageIdentity, 0, count)
	for contract := range core.PrimitiveArchitecture().Packages() {
		if contract.Kind != core.PackageKindProduction {
			continue
		}
		doors = append(doors, contract.Identity)
		if len(doors) == count {
			return doors
		}
	}
	return append(doors, make([]core.PackageIdentity, count-len(doors))...)
}

func wiringMaximumRequest() Request[wiringTestIdentity] {
	definitions := make([]Definition[wiringTestIdentity], ComponentMaximum)
	doors := wiringProductionDoors(PrimitiveDoorMaximum)
	for index := range definitions {
		identity := wiringTestIdentity(index + 1)
		lastDependency := min(index+1+DependencyMaximum, len(definitions))
		dependencies := make([]wiringTestIdentity, 0, lastDependency-index-1)
		for dependencyIndex := index + 1; dependencyIndex < lastDependency; dependencyIndex++ {
			dependencies = append(dependencies, wiringTestIdentity(dependencyIndex+1))
		}
		definitions[index] = Definition[wiringTestIdentity]{
			Identity: identity, Dependencies: dependencies, PrimitiveDoors: doors,
		}
	}
	return wiringRequest(wiringTestIdentityRoot, definitions...)
}

func equalWiringDefinition(left, right Definition[wiringTestIdentity]) bool {
	return left.Identity == right.Identity &&
		slices.Equal(left.Dependencies, right.Dependencies) &&
		slices.Equal(left.PrimitiveDoors, right.PrimitiveDoors)
}

var _ core.Validatable = wiringTestIdentityUnknown
