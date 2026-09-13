package accesspermit

import "github.com/deliri/primitive/v2026/core"

type Domain uint8

const DomainV1 Domain = 1
const DomainV1Token = "primitive-access-permit-2026-1"

func (d Domain) IsValid() bool { return d == DomainV1 }
func (d Domain) Validate() error {
	if !d.IsValid() {
		return core.ErrAccessPermitContract
	}
	return nil
}
func (d Domain) String() string {
	if d.IsValid() {
		return DomainV1Token
	}
	return core.UnknownEnumDiagnostic
}
func (Domain) OffWireEnum() {}
func (d Domain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(DomainV1Token), nil
}
func (Domain) ParseCanonicalText(data []byte) (Domain, error) {
	if string(data) != DomainV1Token {
		return 0, core.ErrAccessPermitContract
	}
	return DomainV1, nil
}
