package googleidentity

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	GoogleCloudIdentityHeaderMaximumBytes = 4 << 10
	googleCloudAlgorithmRS256Text         = "RS256"
	googleCloudTokenTypeJWT               = "JWT"
)

type googleCloudSigningAlgorithm uint8

const (
	googleCloudSigningAlgorithmUnknown googleCloudSigningAlgorithm = iota
	googleCloudSigningAlgorithmRS256
)

func (a googleCloudSigningAlgorithm) Validate() error {
	if a != googleCloudSigningAlgorithmRS256 {
		return core.ErrGoogleIdentityContract
	}
	return nil
}

func (a googleCloudSigningAlgorithm) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(googleCloudAlgorithmRS256Text)
}

func (a *googleCloudSigningAlgorithm) UnmarshalJSON(data []byte) error {
	if a == nil {
		return errors.Join(core.ErrJSONContract, core.ErrGoogleIdentityContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil || value != googleCloudAlgorithmRS256Text {
		return errors.Join(core.ErrJSONContract, core.ErrGoogleIdentityContract, err)
	}
	*a = googleCloudSigningAlgorithmRS256
	return nil
}

// This is the owned service-account JOSE header. IAP's signing authority and
// alternative algorithms are separate protocols and never reach this SDK call.
type googleCloudJWTHeader struct {
	Algorithm googleCloudSigningAlgorithm `json:"alg"`
	KeyID     string                      `json:"kid"`
	Type      string                      `json:"typ,omitempty"`
}

func (h googleCloudJWTHeader) Validate() error {
	if !validGoogleCloudIdentityText(h.KeyID) || (h.Type != "" && h.Type != googleCloudTokenTypeJWT) {
		return core.ErrGoogleIdentityContract
	}
	return h.Algorithm.Validate()
}

func validateGoogleCloudJWTHeader(token string) error {
	segment, _, found := strings.Cut(token, ".")
	if !found || base64.RawURLEncoding.DecodedLen(len(segment)) > GoogleCloudIdentityHeaderMaximumBytes {
		return core.ErrGoogleIdentityContract
	}
	encoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return contractError(err)
	}
	header, err := core.DecodeStrictJSONStructure[googleCloudJWTHeader](encoded, core.DefaultStrictJSONLimits())
	if err != nil {
		return contractError(err)
	}
	return header.Validate()
}
