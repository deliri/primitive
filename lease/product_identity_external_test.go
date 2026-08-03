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
		want     string
		offering core.Offering
	}{
		{name: "Bug preserves its published Lease identity", offering: core.OfferingBug, want: lease.ProductBugToken},
		{name: "Witness preserves its published Lease identity", offering: core.OfferingWitness, want: lease.ProductWitnessToken},
		{name: "Peachfuzz receives its published Lease identity", offering: core.OfferingPeachfuzz, want: lease.ProductPeachfuzzToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := lease.ProductForOffering(tc.offering)
			if gotErr != nil {
				t.Fatalf("ProductForOffering(%d) error = %v, want nil", tc.offering, gotErr)
			}
			if got.String() != tc.want {
				t.Fatalf("ProductForOffering(%d) = %q, want %q", tc.offering, got.String(), tc.want)
			}
			parsed, parseErr := lease.ParseProduct(tc.want)
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
