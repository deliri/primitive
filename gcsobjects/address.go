package gcsobjects

import (
	"net/url"

	"github.com/deliri/primitive/v2026/core"
)

// THE ADDRESS OF A PUBLISHED OBJECT.
//
// UploadMedia writes an object a browser or CDN will fetch. Until now the
// package would publish that object and then decline to say where it had put
// it, so every consumer rebuilt the provider's URL by hand from a copied host
// and two slashes. That is the projection rule of section 3.3 broken by
// omission: the package that owns the object refused to own its address, so the
// address grew copies outside the package instead.
//
// The address is the canonical public form. It is not a capability and grants
// nothing: whether a reader may fetch it is the bucket's access policy, not a
// property of the string. A consumer serving through a CDN composes its own
// origin and does not use this.
//
// ObjectAddress is a pure value derivation with no effect, so it neither reads
// from the provider nor proves the object exists.
func ObjectAddress(bucket GCSBucket, name GCSObjectName) (core.HTTPEndpoint, error) {
	if err := bucket.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	if err := name.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	// The composition is this package's projection because this package owns
	// the object. core owns the two protocol values it is built from and the
	// validation it is checked by, and the standard library owns the escaping:
	// url.URL percent-encodes what a path may not carry literally and leaves
	// the separator alone, so a nested object name keeps its hierarchy instead
	// of collapsing into one escaped segment.
	address := url.URL{
		Scheme: core.SchemeHTTPS,
		Host:   core.GoogleCloudStorageHost,
		Path:   "/" + bucket.String() + "/" + name.String(),
	}
	return core.ParseHTTPEndpoint(address.String())
}

// Address returns the canonical public address of the object this metadata
// describes.
//
// Validating the whole metadata first is deliberate: an address derived from a
// half-populated result would be a plausible URL for an object that was never
// accepted, and a caller storing that in a manifest would have recorded a
// publication that did not happen.
func (m GCSObjectMetadata) Address() (core.HTTPEndpoint, error) {
	if err := m.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return ObjectAddress(m.Bucket(), m.Name())
}
