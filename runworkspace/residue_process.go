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

func (k ResidueProbeKind) IsValid() bool { return k.Validate() == nil }

func (k ResidueProbeKind) String() string {
	if !k.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "processes", "control-groups", "namespaces", "mounts", "descriptors", "sockets", "credential-custody", "secret-custody"}[k]
}

func (k ResidueProbeKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *ResidueProbeKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	for candidate := ResidueProbeProcesses; candidate < residueProbeKindLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

// ResidueProbe binds one reviewed host observation command to one residue
// dimension. The process contract owns its executable, exact argv,
// environment, working directory, output ceiling, and cancellation behavior.
type ResidueProbe struct {
	Plan process.Plan
	Kind ResidueProbeKind
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
	digits, err := residueCountDigits(output)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(digits, 10, 32)
	if err != nil || value > math.MaxUint32 {
		return 0, errors.Join(core.ErrPrimitiveContract, err)
	}
	return uint32(value), nil
}

func residueCountDigits(output string) (string, error) {
	if len(output) == 0 || len(output) > 11 {
		return "", errors.Join(core.ErrPrimitiveContract, errors.New("residue count must be one bounded unsigned decimal line"))
	}
	if strings.Count(output, "\n") > 1 {
		return "", errors.Join(core.ErrPrimitiveContract, errors.New("residue count contains multiple lines"))
	}
	digits := strings.TrimSuffix(output, "\n")
	if strings.Contains(digits, "\n") {
		return "", errors.Join(core.ErrPrimitiveContract, errors.New("residue count newline is not terminal"))
	}
	if !canonicalDecimal(digits) {
		return "", errors.Join(core.ErrPrimitiveContract, errors.New("residue count is not canonical unsigned decimal"))
	}
	return digits, nil
}

func canonicalDecimal(value string) bool {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func assignResidueCount(residue *Residue, kind ResidueProbeKind, count uint32) {
	if kind == ResidueProbeProcesses {
		residue.Processes = count
	}
	if kind == ResidueProbeControlGroups {
		residue.ControlGroups = count
	}
	if kind == ResidueProbeNamespaces {
		residue.Namespaces = count
	}
	if kind == ResidueProbeMounts {
		residue.Mounts = count
	}
	if kind == ResidueProbeDescriptors {
		residue.Descriptors = count
	}
	if kind == ResidueProbeSockets {
		residue.Sockets = count
	}
	if kind == ResidueProbeCredentialCustody {
		residue.CredentialCustody = count
	}
	if kind == ResidueProbeSecretCustody {
		residue.SecretCustody = count
	}
}

var (
	_ core.Validatable = ResidueProbeUnknown
	_ core.Validatable = ResidueProbe{}
	_ core.Validatable = ProcessResidueSource{}
	_ ResidueSource    = ProcessResidueSource{}
)
