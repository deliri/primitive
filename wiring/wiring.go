// Package wiring derives and validates one bounded runtime component graph.
//
// Products own their component vocabulary and construct their real command
// objects. Wiring owns only the product-neutral proof that those objects form
// one complete, connected, acyclic graph before the command reports ready.
package wiring

import (
	"iter"
	"reflect"
	"slices"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// ComponentMaximum bounds one command's complete runtime wiring snapshot.
	ComponentMaximum = 255
	// DependencyMaximum bounds the direct callees declared by one component.
	DependencyMaximum = 64
	// PrimitiveDoorMaximum bounds the Primitive packages one component may use.
	PrimitiveDoorMaximum = 16
)

// Identity is the contract a product-owned closed component enum satisfies.
type Identity interface {
	comparable
	core.Validatable
}

// Definition is one component identity and its direct runtime dependencies.
type Definition[I Identity] struct {
	Dependencies   []I
	PrimitiveDoors []core.PackageIdentity
	Identity       I
}

// Validate rejects an invalid identity, an excessive dependency set, an
// invalid dependency, or one dependency repeated by the same owner.
func (d Definition[I]) Validate() error {
	if err := d.Identity.Validate(); err != nil {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindComponent, owner: d.Identity, cause: err})
	}
	if len(d.Dependencies) > DependencyMaximum {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindComponent, owner: d.Identity})
	}
	if err := validatePrimitiveDoors(d.Identity, d.PrimitiveDoors); err != nil {
		return err
	}
	seen := make(map[I]struct{}, len(d.Dependencies))
	for _, dependency := range d.Dependencies {
		if err := dependency.Validate(); err != nil {
			return wiringContractError(contractErrorRequest[I]{kind: ErrorKindDependency, owner: d.Identity, peer: dependency, cause: err})
		}
		if _, exists := seen[dependency]; exists {
			return wiringContractError(contractErrorRequest[I]{kind: ErrorKindDuplicateDependency, owner: d.Identity, peer: dependency})
		}
		seen[dependency] = struct{}{}
	}
	return nil
}

// Describer exposes the wiring definition of one real runtime component.
type Describer[I Identity] interface {
	WiringDefinition() Definition[I]
}

// Request supplies the product-owned command root and the actual components
// constructed for that command.
type Request[I Identity] struct {
	Components []Describer[I]
	Root       I
}

// Validate rejects malformed request extent and nil runtime components before
// any component method is invoked.
func (r Request[I]) Validate() error {
	if err := r.Root.Validate(); err != nil {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindRequest, owner: r.Root, cause: err})
	}
	if len(r.Components) == 0 || len(r.Components) > ComponentMaximum {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindRequest, owner: r.Root})
	}
	for _, component := range r.Components {
		if describerIsNil(component) {
			return wiringContractError(contractErrorRequest[I]{kind: ErrorKindComponent})
		}
	}
	return nil
}

// Manifest is one validated immutable snapshot of a command's runtime wiring.
type Manifest[I Identity] struct {
	definitions []Definition[I]
	root        I
}

// Root returns the command component from which every wired component is
// reachable.
func (m Manifest[I]) Root() I { return m.root }

// Count returns the fixed admitted component cardinality.
func (m Manifest[I]) Count() uint16 { return uint16(len(m.definitions)) }

// Components yields defensive copies in caller construction order.
func (m Manifest[I]) Components() iter.Seq[Definition[I]] {
	return func(yield func(Definition[I]) bool) {
		for _, definition := range m.definitions {
			if !yield(cloneDefinition(definition)) {
				return
			}
		}
	}
}

// Validate re-proves the complete immutable graph at a package boundary.
func (m Manifest[I]) Validate() error {
	if err := m.root.Validate(); err != nil {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindRequest, owner: m.root, cause: err})
	}
	if len(m.definitions) == 0 || len(m.definitions) > ComponentMaximum {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindRequest, owner: m.root})
	}
	return validateDefinitions(m.root, m.definitions)
}

// Derive snapshots the definitions exposed by the actual runtime components,
// then rejects every incomplete or contradictory graph.
func Derive[I Identity](request Request[I]) (Manifest[I], error) {
	if err := request.Validate(); err != nil {
		return Manifest[I]{}, err
	}
	definitions := make([]Definition[I], len(request.Components))
	for index, component := range request.Components {
		definitions[index] = cloneDefinition(component.WiringDefinition())
	}
	manifest := Manifest[I]{root: request.Root, definitions: definitions}
	if err := manifest.Validate(); err != nil {
		return Manifest[I]{}, err
	}
	return manifest, nil
}

func validateDefinitions[I Identity](root I, definitions []Definition[I]) error {
	byIdentity, err := indexDefinitions(definitions)
	if err != nil {
		return err
	}
	if _, exists := byIdentity[root]; !exists {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindMissingRoot, owner: root})
	}
	if err := validateDependenciesExist(definitions, byIdentity); err != nil {
		return err
	}
	cycleOwner, cyclePeer, cycle := wiringCycle(definitions, byIdentity)
	if cycle {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindCycle, owner: cycleOwner, peer: cyclePeer})
	}
	return validateConnected(root, definitions, byIdentity)
}

