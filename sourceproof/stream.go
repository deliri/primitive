package sourceproof

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
)

// Resolver obtains the independently retained result for one exact authored
// claim. Storage and execution history remain outside sourceproof.
type Resolver interface {
	ResolveResult(context.Context, sourceclaim.Claim) (Result, error)
}

// Emit receives one structurally verified, independently addressable result.
type Emit func(Result) error

// VerifyClaims joins the canonical authored stream to one result per claim.
// It forwards each result and retains only exact counters plus the current
// claim/result pair, so repository size does not become a memory quota.
func VerifyClaims(ctx context.Context, claims sourceclaim.Stream, resolver Resolver, destination Emit) (Summary, error) {
	if ctx == nil || claims == nil || resolver == nil || destination == nil {
		return Summary{}, contractError(errors.New("source proof stream dependency is nil"))
	}
	verifier := claimVerifier{ctx: ctx, resolver: resolver, destination: destination, digest: core.NewDigestWriter()}
	claimSummary, err := sourceclaim.Consume(claims, verifier.verify)
	if err != nil {
		return Summary{}, err
	}
	if verifier.summary.Claims != claimSummary.Claims {
		return Summary{}, conflictError(errors.New("source proof stream accounting differs from authored claims"))
	}
	if err := verifier.seal(); err != nil {
		return Summary{}, err
	}
	return verifier.summary, nil
}

type claimVerifier struct {
	ctx         context.Context
	resolver    Resolver
	destination Emit
	digest      *core.DigestWriter
	summary     Summary
}

func (v *claimVerifier) verify(claim sourceclaim.Claim) error {
	if err := v.ctx.Err(); err != nil {
		return err
	}
	result, err := v.resolver.ResolveResult(v.ctx, claim)
	if err != nil {
		return contractError(err)
	}
	if err := result.ValidateAgainst(claim); err != nil {
		return err
	}
	v.summary, err = v.summary.add(result)
	if err != nil {
		return err
	}
	if err := writeResultRecord(v.digest, result); err != nil {
		return err
	}
	return v.destination(result)
}

func (v *claimVerifier) seal() error {
	digest, length, err := v.digest.Seal()
	if err != nil {
		return contractError(err)
	}
	v.summary.Digest = digest
	v.summary.Bytes = length
	return v.summary.Validate()
}

func writeResultRecord(destination *core.DigestWriter, result Result) error {
	encoded, err := result.MarshalJSON()
	if err != nil {
		return err
	}
	if _, err := destination.Write(encoded); err != nil {
		return contractError(err)
	}
	if _, err := destination.Write([]byte{'\n'}); err != nil {
		return contractError(err)
	}
	return nil
}
