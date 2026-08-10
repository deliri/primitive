package submission

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

// UploadCallRequest binds a caller-owned source and transfer policy to the
// exact declaration that obtained an authenticated upload decision.
type UploadCallRequest struct {
	Source   io.Reader
	Observer objectstore.ProgressObserver
	Request  RequestPayload
	Policy   objectstore.Policy
}

// Validate closes all caller-owned upload inputs without reading the source.
func (r UploadCallRequest) Validate() error {
	if r.Source == nil {
		return contractError(errors.New("submission upload source is nil"))
	}
	if err := errors.Join(r.Request.Validate(), r.Policy.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// UploadCall projects one authenticated upload decision into the blind
// Objectstore operation it authorized. It neither reads nor transfers bytes.
func (v VerifiedDecision) UploadCall(
	request UploadCallRequest,
) (objectstore.UploadCapabilityRequest, error) {
	if err := errors.Join(v.Validate(), request.Validate()); err != nil {
		return objectstore.UploadCapabilityRequest{}, contractError(err)
	}
	grant, ok := v.Grant()
	if !ok {
		return objectstore.UploadCapabilityRequest{}, bindingError(errors.New("submission decision does not authorize upload"))
	}
	payload, err := grant.Payload()
	if err != nil {
		return objectstore.UploadCapabilityRequest{}, contractError(err)
	}
	commitment, err := CommitRequest(request.Request)
	if err != nil {
		return objectstore.UploadCapabilityRequest{}, contractError(err)
	}
	if commitment != payload.Request {
		return objectstore.UploadCapabilityRequest{}, bindingError(errors.New("submission upload request differs from grant"))
	}
	capability, err := grant.Capability()
	if err != nil {
		return objectstore.UploadCapabilityRequest{}, contractError(err)
	}
	call := objectstore.UploadCapabilityRequest{
		Source: request.Source, ContentType: request.Request.Declaration.ContentType,
		Capability: capability, Integrity: request.Request.Declaration.Integrity(),
		Policy: request.Policy, Observer: request.Observer,
	}
	if err := call.Validate(); err != nil {
		return objectstore.UploadCapabilityRequest{}, contractError(err)
	}
	return call, nil
}

var _ core.Validatable = UploadCallRequest{}
