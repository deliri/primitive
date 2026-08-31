package runnercontrol

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
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

func (p NetworkProtocol) String() string {
	switch p {
	case NetworkTCP:
		return "tcp"
	case NetworkUDP:
		return "udp"
	default:
		return ""
	}
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
	case "tcp":
		*p = NetworkTCP
	case "udp":
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

func (m EgressMode) String() string {
	switch m {
	case EgressDenied:
		return "denied"
	case EgressPinned:
		return "pinned"
	default:
		return ""
	}
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
	case "denied":
		*m = EgressDenied
	case "pinned":
		*m = EgressPinned
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type EgressRule struct {
	Service     projectstandards.Identifier `json:"service"`
	Endpoint    core.HTTPEndpoint           `json:"endpoint"`
	Protocol    NetworkProtocol             `json:"protocol"`
	Port        uint16                      `json:"port"`
	Certificate core.SHA256Digest           `json:"certificate"`
}

func (r EgressRule) Validate() error {
	if r.Port == 0 {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Service.Validate(), r.Endpoint.Validate(), r.Protocol.Validate(), r.Certificate.Validate())
}

type EgressPolicy struct {
	Mode      EgressMode        `json:"mode"`
	Rules     []EgressRule      `json:"rules"`
	DNSPolicy core.SHA256Digest `json:"dns_policy"`
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
	leftKey := left.Service.String() + "\x00" + left.Protocol.String() + "\x00" + fmt.Sprintf("%05d", left.Port) + "\x00" + left.Endpoint.String()
	rightKey := right.Service.String() + "\x00" + right.Protocol.String() + "\x00" + fmt.Sprintf("%05d", right.Port) + "\x00" + right.Endpoint.String()
	return leftKey < rightKey
}

func egressRuleDuplicatesEarlier(rules []EgressRule, index int) bool {
	for previous := 0; previous < index; previous++ {
		if rules[previous].Service == rules[index].Service && rules[previous].Port == rules[index].Port && rules[previous].Protocol == rules[index].Protocol {
			return true
		}
	}
	return false
}

type IsolationPolicy struct {
	Identity             core.SHA256Digest           `json:"identity"`
	ProcessUser          projectstandards.Identifier `json:"process_user"`
	WorkspaceRoot        core.SHA256Digest           `json:"workspace_root"`
	CPUCount             uint16                      `json:"cpu_count"`
	MemoryBytes          core.ByteCount              `json:"memory_bytes"`
	ProcessMaximum       uint16                      `json:"process_maximum"`
	FileMaximum          uint32                      `json:"file_maximum"`
	Egress               EgressPolicy                `json:"egress"`
	ControlSocketDenied  bool                        `json:"control_socket_denied"`
	CloudMetadataDenied  bool                        `json:"cloud_metadata_denied"`
	HostCredentialDenied bool                        `json:"host_credential_denied"`
}

func (p IsolationPolicy) Validate() error {
	if err := errors.Join(p.Identity.Validate(), p.ProcessUser.Validate(), p.WorkspaceRoot.Validate(), p.MemoryBytes.Validate(), p.Egress.Validate()); err != nil {
		return err
	}
	if p.CPUCount == 0 || p.ProcessMaximum == 0 || p.FileMaximum == 0 || !p.ControlSocketDenied || !p.CloudMetadataDenied || !p.HostCredentialDenied {
		return core.ErrPrimitiveContract
	}
	return nil
}

type SecretMemoryPolicy struct {
	ConfiguredSwapBytes    core.ByteLength `json:"configured_swap_bytes"`
	ActiveSwapBytes        core.ByteLength `json:"active_swap_bytes"`
	SuspendDisabled        bool            `json:"suspend_disabled"`
	HibernateDisabled      bool            `json:"hibernate_disabled"`
	CoreDumpsDisabled      bool            `json:"core_dumps_disabled"`
	MemoryLockingAvailable bool            `json:"memory_locking_available"`
	PtraceDenied           bool            `json:"ptrace_denied"`
	DedicatedIdentity      bool            `json:"dedicated_identity"`
}

func (p SecretMemoryPolicy) Validate() error {
	if err := errors.Join(p.ConfiguredSwapBytes.Validate(), p.ActiveSwapBytes.Validate()); err != nil {
		return err
	}
	if p.ConfiguredSwapBytes.Uint64() != 0 || p.ActiveSwapBytes.Uint64() != 0 || !p.SuspendDisabled || !p.HibernateDisabled || !p.CoreDumpsDisabled || !p.MemoryLockingAvailable || !p.PtraceDenied || !p.DedicatedIdentity {
		return core.ErrPrimitiveContract
	}
	return nil
}

type MachineSessionBudget struct {
	Startup          temporal.Duration `json:"startup"`
	Readiness        temporal.Duration `json:"readiness"`
	PlanCriticalPath temporal.Duration `json:"plan_critical_path"`
	Handoff          temporal.Duration `json:"handoff"`
	Cleanup          temporal.Duration `json:"cleanup"`
	IdleGrace        temporal.Duration `json:"idle_grace"`
	ShutdownReserve  temporal.Duration `json:"shutdown_reserve"`
}

func (b MachineSessionBudget) Validate() error {
	parts := []temporal.Duration{b.Startup, b.Readiness, b.PlanCriticalPath, b.Handoff, b.Cleanup, b.IdleGrace, b.ShutdownReserve}
	var total int64
	for _, part := range parts {
		if err := part.Validate(); err != nil || part.IsZero() {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
		if total > math.MaxInt64-part.Nanoseconds() {
			return core.ErrPrimitiveContract
		}
		total += part.Nanoseconds()
	}
	maximum, err := temporal.DurationFromHours(MachineSessionMaximumHours)
	if err != nil || total > maximum.Nanoseconds() {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}
func (b MachineSessionBudget) Total() (temporal.Duration, error) {
	if err := b.Validate(); err != nil {
		return temporal.Duration{}, err
	}
	parts := []temporal.Duration{b.Startup, b.Readiness, b.PlanCriticalPath, b.Handoff, b.Cleanup, b.IdleGrace, b.ShutdownReserve}
	total, err := temporal.DurationFromNanoseconds(0)
	if err != nil {
		return temporal.Duration{}, err
	}
	for _, part := range parts {
		total, err = total.Add(part)
		if err != nil {
			return temporal.Duration{}, err
		}
	}
	return total, nil
}

type BatchSessionBudget struct {
	Elapsed          temporal.Duration    `json:"elapsed"`
	Remaining        MachineSessionBudget `json:"remaining"`
	AbsoluteDeadline temporal.Instant     `json:"absolute_deadline"`
}

func (b BatchSessionBudget) Validate() error {
	if err := errors.Join(b.Elapsed.Validate(), b.Remaining.Validate(), b.AbsoluteDeadline.Validate()); err != nil {
		return err
	}
	remaining, err := b.Remaining.Total()
	if err != nil {
		return err
	}
	total, err := b.Elapsed.Add(remaining)
	maximum, maxErr := temporal.DurationFromHours(MachineSessionMaximumHours)
	if err != nil || maxErr != nil || total.Nanoseconds() > maximum.Nanoseconds() {
		return errors.Join(core.ErrPrimitiveContract, err, maxErr)
	}
	return nil
}

type ResourceRequirement struct {
	CPUCount       uint16         `json:"cpu_count"`
	MemoryBytes    core.ByteCount `json:"memory_bytes"`
	ProcessMaximum uint16         `json:"process_maximum"`
	FileMaximum    uint32         `json:"file_maximum"`
	Exclusive      bool           `json:"exclusive"`
	Egress         EgressPolicy   `json:"egress"`
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
	Experiments []projectstandards.ExperimentID `json:"experiments"`
	Required    ResourceRequirement             `json:"required"`
	WaveWidth   uint16                          `json:"wave_width"`
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

func experimentDuplicatesEarlier(experiments []projectstandards.ExperimentID, index int) bool {
	for previous := 0; previous < index; previous++ {
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
	_ core.Validatable = IsolationPolicy{}
	_ core.Validatable = SecretMemoryPolicy{}
	_ core.Validatable = MachineSessionBudget{}
	_ core.Validatable = BatchSessionBudget{}
	_ core.Validatable = ResourceRequirement{}
	_ core.Validatable = ResourceWave{}
)
