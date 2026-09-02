package process

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	ExecutionPlanSchemaVersion    uint16 = 1
	ExecutionPlanJSONMaximumBytes        = 1 << 20
)

// Plan is the stream-free, exact execution capability carried across a
// trusted control boundary. The authority compiles it from closed policy;
// the runner supplies only its owned streams before execution.
type Plan struct {
	Command          core.AbsolutePath
	WorkingDirectory core.AbsolutePath
	Arguments        []Argument
	Environment      Environment
	OutputLimit      core.ByteCount
	WaitDelay        temporal.Duration
	SchemaVersion    uint16
	Containment      Containment
}

func (p Plan) Validate() error {
	if p.SchemaVersion != ExecutionPlanSchemaVersion {
		return contractError("execution plan schema version is unsupported")
	}
	request := Request{
		Command: p.Command, WorkingDirectory: p.WorkingDirectory,
		Arguments: p.Arguments, Environment: p.Environment,
		OutputLimit: p.OutputLimit, WaitDelay: p.WaitDelay,
		Containment: p.Containment,
	}
	if err := validateRequestHead(request); err != nil {
		return err
	}
	if p.Environment.Mode != EnvironmentModeExact {
		return contractError("execution plan environment is not exact")
	}
	if err := validateOutputLimit(p.OutputLimit); err != nil {
		return err
	}
	if err := p.WaitDelay.Validate(); err != nil || p.WaitDelay.IsZero() {
		return errors.Join(core.ErrProcessContract, err)
	}
	return p.Containment.Validate()
}

// Bind supplies the runner-owned streams without changing any authority-owned
// execution fact.
func (p Plan) Bind(streams Streams) (Request, error) {
	if err := errors.Join(p.Validate(), streams.Validate()); err != nil {
		return Request{}, errors.Join(core.ErrProcessContract, err)
	}
	request := Request{
		Streams: streams, Command: p.Command, WorkingDirectory: p.WorkingDirectory,
		Arguments: append([]Argument(nil), p.Arguments...),
		Environment: Environment{
			Mode:      p.Environment.Mode,
			Variables: append([]EnvironmentVariable(nil), p.Environment.Variables...),
		},
		OutputLimit: p.OutputLimit, WaitDelay: p.WaitDelay, Containment: p.Containment,
	}
	return request, request.Validate()
}

type planWire struct {
	Command          core.AbsolutePath `json:"command"`
	WorkingDirectory core.AbsolutePath `json:"working_directory"`
	Isolation        string            `json:"isolation"`
	CancelSignal     string            `json:"cancel_signal"`
	Arguments        []string          `json:"arguments"`
	Environment      []string          `json:"environment"`
	OutputLimit      core.ByteCount    `json:"output_limit"`
	WaitDelay        temporal.Duration `json:"wait_delay"`
	SchemaVersion    uint16            `json:"schema_version"`
}

func (p Plan) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	arguments := make([]string, len(p.Arguments))
	for index := range p.Arguments {
		value, err := p.Arguments[index].Value()
		if err != nil {
			return nil, errors.Join(core.ErrJSONContract, err)
		}
		arguments[index] = value
	}
	environment, err := p.Environment.Strings()
	if err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(planWire{
		SchemaVersion: p.SchemaVersion, Command: p.Command,
		WorkingDirectory: p.WorkingDirectory, Arguments: arguments,
		Environment: environment, OutputLimit: p.OutputLimit,
		WaitDelay: p.WaitDelay, Isolation: p.Containment.Isolation.String(),
		CancelSignal: p.Containment.CancelSignal.String(),
	})
	if err != nil || len(encoded) > ExecutionPlanJSONMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrProcessContract, err)
	}
	return encoded, nil
}

func (p *Plan) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, core.ErrProcessContract)
	}
	wire, err := core.DecodeStrictJSONStructure[planWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	arguments, argumentsErr := ParseArguments(wire.Arguments)
	environment, environmentErr := ParseExactEnvironment(wire.Environment)
	containment, containmentErr := parsePlanContainment(wire.Isolation, wire.CancelSignal)
	if err := errors.Join(argumentsErr, environmentErr, containmentErr); err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrProcessContract, err)
	}
	candidate := Plan{
		SchemaVersion: wire.SchemaVersion, Command: wire.Command,
		WorkingDirectory: wire.WorkingDirectory, Arguments: arguments,
		Environment: environment, OutputLimit: wire.OutputLimit,
		WaitDelay: wire.WaitDelay, Containment: containment,
	}
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

func parsePlanContainment(isolation, signal string) (Containment, error) {
	parsedIsolation := IsolationUnknown
	for candidate := IsolationDirect; candidate < isolationLimit; candidate++ {
		if candidate.String() == isolation {
			parsedIsolation = candidate
			break
		}
	}
	parsedSignal := CancelSignalUnknown
	for candidate := CancelSignalKill; candidate < cancelSignalLimit; candidate++ {
		if candidate.String() == signal {
			parsedSignal = candidate
			break
		}
	}
	containment := Containment{Isolation: parsedIsolation, CancelSignal: parsedSignal}
	return containment, containment.Validate()
}

var (
	_ core.ValidatedJSONMarshaler = Plan{}
	_ json.Unmarshaler            = (*Plan)(nil)
)
