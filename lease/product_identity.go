package lease

import "github.com/deliri/primitive/v2026/core"

const (
	// ProductBugToken is Bug's stable opaque Lease product identity.
	ProductBugToken = "4f47532d4255472d323032362d563101"
	// ProductWitnessToken is Witness's stable opaque Lease product identity.
	ProductWitnessToken = "4f47532d5749544e4553532d56310101"
	// ProductPeachfuzzToken is Peachfuzz's stable opaque Lease product identity.
	ProductPeachfuzzToken = "4f47532d504541434846555a5a2d5601"
)

// ProductForOffering is the sole projection from Primitive's released-product
// identity into its opaque Lease identity. Consumers never copy product bytes.
func ProductForOffering(offering core.Offering) (Product, error) {
	if err := offering.Validate(); err != nil {
		return Product{}, contractError(err)
	}
	var token string
	switch offering {
	case core.OfferingBug:
		token = ProductBugToken
	case core.OfferingWitness:
		token = ProductWitnessToken
	case core.OfferingPeachfuzz:
		token = ProductPeachfuzzToken
	default:
		return Product{}, contractError(core.ErrPrimitiveContract)
	}
	product, err := ParseProduct(token)
	if err != nil {
		return Product{}, contractError(err)
	}
	return product, nil
}
