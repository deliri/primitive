package runnercontrol

import "github.com/deliri/primitive/v2026/core"

type GoEventAction uint8

const (
	GoEventActionUnknown GoEventAction = iota
	GoEventActionStart
	GoEventActionRun
	GoEventActionPause
	GoEventActionContinue
	GoEventActionPass
	GoEventActionBenchmark
	GoEventActionFail
	GoEventActionOutput
	GoEventActionSkip
	GoEventActionBuildOutput
	GoEventActionBuildFail
)

func decodeGoEventAction(value string) (GoEventAction, error) {
	for action := GoEventActionStart; action <= GoEventActionBuildFail; action++ {
		if value == action.String() {
			return action, nil
		}
	}
	return GoEventActionUnknown, goJSONFailure()
}

type GoEventOutputKind uint8

const (
	GoEventOutputOrdinary GoEventOutputKind = iota
	GoEventOutputFrame
	GoEventOutputError
	GoEventOutputErrorContinue
)

func decodeGoOutputKind(value string) (GoEventOutputKind, error) {
	for kind := GoEventOutputOrdinary; kind <= GoEventOutputErrorContinue; kind++ {
		if value == kind.String() {
			return kind, nil
		}
	}
	return GoEventOutputOrdinary, goJSONFailure()
}

func (a GoEventAction) Validate() error {
	if a < GoEventActionStart || a > GoEventActionBuildFail {
		return goJSONFailure()
	}
	return nil
}
func (a GoEventAction) String() string {
	switch a {
	case GoEventActionStart:
		return "start"
	case GoEventActionRun:
		return "run"
	case GoEventActionPause:
		return "pause"
	case GoEventActionContinue:
		return "cont"
	case GoEventActionPass:
		return "pass"
	case GoEventActionBenchmark:
		return "bench"
	case GoEventActionFail:
		return "fail"
	case GoEventActionOutput:
		return "output"
	case GoEventActionSkip:
		return "skip"
	case GoEventActionBuildOutput:
		return "build-output"
	case GoEventActionBuildFail:
		return "build-fail"
	default:
		return invalidEnumString()
	}
}
func (a GoEventAction) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(a.String())
}
func (k GoEventOutputKind) Validate() error {
	if k > GoEventOutputErrorContinue {
		return goJSONFailure()
	}
	return nil
}
func (k GoEventOutputKind) String() string {
	switch k {
	case GoEventOutputOrdinary:
		return ""
	case GoEventOutputFrame:
		return "frame"
	case GoEventOutputError:
		return "error"
	case GoEventOutputErrorContinue:
		return "error-continue"
	default:
		return invalidEnumString()
	}
}
func (k GoEventOutputKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(k.String())
}
func (f GoEventField) Validate() error {
	if f < GoEventFieldAction || f > GoEventFieldImportPath {
		return goJSONFailure()
	}
	return nil
}
