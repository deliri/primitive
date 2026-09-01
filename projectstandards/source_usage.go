package projectstandards

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	ObservedConsumerMaximum = 32
	FunctionUsageMaximum    = 32
)

type SourceAnalysisCompleteness uint8

const (
	SourceAnalysisUnknown SourceAnalysisCompleteness = iota
	SourceAnalysisComplete
	SourceAnalysisUnresolved
	sourceAnalysisLimit
)

func sourceAnalysisLabels() []string { return []string{"", "complete", "unresolved"} }
func (c SourceAnalysisCompleteness) Validate() error {
	return validateEnum(uint8(c), sourceAnalysisLabels(), "project standards source analysis completeness is invalid")
}
func (c SourceAnalysisCompleteness) IsValid() bool { return c.Validate() == nil }
func (c SourceAnalysisCompleteness) String() string {
	return enumString(uint8(c), sourceAnalysisLabels())
}
func (c SourceAnalysisCompleteness) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(c), sourceAnalysisLabels(), "project standards source analysis completeness is invalid")
}
func (c *SourceAnalysisCompleteness) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil project standards source analysis receiver"))
	}
	value, err := unmarshalEnum(data, sourceAnalysisLabels(), "project standards source analysis completeness is invalid")
	if err == nil {
		*c = SourceAnalysisCompleteness(value)
	}
	return err
}

type FunctionReferencePosture uint8

const (
	FunctionReferenceUnknown FunctionReferencePosture = iota
	FunctionNoReferenceObserved
	FunctionTestReferenceObserved
	FunctionReferenceUnresolved
	FunctionProductionReferenceObserved
	FunctionRuntimeEntryPoint
	functionReferenceLimit
)

func functionReferenceLabels() []string {
	return []string{"", "no_reference", "test_reference", "unresolved", "production_reference", "runtime_entry_point"}
}
func (p FunctionReferencePosture) Validate() error {
	return validateEnum(uint8(p), functionReferenceLabels(), "project standards function reference posture is invalid")
}
func (p FunctionReferencePosture) IsValid() bool { return p.Validate() == nil }
func (p FunctionReferencePosture) String() string {
	return enumString(uint8(p), functionReferenceLabels())
}
func (p FunctionReferencePosture) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(p), functionReferenceLabels(), "project standards function reference posture is invalid")
}
func (p *FunctionReferencePosture) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil project standards function posture receiver"))
	}
	value, err := unmarshalEnum(data, functionReferenceLabels(), "project standards function reference posture is invalid")
	if err == nil {
		*p = FunctionReferencePosture(value)
	}
	return err
}

type FunctionUsage struct {
	Function             CodeReference            `json:"function"`
	ObservedConsumers    []SourcePath             `json:"observed_consumers"`
	DeclarationLine      uint32                   `json:"declaration_line"`
	ProductionReferences uint32                   `json:"production_references"`
	TestReferences       uint32                   `json:"test_references"`
	Exported             bool                     `json:"exported"`
	ReferencePosture     FunctionReferencePosture `json:"reference_posture"`
}

func (u FunctionUsage) Validate() error {
	if len(u.ObservedConsumers) > ObservedConsumerMaximum || u.Function.Symbol == nil || u.DeclarationLine == 0 {
		return contractError(errors.New("project standards function usage shape is invalid"))
	}
	if err := contractJoin(u.Function.Validate(), u.ReferencePosture.Validate()); err != nil {
		return err
	}
	if !functionUsageCountsMatch(u) {
		return conflictError(errors.New("project standards function usage counts disagree with posture"))
	}
	for index := range u.ObservedConsumers {
		if err := u.ObservedConsumers[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if u.ObservedConsumers[previous] == u.ObservedConsumers[index] {
				return conflictError(errors.New("project standards function consumer is duplicated"))
			}
		}
	}
	return nil
}

