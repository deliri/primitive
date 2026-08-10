package distribution

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

// PublicationSource is one exact object stream and optional progress observer.
type PublicationSource struct {
	Reader   io.Reader
	Observer objectstore.ProgressObserver
}

func (s PublicationSource) Validate() error {
	if s.Reader == nil {
		return contractError(errors.New("publication source is nil"))
	}
	return nil
}

// PublicationPlanRequest joins an authenticated grant to the exact verified
// manifest and its fixed source streams. No source is read during preparation.
type PublicationPlanRequest struct {
	Sources  [release.PublicationObjectCount]PublicationSource
	Grant    VerifiedPublicationGrant
	Manifest release.VerifiedManifest
	Policy   objectstore.Policy
}

func (r PublicationPlanRequest) Validate() error {
	if err := errors.Join(r.Grant.Validate(), r.Manifest.Validate(), r.Policy.Validate()); err != nil {
		return contractError(err)
	}
	request, err := r.Grant.Request()
	if err != nil || request.Manifest != r.Manifest.Document() {
		return bindingError(errors.New("publication plan manifest differs from grant request"), err)
	}
	for _, source := range r.Sources {
		if err := source.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// PreparePublicationPlan projects the signed bearer set into Deploy's exact
// create-only execution plan.
func PreparePublicationPlan(request PublicationPlanRequest) (deploy.ReleasePlan, error) {
	if err := request.Validate(); err != nil {
		return deploy.ReleasePlan{}, err
	}
	var items [release.PublicationObjectCount]deploy.UploadItem
	for index, source := range request.Sources {
		role, ok := release.PublicationRoleAt(index)
		if !ok {
			return deploy.ReleasePlan{}, contractError(errors.New("publication plan role slot is invalid"))
		}
		integrity, err := request.Manifest.PublicationIntegrity(role)
		if err != nil {
			return deploy.ReleasePlan{}, contractError(err)
		}
		capability, commitment, err := request.Grant.Capability(role)
		if err != nil {
			return deploy.ReleasePlan{}, err
		}
		item, err := deploy.NewUploadItem(deploy.UploadItemRequest{
			Source: source.Reader, Observer: source.Observer,
			Capability: capability, Commitment: commitment,
			Integrity: integrity, Role: role,
		})
		if err != nil {
			return deploy.ReleasePlan{}, contractError(err)
		}
		items[index] = item
	}
	return deploy.PrepareRelease(deploy.ReleasePlanRequest{
		Items: items, Manifest: request.Manifest, Policy: request.Policy,
	})
}

var (
	_ core.Validatable = PublicationSource{}
	_ core.Validatable = PublicationPlanRequest{}
)
