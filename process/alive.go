package process

import "github.com/deliri/primitive/v2026/core"

// Liveness is the closed set of things one interrogated process identity can
// turn out to be.
//
// Closed rather than a boolean because the zero value must be rejectable: an
// identity nobody interrogated is not the same fact as a process observed to
// be gone.
type Liveness uint8

const (
	// LivenessUnknown is outside the admitted domain.
	LivenessUnknown Liveness = iota
	// LivenessAlive means a process with this identity exists right now,
	// including one the caller lacks permission to signal.
	LivenessAlive
	// LivenessGone means no process carries this identity right now.
	LivenessGone
	livenessLimit
)

func livenessDiagnostics() [livenessLimit]string {
	return [...]string{
		LivenessAlive: "alive",
		LivenessGone:  "gone",
	}
}

// Validate rejects values outside the closed liveness domain.
func (l Liveness) Validate() error {
	if !l.IsValid() {
		return contractError("liveness is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether l is admitted.
func (l Liveness) IsValid() bool {
	diagnostics := livenessDiagnostics()
	return l > LivenessUnknown && l < livenessLimit && diagnostics[l] != ""
}

// OffWireEnum declares that Liveness is not a wire encoding.
func (Liveness) OffWireEnum() {}

// String returns the compiler-owned label for l.
func (l Liveness) String() string {
	diagnostics := livenessDiagnostics()
	if l < livenessLimit && diagnostics[l] != "" {
		return diagnostics[l]
	}
	return core.UnknownEnumDiagnostic
}

// Alive reports whether a process with the given identity exists right now.
//
// The answer is a moment's observation of a name, not a capability over the
// process: the identity may be reused the instant after a gone answer, and a
// supervisor deciding "my counterpart crashed" owns that judgement and its
// window. Alive exists because the question is asked with a kernel probe
// that no consumer may perform itself.
func Alive(identity ProcessIdentity) (Liveness, error) {
	if err := identity.Validate(); err != nil {
		return LivenessUnknown, err
	}
	return observedLiveness(identity)
}
