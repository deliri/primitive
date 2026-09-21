package distributionauth

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/permit"
)

// UpgradeDeliveryProjection emits both the download capability and the exact
// existing-device permission transfer. Neither component substitutes for the
// other's independent verification at the client.
type UpgradeDeliveryProjection struct {
	Download distribution.UpgradeGrantProjection `json:"download"`
	Transfer permit.BuildTransfer                `json:"transfer"`
}

func (p UpgradeDeliveryProjection) Validate() error {
	return errors.Join(p.Download.Validate(), p.Transfer.Validate())
}

func (p UpgradeDeliveryProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	type wire UpgradeDeliveryProjection
	return core.MarshalCanonicalJSONDocument(wire(p))
}

// ValidateJSONProjection proves the outbound bearer through its receiving
// agreement; the issuing projection deliberately cannot decode capabilities.
func (p UpgradeDeliveryProjection) ValidateJSONProjection(encoded []byte, limits core.StrictJSONLimits) error {
	return core.ValidateReceiveOnlyJSONProjection[UpgradeDeliveryProjection, UpgradeDeliveryDocument, *UpgradeDeliveryDocument](p, encoded, limits)
}

// UpgradeDeliveryDocument is the receiving half of UpgradeDeliveryProjection.
// Structural admission does not authorize a download or execution.
type UpgradeDeliveryDocument struct {
	Download distribution.UpgradeGrantDocument `json:"download"`
	Transfer permit.BuildTransfer              `json:"transfer"`
}

func (d UpgradeDeliveryDocument) Validate() error {
	return errors.Join(d.Download.Validate(), d.Transfer.Validate())
}

func (d *UpgradeDeliveryDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return core.ErrDistributionContract
	}
	type wire UpgradeDeliveryDocument
	value, err := core.DecodeStrictJSONStructure[wire](data, core.ExtensibleJSONLimits())
	if err != nil {
		return errors.Join(core.ErrDistributionContract, err)
	}
	got := UpgradeDeliveryDocument(value)
	if err := got.Validate(); err != nil {
		return errors.Join(core.ErrDistributionContract, err)
	}
	*d = got
	return nil
}
