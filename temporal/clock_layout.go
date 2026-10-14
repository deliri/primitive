package temporal

import "github.com/deliri/primitive/v2026/core"

// ClockLayout is the closed set of native clock-only display policies. Calendar
// layouts use TimeLayout; choosing both is a contract refusal.
type ClockLayout uint8

const (
	ClockLayoutUnknown ClockLayout = iota
	// ClockHourMinuteSecond renders Go's 24-hour HH:MM:SS TimeOnly format.
	ClockHourMinuteSecond
)

// Validate refuses unset and future clock policies.
func (l ClockLayout) Validate() error {
	if l != ClockHourMinuteSecond {
		return contractError("clock layout is invalid")
	}
	return nil
}

// IsValid reports membership in the closed native clock layout domain.
func (l ClockLayout) IsValid() bool { return l.Validate() == nil }

// OffWireEnum marks clock display policy as a compiler-owned enum.
func (ClockLayout) OffWireEnum() {}

// String returns a diagnostic; the native layout string remains execution data.
func (l ClockLayout) String() string {
	if l == ClockHourMinuteSecond {
		return "hour-minute-second"
	}
	return core.UnknownEnumDiagnostic
}

var _ core.OffWireEnum = ClockLayoutUnknown
