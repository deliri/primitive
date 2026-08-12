package process

import (
	"errors"
	"io"
	"os/exec"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// ArgumentCountMaximum is the largest argv cardinality admitted at the
	// execution boundary.
	ArgumentCountMaximum uint32 = 4 << 10
	// ArgumentMaximumBytes bounds one exact argv value.
	ArgumentMaximumBytes uint64 = 1 << 20
	// ArgumentProjectionMaximumBytes bounds the complete NUL-delimited argv
	// projection handed to the operating system.
	ArgumentProjectionMaximumBytes uint64 = 1 << 20

	// EnvironmentVariableCountMaximum is the largest exact environment
	// cardinality admitted at the execution boundary.
	EnvironmentVariableCountMaximum uint32 = 4 << 10
	// EnvironmentNameMaximumBytes bounds one exact variable name.
	EnvironmentNameMaximumBytes uint64 = 4 << 10
	// EnvironmentValueMaximumBytes bounds one exact variable value.
	EnvironmentValueMaximumBytes uint64 = 1 << 20
	// EnvironmentProjectionMaximumBytes bounds the complete NUL-delimited
	// name=value projection handed to the operating system.
	EnvironmentProjectionMaximumBytes uint64 = 1 << 20

	environmentVariableCountMaximumDiagnostic = "environment variable count exceeds the admitted maximum"
)

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

// ParseArguments raises an ordered argv projection into compiler-owned exact
// arguments without changing boundaries or shell-interpreting any value.
func ParseArguments(values []string) ([]Argument, error) {
	if uint64(len(values)) > uint64(ArgumentCountMaximum) {
		return nil, contractError("argument count exceeds the admitted maximum")
	}
	arguments := make([]Argument, len(values))
	var projectedBytes uint64
	for index, value := range values {
		argument, err := NewArgument(value)
		if err != nil {
			return nil, err
		}
		projectedBytes, err = extendProjection(
			projectedBytes,
			uint64(len(value)),
			ArgumentProjectionMaximumBytes,
			"argument projection exceeds the admitted maximum",
		)
		if err != nil {
			return nil, err
		}
		arguments[index] = argument
	}
	return arguments, nil
}

// Validate rejects an unset or NUL-containing argument.
func (a Argument) Validate() error {
	if !a.set {
		return contractError("argument is unset")
	}
	if strings.IndexByte(a.value, 0) >= 0 {
		return contractError("argument contains NUL")
	}
	if uint64(len(a.value)) > ArgumentMaximumBytes {
		return contractError("argument extent exceeds the admitted maximum")
	}
	return nil
}

// Value returns the exact validated argv value for a caller-owned execution
// policy that composes Primitive values over os/exec.
func (a Argument) Value() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}
	return a.value, nil
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
	if uint64(len(n.value)) > EnvironmentNameMaximumBytes {
		return contractError("environment name extent exceeds the admitted maximum")
	}
	return nil
}

func (n EnvironmentName) text() string {
	return n.value
}

// Value returns the exact admitted environment variable name.
func (n EnvironmentName) Value() (string, error) {
	if err := n.Validate(); err != nil {
		return "", err
	}
	return n.value, nil
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
	if uint64(len(v.value)) > EnvironmentValueMaximumBytes {
		return contractError("environment value extent exceeds the admitted maximum")
	}
	return nil
}

func (v EnvironmentValue) text() string {
	return v.value
}

// Value returns the exact admitted environment value. Callers distinguish an
// admitted empty value from absence through EnvironmentLookup.Presence.
func (v EnvironmentValue) Value() (string, error) {
	if err := v.Validate(); err != nil {
		return "", err
	}
	return v.value, nil
}

// EnvironmentPresence distinguishes an absent variable from one whose exact
// admitted value is empty.
type EnvironmentPresence uint8

const (
	EnvironmentPresenceUnknown EnvironmentPresence = iota
	EnvironmentPresenceAbsent
	EnvironmentPresencePresent
	environmentPresenceLimit
)

func environmentPresenceDiagnostics() [environmentPresenceLimit]string {
	return [...]string{
		EnvironmentPresenceAbsent:  "absent",
		EnvironmentPresencePresent: "present",
	}
}

