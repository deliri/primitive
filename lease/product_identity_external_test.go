package lease_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func TestProductForOfferingProjectsEveryPublishedIdentityExactly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		offering core.Offering
	}{
		{name: "Bug preserves its published Lease identity", offering: core.OfferingBug},
		{name: "Witness preserves its published Lease identity", offering: core.OfferingWitness},
		{name: "Peachfuzz receives its published Lease identity", offering: core.OfferingPeachfuzz},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := lease.ProductForOffering(tc.offering)
			if gotErr != nil {
				t.Fatalf("ProductForOffering(%d) error = %v, want nil", tc.offering, gotErr)
			}
			parsed, parseErr := lease.ParseProduct(got.String())
			if parseErr != nil || parsed != got {
				t.Fatalf("ParseProduct(ProductForOffering(%d)) = (%v, %v), want (%v, nil)", tc.offering, parsed, parseErr, got)
			}
		})
	}
}

func TestProductForOfferingRejectsCompleteUnpublishedByteDomain(t *testing.T) {
	t.Parallel()

	for value := range 256 {
		offering := core.Offering(value)
		published := offering == core.OfferingBug || offering == core.OfferingWitness || offering == core.OfferingPeachfuzz
		got, gotErr := lease.ProductForOffering(offering)
		if published {
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("ProductForOffering(%d) = (%v, %v), want valid product and nil", value, got, gotErr)
			}
			continue
		}
		if !errors.Is(gotErr, core.ErrLeaseContract) {
			t.Fatalf("ProductForOffering(%d) error = %v, want %v", value, gotErr, core.ErrLeaseContract)
		}
		if got != (lease.Product{}) {
			t.Fatalf("ProductForOffering(%d) rejected product = %v, want zero value", value, got)
		}
	}
}

func TestOfferingLeaseProductIdentitiesCannotCollide(t *testing.T) {
	t.Parallel()

	products := make([]lease.Product, 0, 3)
	for _, offering := range []core.Offering{core.OfferingBug, core.OfferingWitness, core.OfferingPeachfuzz} {
		product, err := lease.ProductForOffering(offering)
		if err != nil {
			t.Fatalf("ProductForOffering(%d) error = %v, want nil", offering, err)
		}
		for _, prior := range products {
			if product == prior {
				t.Fatalf("ProductForOffering(%d) = %v, want identity distinct from %v", offering, product, prior)
			}
		}
		products = append(products, product)
	}
}

func TestOfferingForProductDerivesTheExactInverseWithoutASecondCatalog(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		offering core.Offering
	}{
		{name: "Bug round trips through its opaque product", offering: core.OfferingBug},
		{name: "Witness round trips through its opaque product", offering: core.OfferingWitness},
		{name: "Peachfuzz round trips through its opaque product", offering: core.OfferingPeachfuzz},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			product, err := lease.ProductForOffering(tc.offering)
			if err != nil {
				t.Fatalf("ProductForOffering(%d) error = %v, want nil", tc.offering, err)
			}
			got, gotErr := lease.OfferingForProduct(product)
			if gotErr != nil || got != tc.offering {
				t.Fatalf("OfferingForProduct(ProductForOffering(%d)) = (%d, %v), want (%d, nil)", tc.offering, got, gotErr, tc.offering)
			}
		})
	}
}

func TestOfferingForProductRejectsUnsetProduct(t *testing.T) {
	t.Parallel()

	got, gotErr := lease.OfferingForProduct(lease.Product{})
	if got != core.OfferingUnknown || !errors.Is(gotErr, core.ErrLeaseContract) {
		t.Fatalf("OfferingForProduct(zero) = (%d, %v), want (%d, %v)", got, gotErr, core.OfferingUnknown, core.ErrLeaseContract)
	}
}

func TestOfferingForProductRejectsUnpublishedProducts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		product lease.Product
	}{
		{name: "unpublished minimum identity", product: productIdentityFixture(t, 0x01)},
		{name: "unpublished midpoint identity", product: productIdentityFixture(t, 0x80)},
		{name: "unpublished maximum identity", product: productIdentityFixture(t, 0xff)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := lease.OfferingForProduct(tc.product)
			if got != core.OfferingUnknown || !errors.Is(gotErr, core.ErrLeaseProduct) {
				t.Fatalf("OfferingForProduct(%v) = (%d, %v), want (%d, %v)", tc.product, got, gotErr, core.OfferingUnknown, core.ErrLeaseProduct)
			}
		})
	}
}

