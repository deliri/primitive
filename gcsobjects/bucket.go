package gcsobjects

import (
	"errors"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// GCSProjectIDMinimumBytes is Google's minimum project-ID extent.
	// Source: https://cloud.google.com/resource-manager/docs/creating-managing-projects#identifying_projects
	GCSProjectIDMinimumBytes = 6
	// GCSProjectIDMaximumBytes is Google's maximum project-ID extent.
	// Source: https://cloud.google.com/resource-manager/docs/creating-managing-projects#identifying_projects
	GCSProjectIDMaximumBytes = 30
	// GCSLocationMaximumBytes is Primitive's custody ceiling for one provider
	// location token; Google publishes the available values rather than a wire extent.
	// Source: https://cloud.google.com/storage/docs/locations
	GCSLocationMaximumBytes = 63
)

// GCSProjectID is one validated Google Cloud project identifier used for
// provider-owned bucket provisioning.
type GCSProjectID struct{ value string }

// ParseGCSProjectID admits one lowercase DNS-shaped project identifier.
func ParseGCSProjectID(value string) (GCSProjectID, error) {
	project := GCSProjectID{value: value}
	if err := project.Validate(); err != nil {
		return GCSProjectID{}, err
	}
	return project, nil
}

// String returns the validated provider project identifier.
func (p GCSProjectID) String() string { return p.value }

