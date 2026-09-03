package runprotocol

const (
	// GoJSONOutputArgument asks cmd/go for its machine-readable projection.
	GoJSONOutputArgument = "-json"
	// GoRaceText is the shared Go race-instrumentation identity.
	GoRaceText = "race"
	// GoTestFileSuffix identifies Go's compiler-owned test source convention.
	GoTestFileSuffix = "_test.go"
	// GoTestText identifies cmd/go's test subcommand.
	GoTestText = "test"
)

// Package-owned compiler strings keep runprotocol's closed enum spellings and
// diagnostics in one visible contract. They are not wire
// vocabulary for unrelated Primitive packages.
const (
	admittedTargetInvalidDiagnostic           = "run protocol admitted target is invalid"
	cachePostureInvalidDiagnostic             = "run protocol cache posture is invalid"
	canonicalCancelledText                    = "cancelled"
	canonicalExperimentText                   = "experiment"
	canonicalSelectionText                    = "selection"
	canonicalSourceText                       = "source"
	canonicalTimedOutText                     = "timed_out"
	canonicalToolText                         = "tool"
	dispositionKindInvalidDiagnostic          = "run protocol disposition kind is invalid"
	externalSuiteText                         = "external_suite"
	infrastructureStageInvalidDiagnostic      = "run protocol infrastructure stage is invalid"
	machineChangeFieldInvalidDiagnostic       = "run protocol machine change field is invalid"
	machineInstallModeInvalidDiagnostic       = "run protocol machine install mode is invalid"
	machineMaintenancePolicyInvalidDiagnostic = "run protocol machine maintenance policy is invalid"
	machineProvisioningModelInvalidDiagnostic = "run protocol machine provisioning model is invalid"
	machineToolchainKindInvalidDiagnostic     = "run protocol machine toolchain kind is invalid"
	observationKindInvalidDiagnostic          = "run protocol observation kind is invalid"
	observationPayloadDisagreementDiagnostic  = "run protocol observation payload, kind, and probe role disagree"
	outcomeInvalidDiagnostic                  = "run protocol outcome is invalid"
	probeKindInvalidDiagnostic                = "run protocol probe kind is invalid"
	probeRoleInvalidDiagnostic                = "run protocol probe role is invalid"
	probeTargetEscapedDiagnostic              = "run protocol probe target escaped its domain"
	probeTargetKindInvalidDiagnostic          = "run protocol probe target kind is invalid"
	refusalReasonInvalidDiagnostic            = "run protocol refusal reason is invalid"
	terminalStateInvalidDiagnostic            = "run protocol terminal state is invalid"
)
