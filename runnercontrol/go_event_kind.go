package runnercontrol

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
)

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
	GoEventActionAttribute
	GoEventActionArtifacts
)

func decodeGoEventAction(value string) (GoEventAction, error) {
	for action := GoEventActionStart; action <= GoEventActionArtifacts; action++ {
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
	if a < GoEventActionStart || a > GoEventActionArtifacts {
		return goJSONFailure()
	}
	return nil
}
func (a GoEventAction) IsValid() bool { return a.Validate() == nil }
func (a GoEventAction) String() string {
	if !a.IsValid() {
		return invalidEnumString()
	}
	return [...]string{"", "start", "run", "pause", "cont", "pass", "bench", "fail", "output", "skip", "build-output", "build-fail", "attr", "artifacts"}[a]
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
		return goEventErrorText
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
	if f < GoEventFieldAction || f > GoEventFieldPath {
		return goJSONFailure()
	}
	return nil
}

func (k GoEventOutputKind) IsValid() bool { return k.Validate() == nil }

func (a *GoEventAction) UnmarshalJSON(data []byte) error {
	if a == nil {
		return goJSONFailure()
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(goJSONFailure(), err)
	}
	candidate, err := decodeGoEventAction(text)
	if err != nil {
		return err
	}
	*a = candidate
	return nil
}

func (k *GoEventOutputKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return goJSONFailure()
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(goJSONFailure(), err)
	}
	candidate, err := decodeGoOutputKind(text)
	if err != nil {
		return err
	}
	*k = candidate
	return nil
}

func (f GoEventField) IsValid() bool { return f.Validate() == nil }

func decodeGoEventField(text string) (GoEventField, error) {
	for field := GoEventFieldAction; field <= GoEventFieldPath; field++ {
		if field.String() == text {
			return field, nil
		}
	}
	return GoEventFieldUnknown, goJSONFailure()
}

// OffWireEnum declares the streaming observer's field discriminator. Field
// fragments are delivered through Go callbacks and have no JSON representation.
func (GoEventField) OffWireEnum() {}

func (f GoEventField) String() string {
	if !f.IsValid() {
		return invalidEnumString()
	}
	return [...]string{"", "Action", "Package", "Test", "Output", "OutputType", "Time", "FailedBuild", "Elapsed", "ImportPath", "Key", "Value", "Path"}[f]
}

var _ core.OffWireEnum = GoEventFieldUnknown

const goEventErrorText = "error"