func (p EnvironmentPresence) Validate() error {
	if !p.IsValid() {
		return contractError("environment presence is outside the admitted domain")
	}
	return nil
}

func (p EnvironmentPresence) IsValid() bool {
	diagnostics := environmentPresenceDiagnostics()
	return p > EnvironmentPresenceUnknown &&
		p < environmentPresenceLimit &&
		diagnostics[p] != ""
}

func (EnvironmentPresence) OffWireEnum() {}

// String returns the compiler-owned label for p.
func (p EnvironmentPresence) String() string {
	diagnostics := environmentPresenceDiagnostics()
	if p < environmentPresenceLimit && diagnostics[p] != "" {
		return diagnostics[p]
	}
	return core.UnknownEnumDiagnostic
}

// EnvironmentLookup is one typed observation of one ambient variable.
type EnvironmentLookup struct {
	Value    EnvironmentValue
	Presence EnvironmentPresence
}

func (l EnvironmentLookup) Validate() error {
	if err := l.Presence.Validate(); err != nil {
		return err
	}
	if l.Presence == EnvironmentPresenceAbsent {
		if l.Value != (EnvironmentValue{}) {
			return contractError("absent environment lookup contains a value")
		}
		return nil
	}
	return l.Value.Validate()
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
	environmentModeLimit
)

func environmentModeDiagnostics() [environmentModeLimit]string {
	return [...]string{
		EnvironmentModeInherit: "inherit",
		EnvironmentModeExact:   "exact",
	}
}

