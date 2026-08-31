package runnercontrol

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
)

// SubjectIsolationEngine is the closed host boundary Primitive may compile.
// The initial reviewed machine baseline uses systemd transient services.
type SubjectIsolationEngine uint8

const (
	SubjectIsolationEngineUnknown SubjectIsolationEngine = iota
	SubjectIsolationSystemd
	subjectIsolationEngineLimit
)

func (e SubjectIsolationEngine) Validate() error {
	if e <= SubjectIsolationEngineUnknown || e >= subjectIsolationEngineLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (e SubjectIsolationEngine) String() string {
	switch e {
	case SubjectIsolationSystemd:
		return "systemd-transient-service"
	default:
		return ""
	}
}

func (e SubjectIsolationEngine) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(e.String())
}

func (e *SubjectIsolationEngine) UnmarshalJSON(data []byte) error {
	if e == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	if value != SubjectIsolationSystemd.String() {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	*e = SubjectIsolationSystemd
	return nil
}

// SubjectExecution binds the host-owned isolation implementation to the
// centrally reviewed policy identity. Each host authority path is explicit so
// a subject capability cannot omit one protection behind an untyped list.
type SubjectExecution struct {
	Engine               SubjectIsolationEngine      `json:"engine"`
	Supervisor           core.AbsolutePath           `json:"supervisor"`
	Controller           core.AbsolutePath           `json:"controller"`
	PolicyIdentity       core.SHA256Digest           `json:"policy_identity"`
	ProcessUser          projectstandards.Identifier `json:"process_user"`
	SourceRoot           core.AbsolutePath           `json:"source_root"`
	EgressPolicyIdentity core.SHA256Digest           `json:"egress_policy_identity"`
	NetworkNamespace     *core.AbsolutePath          `json:"network_namespace,omitempty"`
	NetworkController    *core.AbsolutePath          `json:"network_controller,omitempty"`
	ControlSocket        core.AbsolutePath           `json:"control_socket"`
	HostCredentials      core.AbsolutePath           `json:"host_credentials"`
	SigningState         core.AbsolutePath           `json:"signing_state"`
	ExecutableState      core.AbsolutePath           `json:"executable_state"`
}

func (s SubjectExecution) Validate(processPlan process.Plan, workspace WritableWorkspace) error {
	if err := errors.Join(s.Engine.Validate(), s.Supervisor.Validate(), s.Controller.Validate(), s.PolicyIdentity.Validate(), s.ProcessUser.Validate(), s.SourceRoot.Validate(), s.EgressPolicyIdentity.Validate(), s.ControlSocket.Validate(), s.HostCredentials.Validate(), s.SigningState.Validate(), s.ExecutableState.Validate(), processPlan.Validate(), workspace.Validate()); err != nil {
		return err
	}
	if s.NetworkNamespace != nil {
		if err := s.NetworkNamespace.Validate(); err != nil {
			return err
		}
	}
	if s.NetworkController != nil {
		if err := s.NetworkController.Validate(); err != nil {
			return err
		}
	}
	if (s.NetworkNamespace == nil) != (s.NetworkController == nil) {
		return errors.Join(core.ErrPrimitiveContract, errors.New("subject network namespace and Primitive controller must be configured together"))
	}
	if s.Engine != SubjectIsolationSystemd {
		return core.ErrPrimitiveContract
	}
	if err := s.validateWorkspaceBoundary(processPlan, workspace); err != nil {
		return err
	}
	if s.NetworkNamespace != nil && absolutePathsOverlap(*s.NetworkNamespace, workspace.Root) {
		return errors.Join(core.ErrPrimitiveContract, errors.New("subject network namespace overlaps the writable workspace"))
	}
	return s.validateDeniedPaths()
}

func (s SubjectExecution) validateWorkspaceBoundary(processPlan process.Plan, workspace WritableWorkspace) error {
	if _, err := processPlan.WorkingDirectory.RelativeTo(s.SourceRoot); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if _, err := workspace.Root.RelativeTo(s.SourceRoot); err == nil {
		return core.ErrPrimitiveContract
	}
	if _, err := s.SourceRoot.RelativeTo(workspace.Root); err == nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (s SubjectExecution) validateDeniedPaths() error {
	denied := s.deniedPaths()
	for index := range denied {
		if s.NetworkNamespace != nil && absolutePathsOverlap(denied[index], *s.NetworkNamespace) {
			return errors.Join(core.ErrPrimitiveContract, errors.New("subject network namespace overlaps a denied host authority path"))
		}
		for previous := 0; previous < index; previous++ {
			if absolutePathsOverlap(denied[previous], denied[index]) {
				return errors.Join(core.ErrPrimitiveContract, errors.New("subject authority paths must be distinct and non-overlapping"))
			}
		}
	}
	if s.NetworkNamespace != nil && (absolutePathsOverlap(*s.NetworkNamespace, s.SourceRoot) || absolutePathsOverlap(*s.NetworkNamespace, s.ExecutableState)) {
		return errors.Join(core.ErrPrimitiveContract, errors.New("subject network namespace overlaps source or executable state"))
	}
	if s.NetworkController != nil {
		if absolutePathsOverlap(*s.NetworkController, s.SourceRoot) || absolutePathsOverlap(*s.NetworkController, *s.NetworkNamespace) {
			return errors.Join(core.ErrPrimitiveContract, errors.New("subject network controller overlaps source or namespace state"))
		}
		for _, denied := range denied {
			if absolutePathsOverlap(*s.NetworkController, denied) {
				return errors.Join(core.ErrPrimitiveContract, errors.New("subject network controller overlaps a denied host authority path"))
			}
		}
	}
	return nil
}

func absolutePathsOverlap(left, right core.AbsolutePath) bool {
	if left == right {
		return true
	}
	if _, err := left.RelativeTo(right); err == nil {
		return true
	}
	_, err := right.RelativeTo(left)
	return err == nil
}

func (s SubjectExecution) deniedPaths() [4]core.AbsolutePath {
	return [4]core.AbsolutePath{s.ControlSocket, s.HostCredentials, s.SigningState, s.ExecutableState}
}

// CompileSubjectProcess lowers one signed experiment into the exact
// systemd-run argv that creates its cgroup, namespace, filesystem, identity,
// and resource boundary. Anvil does not interpret or widen this plan.
func CompileSubjectProcess(capability ExperimentCapability) (process.Plan, error) {
	if err := capability.Validate(); err != nil {
		return process.Plan{}, err
	}
	arguments, err := compileSystemdSubjectArguments(capability)
	if err != nil {
		return process.Plan{}, err
	}
	parsed, err := process.ParseArguments(arguments)
	if err != nil {
		return process.Plan{}, err
	}
	environment, err := process.ParseExactEnvironment(nil)
	if err != nil {
		return process.Plan{}, err
	}
	target := capability.Execution.Process
	compiled := process.Plan{
		SchemaVersion: process.ExecutionPlanSchemaVersion,
		Command:       capability.Execution.Subject.Supervisor, WorkingDirectory: target.WorkingDirectory,
		Arguments: parsed, Environment: environment, OutputLimit: target.OutputLimit,
		WaitDelay:   target.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
	}
	return compiled, compiled.Validate()
}

func compileSystemdSubjectArguments(capability ExperimentCapability) ([]string, error) {
	target := capability.Execution.Process
	environment, err := target.Environment.Strings()
	if err != nil {
		return nil, err
	}
	memory, err := capability.Resources.MemoryBytes.Uint64()
	if err != nil {
		return nil, err
	}
	network, err := compileSubjectNetworkArguments(capability.Resources.Egress, capability.Execution.Subject.NetworkNamespace)
	if err != nil {
		return nil, err
	}
	runtimeMaximum, err := capability.Execution.Budget.Effective.Stdlib()
	if err != nil {
		return nil, err
	}
	stopTimeout, err := target.WaitDelay.Stdlib()
	if err != nil {
		return nil, err
	}
	arguments := []string{
		"--quiet", "--wait", "--pipe", "--collect", "--service-type=exec",
		"--unit=anvil-" + capability.Experiment.String(),
		"--uid=" + capability.Execution.Subject.ProcessUser.String(),
		"--working-directory=" + target.WorkingDirectory.String(),
		"--property=CPUQuota=" + strconv.FormatUint(uint64(capability.Resources.CPUCount)*100, 10) + "%",
		"--property=MemoryMax=" + strconv.FormatUint(memory, 10),
		"--property=MemoryAccounting=yes", "--property=CPUAccounting=yes", "--property=IOAccounting=yes", "--property=IPAccounting=yes",
		"--property=RuntimeMaxSec=" + runtimeMaximum.String(), "--property=TimeoutStopSec=" + stopTimeout.String(),
		"--property=KillMode=control-group", "--property=SendSIGKILL=yes",
		"--property=TasksMax=" + strconv.FormatUint(uint64(capability.Resources.ProcessMaximum), 10),
		"--property=LimitNOFILE=" + strconv.FormatUint(uint64(capability.Resources.FileMaximum), 10),
		"--property=NoNewPrivileges=yes", "--property=PrivateTmp=yes",
		"--property=PrivateDevices=yes", "--property=ProtectSystem=strict", "--property=ProtectHome=yes",
		"--property=ProtectControlGroups=yes", "--property=ProtectKernelTunables=yes", "--property=ProtectKernelModules=yes",
		"--property=ProtectProc=invisible", "--property=ProcSubset=pid", "--property=RestrictSUIDSGID=yes",
		"--property=RestrictRealtime=yes", "--property=LockPersonality=yes", "--property=RemoveIPC=yes",
		"--property=UMask=0077",
		"--property=ReadOnlyPaths=" + capability.Execution.Subject.SourceRoot.String(),
		"--property=ReadWritePaths=" + capability.Execution.Workspace.Root.String(),
	}
	arguments = append(arguments, network...)
	for _, denied := range capability.Execution.Subject.deniedPaths() {
		arguments = append(arguments, "--property=InaccessiblePaths="+denied.String())
	}
	for index := range environment {
		arguments = append(arguments, "--setenv="+environment[index])
	}
	arguments = append(arguments, "--", target.Command.String())
	for index := range target.Arguments {
		value, valueErr := target.Arguments[index].Value()
		if valueErr != nil {
			return nil, fmt.Errorf("project subject argument %d: %w", index, valueErr)
		}
		arguments = append(arguments, value)
	}
	return arguments, nil
}

func compileSubjectNetworkArguments(policy EgressPolicy, namespace *core.AbsolutePath) ([]string, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	switch policy.Mode {
	case EgressDenied:
		if namespace != nil {
			return nil, errors.Join(core.ErrPrimitiveContract, errors.New("deny-all subject cannot enter a prepared network namespace"))
		}
		return []string{"--property=PrivateNetwork=yes"}, nil
	case EgressPinned:
		if namespace == nil {
			return nil, errors.Join(core.ErrPrimitiveContract, errors.New("pinned subject egress has no Primitive-prepared network namespace"))
		}
		return []string{"--property=PrivateNetwork=no", "--property=NetworkNamespacePath=" + namespace.String()}, nil
	default:
		return nil, core.ErrPrimitiveContract
	}
}

var (
	_ core.Validatable = SubjectIsolationEngineUnknown
	_ json.Unmarshaler = (*SubjectIsolationEngine)(nil)
)
