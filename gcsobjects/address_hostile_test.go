package gcsobjects_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gcsobjects"
)

// ObjectAddress is the package's projection from a bucket and an object name to
// the canonical public address. It is pure, so the pressure is entirely on what
// it composes and what it refuses to compose.

func TestObjectAddressComposesCanonicalAddresses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		bucket string
		object string
		want   string
	}{
		{
			name:   "flat object name",
			bucket: "offgridsoftware-legal",
			object: "ogs_terms_of_service_2026-08-02.pdf",
			want:   "https://storage.googleapis.com/offgridsoftware-legal/ogs_terms_of_service_2026-08-02.pdf",
		},
		{
			name:   "nested object name keeps its hierarchy",
			bucket: "cleanlift-legal",
			object: "legal/2026/privacy_policy.pdf",
			want:   "https://storage.googleapis.com/cleanlift-legal/legal/2026/privacy_policy.pdf",
		},
		{
			name:   "shortest legal bucket name",
			bucket: "abc",
			object: "a",
			want:   "https://storage.googleapis.com/abc/a",
		},
		{
			name:   "bucket carrying dots and dashes",
			bucket: "a-b.c-d",
			object: "object",
			want:   "https://storage.googleapis.com/a-b.c-d/object",
		},
		{
			name:   "object name needing percent encoding",
			bucket: "bucket-name",
			object: "terms of service.pdf",
			want:   "https://storage.googleapis.com/bucket-name/terms%20of%20service.pdf",
		},
		{
			name:   "a question mark cannot open a query string",
			bucket: "bucket-name",
			object: "terms?v=2.pdf",
			want:   "https://storage.googleapis.com/bucket-name/terms%3Fv=2.pdf",
		},
		{
			name:   "a fragment marker cannot open a fragment",
			bucket: "bucket-name",
			object: "terms#2.pdf",
			want:   "https://storage.googleapis.com/bucket-name/terms%232.pdf",
		},
		{
			name:   "a percent is escaped rather than read as an escape",
			bucket: "bucket-name",
			object: "100%.pdf",
			want:   "https://storage.googleapis.com/bucket-name/100%25.pdf",
		},
		{
			name:   "non ascii is encoded",
			bucket: "bucket-name",
			object: "québec.pdf",
			want:   "https://storage.googleapis.com/bucket-name/qu%C3%A9bec.pdf",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			bucket, err := gcsobjects.ParseGCSBucket(testCase.bucket)
			if err != nil {
				t.Fatalf("ParseGCSBucket(%q) error = %v, want nil", testCase.bucket, err)
			}
			object, err := gcsobjects.ParseGCSObjectName(testCase.object)
			if err != nil {
				t.Fatalf("ParseGCSObjectName(%q) error = %v, want nil", testCase.object, err)
			}

			address, err := gcsobjects.ObjectAddress(bucket, object)
			if err != nil {
				t.Fatalf("ObjectAddress() error = %v, want nil", err)
			}
			if got := address.String(); got != testCase.want {
				t.Fatalf("ObjectAddress() = %q, want %q", got, testCase.want)
			}
			if err := address.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

// A zero or unvalidated value must never produce a plausible address. An
// address for an object that was never accepted is worse than no address: a
// consumer would record it as a publication that did not happen.
func TestObjectAddressRefusesUnvalidatedValues(t *testing.T) {
	t.Parallel()

	validBucket, err := gcsobjects.ParseGCSBucket("offgridsoftware-legal")
	if err != nil {
		t.Fatalf("ParseGCSBucket() error = %v, want nil", err)
	}
	validObject, err := gcsobjects.ParseGCSObjectName("terms.pdf")
	if err != nil {
		t.Fatalf("ParseGCSObjectName() error = %v, want nil", err)
	}

	cases := []struct {
		name   string
		bucket gcsobjects.GCSBucket
		object gcsobjects.GCSObjectName
	}{
		{name: "zero bucket with valid object", object: validObject},
		{name: "valid bucket with zero object", bucket: validBucket},
		{name: "both zero", bucket: gcsobjects.GCSBucket{}, object: gcsobjects.GCSObjectName{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			address, err := gcsobjects.ObjectAddress(testCase.bucket, testCase.object)
			if !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatalf("ObjectAddress() error = %v, want %v", err, core.ErrObjectStoreContract)
			}
			if got := address.String(); got != "" {
				t.Fatalf("ObjectAddress() = %q, want the zero endpoint", got)
			}
		})
	}
}

// Metadata that has not been populated by an accepted provider result must not
// yield an address either.
func TestObjectMetadataAddressRefusesZeroMetadata(t *testing.T) {
	t.Parallel()

	address, err := gcsobjects.GCSObjectMetadata{}.Address()
	if !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("Address() error = %v, want %v", err, core.ErrObjectStoreContract)
	}
	if got := address.String(); got != "" {
		t.Fatalf("Address() = %q, want the zero endpoint", got)
	}
}

// The address names the object it was derived from and nothing else. This is
// the property a manifest depends on: the recorded address must resolve to the
// exact bucket and name that were published, not to a neighbour.
func TestObjectAddressIdentifiesExactlyOneObject(t *testing.T) {
	t.Parallel()

	bucket, err := gcsobjects.ParseGCSBucket("cleanlift-legal")
	if err != nil {
		t.Fatalf("ParseGCSBucket() error = %v, want nil", err)
	}
	first, err := gcsobjects.ParseGCSObjectName("terms_2026-08-09.pdf")
	if err != nil {
		t.Fatalf("ParseGCSObjectName() error = %v, want nil", err)
	}
	second, err := gcsobjects.ParseGCSObjectName("terms_2026-08-10.pdf")
	if err != nil {
		t.Fatalf("ParseGCSObjectName() error = %v, want nil", err)
	}

	firstAddress, err := gcsobjects.ObjectAddress(bucket, first)
	if err != nil {
		t.Fatalf("ObjectAddress(first) error = %v, want nil", err)
	}
	secondAddress, err := gcsobjects.ObjectAddress(bucket, second)
	if err != nil {
		t.Fatalf("ObjectAddress(second) error = %v, want nil", err)
	}

	if firstAddress.String() == secondAddress.String() {
		t.Fatalf("two object names share one address %q", firstAddress)
	}
	if !strings.HasSuffix(firstAddress.String(), first.String()) {
		t.Fatalf("address %q does not name object %q", firstAddress, first)
	}
	if !firstAddress.SameOrigin(secondAddress) {
		t.Fatalf("SameOrigin() = false, want true for two objects in one bucket")
	}
}
