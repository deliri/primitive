package lease

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const enumJSONWhitespaceAllowance = 256

type enumFact struct {
	token string
}

func marshalEnum(token string, maximum int) ([]byte, error) {
	encoded, err := json.Marshal(token)
	if err != nil {
		return nil, jsonError(err)
	}
	if len(encoded) > maximum {
		return nil, jsonError(errors.New(enumJSONExtentInvalidText))
	}
	return encoded, nil
}

func decodeEnum(data []byte, maximum int) (string, error) {
	if len(data) == 0 || len(data) > maximum {
		return "", jsonError(errors.New(enumJSONExtentInvalidText))
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return "", jsonError(err)
	}
	return token, nil
}

// Revision is the closed lease wire revision.
type Revision uint8

const (
	RevisionUnknown Revision = iota
	// RevisionV1 is the first exact lease decision contract.
	RevisionV1
	revisionLimit
)

const (
	revisionV1Token           = "v1"
	revisionUnknownDiagnostic = "unknown"
	// RevisionCanonicalJSONMaximumBytes is the exact compact revision maximum.
	RevisionCanonicalJSONMaximumBytes = len(`"` + revisionV1Token + `"`)
	// RevisionJSONMaximumBytes bounds accepted revision JSON.
	RevisionJSONMaximumBytes = RevisionCanonicalJSONMaximumBytes +
		enumJSONWhitespaceAllowance
)

func revisionFacts() [revisionLimit]enumFact {
	return [...]enumFact{
		RevisionV1: {token: revisionV1Token},
	}
}

func (r Revision) fact() (enumFact, error) {
	if r <= RevisionUnknown || r >= revisionLimit {
		return enumFact{}, contractError(errors.New("lease revision is outside the closed domain"))
	}
	fact := revisionFacts()[r]
	if fact.token == "" {
		return enumFact{}, contractError(errors.New("lease revision has no contract"))
	}
	return fact, nil
}

func (r Revision) Validate() error { _, err := r.fact(); return err }
func (r Revision) IsValid() bool   { return r.Validate() == nil }
func (r Revision) String() string {
	fact, err := r.fact()
	if err != nil {
		return revisionUnknownDiagnostic
	}
	return fact.token
}

// ParseRevision parses one exact revision token.
func ParseRevision(token string) (Revision, error) {
	for value := RevisionUnknown + 1; value < revisionLimit; value++ {
		if value.String() == token {
			return value, nil
		}
	}
	return RevisionUnknown, contractError(errors.New("lease revision token is unsupported"))
}

func (r Revision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return marshalEnum(r.String(), RevisionCanonicalJSONMaximumBytes)
}

func (r *Revision) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("revision receiver is nil"))
	}
	token, err := decodeEnum(data, RevisionJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := ParseRevision(token)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

// Outcome is the signed decision variant.
type Outcome uint8

const (
	OutcomeUnknown Outcome = iota
	// OutcomeGrant carries a usable timeline.
	OutcomeGrant
	// OutcomeRefusal carries a recoverable denial.
	OutcomeRefusal
	// OutcomeRevocation carries a for-cause denial.
	OutcomeRevocation
	outcomeLimit
)

const (
	outcomeGrantToken                = "grant"
	outcomeRefusalToken              = "refusal"
	outcomeRevocationToken           = "revocation"
	outcomeUnknownDiagnostic         = "unknown"
	OutcomeCanonicalJSONMaximumBytes = len(`"` + outcomeRevocationToken + `"`)
	OutcomeJSONMaximumBytes          = OutcomeCanonicalJSONMaximumBytes +
		enumJSONWhitespaceAllowance
)

func outcomeFacts() [outcomeLimit]enumFact {
	return [...]enumFact{
		OutcomeGrant:      {token: outcomeGrantToken},
		OutcomeRefusal:    {token: outcomeRefusalToken},
		OutcomeRevocation: {token: outcomeRevocationToken},
	}
}

func (o Outcome) fact() (enumFact, error) {
	if o <= OutcomeUnknown || o >= outcomeLimit {
		return enumFact{}, contractError(errors.New("lease outcome is outside the closed domain"))
	}
	fact := outcomeFacts()[o]
	if fact.token == "" {
		return enumFact{}, contractError(errors.New("lease outcome has no contract"))
	}
	return fact, nil
}

func (o Outcome) Validate() error { _, err := o.fact(); return err }
func (o Outcome) IsValid() bool   { return o.Validate() == nil }
func (o Outcome) String() string {
	fact, err := o.fact()
	if err != nil {
		return outcomeUnknownDiagnostic
	}
	return fact.token
}

// ParseOutcome parses one exact outcome token.
func ParseOutcome(token string) (Outcome, error) {
	for value := OutcomeUnknown + 1; value < outcomeLimit; value++ {
		if value.String() == token {
			return value, nil
		}
	}
	return OutcomeUnknown, contractError(errors.New("lease outcome token is unsupported"))
}

func (o Outcome) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return marshalEnum(o.String(), OutcomeCanonicalJSONMaximumBytes)
}