// TestOfferingForProductRejectsEveryNearMissOfAPublishedIdentity attacks the
// comparison itself. Uniform-byte fixtures cannot catch a prefix, suffix, or
// truncated compare; a published token with exactly one nibble changed can.
// Every mutation is one hex digit away from a real product identity.
func TestOfferingForProductRejectsEveryNearMissOfAPublishedIdentity(t *testing.T) {
	t.Parallel()

	positions := []struct {
		name  string
		index int
	}{
		{name: "first nibble", index: 0},
		{name: "second nibble", index: 1},
		{name: "midpoint nibble", index: lease.IdentifierHexBytes / 2},
		{name: "penultimate nibble", index: lease.IdentifierHexBytes - 2},
		{name: "final nibble", index: lease.IdentifierHexBytes - 1},
	}
	offerings := []struct {
		name     string
		offering core.Offering
	}{
		{name: "Bug", offering: core.OfferingBug},
		{name: "Witness", offering: core.OfferingWitness},
		{name: "Peachfuzz", offering: core.OfferingPeachfuzz},
	}
	for _, published := range offerings {
		publishedProduct, err := lease.ProductForOffering(published.offering)
		if err != nil {
			t.Fatalf("ProductForOffering(%d) error = %v, want nil", published.offering, err)
		}
		for _, position := range positions {
			name := published.name + " with a changed " + position.name
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				mutated := mutatedProductToken(t, publishedProduct.String(), position.index)
				product, err := lease.ParseProduct(mutated)
				if err != nil {
					t.Fatalf("ParseProduct(%q) error = %v, want nil", mutated, err)
				}
				got, gotErr := lease.OfferingForProduct(product)
				if got != core.OfferingUnknown || !errors.Is(gotErr, core.ErrLeaseProduct) {
					t.Fatalf("OfferingForProduct(%q) = (%d, %v), want (%d, %v)",
						mutated, got, gotErr, core.OfferingUnknown, core.ErrLeaseProduct)
				}
			})
		}
	}
}

// TestProductIdentityProjectionIsTotalAndInjectiveOverTheOfferingDomain sweeps
// the complete compiler-derived offering domain in both directions: every value
// that projects a product inverts to exactly itself, no two values share a
// product, and no value outside the published set reaches an identity. It is
// the ratchet that catches a forward catalog and an inverse scan drifting apart
// when a fourth offering lands.
func TestProductIdentityProjectionIsTotalAndInjectiveOverTheOfferingDomain(t *testing.T) {
	t.Parallel()

	owners := make(map[lease.Product]core.Offering)
	projected := 0
	published := 0
	domainLimit := uint64(^core.Offering(0))
	for value := uint64(0); value <= domainLimit; value++ {
		offering := core.Offering(value)
		product, err := lease.ProductForOffering(offering)
		if offering.Validate() != nil {
			if !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("ProductForOffering(%d) error = %v, want %v", value, err, core.ErrLeaseContract)
			}
			if product != (lease.Product{}) {
				t.Fatalf("ProductForOffering(%d) rejected product = %v, want zero value", value, product)
			}
			continue
		}
		published++
		if err != nil {
			t.Fatalf("ProductForOffering(published %d) error = %v, want nil", value, err)
		}
		projected++
		if prior, found := owners[product]; found {
			t.Fatalf("ProductForOffering(%d) = %v, want an identity offering %d does not already own",
				value, product, prior)
		}
		owners[product] = offering
		got, gotErr := lease.OfferingForProduct(product)
		if gotErr != nil || got != offering {
			t.Fatalf("OfferingForProduct(ProductForOffering(%d)) = (%d, %v), want (%d, nil)",
				value, got, gotErr, value)
		}
	}
	if projected != len(owners) {
		t.Fatalf("projected offerings = %d, distinct identities = %d, want equal counts", projected, len(owners))
	}
	if projected != published {
		t.Fatalf("projected offerings = %d, Core-published offerings = %d, want equal counts", projected, published)
	}
}

func mutatedProductToken(t testing.TB, token string, index int) string {
	t.Helper()

	if len(token) != lease.IdentifierHexBytes || index < 0 || index >= len(token) {
		t.Fatalf("mutation target = (%q, %d), want a hex token of %d digits and an in-range index",
			token, index, lease.IdentifierHexBytes)
	}
	digits := []byte(token)
	if digits[index] == '0' {
		digits[index] = '1'
		return string(digits)
	}
	digits[index] = '0'
	return string(digits)
}

func productIdentityFixture(t testing.TB, marker byte) lease.Product {
	t.Helper()

	var value [lease.IdentifierBytes]byte
	for index := range value {
		value[index] = marker
	}
	product, err := lease.NewProduct(value)
	if err != nil {
		t.Fatalf("NewProduct(%x) error = %v, want nil", value, err)
	}
	return product
}
