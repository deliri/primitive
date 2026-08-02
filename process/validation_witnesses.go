package process

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = Argument{}
	_ core.Validatable = EnvironmentName{}
	_ core.Validatable = EnvironmentValue{}
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
	_ core.Validatable = (*TruncatingWriter)(nil)
)

var (
	_ core.OffWireEnum = EnvironmentModeUnknown
	_ core.OffWireEnum = StreamUnknown
	_ core.OffWireEnum = FailureKindUnknown
)
