package gcsobjects

import (
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// GCSObjectAddress is one canonical provider address reversed into the exact
// bucket and flat object identity it names. It is an address, never a read
// capability.
type GCSObjectAddress struct {
	bucket GCSBucket
	name   GCSObjectName
}

// ParseObjectAddress admits only the canonical path-style address emitted by
// ObjectAddress. Signed URLs, alternate provider hosts, ports, queries, and
// noncanonical escaping are different contracts and fail closed.
func ParseObjectAddress(address core.HTTPEndpoint) (GCSObjectAddress, error) {
	if err := address.Validate(); err != nil {
		return GCSObjectAddress{}, core.ErrObjectStoreContract
	}
	parsed := address.HTTPURL()
	if !canonicalGCSObjectAuthority(parsed) {
		return GCSObjectAddress{}, core.ErrObjectStoreContract
	}
	bucketText, nameText, ok := strings.Cut(strings.TrimPrefix(parsed.Path, "/"), "/")
	if !ok || bucketText == "" || nameText == "" {
		return GCSObjectAddress{}, core.ErrObjectStoreContract
	}
	bucket, bucketErr := ParseGCSBucket(bucketText)
	name, nameErr := ParseGCSObjectName(nameText)
	if bucketErr != nil || nameErr != nil {
		return GCSObjectAddress{}, core.ErrObjectStoreContract
	}
	result := GCSObjectAddress{bucket: bucket, name: name}
	canonical, err := ObjectAddress(bucket, name)
	if err != nil || canonical != address {
		return GCSObjectAddress{}, core.ErrObjectStoreContract
	}
	return result, result.Validate()
}

func canonicalGCSObjectAuthority(address url.URL) bool {
	return address.Scheme == core.SchemeHTTPS &&
		address.Host == core.GoogleCloudStorageHost &&
		address.Port() == "" && address.RawQuery == "" && !address.ForceQuery &&
		address.Fragment == "" && address.RawFragment == "" &&
		strings.HasPrefix(address.Path, "/")
}

// Validate proves both provider identities and their canonical combined
// address remain valid.
func (a GCSObjectAddress) Validate() error {
	if err := a.bucket.Validate(); err != nil {
		return core.ErrObjectStoreContract
	}
	if err := a.name.Validate(); err != nil {
		return core.ErrObjectStoreContract
	}
	_, err := ObjectAddress(a.bucket, a.name)
	return err
}

// Bucket returns the exact validated bucket identity.
func (a GCSObjectAddress) Bucket() GCSBucket { return a.bucket }

// Name returns the exact validated flat object identity.
func (a GCSObjectAddress) Name() GCSObjectName { return a.name }

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
