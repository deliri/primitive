package retrieval

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

// DownloadCallRequest supplies only caller-owned execution facts. The
// authenticated grant owns the remote bearer, object identity, media type,
// and exact integrity declaration.
type DownloadCallRequest struct {
	Destination io.Writer
	Observer    objectstore.ProgressObserver
	Policy      objectstore.Policy
}

// Validate closes the local destination and transfer policy without effects.
func (r DownloadCallRequest) Validate() error {
	if r.Destination == nil {
		return contractError(errors.New("retrieval download destination is nil"))
	}
	if err := r.Policy.Validate(); err != nil {
		return contractError(err)
	}
	if r.Observer != nil {
		if err := r.Observer.Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

// DownloadCall projects one authenticated grant into the exact blind
// Objectstore call it authorized. It performs no read, write, or network call.
func (v VerifiedGrant) DownloadCall(
	request DownloadCallRequest,
) (objectstore.DownloadCapabilityRequest, error) {
	if err := errors.Join(v.Validate(), request.Validate()); err != nil {
		return objectstore.DownloadCapabilityRequest{}, contractError(err)
	}
	payload, err := v.Payload()
	if err != nil {
		return objectstore.DownloadCapabilityRequest{}, contractError(err)
	}
	capability, err := v.Capability()
	if err != nil {
		return objectstore.DownloadCapabilityRequest{}, contractError(err)
	}
	body := payload.Entry.Evidence.Payload.Body
	call := objectstore.DownloadCapabilityRequest{
		Destination: request.Destination, Capability: capability,
		ContentType: payload.Entry.ContentType,
		Integrity: objectstore.Integrity{
			Length: body.Extent, SHA256: body.SHA256, CRC32C: body.CRC32C,
		},
		Policy: request.Policy, Observer: request.Observer,
	}
	if err := call.Validate(); err != nil {
		return objectstore.DownloadCapabilityRequest{}, contractError(err)
	}
	return call, nil
}

var _ core.Validatable = DownloadCallRequest{}
