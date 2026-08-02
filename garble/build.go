package garble

import (
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
)

const (
	seedArgumentPrefix  = "-seed="
	literalsArgument    = "-literals"
	tinyArgument        = "-tiny"
	buildArgument       = "build"
	preservePolicyLabel = "preserve"
)

// LiteralPolicy selects the upstream opt-in literal obfuscation behavior.
type LiteralPolicy uint8

const (
	// LiteralPolicyUnknown is the invalid zero policy.
	LiteralPolicyUnknown LiteralPolicy = iota
	// LiteralPolicyPreserve leaves literal obfuscation disabled.
	LiteralPolicyPreserve
	// LiteralPolicyObfuscate enables Garble's -literals behavior.
	LiteralPolicyObfuscate
	literalPolicyLimit
)

func literalPolicyLabels() [literalPolicyLimit]string {
	return [...]string{
		LiteralPolicyPreserve:  preservePolicyLabel,
		LiteralPolicyObfuscate: "obfuscate",
	}
}

// Validate rejects an incomplete or future literal policy.
func (p LiteralPolicy) Validate() error {
	if !p.IsValid() {
		return contractError(errors.New("garble literal policy is outside the admitted domain"))
	}
	return nil
}

// IsValid reports membership in the closed literal-policy domain.
func (p LiteralPolicy) IsValid() bool {
	return p > LiteralPolicyUnknown && p < literalPolicyLimit && literalPolicyLabels()[p] != ""
}

// OffWireEnum declares LiteralPolicy as build execution policy rather than a
// wire encoding.
func (LiteralPolicy) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for p.
func (p LiteralPolicy) String() string {
	if !p.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return literalPolicyLabels()[p]
}

// DiagnosticPolicy selects whether Garble retains runtime diagnostic metadata.
type DiagnosticPolicy uint8

const (
	// DiagnosticPolicyUnknown is the invalid zero policy.
	DiagnosticPolicyUnknown DiagnosticPolicy = iota
	// DiagnosticPolicyPreserve retains positions, panic text, and metadata.
	DiagnosticPolicyPreserve
	// DiagnosticPolicyStrip enables Garble's lossy -tiny behavior.
	DiagnosticPolicyStrip
	diagnosticPolicyLimit
)

func diagnosticPolicyLabels() [diagnosticPolicyLimit]string {
	return [...]string{
		DiagnosticPolicyPreserve: preservePolicyLabel,
		DiagnosticPolicyStrip:    "strip",
	}
}

// Validate rejects an incomplete or future diagnostic policy.
func (p DiagnosticPolicy) Validate() error {
	if !p.IsValid() {
		return contractError(errors.New("garble diagnostic policy is outside the admitted domain"))
	}
	return nil
}

// IsValid reports membership in the closed diagnostic-policy domain.
func (p DiagnosticPolicy) IsValid() bool {
	return p > DiagnosticPolicyUnknown && p < diagnosticPolicyLimit &&
		diagnosticPolicyLabels()[p] != ""
}

// OffWireEnum declares DiagnosticPolicy as build execution policy rather than
// a wire encoding.
func (DiagnosticPolicy) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for p.
func (p DiagnosticPolicy) String() string {
	if !p.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return diagnosticPolicyLabels()[p]
}

// BuildRequest carries the complete Garble-owned build-intent prefix.
type BuildRequest struct {
	Tool        ToolIdentity
	Seed        Seed
	Literals    LiteralPolicy
	Diagnostics DiagnosticPolicy
}

// Validate checks tool identity, seed, and both explicit upstream policies.
func (r BuildRequest) Validate() error {
	if err := r.Tool.Validate(); err != nil {
		return err
	}
	if err := r.Seed.Validate(); err != nil {
		return err
	}
	if err := r.Literals.Validate(); err != nil {
		return err
	}
	return r.Diagnostics.Validate()
}

// BuildIntent is the validated Garble-owned portion of a process request.
type BuildIntent struct {
	request BuildRequest
	set     bool
}

// PrepareBuild validates and closes one typed Garble build intent.
func PrepareBuild(request BuildRequest) (BuildIntent, error) {
	if err := request.Validate(); err != nil {
		return BuildIntent{}, buildIntentError(err)
	}
	return BuildIntent{request: request, set: true}, nil
}

