package cloudbilling

import (
	"errors"
	"strconv"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

// These are open provider names, not application classifications. In particular,
// a resource group does not assert that an SKU is eligible for a product price.
type (
	ResourceName        string
	SKUID               string
	Description         string
	ProviderName        string
	RegionName          string
	ServiceDisplayName  string
	ResourceGroup       string
	UsageType           string
	ResourceFamily      string
	EffectiveTime       string
	UsageUnit           string
	AggregationLevel    string
	AggregationInterval string
	PageToken           string
)

// GeographyType is Google's closed geography taxonomy, including its explicit
// unspecified value. Application eligibility for each arm belongs to callers.
type GeographyType uint8

const (
	GeographyUnspecified GeographyType = iota
	GeographyGlobal
	GeographyRegional
	GeographyMultiRegional
)

func (g GeographyType) Validate() error {
	if g > GeographyMultiRegional {
		return core.ErrCloudBillingResponse
	}
	return nil
}

func (g GeographyType) MarshalJSON() ([]byte, error) {
	var token string
	switch g {
	case GeographyUnspecified:
		token = "TYPE_UNSPECIFIED"
	case GeographyGlobal:
		token = "GLOBAL"
	case GeographyRegional:
		token = "REGIONAL"
	case GeographyMultiRegional:
		token = "MULTI_REGIONAL"
	default:
		return nil, core.ErrCloudBillingResponse
	}
	return core.MarshalCanonicalJSONString(token)
}

func (g *GeographyType) UnmarshalJSON(data []byte) error {
	if g == nil {
		return core.ErrCloudBillingResponse
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrCloudBillingResponse, err)
	}
	var value GeographyType
	switch token {
	case "TYPE_UNSPECIFIED":
		value = GeographyUnspecified
	case "GLOBAL":
		value = GeographyGlobal
	case "REGIONAL":
		value = GeographyRegional
	case "MULTI_REGIONAL":
		value = GeographyMultiRegional
	default:
		return core.ErrCloudBillingResponse
	}
	*g = value
	return nil
}

// Money retains Google's exact signed whole units and fractional billionths.
// It never rounds a provider price into a currency's retail minor units.
type Money struct {
	Currency currency.Code
	Units    int64
	Nanos    int32
}

func (m Money) Validate() error {
	if err := m.Currency.Validate(); err != nil {
		return errors.Join(core.ErrCloudBillingResponse, err)
	}
	if m.Nanos < -999999999 || m.Nanos > 999999999 || m.Units > 0 && m.Nanos < 0 || m.Units < 0 && m.Nanos > 0 {
		return core.ErrCloudBillingResponse
	}
	return nil
}

type moneyWire struct {
	Currency currency.Code `json:"currencyCode"`
	Units    *string       `json:"units"`
	Nanos    int32         `json:"nanos"`
}

func (m Money) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	units := strconv.FormatInt(m.Units, 10)
	return core.MarshalCanonicalJSONDocument(moneyWire{Currency: m.Currency, Units: &units, Nanos: m.Nanos})
}

func (m *Money) UnmarshalJSON(data []byte) error {
	if m == nil {
		return core.ErrCloudBillingResponse
	}
	wire, err := core.DecodeStrictJSONStructure[moneyWire](data, core.ExtensibleJSONLimits())
	if err != nil {
		return errors.Join(core.ErrCloudBillingResponse, err)
	}
	// Protobuf JSON omits default-valued fields. Omitted units are zero.
	units := int64(0)
	if wire.Units != nil {
		units, err = strconv.ParseInt(*wire.Units, 10, 64)
		if err != nil {
			return errors.Join(core.ErrCloudBillingResponse, err)
		}
	}
	value := Money{Currency: wire.Currency, Units: units, Nanos: wire.Nanos}
	if err := value.Validate(); err != nil {
		return err
	}
	*m = value
	return nil
}

type Page struct {
	Next PageToken `json:"nextPageToken"`
	SKUs []SKU     `json:"skus"`
}
type SKU struct {
	Name           ResourceName  `json:"name"`
	ID             SKUID         `json:"skuId"`
	Description    Description   `json:"description"`
	Provider       ProviderName  `json:"serviceProviderName"`
	ServiceRegions []RegionName  `json:"serviceRegions"`
	Category       Category      `json:"category"`
	Geo            Geography     `json:"geoTaxonomy"`
	Pricing        []PricingInfo `json:"pricingInfo"`
}
type Category struct {
	Service ServiceDisplayName `json:"serviceDisplayName"`
	Group   ResourceGroup      `json:"resourceGroup"`
	Usage   UsageType          `json:"usageType"`
	Family  ResourceFamily     `json:"resourceFamily"`
}
type Geography struct {
	Kind    GeographyType `json:"type"`
	Regions []RegionName  `json:"regions"`
}
type PricingInfo struct {
	Effective              EffectiveTime     `json:"effectiveTime"`
	Summary                Description       `json:"summary"`
	Aggregation            Aggregation       `json:"aggregationInfo"`
	Expression             PricingExpression `json:"pricingExpression"`
	CurrencyConversionRate float64           `json:"currencyConversionRate"`
}
type Aggregation struct {
	Level    AggregationLevel    `json:"aggregationLevel"`
	Interval AggregationInterval `json:"aggregationInterval"`
	Count    int64               `json:"aggregationCount"`
}
type PricingExpression struct {
	Unit            UsageUnit   `json:"usageUnit"`
	Base            UsageUnit   `json:"baseUnit"`
	UnitDescription Description `json:"usageUnitDescription"`
	BaseDescription Description `json:"baseUnitDescription"`
	Tiers           []TierRate  `json:"tieredRates"`
	Factor          float64     `json:"baseUnitConversionFactor"`
	DisplayQuantity float64     `json:"displayQuantity"`
}
type TierRate struct {
	Price Money   `json:"unitPrice"`
	Start float64 `json:"startUsageAmount"`
}

// PageLimits is caller policy for one aggregate page, not a transfer default.
type PageLimits struct {
	JSON              core.StrictJSONLimits
	TokenMaximumBytes core.ByteCount
}

func (l PageLimits) Validate() error {
	return errors.Join(l.JSON.Validate(), l.TokenMaximumBytes.Validate())
}

func (p Page) Validate() error {
	if p.SKUs == nil || !utf8.ValidString(string(p.Next)) {
		return core.ErrCloudBillingResponse
	}
	for _, sku := range p.SKUs {
		if err := sku.Geo.Kind.Validate(); err != nil {
			return err
		}
		for _, pricing := range sku.Pricing {
			for _, tier := range pricing.Expression.Tiers {
				if err := tier.Price.Validate(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// DecodePage admits one complete provider document. No original bytes escape;
// ListSKUs retains those bytes in the caller's proof sink before this decoder.
func DecodePage(data []byte, limits PageLimits) (Page, error) {
	if err := limits.Validate(); err != nil {
		return Page{}, errors.Join(core.ErrCloudBillingContract, err)
	}
	page, err := core.DecodeStrictJSONStructure[Page](data, limits.JSON)
	if err != nil {
		return Page{}, errors.Join(core.ErrCloudBillingResponse, err)
	}
	maximum, _ := limits.TokenMaximumBytes.Uint64()
	if uint64(len(page.Next)) > maximum {
		return Page{}, core.ErrCloudBillingResponse
	}
	if err := page.Validate(); err != nil {
		return Page{}, err
	}
	return page, nil
}
