package runnercontrol

import (
	json "encoding/json/v2"
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/standard"
)

const NetworkRuleMaximum = 32

type NetworkProtocol uint8

const (
	NetworkProtocolUnknown NetworkProtocol = iota
	NetworkTCP
	NetworkUDP
	networkProtocolLimit
)

func (p NetworkProtocol) Validate() error {
	if p <= NetworkProtocolUnknown || p >= networkProtocolLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p NetworkProtocol) IsValid() bool { return p.Validate() == nil }

func (p NetworkProtocol) String() string {
	if !p.IsValid() {
		return invalidEnumString()
	}
	return []string{"", networkTCPText, networkUDPText}[p]
}

func (p NetworkProtocol) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(p.String())
}

func (p *NetworkProtocol) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	switch value {
	case networkTCPText:
		*p = NetworkTCP
	case networkUDPText:
		*p = NetworkUDP
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type EgressMode uint8

const (
	EgressModeUnknown EgressMode = iota
	EgressDenied
	EgressPinned
	egressModeLimit
)

func (m EgressMode) Validate() error {
	if m <= EgressModeUnknown || m >= egressModeLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (m EgressMode) IsValid() bool { return m.Validate() == nil }

func (m EgressMode) String() string {
	if !m.IsValid() {
		return invalidEnumString()
	}
	return []string{"", isolationDeniedText, isolationPinnedText}[m]
}

func (m EgressMode) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(m.String())
}

func (m *EgressMode) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	switch value {
	case isolationDeniedText:
		*m = EgressDenied
	case isolationPinnedText:
		*m = EgressPinned
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type EgressRule struct {
	Service     standard.Identifier `json:"service"`
	Endpoint    core.HTTPEndpoint   `json:"endpoint"`
	Protocol    NetworkProtocol     `json:"protocol"`
	Port        uint16              `json:"port"`
	Certificate core.SHA256Digest   `json:"certificate"`
}

func (r EgressRule) Validate() error {
	if r.Port == 0 {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Service.Validate(), r.Endpoint.Validate(), r.Protocol.Validate(), r.Certificate.Validate())
}

type EgressPolicy struct {
	Rules     []EgressRule      `json:"rules"`
	DNSPolicy core.SHA256Digest `json:"dns_policy"`
	Mode      EgressMode        `json:"mode"`
}

func (p EgressPolicy) Validate() error {
	if err := errors.Join(p.Mode.Validate(), p.DNSPolicy.Validate()); err != nil {
		return err
	}
	if len(p.Rules) > NetworkRuleMaximum {
		return core.ErrPrimitiveContract
	}
	if p.Mode == EgressDenied && len(p.Rules) != 0 {
		return core.ErrPrimitiveContract
	}
	if p.Mode == EgressPinned && len(p.Rules) == 0 {
		return core.ErrPrimitiveContract
	}
	return p.validateRules()
}

// Digest seals the complete ordered network authority for binding a prepared
// host namespace to the experiment capability that may enter it.
func (p EgressPolicy) Digest() (core.SHA256Digest, error) {
	if err := p.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func (p EgressPolicy) validateRules() error {
	for index := range p.Rules {
		if err := p.Rules[index].Validate(); err != nil {
			return err
		}
		if egressRuleDuplicatesEarlier(p.Rules, index) {
			return core.ErrPrimitiveContract
		}
		if index > 0 && !egressRuleLess(p.Rules[index-1], p.Rules[index]) {
			return errors.Join(core.ErrPrimitiveContract, errors.New("egress rules are not in canonical order"))
		}
	}
	return nil
}

func egressRuleLess(left, right EgressRule) bool {
	leftKey := left.Service.String() + "\x00" + left.Protocol.String() + "\x00" + fmt.Sprintf(sequenceWidthFormat, left.Port) + "\x00" + left.Endpoint.String()
	rightKey := right.Service.String() + "\x00" + right.Protocol.String() + "\x00" + fmt.Sprintf(sequenceWidthFormat, right.Port) + "\x00" + right.Endpoint.String()
	return leftKey < rightKey
}

func egressRuleDuplicatesEarlier(rules []EgressRule, index int) bool {
	for previous := range index {
		if rules[previous].Service == rules[index].Service && rules[previous].Port == rules[index].Port && rules[previous].Protocol == rules[index].Protocol {
			return true
		}
	}
	return false
}

type ResourceRequirement struct {
	Egress         EgressPolicy   `json:"egress"`
	MemoryBytes    core.ByteCount `json:"memory_bytes"`
	FileMaximum    uint32         `json:"file_maximum"`
	CPUCount       uint16         `json:"cpu_count"`
	ProcessMaximum uint16         `json:"process_maximum"`
	Exclusive      bool           `json:"exclusive"`
}

func (r ResourceRequirement) Validate() error {
	if err := errors.Join(r.MemoryBytes.Validate(), r.Egress.Validate()); err != nil {
		return err
	}
	memory, err := r.MemoryBytes.Uint64()
	if err != nil || memory == 0 || r.CPUCount == 0 || r.ProcessMaximum == 0 || r.FileMaximum == 0 {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type ResourceWave struct {
	Experiments []standard.ExperimentID `json:"experiments"`
	Required    ResourceRequirement     `json:"required"`
	WaveWidth   uint16                  `json:"wave_width"`
}

func (w ResourceWave) Validate() error {
	if err := w.Required.Validate(); err != nil {
		return err
	}
	if err := w.validateShape(); err != nil {
		return err
	}
	for index := range w.Experiments {
		if err := w.Experiments[index].Validate(); err != nil {
			return err
		}
		if experimentDuplicatesEarlier(w.Experiments, index) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (w ResourceWave) validateShape() error {
	if len(w.Experiments) == 0 || len(w.Experiments) > SchedulingMemberMaximum {
		return core.ErrPrimitiveContract
	}
	if w.WaveWidth == 0 || int(w.WaveWidth) != len(w.Experiments) {
		return core.ErrPrimitiveContract
	}
	if w.Required.Exclusive && len(w.Experiments) != 1 {
		return core.ErrPrimitiveContract
	}
	return nil
}

func experimentDuplicatesEarlier(experiments []standard.ExperimentID, index int) bool {
	for previous := range index {
		if experiments[previous] == experiments[index] {
			return true
		}
	}
	return false
}

var (
	_ core.Validatable = NetworkProtocolUnknown
	_ json.Unmarshaler = (*NetworkProtocol)(nil)
	_ core.Validatable = EgressModeUnknown
	_ json.Unmarshaler = (*EgressMode)(nil)
	_ core.Validatable = EgressRule{}
	_ core.Validatable = EgressPolicy{}
	_ core.Validatable = ResourceRequirement{}
	_ core.Validatable = ResourceWave{}
)