// Validate rejects unset or structurally invalid build intent.
func (i BuildIntent) Validate() error {
	if !i.set {
		return buildIntentError(errors.New("garble build intent is unset"))
	}
	if err := i.request.Validate(); err != nil {
		return buildIntentError(err)
	}
	return nil
}

// ArgumentKind is the closed Garble-owned CLI argument domain.
type ArgumentKind uint8

const argumentKindDomainDetail = "garble argument kind is outside the admitted domain"

const (
	// ArgumentKindUnknown is the invalid zero kind.
	ArgumentKindUnknown ArgumentKind = iota
	// ArgumentKindSeed identifies the required deterministic seed flag.
	ArgumentKindSeed
	// ArgumentKindLiterals identifies the opt-in literal-obfuscation flag.
	ArgumentKindLiterals
	// ArgumentKindTiny identifies the lossy diagnostic-stripping flag.
	ArgumentKindTiny
	// ArgumentKindBuild identifies the Go build subcommand.
	ArgumentKindBuild
	argumentKindLimit
)

func argumentKindLabels() [argumentKindLimit]string {
	return [...]string{
		ArgumentKindSeed:     "seed",
		ArgumentKindLiterals: "literals",
		ArgumentKindTiny:     "tiny",
		ArgumentKindBuild:    "build",
	}
}

// Validate rejects kinds outside the closed Garble argument domain.
func (k ArgumentKind) Validate() error {
	if !k.IsValid() {
		return buildIntentError(errors.New(argumentKindDomainDetail))
	}
	return nil
}

// IsValid reports membership in the closed argument-kind domain.
func (k ArgumentKind) IsValid() bool {
	return k > ArgumentKindUnknown && k < argumentKindLimit && argumentKindLabels()[k] != ""
}

// OffWireEnum declares ArgumentKind as CLI lowering policy rather than a wire
// encoding.
func (ArgumentKind) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for k.
func (k ArgumentKind) String() string {
	if !k.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return argumentKindLabels()[k]
}

// Argument is one typed Garble-owned CLI argument. Text lowering is explicit
// and remains unavailable until the argument validates.
type Argument struct {
	kind ArgumentKind
	seed Seed
}

// Kind returns the validated compiler-owned argument role.
func (a Argument) Kind() (ArgumentKind, error) {
	if err := a.Validate(); err != nil {
		return ArgumentKindUnknown, err
	}
	return a.kind, nil
}

// Text performs the final Garble-owned projection to one CLI argument.
func (a Argument) Text() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}
	switch a.kind {
	case ArgumentKindSeed:
		seed, err := a.seed.Encoded()
		if err != nil {
			return "", buildIntentError(err)
		}
		return seedArgumentPrefix + seed, nil
	case ArgumentKindLiterals:
		return literalsArgument, nil
	case ArgumentKindTiny:
		return tinyArgument, nil
	case ArgumentKindBuild:
		return buildArgument, nil
	default:
		return "", buildIntentError(errors.New(argumentKindDomainDetail))
	}
}

// Validate enforces the exact relation between kind and carried seed.
func (a Argument) Validate() error {
	if err := a.kind.Validate(); err != nil {
		return err
	}
	if a.kind == ArgumentKindSeed {
		if err := a.seed.Validate(); err != nil {
			return buildIntentError(err)
		}
		return nil
	}
	if a.seed != (Seed{}) {
		return buildIntentError(errors.New("non-seed garble argument carries a seed"))
	}
	return nil
}

// Arguments streams the exact Garble-owned CLI prefix in upstream-required
// order. Garble flags precede the build command.
func (i BuildIntent) Arguments() (iter.Seq[Argument], error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	return func(yield func(Argument) bool) {
		if !yield(Argument{kind: ArgumentKindSeed, seed: i.request.Seed}) {
			return
		}
		if i.request.Literals == LiteralPolicyObfuscate &&
			!yield(Argument{kind: ArgumentKindLiterals}) {
			return
		}
		if i.request.Diagnostics == DiagnosticPolicyStrip &&
			!yield(Argument{kind: ArgumentKindTiny}) {
			return
		}
		yield(Argument{kind: ArgumentKindBuild})
	}, nil
}
