package manual

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	viewHelpToken       = "help"
	viewManualToken     = "manual"
	selectionIndexToken = "index"
	selectionTopicToken = "topic"
)

func (v View) IsValid() bool { return v.Validate() == nil }
func (View) OffWireEnum()    {}
func (v View) String() string {
	if v >= viewLimit {
		return core.UnknownEnumDiagnostic
	}
	return viewLabels()[v]
}
func viewLabels() [viewLimit]string {
	return [...]string{core.UnknownEnumDiagnostic, viewHelpToken, viewManualToken}
}
func (v View) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(v.String())
}
func (v *View) UnmarshalJSON(data []byte) error {
	if v == nil {
		return errors.Join(core.ErrJSONContract, contractError("manual view receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrManualContract, err)
	}
	parsed, err := parseView(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*v = parsed
	return nil
}
func parseView(value string) (View, error) {
	switch value {
	case viewHelpToken:
		return ViewHelp, nil
	case viewManualToken:
		return ViewManual, nil
	}
	return ViewUnknown, contractError("manual view token is unsupported")
}

// Validate rejects modes outside the closed domain.
func (m SelectionMode) Validate() error {
	if m <= SelectionModeUnknown || m >= selectionModeLimit {
		return contractError("manual selection mode is outside the closed domain")
	}
	return nil
}
func (m SelectionMode) IsValid() bool { return m.Validate() == nil }
func (SelectionMode) OffWireEnum()    {}
func (m SelectionMode) String() string {
	if m >= selectionModeLimit {
		return core.UnknownEnumDiagnostic
	}
	return selectionModeLabels()[m]
}
func selectionModeLabels() [selectionModeLimit]string {
	return [...]string{core.UnknownEnumDiagnostic, selectionIndexToken, selectionTopicToken}
}
func (m SelectionMode) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(m.String())
}
func (m *SelectionMode) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, contractError("manual selection receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrManualContract, err)
	}
	parsed, err := parseSelectionMode(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*m = parsed
	return nil
}
func parseSelectionMode(value string) (SelectionMode, error) {
	switch value {
	case selectionIndexToken:
		return SelectionModeIndex, nil
	case selectionTopicToken:
		return SelectionModeTopic, nil
	}
	return SelectionModeUnknown, contractError("manual selection token is unsupported")
}
