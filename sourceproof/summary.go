package sourceproof

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Summary reports claim and requirement cardinality separately. It does not
// synthesize a claim verdict or choose precedence among requirement states;
// that evaluation belongs to the applying policy.
type Summary struct {
	Digest                          core.SHA256Digest `json:"digest"`
	Bytes                           core.ByteLength   `json:"bytes"`
	ProjectClaims                   uint64            `json:"project_claims"`
	PackageClaims                   uint64            `json:"package_claims"`
	FileClaims                      uint64            `json:"file_claims"`
	Claims                          uint64            `json:"claims"`
	ProvenRequirements              uint64            `json:"proven_requirements"`
	ContradictedRequirements        uint64            `json:"contradicted_requirements"`
	UnprovenRequirements            uint64            `json:"unproven_requirements"`
	StaleRequirements               uint64            `json:"stale_requirements"`
	UnavailableRequirements         uint64            `json:"unavailable_requirements"`
	HumanReviewRequiredRequirements uint64            `json:"human_review_required_requirements"`
	Requirements                    uint64            `json:"requirements"`
}

func (s Summary) add(result Result) (Summary, error) {
	if err := result.Validate(); err != nil {
		return Summary{}, err
	}
	claimCounter, err := subjectCount(&s, result.Subject.Kind)
	if err != nil {
		return Summary{}, err
	}
	if s.Claims == ^uint64(0) || *claimCounter == ^uint64(0) {
		return Summary{}, contractError(errors.New("source proof claim count overflows uint64"))
	}
	s.Claims++
	*claimCounter++
	for _, requirement := range result.Requirements {
		if err := s.addRequirement(requirement.State); err != nil {
			return Summary{}, err
		}
	}
	return s, nil
}

func subjectCount(summary *Summary, kind core.SourceSubjectKind) (*uint64, error) {
	switch kind {
	case core.SourceSubjectProject:
		return &summary.ProjectClaims, nil
	case core.SourceSubjectPackage:
		return &summary.PackageClaims, nil
	case core.SourceSubjectFile:
		return &summary.FileClaims, nil
	default:
		return nil, contractError(errors.New("source proof subject kind is invalid"))
	}
}

func (s *Summary) addRequirement(state State) error {
	counter, err := requirementStateCount(s, state)
	if err != nil {
		return err
	}
	if s.Requirements == ^uint64(0) || *counter == ^uint64(0) {
		return contractError(errors.New("source proof requirement count overflows uint64"))
	}
	s.Requirements++
	*counter++
	return nil
}

func requirementStateCount(summary *Summary, state State) (*uint64, error) {
	switch state {
	case StateProven:
		return &summary.ProvenRequirements, nil
	case StateContradicted:
		return &summary.ContradictedRequirements, nil
	case StateUnproven:
		return &summary.UnprovenRequirements, nil
	case StateStale:
		return &summary.StaleRequirements, nil
	case StateUnavailable:
		return &summary.UnavailableRequirements, nil
	case StateHumanReviewRequired:
		return &summary.HumanReviewRequiredRequirements, nil
	default:
		return nil, contractError(errors.New("source proof requirement state is invalid"))
	}
}

// Validate proves scope and requirement-state accounting independently.
func (s Summary) Validate() error {
	if err := errors.Join(s.Digest.Validate(), s.Bytes.Validate()); err != nil {
		return contractError(err)
	}
	if s.Claims == 0 || s.Bytes.Uint64() == 0 ||
		!sumEquals(s.Claims, s.ProjectClaims, s.PackageClaims, s.FileClaims) ||
		!sumEquals(
			s.Requirements,
			s.ProvenRequirements,
			s.ContradictedRequirements,
			s.UnprovenRequirements,
			s.StaleRequirements,
			s.UnavailableRequirements,
			s.HumanReviewRequiredRequirements,
		) {
		return conflictError(errors.New("source proof summary accounting does not close"))
	}
	return nil
}

func sumEquals(want uint64, values ...uint64) bool {
	var total uint64
	for _, value := range values {
		if total > ^uint64(0)-value {
			return false
		}
		total += value
	}
	return total == want
}
