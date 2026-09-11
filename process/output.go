package process

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// OutputMode makes the caller's output contract explicit. Streaming imposes
// no total byte quota; bounded execution refuses the first byte over Maximum.
type OutputMode uint8

const (
	OutputModeUnknown OutputMode = iota
	OutputModeStreaming
	OutputModeBounded
)

func (m OutputMode) Validate() error {
	switch m {
	case OutputModeStreaming, OutputModeBounded:
		return nil
	default:
		return core.ErrProcessContract
	}
}

// OutputPolicy is shared by direct requests and serialized execution plans.
// A streaming policy cannot carry an ignored bound. Zero never selects a mode.
type OutputPolicy struct {
	Mode    OutputMode     `json:"mode"`
	Maximum core.ByteCount `json:"maximum,omitzero"`
}

func (p OutputPolicy) Validate() error {
	switch p.Mode {
	case OutputModeStreaming:
		if p.Maximum != (core.ByteCount{}) {
			return core.ErrProcessContract
		}
		return nil
	case OutputModeBounded:
		return validateOutputLimit(p.Maximum)
	default:
		return core.ErrProcessContract
	}
}

func (m OutputMode) IsValid() bool { return m == OutputModeStreaming || m == OutputModeBounded }

func (m OutputMode) String() string {
	if !m.IsValid() {
		return "unknown"
	}
	return [...]string{OutputModeStreaming: "streaming", OutputModeBounded: "bounded"}[m]
}

func (m OutputMode) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(m.String())
}

func (m *OutputMode) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, core.ErrProcessContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	for _, candidate := range [...]OutputMode{OutputModeStreaming, OutputModeBounded} {
		if value == candidate.String() {
			*m = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrProcessContract)
}
