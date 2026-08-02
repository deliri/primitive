package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	manifestDomainToken = "primitive-release-manifest-v1"
	latestDomainToken   = "primitive-release-latest-v1"
	currentDiagnostic   = "current"
)

// Domain is the closed Attest domain for Release documents.
type Domain uint8

const (
	DomainUnknown Domain = iota
	DomainManifestV1
	DomainLatestV1
	domainLimit
)

func (d Domain) Validate() error {
	if d <= DomainUnknown || d >= domainLimit || domainTokens()[d] == "" {
		return contractError(errors.New("signing domain is outside the closed domain"))
	}
	return nil
}

func (d Domain) IsValid() bool { return d.Validate() == nil }
func (Domain) OffWireEnum()    {}

func (d Domain) String() string {
	if d >= domainLimit || domainTokens()[d] == "" {
		return core.UnknownEnumDiagnostic
	}
	return domainTokens()[d]
}

func domainTokens() [domainLimit]string {
	return [...]string{"", manifestDomainToken, latestDomainToken}
}

func (d Domain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

func (Domain) ParseCanonicalText(text []byte) (Domain, error) {
	switch string(text) {
	case manifestDomainToken:
		return DomainManifestV1, nil
	case latestDomainToken:
		return DomainLatestV1, nil
	default:
		return DomainUnknown, contractError(errors.New("signing domain text is unsupported"))
	}
}

var (
	_ core.Validatable = DomainUnknown
	_ core.OffWireEnum = DomainUnknown
)
