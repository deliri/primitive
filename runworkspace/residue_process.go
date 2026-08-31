package runworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const residueProbeOutputMaximumBytes uint64 = 64

// ResidueProbeKind is the closed machine-state dimension a reviewed host
// probe counts. Each configured dimension must occur exactly once.
type ResidueProbeKind uint8

const (
	ResidueProbeUnknown ResidueProbeKind = iota
	ResidueProbeProcesses
	ResidueProbeControlGroups
	ResidueProbeNamespaces
	ResidueProbeMounts
	ResidueProbeDescriptors
	ResidueProbeSockets
	ResidueProbeCredentialCustody
	ResidueProbeSecretCustody
	residueProbeKindLimit
)

func (k ResidueProbeKind) Validate() error {
	if k <= ResidueProbeUnknown || k >= residueProbeKindLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k ResidueProbeKind) String() string {
	switch k {
	case ResidueProbeProcesses:
		return "processes"
	case ResidueProbeControlGroups:
		return "control-groups"
	case ResidueProbeNamespaces:
		return "namespaces"
	case ResidueProbeMounts:
		return "mounts"
	case ResidueProbeDescriptors:
		return "descriptors"
	case ResidueProbeSockets:
		return "sockets"
	case ResidueProbeCredentialCustody:
		return "credential-custody"
	case ResidueProbeSecretCustody:
		return "secret-custody"
	default:
		return ""
	}
}

// ResidueProbe binds one reviewed host observation command to one residue
// dimension. The process contract owns its executable, exact argv,
// environment, working directory, output ceiling, and cancellation behavior.
type ResidueProbe struct {
	Kind ResidueProbeKind
	Plan process.Plan
}

func (p ResidueProbe) Validate() error {
	if err := errors.Join(p.Kind.Validate(), p.Plan.Validate()); err != nil {
		return err
	}
	maximum, err := p.Plan.OutputLimit.Uint64()
	if err != nil || maximum == 0 || maximum > residueProbeOutputMaximumBytes {
		return errors.Join(core.ErrPrimitiveContract, err, errors.New("residue probe output limit exceeds the fixed count grammar"))
	}
	return nil
}

// ProcessResidueSource executes the complete reviewed residue inventory. It
// accepts only one canonical probe per dimension and never interprets product
// policy or scans host state itself.
type ProcessResidueSource struct {
	probes [8]ResidueProbe
}

func NewProcessResidueSource(probes []ResidueProbe) (ProcessResidueSource, error) {
	if len(probes) != int(residueProbeKindLimit-1) {
		return ProcessResidueSource{}, errors.Join(core.ErrPrimitiveContract, errors.New("residue inventory must configure exactly eight probes"))
	}
	var source ProcessResidueSource
	for index := range probes {
		if err := probes[index].Validate(); err != nil {
			return ProcessResidueSource{}, fmt.Errorf("validate residue probe %d: %w", index, err)
		}
		want := ResidueProbeKind(index + 1)
		if probes[index].Kind != want {
			return ProcessResidueSource{}, fmt.Errorf("residue probe %d kind = %s, want canonical %s: %w", index, probes[index].Kind.String(), want.String(), core.ErrPrimitiveContract)
		}
		source.probes[index] = probes[index]
	}
	return source, nil
}

func (s ProcessResidueSource) Validate() error {
	for index := range s.probes {
		if err := s.probes[index].Validate(); err != nil {
			return fmt.Errorf("validate configured residue probe %d: %w", index, err)
		}
		if s.probes[index].Kind != ResidueProbeKind(index+1) {
			return errors.Join(core.ErrPrimitiveContract, errors.New("configured residue probes are not canonical"))
		}
	}
	return nil
}

func (s ProcessResidueSource) ObserveResidue(ctx context.Context) (Residue, error) {
	if err := s.Validate(); err != nil {
		return Residue{}, err
	}
	var residue Residue
	for index := range s.probes {
		count, err := observeResidueCount(ctx, s.probes[index])
		if err != nil {
			return Residue{}, err
		}
		assignResidueCount(&residue, s.probes[index].Kind, count)
	}
	return residue, residue.Validate()
}

func observeResidueCount(ctx context.Context, probe ResidueProbe) (uint32, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	request, err := probe.Plan.Bind(process.Streams{Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		return 0, fmt.Errorf("bind %s residue probe: %w", probe.Kind.String(), err)
	}
	result, runErr := process.Run(ctx, request)
	if validationErr := result.Validate(); validationErr != nil {
		return 0, fmt.Errorf("execute %s residue probe without a reaped result: %w", probe.Kind.String(), errors.Join(runErr, validationErr))
	}
	exit, exitErr := result.ExitCode()
	success, successErr := exit.Success()
	if runErr != nil || exitErr != nil || successErr != nil || !success {
		return 0, fmt.Errorf("execute %s residue probe successfully; stderr = %q: %w", probe.Kind.String(), stderr.String(), errors.Join(core.ErrProcessContract, runErr, exitErr, successErr))
	}
	if stderr.Len() != 0 {
		return 0, fmt.Errorf("%s residue probe wrote unexpected stderr = %q: %w", probe.Kind.String(), stderr.String(), core.ErrProcessContract)
	}
	count, parseErr := ParseResidueCount(stdout.String())
	if parseErr != nil {
		return 0, fmt.Errorf("parse %s residue probe stdout = %q: %w", probe.Kind.String(), stdout.String(), parseErr)
	}
	return count, nil
}

// ParseResidueCount admits the complete host-probe output grammar: one
// canonical base-ten uint32, optionally terminated by one newline.
func ParseResidueCount(output string) (uint32, error) {
	if len(output) == 0 || len(output) > 11 || strings.TrimSpace(output) != strings.TrimSuffix(output, "\n") || strings.Count(output, "\n") > 1 {
		return 0, errors.Join(core.ErrPrimitiveContract, errors.New("residue count must be one canonical unsigned decimal line"))
	}
	digits := strings.TrimSuffix(output, "\n")
	if digits == "" || (len(digits) > 1 && digits[0] == '0') {
		return 0, errors.Join(core.ErrPrimitiveContract, errors.New("residue count is empty or has a leading zero"))
	}
	for _, character := range digits {
		if character < '0' || character > '9' {
			return 0, errors.Join(core.ErrPrimitiveContract, errors.New("residue count contains a non-decimal character"))
		}
	}
	value, err := strconv.ParseUint(digits, 10, 32)
	if err != nil || value > math.MaxUint32 {
		return 0, errors.Join(core.ErrPrimitiveContract, err)
	}
	return uint32(value), nil
}

func assignResidueCount(residue *Residue, kind ResidueProbeKind, count uint32) {
	switch kind {
	case ResidueProbeProcesses:
		residue.Processes = count
	case ResidueProbeControlGroups:
		residue.ControlGroups = count
	case ResidueProbeNamespaces:
		residue.Namespaces = count
	case ResidueProbeMounts:
		residue.Mounts = count
	case ResidueProbeDescriptors:
		residue.Descriptors = count
	case ResidueProbeSockets:
		residue.Sockets = count
	case ResidueProbeCredentialCustody:
		residue.CredentialCustody = count
	case ResidueProbeSecretCustody:
		residue.SecretCustody = count
	}
}

var (
	_ core.Validatable = ResidueProbeUnknown
	_ core.Validatable = ResidueProbe{}
	_ core.Validatable = ProcessResidueSource{}
	_ ResidueSource    = ProcessResidueSource{}
)