// Validate rejects values outside the closed mode domain.
func (m EnvironmentMode) Validate() error {
	if !m.IsValid() {
		return contractError("environment mode is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether m is admitted.
func (m EnvironmentMode) IsValid() bool {
	diagnostics := environmentModeDiagnostics()
	return m > EnvironmentModeUnknown &&
		m < environmentModeLimit &&
		diagnostics[m] != ""
}

// OffWireEnum declares that EnvironmentMode is not a wire encoding.
func (EnvironmentMode) OffWireEnum() {}

// String returns the compiler-owned label for m.
func (m EnvironmentMode) String() string {
	diagnostics := environmentModeDiagnostics()
	if m < environmentModeLimit && diagnostics[m] != "" {
		return diagnostics[m]
	}
	return core.UnknownEnumDiagnostic
}

// Environment describes ambient inheritance or one exact ordered environment.
type Environment struct {
	Variables []EnvironmentVariable
	Mode      EnvironmentMode
}

// ParseExactEnvironment raises ordered os/exec environment projections into
// compiler-owned name/value pairs. The first '=' separates the name, so '='
// bytes in values remain exact.
func ParseExactEnvironment(values []string) (Environment, error) {
	if uint64(len(values)) > uint64(EnvironmentVariableCountMaximum) {
		return Environment{}, contractError(environmentVariableCountMaximumDiagnostic)
	}
	variables := make([]EnvironmentVariable, len(values))
	for index, projection := range values {
		variable, err := parseEnvironmentVariable(projection)
		if err != nil {
			return Environment{}, err
		}
		variables[index] = variable
	}
	environment := Environment{Mode: EnvironmentModeExact, Variables: variables}
	if err := environment.Validate(); err != nil {
		return Environment{}, err
	}
	return environment, nil
}

// ParseEffectiveEnvironment raises the environment os/exec would actually
// supply after its OS-aware last-value-wins projection and required critical
// additions. Primitive validates every caller projection before delegating, so
// malformed or NUL-containing input cannot be silently omitted by os/exec.
func ParseEffectiveEnvironment(values []string) (Environment, error) {
	if uint64(len(values)) > uint64(EnvironmentVariableCountMaximum) {
		return Environment{}, contractError(environmentVariableCountMaximumDiagnostic)
	}
	var projectedBytes uint64
	for _, projection := range values {
		if _, err := parseEnvironmentVariable(projection); err != nil {
			return Environment{}, err
		}
		var err error
		projectedBytes, err = extendProjection(
			projectedBytes,
			uint64(len(projection)),
			EnvironmentProjectionMaximumBytes,
			"environment projection exceeds the admitted maximum",
		)
		if err != nil {
			return Environment{}, err
		}
	}
	command := exec.Cmd{Env: values}
	if values == nil {
		// A nil Cmd.Env inherits the ambient environment. The caller supplied an
		// exact empty projection, so preserve that distinction for os/exec.
		command.Env = []string{}
	}
	return ParseExactEnvironment(command.Environ())
}

func parseEnvironmentVariable(projection string) (EnvironmentVariable, error) {
	nameText, valueText, found := strings.Cut(projection, "=")
	if !found {
		return EnvironmentVariable{}, contractError("environment projection has no separator")
	}
	name, err := NewEnvironmentName(nameText)
	if err != nil {
		return EnvironmentVariable{}, err
	}
	value, err := NewEnvironmentValue(valueText)
	if err != nil {
		return EnvironmentVariable{}, err
	}
	return EnvironmentVariable{Name: name, Value: value}, nil
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
	if uint64(len(e.Variables)) > uint64(EnvironmentVariableCountMaximum) {
		return contractError(environmentVariableCountMaximumDiagnostic)
	}
	seen := make(map[EnvironmentName]struct{}, len(e.Variables))
	var projectedBytes uint64
	for _, variable := range e.Variables {
		if err := variable.Validate(); err != nil {
			return err
		}
		if _, duplicate := seen[variable.Name]; duplicate {
			return contractError("exact environment contains a duplicate name")
		}
		seen[variable.Name] = struct{}{}
		var err error
		projectedBytes, err = extendProjection(
			projectedBytes,
			uint64(len(variable.Name.text()))+1+uint64(len(variable.Value.text())),
			EnvironmentProjectionMaximumBytes,
			"environment projection exceeds the admitted maximum",
		)
		if err != nil {
			return err
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

// Strings returns the exact ordered os/exec environment projection. A nil
// result means ambient inheritance; an empty non-nil result means an exact
// empty environment.
func (e Environment) Strings() ([]string, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e.project(), nil
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
	Containment      Containment
}

// Validate closes every owned request contract before execution.
func (r Request) Validate() error {
	if err := validateRequestHead(r); err != nil {
		return err
	}
	if err := r.Streams.Validate(); err != nil {
		return err
	}
	if err := validateOutputLimit(r.OutputLimit); err != nil {
		return err
	}
	if err := r.WaitDelay.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if r.WaitDelay.IsZero() {
		return contractError("wait delay is zero")
	}
	return r.Containment.orDefault().Validate()
}

func validateOutputLimit(limit core.ByteCount) error {
	if _, err := limit.Int64(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return nil
}

func validateRequestHead(r Request) error {
	if err := r.Command.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if err := validateArguments(r.Arguments); err != nil {
		return err
	}
	if err := r.Environment.Validate(); err != nil {
		return err
	}
	if err := r.WorkingDirectory.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return nil
}

func validateArguments(arguments []Argument) error {
	if uint64(len(arguments)) > uint64(ArgumentCountMaximum) {
		return contractError("argument count exceeds the admitted maximum")
	}
	var projectedBytes uint64
	for _, argument := range arguments {
		if err := argument.Validate(); err != nil {
			return err
		}
		var err error
		projectedBytes, err = extendProjection(
			projectedBytes,
			uint64(len(argument.value)),
			ArgumentProjectionMaximumBytes,
			"argument projection exceeds the admitted maximum",
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func extendProjection(current, valueBytes, maximum uint64, diagnostic string) (uint64, error) {
	if valueBytes >= maximum || current > maximum-(valueBytes+1) {
		return 0, contractError(diagnostic)
	}
	return current + valueBytes + 1, nil
}

func (r Request) projectArguments() []string {
	arguments := make([]string, len(r.Arguments))
	for index, argument := range r.Arguments {
		arguments[index] = argument.value
	}
	return arguments
}

func contractError(reason string) error {
	return errors.Join(core.ErrProcessContract, errors.New(reason))
}
