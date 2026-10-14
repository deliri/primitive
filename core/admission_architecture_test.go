package core

import (
	"errors"
	"slices"
	"testing"
)

func TestAdmissionErrorIdentitiesKeepCapacitySeparateFromContract(t *testing.T) {
	t.Parallel()
	for _, identity := range []ErrorIdentity{ErrAdmissionContract, ErrAdmissionCapacity} {
		if err := identity.Validate(); err != nil {
			t.Fatalf("identity %v validation = %v, want nil", identity, err)
		}
		if !errors.Is(identity, identity) || !errors.Is(identity, ErrPrimitiveContract) {
			t.Fatalf("identity %v lost its exact or Primitive identity", identity)
		}
	}
	if errors.Is(ErrAdmissionCapacity, ErrAdmissionContract) || errors.Is(ErrAdmissionContract, ErrAdmissionCapacity) {
		t.Fatal("capacity and invalid contract match, want separate admission identities")
	}
}

// Multipart framing belongs to Exchange and its AI consumer. Requiring both
// actual named consumers preserves a stronger contract than incorrectly
// classifying this standard MIME value as an AI-only provider convention.
func TestMultipartMediaTypeRetainsBothOwningConsumers(t *testing.T) {
	t.Parallel()
	exports, err := collectCoreTopLevelExports(".")
	if err != nil {
		t.Fatalf("collect exports = %v, want nil", err)
	}
	if err := collectCoreExportConsumers("..", &exports); err != nil {
		t.Fatalf("collect consumers = %v, want nil", err)
	}
	contract, ok := exports.Lookup("AIMultipartMediaType")
	if !ok {
		t.Fatal("multipart MIME export absent, want shared owning contract")
	}
	got := contract.ConsumerIdentities()
	want := []PackageIdentity{PackageExchange, PackageAI}
	if !slices.Equal(got, want) {
		t.Fatalf("multipart MIME consumers = %v, want %v", got, want)
	}
}
