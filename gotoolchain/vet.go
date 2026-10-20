package gotoolchain

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

// VetAnalyzer names native cmd/vet analyzers supported by this invocation.
// An empty selection runs cmd/go's default analyzer set.
type VetAnalyzer uint8

const (
	VetAnalyzerUnknown VetAnalyzer = iota
	VetAnalyzerLostCancel
	VetAnalyzerWaitGroup
)

func (a VetAnalyzer) Validate() error {
	if a != VetAnalyzerLostCancel && a != VetAnalyzerWaitGroup {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (a VetAnalyzer) IsValid() bool { return a.Validate() == nil }
func (VetAnalyzer) OffWireEnum()    {}
func (a VetAnalyzer) String() string {
	switch a {
	case VetAnalyzerLostCancel:
		return "lostcancel"
	case VetAnalyzerWaitGroup:
		return "waitgroup"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// VetRequest selects one exact native package. Cmd/go owns build selection,
// aliases, receiver types, binding scopes and analyzer control-flow semantics.
// Go includes its test variants; consumers decide which source findings apply.
type VetRequest struct {
	WorkingDirectory core.AbsolutePath
	Package          gomodule.ImportPath
	Analyzers        []VetAnalyzer
	Visit            func(runnercontrol.GoVetObservation) error
}

func (VetRequest) goToolchainCapabilityWrapper() {}
func (r VetRequest) Validate() error {
	if err := errors.Join(r.WorkingDirectory.Validate(), r.Package.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if r.Visit == nil {
		return core.ErrGoToolchainContract
	}
	for _, analyzer := range r.Analyzers {
		if err := analyzer.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ObserveVet executes cmd/go's real vet tool and streams its completed native
// JSON output. JSON mode can exit zero while reporting diagnostics; this API
// does not infer a clean verdict from that exit. Nonzero process exits, source
// errors, callback refusals and cancellation return a zero receipt and preserve
// native identities. Earlier callback loans certify only their admitted prefix.
func (c Capability) ObserveVet(ctx context.Context, request VetRequest) (process.ResultObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return process.ResultObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	arguments := []string{"vet", "-json"}
	for _, analyzer := range request.Analyzers {
		arguments = append(arguments, "-"+analyzer.String())
	}
	arguments = append(arguments, request.Package.String())
	var receipt process.ResultObservation
	err := c.withCompletedCommandOutput(ctx, request.WorkingDirectory, arguments, func(source io.Reader, observed process.ResultObservation) error {
		if err := runnercontrol.ReadGoVetStream(ctx, runnercontrol.GoVetStreamRequest{Source: source, Visit: request.Visit}); err != nil {
			return errors.Join(core.ErrGoToolchainOutput, err)
		}
		receipt = observed
		return nil
	})
	err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
	if err != nil {
		return process.ResultObservation{}, err
	}
	return receipt, nil
}

var _ core.OffWireEnum = VetAnalyzerUnknown