func (o *Outcome) UnmarshalJSON(data []byte) error {
	if o == nil {
		return jsonError(errors.New("outcome receiver is nil"))
	}
	token, err := decodeEnum(data, OutcomeJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := ParseOutcome(token)
	if err != nil {
		return err
	}
	*o = parsed
	return nil
}

// RevocationReason is one contract-authorized for-cause ground.
type RevocationReason uint8

const (
	RevocationReasonUnknown RevocationReason = iota
	RevocationReasonLicenceBreach
	RevocationReasonUnlawfulOrAbusiveUse
	RevocationReasonSecurityOrPlatformRisk
	RevocationReasonInsolvency
	revocationReasonLimit
)

const (
	revocationLicenceBreachToken              = "licence-breach"
	revocationUnlawfulOrAbusiveUseToken       = "unlawful-or-abusive-use"
	revocationSecurityOrPlatformRiskToken     = "security-or-platform-risk"
	revocationInsolvencyToken                 = "insolvency"
	revocationReasonUnknownDiagnostic         = "unknown"
	RevocationReasonCanonicalJSONMaximumBytes = len(`"` + revocationSecurityOrPlatformRiskToken + `"`)
	RevocationReasonJSONMaximumBytes          = RevocationReasonCanonicalJSONMaximumBytes +
		enumJSONWhitespaceAllowance
)

func revocationReasonFacts() [revocationReasonLimit]enumFact {
	return [...]enumFact{
		RevocationReasonLicenceBreach:          {token: revocationLicenceBreachToken},
		RevocationReasonUnlawfulOrAbusiveUse:   {token: revocationUnlawfulOrAbusiveUseToken},
		RevocationReasonSecurityOrPlatformRisk: {token: revocationSecurityOrPlatformRiskToken},
		RevocationReasonInsolvency:             {token: revocationInsolvencyToken},
	}
}

func (r RevocationReason) fact() (enumFact, error) {
	if r <= RevocationReasonUnknown || r >= revocationReasonLimit {
		return enumFact{}, contractError(errors.New("revocation reason is outside the closed domain"))
	}
	fact := revocationReasonFacts()[r]
	if fact.token == "" {
		return enumFact{}, contractError(errors.New("revocation reason has no contract"))
	}
	return fact, nil
}

func (r RevocationReason) Validate() error { _, err := r.fact(); return err }
func (r RevocationReason) IsValid() bool   { return r.Validate() == nil }
func (r RevocationReason) String() string {
	fact, err := r.fact()
	if err != nil {
		return revocationReasonUnknownDiagnostic
	}
	return fact.token
}

// ParseRevocationReason parses one exact revocation token.
func ParseRevocationReason(token string) (RevocationReason, error) {
	for value := RevocationReasonUnknown + 1; value < revocationReasonLimit; value++ {
		if value.String() == token {
			return value, nil
		}
	}
	return RevocationReasonUnknown, contractError(errors.New("revocation reason token is unsupported"))
}

func (r RevocationReason) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return marshalEnum(r.String(), RevocationReasonCanonicalJSONMaximumBytes)
}

func (r *RevocationReason) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("revocation reason receiver is nil"))
	}
	token, err := decodeEnum(data, RevocationReasonJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := ParseRevocationReason(token)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}
