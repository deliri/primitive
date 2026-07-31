package upgrade

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = SlotUnknown
	_ core.Validatable = TrialOutcomeUnknown
	_ core.Validatable = FailurePhaseUnknown
	_ core.Validatable = AttemptError{}
	_ core.Validatable = DownloadSource{}
	_ core.Validatable = StagePolicy{}
	_ core.Validatable = StageRequest{}
	_ core.Validatable = BootstrapRequest{}
	_ core.Validatable = Primary{}
	_ core.Validatable = ResolveRequest{}
	_ core.Validatable = PromoteRequest{}
	_ core.Validatable = DiscardTrialRequest{}
	_ core.Validatable = TrialTarget{}
	_ core.Validatable = TrialReport{}
	_ core.Validatable = Promotion{}
)
