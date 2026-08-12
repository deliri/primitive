package timeproof

import (
	"encoding/json"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// RequestMaximumBytes bounds one DER TimeStampReq.
	RequestMaximumBytes = 1024
	// ResponseMaximumBytes bounds one DER TimeStampResp and CMS token.
	ResponseMaximumBytes = 128 * 1024
	// NonceBytes is the fixed request-nonce width.
	NonceBytes = 16
	// SerialMaximumBits is RFC 5280's certificate-serial ceiling.
	SerialMaximumBits = 160
)

// Authority is a closed timestamp-authority identity.
type Authority uint8

const (
	// AuthorityUnknown is the invalid zero authority.
	AuthorityUnknown Authority = iota
	// AuthorityFreeTSA identifies the reviewed FreeTSA contract.
	AuthorityFreeTSA
	// AuthorityDigiCert identifies the reviewed DigiCert RFC 3161 contract.
	AuthorityDigiCert
	authorityLimit
)

// Validate rejects authorities outside the closed enum. Every admitted member
// must also carry a registry contract; authorityRegistry owns that gate.
func (a Authority) Validate() error {
	if a <= AuthorityUnknown || a >= authorityLimit || authorityTokens()[a] == "" {
		return contractError(nil)
	}
	return nil
}

// IsValid reports whether a is registered.
func (a Authority) IsValid() bool { return a.Validate() == nil }

// String returns the canonical persisted token.
func (a Authority) String() string {
	if a >= authorityLimit {
		return ""
	}
	return authorityTokens()[a]
}

func authorityTokens() [authorityLimit]string {
	return [...]string{"", "freetsa", "digicert"}
}

// Endpoint returns the authority's reviewed RFC 3161 transport target.
func (a Authority) Endpoint() (core.HTTPEndpoint, error) {
	contract, err := authorityRegistry(a)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	return contract.endpoint, nil
}

// WireEnum declares that Authority crosses persistence boundaries.
func (Authority) WireEnum() {}

// MarshalJSON emits the canonical authority token.
func (a Authority) MarshalJSON() ([]byte, error) {
	return marshalEnum(a)
}

// UnmarshalJSON accepts one canonical authority token.
func (a *Authority) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(data, a, parseAuthority)
}

// parseAuthority projects one token through the closed enum's own forward
// mapping, so no second token table can drift from String.
func parseAuthority(token string) (Authority, error) {
	for authority := AuthorityUnknown + 1; authority < authorityLimit; authority++ {
		if token != "" && token == authority.String() {
			return authority, nil
		}
	}
	return AuthorityUnknown, contractError(nil)
}

// TimestampPolicy is a closed authority policy identity.
type TimestampPolicy uint8

const (
	// TimestampPolicyUnknown is the invalid zero policy.
	TimestampPolicyUnknown TimestampPolicy = iota
	// TimestampPolicyFreeTSA identifies FreeTSA's reviewed policy OID.
	TimestampPolicyFreeTSA
	// TimestampPolicyDigiCert identifies DigiCert's reviewed policy OID.
	TimestampPolicyDigiCert
	timestampPolicyLimit
)

// Validate rejects policy identities outside the closed enum.
func (p TimestampPolicy) Validate() error {
	if p <= TimestampPolicyUnknown || p >= timestampPolicyLimit ||
		timestampPolicyTokens()[p] == "" {
		return contractError(nil)
	}
	return nil
}

// IsValid reports whether p is registered.
func (p TimestampPolicy) IsValid() bool { return p.Validate() == nil }

// String returns the canonical OID token projected from the one ASN.1 policy
// identity the package owns.
func (p TimestampPolicy) String() string {
	if p >= timestampPolicyLimit {
		return ""
	}
	return timestampPolicyTokens()[p]
}

func timestampPolicyTokens() [timestampPolicyLimit]string {
	return [...]string{"", freeTSAPolicyOID.String(), digiCertPolicyOID.String()}
}

// WireEnum declares that TimestampPolicy crosses persistence boundaries.
func (TimestampPolicy) WireEnum() {}

// MarshalJSON emits the canonical policy OID.
func (p TimestampPolicy) MarshalJSON() ([]byte, error) {
	return marshalEnum(p)
}

// UnmarshalJSON accepts one canonical policy OID.
func (p *TimestampPolicy) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(data, p, parseTimestampPolicy)
}

// parseTimestampPolicy projects one token through the closed enum's own
// forward mapping, so no second token table can drift from String.
func parseTimestampPolicy(token string) (TimestampPolicy, error) {
	for policy := TimestampPolicyUnknown + 1; policy < timestampPolicyLimit; policy++ {
		if token != "" && token == policy.String() {
			return policy, nil
		}
	}
	return TimestampPolicyUnknown, contractError(nil)
}

var (
	_ json.Unmarshaler = (*Authority)(nil)
	_ json.Unmarshaler = (*TimestampPolicy)(nil)
)
