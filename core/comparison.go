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

// comparisonDiagnostics is indexed by Comparison and sized by comparisonLimit.
// A member added before comparisonLimit without its row fails to compile, so the
// closed domain is enforced by the build rather than by a switch default that
// would quietly project the new member as unknown.
func comparisonDiagnostics() [comparisonLimit]string {
	return [...]string{
		ComparisonLess:    comparisonLessDiagnostic,
		ComparisonEqual:   comparisonEqualDiagnostic,
		ComparisonGreater: comparisonGreaterDiagnostic,
	}
}

// Validate rejects values outside the closed comparison domain. It refuses a
// zero row too, so a member that is added to the table with no diagnostic can
// never be admitted by the range check alone.
func (c Comparison) Validate() error {
	if c <= ComparisonUnknown || c >= comparisonLimit || comparisonDiagnostics()[c] == "" {
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
	return comparisonDiagnostics()[c]
}
