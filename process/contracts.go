package process

import (
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const unknownEnumLabel = "unknown"

// Argument is one exact non-NUL argv value. Its zero value is invalid even
// though a constructed empty argument is valid.
type Argument struct {
	value string
	set   bool
}

// NewArgument constructs one exact argv value.
func NewArgument(value string) (Argument, error) {
	argument := Argument{value: value, set: true}
	if err := argument.Validate(); err != nil {
		return Argument{}, err
	}
	return argument, nil
}

// Validate rejects an unset or NUL-containing argument.
func (a Argument) Validate() error {
	if !a.set {
		return contractError("argument is unset")
	}
	if strings.IndexByte(a.value, 0) >= 0 {
		return contractError("argument contains NUL")
	}
	return nil
}

func (a Argument) text() string {
	return a.value
}

// EnvironmentName is one exact environment variable name.
type EnvironmentName struct {
	value string
}

// NewEnvironmentName constructs a nonempty name without '=' or NUL.
func NewEnvironmentName(value string) (EnvironmentName, error) {
	name := EnvironmentName{value: value}
	if err := name.Validate(); err != nil {
		return EnvironmentName{}, err
	}
	return name, nil
}

// Validate rejects an unset or ambiguous environment variable name.
func (n EnvironmentName) Validate() error {
	if n.value == "" {
		return contractError("environment name is unset")
	}
	if strings.ContainsAny(n.value, "=\x00") {
		return contractError("environment name contains a reserved byte")
	}
	return nil
}

func (n EnvironmentName) text() string {
	return n.value
}

// EnvironmentValue is one exact non-NUL environment value. Its zero value is
// invalid even though a constructed empty value is valid.
type EnvironmentValue struct {
	value string
	set   bool
}

// NewEnvironmentValue constructs one exact environment value.
func NewEnvironmentValue(value string) (EnvironmentValue, error) {
	environmentValue := EnvironmentValue{value: value, set: true}
	if err := environmentValue.Validate(); err != nil {
		return EnvironmentValue{}, err
	}
	return environmentValue, nil
}

// Validate rejects an unset or NUL-containing environment value.
func (v EnvironmentValue) Validate() error {
	if !v.set {
		return contractError("environment value is unset")
	}
	if strings.IndexByte(v.value, 0) >= 0 {
		return contractError("environment value contains NUL")
	}
	return nil
}

func (v EnvironmentValue) text() string {
	return v.value
}

// EnvironmentVariable is one exact name/value pair.
type EnvironmentVariable struct {
	Name  EnvironmentName
	Value EnvironmentValue
}

// Validate rejects an unset name or value.
func (v EnvironmentVariable) Validate() error {
	if err := v.Name.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if err := v.Value.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return nil
}

// EnvironmentMode selects ambient inheritance or an exact environment.
type EnvironmentMode uint8

const (
	// EnvironmentModeUnknown is outside the admitted domain.
	EnvironmentModeUnknown EnvironmentMode = iota
	// EnvironmentModeInherit leaves environment inheritance to os/exec.
	EnvironmentModeInherit
	// EnvironmentModeExact supplies exactly Environment.Variables.
	EnvironmentModeExact
)

// Validate rejects values outside the closed mode domain.
func (m EnvironmentMode) Validate() error {
	if !m.IsValid() {
		return contractError("environment mode is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether m is admitted.
func (m EnvironmentMode) IsValid() bool {
	return m == EnvironmentModeInherit || m == EnvironmentModeExact
}

// OffWireEnum declares that EnvironmentMode is not a wire encoding.
func (EnvironmentMode) OffWireEnum() {}

// String returns the compiler-owned label for m.
func (m EnvironmentMode) String() string {
	switch m {
	case EnvironmentModeInherit:
		return "inherit"
	case EnvironmentModeExact:
		return "exact"
	default:
		return unknownEnumLabel
	}
}

// Environment describes ambient inheritance or one exact ordered environment.
type Environment struct {
	Variables []EnvironmentVariable
	Mode      EnvironmentMode
}

// Validate rejects contradictory modes, unset variables, and duplicate names.
func (e Environment) Validate() error {
	if err := e.Mode.Validate(); err != nil {
		return err
	}
	if e.Mode == EnvironmentModeInherit {
		if len(e.Variables) != 0 {
			return contractError("inherited environment contains exact variables")
		}
		return nil
	}
	for index := range e.Variables {
		if err := e.validateVariable(index); err != nil {
			return err
		}
	}
	return nil
}

func (e Environment) validateVariable(index int) error {
	current := e.Variables[index]
	if err := current.Validate(); err != nil {
		return err
	}
	for prior := 0; prior < index; prior++ {
		if e.Variables[prior].Name == current.Name {
			return contractError("exact environment contains a duplicate name")
		}
	}
	return nil
}

func (e Environment) project() []string {
	if e.Mode == EnvironmentModeInherit {
		return nil
	}
	projected := make([]string, len(e.Variables))
	for index, variable := range e.Variables {
		projected[index] = variable.Name.text() + "=" + variable.Value.text()
	}
	return projected
}

// Streams are the caller-owned byte streams for one direct child.
type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Validate rejects a literal nil stream.
func (s Streams) Validate() error {
	if s.Stdin == nil {
		return contractError("stdin is nil")
	}
	if s.Stdout == nil {
		return contractError("stdout is nil")
	}
	if s.Stderr == nil {
		return contractError("stderr is nil")
	}
	return nil
}

// Request contains every input required to execute one direct child.
type Request struct {
	Streams          Streams
	Command          core.AbsolutePath
	WorkingDirectory core.AbsolutePath
	Arguments        []Argument
	Environment      Environment
	OutputLimit      core.ByteCount
	WaitDelay        temporal.Duration
}

// Validate closes every owned request contract before execution.
func (r Request) Validate() error {
	if err := validateRequestHead(r); err != nil {
		return err
	}
	if err := r.Streams.Validate(); err != nil {
		return err
	}
	if err := r.OutputLimit.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if err := r.WaitDelay.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if r.WaitDelay.IsZero() {
		return contractError("wait delay is zero")
	}
	return nil
}

func validateRequestHead(r Request) error {
	if err := r.Command.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	for _, argument := range r.Arguments {
		if err := argument.Validate(); err != nil {
			return err
		}
	}
	if err := r.Environment.Validate(); err != nil {
		return err
	}
	if err := r.WorkingDirectory.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return nil
}

func (r Request) projectArguments() []string {
	arguments := make([]string, len(r.Arguments))
	for index, argument := range r.Arguments {
		arguments[index] = argument.text()
	}
	return arguments
}

func contractError(reason string) error {
	return errors.Join(core.ErrProcessContract, errors.New(reason))
}