func indexDefinitions[I Identity](definitions []Definition[I]) (map[I]int, error) {
	byIdentity := make(map[I]int, len(definitions))
	for index, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return nil, err
		}
		if _, exists := byIdentity[definition.Identity]; exists {
			return nil, wiringContractError(contractErrorRequest[I]{kind: ErrorKindDuplicateComponent, owner: definition.Identity})
		}
		byIdentity[definition.Identity] = index
	}
	return byIdentity, nil
}

func validateDependenciesExist[I Identity](definitions []Definition[I], byIdentity map[I]int) error {
	for _, definition := range definitions {
		for _, dependency := range definition.Dependencies {
			if _, exists := byIdentity[dependency]; !exists {
				return wiringContractError(contractErrorRequest[I]{kind: ErrorKindMissingDependency, owner: definition.Identity, peer: dependency})
			}
		}
	}
	return nil
}

func wiringCycle[I Identity](definitions []Definition[I], byIdentity map[I]int) (I, I, bool) {
	indegree := wiringIndegrees(definitions, byIdentity)
	if acyclicVisitCount(definitions, byIdentity, indegree) == len(definitions) {
		return zeroIdentity[I](), zeroIdentity[I](), false
	}
	owner, peer, found := remainingCycleEdge(definitions, byIdentity, indegree)
	if found {
		return owner, peer, true
	}
	return zeroIdentity[I](), zeroIdentity[I](), true
}

func wiringIndegrees[I Identity](definitions []Definition[I], byIdentity map[I]int) []uint16 {
	indegree := make([]uint16, len(definitions))
	for _, definition := range definitions {
		for _, dependency := range definition.Dependencies {
			indegree[byIdentity[dependency]]++
		}
	}
	return indegree
}

func acyclicVisitCount[I Identity](
	definitions []Definition[I],
	byIdentity map[I]int,
	indegree []uint16,
) int {
	queue := make([]int, 0, len(definitions))
	for index, count := range indegree {
		if count == 0 {
			queue = append(queue, index)
		}
	}
	visited := 0
	for len(queue) > 0 {
		index := queue[0]
		queue = queue[1:]
		visited++
		for _, dependency := range definitions[index].Dependencies {
			dependencyIndex := byIdentity[dependency]
			indegree[dependencyIndex]--
			if indegree[dependencyIndex] == 0 {
				queue = append(queue, dependencyIndex)
			}
		}
	}
	return visited
}

func remainingCycleEdge[I Identity](
	definitions []Definition[I],
	byIdentity map[I]int,
	indegree []uint16,
) (I, I, bool) {
	for _, definition := range definitions {
		if indegree[byIdentity[definition.Identity]] == 0 {
			continue
		}
		for _, dependency := range definition.Dependencies {
			if indegree[byIdentity[dependency]] > 0 {
				return definition.Identity, dependency, true
			}
		}
	}
	return zeroIdentity[I](), zeroIdentity[I](), false
}

func validateConnected[I Identity](root I, definitions []Definition[I], byIdentity map[I]int) error {
	seen := make(map[I]struct{}, len(definitions))
	queue := []I{root}
	for len(queue) > 0 {
		identity := queue[0]
		queue = queue[1:]
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		queue = append(queue, definitions[byIdentity[identity]].Dependencies...)
	}
	for _, definition := range definitions {
		if _, exists := seen[definition.Identity]; !exists {
			return wiringContractError(contractErrorRequest[I]{kind: ErrorKindDisconnected, owner: definition.Identity, peer: root})
		}
	}
	return nil
}

func cloneDefinition[I Identity](definition Definition[I]) Definition[I] {
	return Definition[I]{
		Identity:       definition.Identity,
		Dependencies:   slices.Clone(definition.Dependencies),
		PrimitiveDoors: slices.Clone(definition.PrimitiveDoors),
	}
}

func validatePrimitiveDoors[I Identity](owner I, doors []core.PackageIdentity) error {
	if len(doors) > PrimitiveDoorMaximum {
		return wiringContractError(contractErrorRequest[I]{kind: ErrorKindPrimitiveDoor, owner: owner})
	}
	catalog := core.PrimitiveArchitecture()
	seen := make(map[core.PackageIdentity]struct{}, len(doors))
	for _, door := range doors {
		contract, exists := catalog.Lookup(door)
		if !exists || contract.Kind != core.PackageKindProduction {
			return wiringContractError(contractErrorRequest[I]{kind: ErrorKindPrimitiveDoor, owner: owner, primitiveDoor: door})
		}
		if _, exists := seen[door]; exists {
			return wiringContractError(contractErrorRequest[I]{kind: ErrorKindDuplicatePrimitiveDoor, owner: owner, primitiveDoor: door})
		}
		seen[door] = struct{}{}
	}
	return nil
}

func describerIsNil[I Identity](describer Describer[I]) bool {
	if describer == nil {
		return true
	}
	value := reflect.ValueOf(describer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func zeroIdentity[I Identity]() I {
	var zero I
	return zero
}
