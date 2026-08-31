package runnercontrol

import (
	"encoding"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const (
	ExperimentCompletionSigningDomainToken = "primitive-anvil-experiment-completion-2026-1"
	RunnerCompletionSigningDomainToken     = "primitive-anvil-runner-completion-2026-1"
	CleanupReceiptSigningDomainToken       = "primitive-anvil-cleanup-receipt-2026-1"
	ExpansionManifestSigningDomainToken    = "primitive-anvil-expansion-manifest-2026-1"
)

type CompletionSigningDomain uint8

const (
	CompletionSigningDomainUnknown CompletionSigningDomain = iota
	CompletionSigningDomainExperimentV1
	CompletionSigningDomainRunnerV1
	CompletionSigningDomainCleanupV1
	CompletionSigningDomainExpansionV1
	completionSigningDomainLimit
)

func completionSigningDomainTokens() [completionSigningDomainLimit]string {
	return [...]string{
		CompletionSigningDomainUnknown:      "",
		CompletionSigningDomainExperimentV1: ExperimentCompletionSigningDomainToken,
		CompletionSigningDomainRunnerV1:     RunnerCompletionSigningDomainToken,
		CompletionSigningDomainCleanupV1:    CleanupReceiptSigningDomainToken,
		CompletionSigningDomainExpansionV1:  ExpansionManifestSigningDomainToken,
	}
}

func (d CompletionSigningDomain) Validate() error {
	if d <= CompletionSigningDomainUnknown || d >= completionSigningDomainLimit || completionSigningDomainTokens()[d] == "" {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (d CompletionSigningDomain) String() string {
	if d.Validate() != nil {
		return ""
	}
	return completionSigningDomainTokens()[d]
}

func (d CompletionSigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

func (CompletionSigningDomain) ParseCanonicalText(text []byte) (CompletionSigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes {
		return CompletionSigningDomainUnknown, core.ErrPrimitiveContract
	}
	for candidate := CompletionSigningDomainUnknown + 1; candidate < completionSigningDomainLimit; candidate++ {
		if candidate.String() == string(text) {
			return candidate, nil
		}
	}
	return CompletionSigningDomainUnknown, core.ErrPrimitiveContract
}

func (d CompletionSigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}

func (d *CompletionSigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := CompletionSigningDomainUnknown.ParseCanonicalText([]byte(value))
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = parsed
	return nil
}

type completionSigningDomainWitness[D attest.SigningDomain[D]] [0]D

var (
	_ core.Validatable            = CompletionSigningDomainUnknown
	_ core.ValidatedJSONMarshaler = CompletionSigningDomain(0)
	_ encoding.TextMarshaler      = CompletionSigningDomainUnknown
	_ json.Unmarshaler            = (*CompletionSigningDomain)(nil)
	_                             = completionSigningDomainWitness[CompletionSigningDomain]{}
)
