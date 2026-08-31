package runnercontrol

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = ArtifactKind(0)
	_ core.ValidatedJSONMarshaler = PeerRole(0)
	_ core.ValidatedJSONMarshaler = ClaimKind(0)
	_ core.ValidatedJSONMarshaler = CleanupOutcomeKind(0)
	_ core.ValidatedJSONMarshaler = EvidenceBodyKind(0)
	_ core.ValidatedJSONMarshaler = GoBuildTag{}
	_ core.ValidatedJSONMarshaler = GoInstrumentation(0)
	_ core.ValidatedJSONMarshaler = GoModuleMode(0)
	_ core.ValidatedJSONMarshaler = ExpansionDisposition(0)
	_ core.ValidatedJSONMarshaler = GoProfileKind(0)
	_ core.ValidatedJSONMarshaler = CoverageMode(0)
	_ core.ValidatedJSONMarshaler = HeartbeatState(0)
	_ core.ValidatedJSONMarshaler = DirectiveKind(0)
	_ core.ValidatedJSONMarshaler = ObservationFormat(0)
	_ core.ValidatedJSONMarshaler = NetworkProtocol(0)
	_ core.ValidatedJSONMarshaler = EgressMode(0)
	_ core.ValidatedJSONMarshaler = RunControlState(0)
	_ core.ValidatedJSONMarshaler = SchedulingUnitKind(0)
	_ core.ValidatedJSONMarshaler = SubjectIsolationEngine(0)
)
