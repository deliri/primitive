package process

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = Argument{}
	_ core.Validatable = EnvironmentName{}
	_ core.Validatable = EnvironmentValue{}
	_ core.Validatable = EnvironmentPresenceUnknown
	_ core.Validatable = EnvironmentLookup{}
	_ core.Validatable = EnvironmentVariable{}
	_ core.Validatable = EnvironmentModeUnknown
	_ core.Validatable = Environment{}
	_ core.Validatable = Streams{}
	_ core.Validatable = Request{}
	_ core.Validatable = StreamUnknown
	_ core.Validatable = FailureKindUnknown
	_ core.Validatable = failure{}
	_ core.Validatable = streamFailure{}
	_ core.Validatable = outputLimitExceeded{}
	_ core.Validatable = ExitCode{}
	_ core.Validatable = Result{}
	_ core.Validatable = ResultObservation{}
	_ core.Validatable = (*TruncatingWriter)(nil)
	_ core.Validatable = Containment{}
	_ core.Validatable = IsolationUnknown
	_ core.Validatable = CancelSignalUnknown
	_ core.Validatable = LivenessUnknown
	_ core.Validatable = ProcessIdentity(0)
	_ core.Validatable = SignalNumber(0)
)

var (
	_ core.OffWireEnum = EnvironmentModeUnknown
	_ core.OffWireEnum = EnvironmentPresenceUnknown
	_ core.OffWireEnum = StreamUnknown
	_ core.OffWireEnum = FailureKindUnknown
	_ core.OffWireEnum = IsolationUnknown
	_ core.OffWireEnum = CancelSignalUnknown
	_ core.OffWireEnum = LivenessUnknown
)
