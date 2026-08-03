package lease

import "github.com/deliri/primitive/v2026/core"

const (
	productBugToken       = "4f47532d4255472d323032362d563101"
	productWitnessToken   = "4f47532d5749544e4553532d56310101"
	productPeachfuzzToken = "4f47532d504541434846555a5a2d5601"
	productCatalogEntries = 3
)

type productCatalogEntry struct {
	token    string
	offering core.Offering
}

func productCatalog() [productCatalogEntries]productCatalogEntry {
	return [...]productCatalogEntry{
		{offering: core.OfferingBug, token: productBugToken},
		{offering: core.OfferingWitness, token: productWitnessToken},
		{offering: core.OfferingPeachfuzz, token: productPeachfuzzToken},
	}
}

// ProductForOffering is the sole projection from Primitive's released-product
// identity into its opaque Lease identity. Consumers never copy product bytes.
func ProductForOffering(offering core.Offering) (Product, error) {
	if err := offering.Validate(); err != nil {
		return Product{}, contractError(err)
	}
	for _, entry := range productCatalog() {
		if entry.offering != offering {
			continue
		}
		product, err := ParseProduct(entry.token)
		if err != nil {
			return Product{}, contractError(err)
		}
		return product, nil
	}
	return Product{}, contractError(core.ErrLeaseProduct)
}

// OfferingForProduct is the exact inverse of ProductForOffering over Core's
// closed released-product domain. Both directions read the same fixed catalog,
// so neither an underlying enum-width change nor a copied inverse mapping can
// make the projection drift.
//
// The scan is exhaustive rather than first-match. A product that two offerings
// both project is a catalog contradiction, and the inverse reports it instead
// of returning the lower offering.
func OfferingForProduct(product Product) (core.Offering, error) {
	if err := product.Validate(); err != nil {
		return core.OfferingUnknown, contractError(err)
	}
	found := core.OfferingUnknown
	for _, entry := range productCatalog() {
		candidate, err := ParseProduct(entry.token)
		if err != nil {
			return core.OfferingUnknown, contractError(err)
		}
		if candidate != product {
			continue
		}
		if found != core.OfferingUnknown {
			return core.OfferingUnknown, contractError(core.ErrLeaseProduct)
		}
		found = entry.offering
	}
	if found == core.OfferingUnknown {
		return core.OfferingUnknown, contractError(core.ErrLeaseProduct)
	}
	return found, nil
}
