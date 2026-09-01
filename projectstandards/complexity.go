package projectstandards

import (
	"errors"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

const (
	ComplexityClaimMaximum      = 16
	ComplexityAssumptionMaximum = 8
	ComplexityCaptureMaximum    = 16
	ComplexitySampleMinimum     = 3
	ComplexitySampleMaximum     = 16
	ComplexityPolynomialMaximum = 16
)

type ComplexityGrowth uint8

const (
	ComplexityGrowthUnknown ComplexityGrowth = iota
	ComplexityConstant
	ComplexityLogarithmic
	ComplexityLinear
	ComplexityLinearithmic
	ComplexityPolynomial
	ComplexityExponential
	complexityGrowthLimit
)

func complexityGrowthLabels() []string {
	return []string{"", "constant", "logarithmic", "linear", "linearithmic", "polynomial", "exponential"}
}
func (g ComplexityGrowth) Validate() error {
	return validateEnum(uint8(g), complexityGrowthLabels(), "project standards complexity growth is invalid")
}
func (g ComplexityGrowth) IsValid() bool  { return g.Validate() == nil }
func (g ComplexityGrowth) String() string { return enumString(uint8(g), complexityGrowthLabels()) }
func (g ComplexityGrowth) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(g), complexityGrowthLabels(), "project standards complexity growth is invalid")
}
func (g *ComplexityGrowth) UnmarshalJSON(data []byte) error {
	if g == nil {
		return jsonError(errors.New("nil project standards complexity growth receiver"))
	}
	value, err := unmarshalEnum(data, complexityGrowthLabels(), "project standards complexity growth is invalid")
	if err == nil {
		*g = ComplexityGrowth(value)
	}
	return err
}

type ComplexityCase uint8

const (
	ComplexityCaseUnknown ComplexityCase = iota
	ComplexityBestCase
	ComplexityExpectedCase
	ComplexityWorstCase
	ComplexityAmortizedCase
	complexityCaseLimit
)

func complexityCaseLabels() []string { return []string{"", "best", "expected", "worst", "amortized"} }
func (c ComplexityCase) Validate() error {
	return validateEnum(uint8(c), complexityCaseLabels(), "project standards complexity case is invalid")
}
func (c ComplexityCase) IsValid() bool  { return c.Validate() == nil }
func (c ComplexityCase) String() string { return enumString(uint8(c), complexityCaseLabels()) }
func (c ComplexityCase) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(c), complexityCaseLabels(), "project standards complexity case is invalid")
}
func (c *ComplexityCase) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil project standards complexity case receiver"))
	}
	value, err := unmarshalEnum(data, complexityCaseLabels(), "project standards complexity case is invalid")
	if err == nil {
		*c = ComplexityCase(value)
	}
	return err
}

type ComplexityAssessmentStatus uint8

const (
	ComplexityAssessmentUnknown ComplexityAssessmentStatus = iota
	ComplexityAssessmentSupported
	ComplexityAssessmentContradicted
	ComplexityAssessmentInconclusive
	complexityAssessmentLimit
)

func complexityAssessmentLabels() []string {
	return []string{"", "supported", "contradicted", "inconclusive"}
}
func (s ComplexityAssessmentStatus) Validate() error {
	return validateEnum(uint8(s), complexityAssessmentLabels(), "project standards complexity assessment is invalid")
}
func (s ComplexityAssessmentStatus) IsValid() bool { return s.Validate() == nil }
func (s ComplexityAssessmentStatus) String() string {
	return enumString(uint8(s), complexityAssessmentLabels())
}
func (s ComplexityAssessmentStatus) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(s), complexityAssessmentLabels(), "project standards complexity assessment is invalid")
}
func (s *ComplexityAssessmentStatus) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil project standards complexity assessment receiver"))
	}
	value, err := unmarshalEnum(data, complexityAssessmentLabels(), "project standards complexity assessment is invalid")
	if err == nil {
		*s = ComplexityAssessmentStatus(value)
	}
	return err
}

type ComplexityBound struct {
	Growth ComplexityGrowth `json:"growth"`
	Case   ComplexityCase   `json:"case"`
	Degree uint8            `json:"degree,omitempty"`
}

func (b ComplexityBound) Validate() error {
	if err := contractJoin(b.Growth.Validate(), b.Case.Validate()); err != nil {
		return err
	}
	if b.Growth == ComplexityPolynomial {
		if b.Degree < 2 || b.Degree > ComplexityPolynomialMaximum {
			return contractError(errors.New("project standards complexity polynomial degree is invalid"))
		}
		return nil
	}
	if b.Degree != 0 {
		return conflictError(errors.New("project standards non-polynomial complexity carries a degree"))
	}
	return nil
}

func (b ComplexityBound) Notation() string {
	if b.Validate() != nil {
		return ""
	}
	if b.Growth == ComplexityConstant {
		return "O(1)"
	}
	if b.Growth == ComplexityLogarithmic {
		return "O(log n)"
	}
	if b.Growth == ComplexityLinear {
		return "O(n)"
	}
	if b.Growth == ComplexityLinearithmic {
		return "O(n log n)"
	}
	if b.Growth == ComplexityPolynomial {
		return "O(n^" + strconv.FormatUint(uint64(b.Degree), 10) + ")"
	}
	if b.Growth == ComplexityExponential {
		return "O(2^n)"
	}
	return ""
}