// Validate rejects project identifiers outside Google's stable lexical shape.
func (p GCSProjectID) Validate() error {
	if len(p.value) < GCSProjectIDMinimumBytes || len(p.value) > GCSProjectIDMaximumBytes ||
		!gcsLowerAlpha(p.value[0]) || !gcsLowerAlphaNumeric(p.value[len(p.value)-1]) {
		return core.ErrObjectStoreContract
	}
	for index := range len(p.value) {
		if !gcsLowerAlphaNumeric(p.value[index]) && p.value[index] != '-' {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}

// GCSLocation is one validated provider location token. The domain is nominal
// rather than an enum because Google may publish regions without a Primitive
// release; the owning provider remains the authority over availability.
type GCSLocation struct{ value string }

// ParseGCSLocation admits one bounded provider location token.
func ParseGCSLocation(value string) (GCSLocation, error) {
	location := GCSLocation{value: value}
	if err := location.Validate(); err != nil {
		return GCSLocation{}, err
	}
	return location, nil
}

// String returns the validated provider location token.
func (l GCSLocation) String() string { return l.value }

// Validate rejects empty, non-ASCII, malformed, or oversized location tokens.
func (l GCSLocation) Validate() error {
	if len(l.value) == 0 || len(l.value) > GCSLocationMaximumBytes || !utf8.ValidString(l.value) ||
		!gcsAlphaNumeric(l.value[0]) || !gcsAlphaNumeric(l.value[len(l.value)-1]) {
		return core.ErrObjectStoreContract
	}
	for index := range len(l.value) {
		if !gcsAlphaNumeric(l.value[index]) && l.value[index] != '-' {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}

// GCSNamespace selects the provider namespace created with a bucket.
type GCSNamespace uint8

const (
	// GCSNamespaceUnknown is the invalid zero namespace.
	GCSNamespaceUnknown GCSNamespace = iota
	// GCSNamespaceFlat uses object-name prefixes as logical directories.
	GCSNamespaceFlat
	// GCSNamespaceHierarchical enables provider-managed hierarchical namespace.
	GCSNamespaceHierarchical
	gcsNamespaceLimit
)

// Validate closes the namespace domain.
func (n GCSNamespace) Validate() error {
	if n <= GCSNamespaceUnknown || n >= gcsNamespaceLimit || gcsNamespaceDiagnostics()[n] == "" {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether n is a published namespace.
func (n GCSNamespace) IsValid() bool { return n.Validate() == nil }

// String returns the diagnostic namespace token.
func (n GCSNamespace) String() string {
	if !n.IsValid() {
		return ""
	}
	return gcsNamespaceDiagnostics()[n]
}

func gcsNamespaceDiagnostics() [gcsNamespaceLimit]string {
	return [...]string{"", "flat", "hierarchical"}
}

// OffWireEnum declares GCSNamespace as provider execution policy.
func (GCSNamespace) OffWireEnum() {}

// GCSBucketCreateRequest is the complete provider-owned bucket provisioning
// input. Provisioning never overwrites or mutates an existing bucket.
type GCSBucketCreateRequest struct {
	Project   GCSProjectID
	Bucket    GCSBucket
	Location  GCSLocation
	Namespace GCSNamespace
}

// Validate closes every provider provisioning fact before execution.
func (r GCSBucketCreateRequest) Validate() error {
	if err := errors.Join(
		r.Project.Validate(), r.Bucket.Validate(), r.Location.Validate(), r.Namespace.Validate(),
	); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// GCSBucketProvisioning is sealed process evidence that the official provider
// client accepted one exact create request.
type GCSBucketProvisioning struct {
	request GCSBucketCreateRequest
	set     bool
}

// Validate rejects unset or malformed provisioning evidence.
func (p GCSBucketProvisioning) Validate() error {
	if !p.set {
		return core.ErrObjectStoreContract
	}
	return p.request.Validate()
}

func (p GCSBucketProvisioning) Project() GCSProjectID   { return p.request.Project }
func (p GCSBucketProvisioning) Bucket() GCSBucket       { return p.request.Bucket }
func (p GCSBucketProvisioning) Location() GCSLocation   { return p.request.Location }
func (p GCSBucketProvisioning) Namespace() GCSNamespace { return p.request.Namespace }

// GCSBucketPublicReadRequest names one bucket whose objects must be readable
// without authentication. The product chooses the bucket; Primitive owns the
// exact provider IAM effect.
type GCSBucketPublicReadRequest struct {
	Bucket GCSBucket
}

// Validate rejects an unset provider bucket before any IAM call.
func (r GCSBucketPublicReadRequest) Validate() error {
	if err := r.Bucket.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// GCSBucketPublicReadChange reports whether the exact public-read membership
// already existed or was added by this operation.
type GCSBucketPublicReadChange uint8

const (
	// GCSBucketPublicReadChangeUnknown is the invalid zero change.
	GCSBucketPublicReadChangeUnknown GCSBucketPublicReadChange = iota
	// GCSBucketPublicReadUnchanged means the exact membership already existed.
	GCSBucketPublicReadUnchanged
	// GCSBucketPublicReadGranted means the membership was added and confirmed.
	GCSBucketPublicReadGranted
	gcsBucketPublicReadChangeLimit
)

// Validate closes the public-read change domain.
func (c GCSBucketPublicReadChange) Validate() error {
	if c <= GCSBucketPublicReadChangeUnknown || c >= gcsBucketPublicReadChangeLimit ||
		gcsBucketPublicReadChangeDiagnostics()[c] == "" {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether c is a published public-read change.
func (c GCSBucketPublicReadChange) IsValid() bool { return c.Validate() == nil }

// String returns the diagnostic public-read change token.
func (c GCSBucketPublicReadChange) String() string {
	if !c.IsValid() {
		return ""
	}
	return gcsBucketPublicReadChangeDiagnostics()[c]
}

func gcsBucketPublicReadChangeDiagnostics() [gcsBucketPublicReadChangeLimit]string {
	return [...]string{"", "public_read_unchanged", "public_read_granted"}
}

// OffWireEnum declares GCSBucketPublicReadChange as provider execution state.
func (GCSBucketPublicReadChange) OffWireEnum() {}

// GCSBucketPublicReadGrant is sealed evidence that the official provider IAM
// policy contains the exact unauthenticated object-read membership.
type GCSBucketPublicReadGrant struct {
	bucket GCSBucket
	change GCSBucketPublicReadChange
	set    bool
}

// Validate rejects unset, malformed, or contradictory grant evidence.
func (g GCSBucketPublicReadGrant) Validate() error {
	if !g.set {
		return core.ErrObjectStoreContract
	}
	if err := errors.Join(g.bucket.Validate(), g.change.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// Bucket returns the exact bucket whose IAM policy was observed.
func (g GCSBucketPublicReadGrant) Bucket() GCSBucket { return g.bucket }

// Change reports whether the exact membership already existed or was added.
func (g GCSBucketPublicReadGrant) Change() GCSBucketPublicReadChange { return g.change }

func gcsLowerAlpha(value byte) bool { return value >= 'a' && value <= 'z' }

func gcsAlphaNumeric(value byte) bool {
	return gcsLowerAlphaNumeric(value) || value >= 'A' && value <= 'Z'
}

var _ core.OffWireEnum = GCSNamespaceUnknown
var _ core.OffWireEnum = GCSBucketPublicReadChangeUnknown
