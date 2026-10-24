package runprotocol

import "errors"

// GoBenchmarkRowKind classifies one Go benchmark result row. Every row that
// a Go stream reports is one of these; none is dropped or invented.
type GoBenchmarkRowKind uint8

const (
	GoBenchmarkRowUnknown GoBenchmarkRowKind = iota
	// GoBenchmarkRowMeasured carries ns/op, B/op and allocs/op.
	GoBenchmarkRowMeasured
	// GoBenchmarkRowMetricsMissing lacks ns/op, B/op or allocs/op: no
	// -benchmem, or ns/op suppressed through testing.B.ReportMetric.
	GoBenchmarkRowMetricsMissing
	// GoBenchmarkRowNameUnrepresentable names more than NameMaximumBytes.
	GoBenchmarkRowNameUnrepresentable
	// GoBenchmarkRowMalformed is benchmark-shaped but its fields are invalid.
	GoBenchmarkRowMalformed
	// GoBenchmarkRowInterrupted was cut by another package's output, a
	// non-output event or the end of the stream before its newline.
	GoBenchmarkRowInterrupted
	// GoBenchmarkRowUnattributed came from a frame whose Output preceded its
	// Action or Package; native test2json encodes those first.
	GoBenchmarkRowUnattributed
	goBenchmarkRowKindLimit
)

func goBenchmarkRowKindLabels() []string {
	return []string{"", "measured", "metrics_missing", "name_unrepresentable", "malformed", "interrupted", "unattributed"}
}
func (k GoBenchmarkRowKind) Validate() error {
	return validateEnum(uint8(k), goBenchmarkRowKindLabels(), goBenchmarkRowKindInvalidDiagnostic)
}
func (k GoBenchmarkRowKind) IsValid() bool  { return k.Validate() == nil }
func (k GoBenchmarkRowKind) String() string { return enumString(uint8(k), goBenchmarkRowKindLabels()) }
func (k GoBenchmarkRowKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), goBenchmarkRowKindLabels(), goBenchmarkRowKindInvalidDiagnostic)
}
func (k *GoBenchmarkRowKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil run protocol go benchmark row kind receiver"))
	}
	value, err := unmarshalEnum(data, goBenchmarkRowKindLabels(), goBenchmarkRowKindInvalidDiagnostic)
	if err == nil {
		*k = GoBenchmarkRowKind(value)
	}
	return err
}

// GoBenchmarkRow is one Go benchmark result row. Measurement is present only
// for a measured row; every other kind carries none.
type GoBenchmarkRow struct {
	Kind        GoBenchmarkRowKind    `json:"kind"`
	Measurement *BenchmarkMeasurement `json:"measurement,omitempty"`
}

func (r GoBenchmarkRow) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if (r.Kind == GoBenchmarkRowMeasured) != (r.Measurement != nil) {
		return contractError(errors.New(goBenchmarkRowMeasurementDiagnostic))
	}
	if r.Measurement != nil {
		return r.Measurement.Validate()
	}
	return nil
}
