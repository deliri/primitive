package core

const (
	comparisonUnknownDiagnostic = "unknown"
	comparisonLessDiagnostic    = "less"
	comparisonEqualDiagnostic   = "equal"
	comparisonGreaterDiagnostic = "greater"
)

// Comparison is the shared closed result of ordering two values.
type Comparison uint8

const (
	// ComparisonUnknown is the invalid zero comparison.
	ComparisonUnknown Comparison = iota
	// ComparisonLess reports that the left value is less than the right.
	ComparisonLess
	// ComparisonEqual reports that both values are equal.
	ComparisonEqual
	// ComparisonGreater reports that the left value is greater than the right.
	ComparisonGreater
	comparisonLimit
)

type comparisonDiagnostic struct {
	text       string
	comparison Comparison
}

// comparisonDiagnostics is unkeyed and compiler-sized. Adding a member without
// a row fails to compile; moving a member without its row fails validation.
func comparisonDiagnostics() [comparisonLimit]comparisonDiagnostic {
	return [...]comparisonDiagnostic{
		{comparison: ComparisonUnknown, text: comparisonUnknownDiagnostic},
		{comparison: ComparisonLess, text: comparisonLessDiagnostic},
		{comparison: ComparisonEqual, text: comparisonEqualDiagnostic},
		{comparison: ComparisonGreater, text: comparisonGreaterDiagnostic},
	}
}

// Validate rejects values outside the closed comparison domain. It refuses a
// zero row too, so a member that is added to the table with no diagnostic can
// never be admitted by the range check alone.
func (c Comparison) Validate() error {
	if c <= ComparisonUnknown || c >= comparisonLimit {
		return ErrPrimitiveContract
	}
	diagnostic := comparisonDiagnostics()[c]
	if diagnostic.comparison != c || diagnostic.text == "" {
		return ErrPrimitiveContract
	}
	return nil
}

// IsValid reports whether c belongs to the closed comparison domain.
func (c Comparison) IsValid() bool {
	return c.Validate() == nil
}

// OffWireEnum declares Comparison as an off-wire enum. The declaration binds
// Comparison to OffWireEnum in validation_witnesses, so the marker is
// compiler-checked rather than a bare method name matched by convention.
func (Comparison) OffWireEnum() {}

// String returns a diagnostic projection of c.
func (c Comparison) String() string {
	if err := c.Validate(); err != nil {
		return comparisonUnknownDiagnostic
	}
	return comparisonDiagnostics()[c].text
}