type ComplexityInput struct {
	Name        Name     `json:"name"`
	Unit        Name     `json:"unit"`
	Meaning     Text     `json:"meaning"`
	SampleSizes []uint64 `json:"sample_sizes"`
	Minimum     uint64   `json:"minimum"`
	Maximum     uint64   `json:"maximum"`
}

func (i ComplexityInput) Validate() error {
	if err := contractJoin(i.Name.Validate(), i.Unit.Validate(), i.Meaning.Validate()); err != nil {
		return err
	}
	if !i.validRange() {
		return contractError(errors.New("project standards complexity input range is invalid"))
	}
	for index := range i.SampleSizes {
		if i.SampleSizes[index] < i.Minimum || i.SampleSizes[index] > i.Maximum {
			return conflictError(errors.New("project standards complexity sample is outside its input range"))
		}
		if index > 0 && i.SampleSizes[index] <= i.SampleSizes[index-1] {
			return conflictError(errors.New("project standards complexity sample sizes are not strictly increasing"))
		}
	}
	return nil
}

func (i ComplexityInput) validRange() bool {
	return i.Minimum > 0 && i.Maximum >= i.Minimum && len(i.SampleSizes) >= ComplexitySampleMinimum && len(i.SampleSizes) <= ComplexitySampleMaximum
}

type ComplexityAssumption struct {
	Title  Name `json:"title"`
	Detail Text `json:"detail"`
}

func (a ComplexityAssumption) Validate() error {
	return contractJoin(a.Title.Validate(), a.Detail.Validate())
}

type CodeReference struct {
	Symbol *Name      `json:"symbol,omitempty"`
	Path   SourcePath `json:"path"`
}

func (r CodeReference) Validate() error {
	if err := r.Path.Validate(); err != nil {
		return err
	}
	if r.Symbol != nil {
		return r.Symbol.Validate()
	}
	return nil
}

type ComplexityClaim struct {
	Operation      CodeReference          `json:"operation"`
	ID             Identifier             `json:"id"`
	SurfaceID      Identifier             `json:"evidence_surface_id"`
	Assumptions    []ComplexityAssumption `json:"assumptions"`
	Input          ComplexityInput        `json:"input"`
	Time           ComplexityBound        `json:"time"`
	AuxiliarySpace ComplexityBound        `json:"auxiliary_space"`
}

func (c ComplexityClaim) Validate() error {
	if c.Operation.Symbol == nil || len(c.Assumptions) == 0 || len(c.Assumptions) > ComplexityAssumptionMaximum {
		return contractError(errors.New("project standards complexity claim is incomplete"))
	}
	if err := contractJoin(c.ID.Validate(), c.Operation.Validate(), c.Input.Validate(), c.Time.Validate(), c.AuxiliarySpace.Validate(), c.SurfaceID.Validate()); err != nil {
		return err
	}
	for index := range c.Assumptions {
		if err := c.Assumptions[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

type ComplexitySample struct {
	InputSize        uint64 `json:"input_size"`
	Iterations       uint64 `json:"iterations"`
	NanosecondsPerOp uint64 `json:"nanoseconds_per_op"`
	BytesPerOp       uint64 `json:"bytes_per_op"`
	AllocationsPerOp uint64 `json:"allocations_per_op"`
}

func (s ComplexitySample) Validate() error {
	if s.InputSize == 0 || s.Iterations == 0 || s.NanosecondsPerOp == 0 {
		return contractError(errors.New("project standards complexity sample is invalid"))
	}
	return nil
}

type ComplexityCapture struct {
	ClaimID                Identifier                 `json:"claim_id"`
	Profile                ProfileIdentity            `json:"profile"`
	Samples                []ComplexitySample         `json:"samples"`
	BudgetSeconds          uint32                     `json:"budget_seconds"`
	EnvironmentFingerprint core.SHA256Digest          `json:"environment_fingerprint"`
	Assessment             ComplexityAssessmentStatus `json:"assessment"`
}

func (c ComplexityCapture) Validate() error {
	if c.BudgetSeconds == 0 || len(c.Samples) < ComplexitySampleMinimum || len(c.Samples) > ComplexitySampleMaximum {
		return contractError(errors.New("project standards complexity capture bounds are invalid"))
	}
	if err := contractJoin(c.ClaimID.Validate(), c.EnvironmentFingerprint.Validate(), c.Profile.Validate(), c.Assessment.Validate()); err != nil {
		return err
	}
	for index := range c.Samples {
		if err := c.Samples[index].Validate(); err != nil {
			return err
		}
		if index > 0 && c.Samples[index].InputSize <= c.Samples[index-1].InputSize {
			return conflictError(errors.New("project standards complexity captures are not strictly increasing"))
		}
	}
	return nil
}
