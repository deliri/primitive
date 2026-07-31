package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const versionComparisonDomainDiagnostic = "version comparison escaped its domain"

type AdvanceLatestRequest struct {
	Retained VerifiedLatest
	Proposed VerifiedLatest
}

type LatestAdvance struct {
	state LatestAdvanceState
	valid bool
}

func (a LatestAdvance) Validate() error {
	if !a.valid {
		return contractError(errors.New("latest advance is unset"))
	}
	return a.state.Validate()
}

func (a LatestAdvance) State() LatestAdvanceState { return a.state }

func AdvanceLatest(request AdvanceLatestRequest) (LatestAdvance, error) {
	if err := request.Retained.Validate(); err != nil {
		return LatestAdvance{}, verificationError(err)
	}
	if err := request.Proposed.Validate(); err != nil {
		return LatestAdvance{}, verificationError(err)
	}
	retained, _ := request.Retained.Fact()
	proposed, _ := request.Proposed.Fact()
	if retained.Identity() != proposed.Identity() ||
		retained.Offering() != proposed.Offering() ||
		retained.Revision() != proposed.Revision() {
		return LatestAdvance{}, conflictError(errors.New("latest stream identity differs"))
	}
	generationOrder := compareGeneration(retained.Generation(), proposed.Generation())
	switch generationOrder {
	case core.ComparisonGreater:
		return LatestAdvance{}, rollbackError(errors.New("latest generation decreased"))
	case core.ComparisonEqual:
		return compareReplay(request)
	case core.ComparisonLess:
		return compareHigherGeneration(request, retained, proposed)
	default:
		return LatestAdvance{}, contractError(errors.New("generation comparison escaped its domain"))
	}
}

func compareGeneration(left, right Generation) core.Comparison {
	switch {
	case left.Uint64() < right.Uint64():
		return core.ComparisonLess
	case left.Uint64() > right.Uint64():
		return core.ComparisonGreater
	default:
		return core.ComparisonEqual
	}
}

func compareReplay(request AdvanceLatestRequest) (LatestAdvance, error) {
	retained, _ := request.Retained.Document()
	proposed, _ := request.Proposed.Document()
	if retained != proposed {
		return LatestAdvance{}, conflictError(errors.New("equal generation carries a different signed document"))
	}
	return newLatestAdvance(LatestAdvanceReplay)
}

func compareHigherGeneration(
	request AdvanceLatestRequest,
	retained LatestFact,
	proposed LatestFact,
) (LatestAdvance, error) {
	if err := requireNondecreasingTimeline(retained, proposed); err != nil {
		return LatestAdvance{}, err
	}
	retainedManifest, _ := request.Retained.Manifest()
	proposedManifest, _ := request.Proposed.Manifest()
	retainedVersion, _ := retainedManifest.Version()
	proposedVersion, _ := proposedManifest.Version()
	order, err := retainedVersion.Compare(proposedVersion)
	if err != nil {
		return LatestAdvance{}, contractError(err)
	}
	switch order {
	case core.ComparisonGreater:
		return LatestAdvance{}, rollbackError(errors.New("latest version decreased"))
	case core.ComparisonEqual:
		return compareEqualVersion(retainedManifest, proposedManifest)
	case core.ComparisonLess:
		return compareGreaterVersion(retainedManifest, proposedManifest)
	default:
		return LatestAdvance{}, contractError(errors.New(versionComparisonDomainDiagnostic))
	}
}

func requireNondecreasingTimeline(retained, proposed LatestFact) error {
	checks := [][2]temporal.Instant{
		{retained.IssuedAt(), proposed.IssuedAt()},
		{retained.ValidFrom(), proposed.ValidFrom()},
		{retained.ValidUntil(), proposed.ValidUntil()},
	}
	for _, check := range checks {
		order, err := check[0].Compare(check[1])
		if err != nil {
			return conflictError(err)
		}
		if order == core.ComparisonGreater {
			return rollbackError(errors.New("latest signed timeline moved backward"))
		}
	}
	return nil
}

func compareEqualVersion(retained, proposed VerifiedManifest) (LatestAdvance, error) {
	retainedDocument, _ := retained.Document()
	proposedDocument, _ := proposed.Document()
	if retainedDocument != proposedDocument {
		return LatestAdvance{}, conflictError(errors.New("equal version selects a different manifest document"))
	}
	return newLatestAdvance(LatestAdvanceAdvanced)
}

func compareGreaterVersion(retained, proposed VerifiedManifest) (LatestAdvance, error) {
	retainedIdentity, _ := retained.Identity()
	proposedIdentity, _ := proposed.Identity()
	if retainedIdentity == proposedIdentity {
		return LatestAdvance{}, conflictError(errors.New("greater version reuses manifest identity"))
	}
	return newLatestAdvance(LatestAdvanceAdvanced)
}

func newLatestAdvance(state LatestAdvanceState) (LatestAdvance, error) {
	result := LatestAdvance{state: state, valid: true}
	if err := result.Validate(); err != nil {
		return LatestAdvance{}, err
	}
	return result, nil
}