func functionUsageCountsMatch(u FunctionUsage) bool {
	if u.ReferencePosture == FunctionNoReferenceObserved {
		return noFunctionReference(u)
	}
	if u.ReferencePosture == FunctionTestReferenceObserved {
		return testOnlyFunctionReference(u)
	}
	if u.ReferencePosture == FunctionReferenceUnresolved {
		return u.ProductionReferences == 0
	}
	if u.ReferencePosture == FunctionProductionReferenceObserved {
		return productionFunctionReference(u)
	}
	if u.ReferencePosture == FunctionRuntimeEntryPoint {
		consumers, err := core.CheckedUint32FromInt(len(u.ObservedConsumers))
		return err == nil && u.ProductionReferences >= consumers
	}
	return false
}

func noFunctionReference(u FunctionUsage) bool {
	return u.ProductionReferences == 0 && u.TestReferences == 0 && len(u.ObservedConsumers) == 0
}

func testOnlyFunctionReference(u FunctionUsage) bool {
	return u.ProductionReferences == 0 && u.TestReferences > 0 && len(u.ObservedConsumers) == 0
}

func productionFunctionReference(u FunctionUsage) bool {
	return u.ProductionReferences > 0 && len(u.ObservedConsumers) > 0
}

type PackageSourceUsage struct {
	Generation               Identifier                 `json:"generation"`
	Package                  SourcePath                 `json:"package"`
	ReviewCandidates         []FunctionUsage            `json:"review_candidates"`
	MostReferenced           []FunctionUsage            `json:"most_referenced"`
	ObservedConsumerPackages []SourcePath               `json:"observed_consumer_packages"`
	AnalyzedAt               temporal.Instant           `json:"analyzed_at"`
	NoReferenceObserved      uint32                     `json:"no_reference_observed"`
	UnresolvedDeclarations   uint32                     `json:"unresolved_declarations"`
	TestReferencedOnly       uint32                     `json:"test_referenced_only"`
	RuntimeEntryPoints       uint32                     `json:"runtime_entry_points"`
	ProductionReferenced     uint32                     `json:"production_referenced"`
	DeclarationCount         uint32                     `json:"declaration_count"`
	Revision                 core.BuildCommit           `json:"revision"`
	Completeness             SourceAnalysisCompleteness `json:"completeness"`
}

func (u PackageSourceUsage) Validate() error {
	if len(u.ObservedConsumerPackages) > ObservedConsumerMaximum || len(u.MostReferenced) > FunctionUsageMaximum || len(u.ReviewCandidates) > FunctionUsageMaximum {
		return contractError(errors.New("project standards package source usage bounds are invalid"))
	}
	if err := contractJoin(u.Generation.Validate(), u.Revision.Validate(), u.Package.Validate(), u.Completeness.Validate(), u.AnalyzedAt.Validate()); err != nil {
		return err
	}
	if u.DeclarationCount != u.ProductionReferenced+u.RuntimeEntryPoints+u.UnresolvedDeclarations+u.TestReferencedOnly+u.NoReferenceObserved {
		return conflictError(errors.New("project standards package source usage accounting does not close"))
	}
	if (u.Completeness == SourceAnalysisComplete) != (u.UnresolvedDeclarations == 0) {
		return conflictError(errors.New("project standards source completeness disagrees with unresolved declarations"))
	}
	if err := validateSourcePaths(u.ObservedConsumerPackages); err != nil {
		return err
	}
	if err := validateFunctionUsages(u.Package, u.MostReferenced); err != nil {
		return err
	}
	return validateFunctionUsages(u.Package, u.ReviewCandidates)
}

func validateSourcePaths(values []SourcePath) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if values[previous] == values[index] {
				return conflictError(errors.New("project standards source path is duplicated"))
			}
		}
	}
	return nil
}

func validateFunctionUsages(owner SourcePath, values []FunctionUsage) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if !pathWithin(owner, values[index].Function.Path, true) {
			return conflictError(errors.New("project standards function usage is outside its package"))
		}
		for previous := range index {
			if codeReferenceEqual(values[previous].Function, values[index].Function) {
				return conflictError(errors.New("project standards function usage is duplicated"))
			}
		}
	}
	return nil
}

func pathWithin(owner, candidate SourcePath, allowRoot bool) bool {
	if allowRoot && owner == candidate {
		return true
	}
	return strings.HasPrefix(candidate.value, owner.value+"/")
}
