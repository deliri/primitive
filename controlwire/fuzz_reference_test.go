package controlwire

import (
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"math/big"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// These reference projections use Go's decoder and arithmetic, independently
// of Controlwire's parsers. Helpers return facts; the callbacks own verdicts.
type cursorReference struct {
	Revision   string `json:"revision"`
	Activation uint64 `json:"activation"`
}
type replayReference struct {
	Offering   core.Offering `json:"offering"`
	Family     string        `json:"route_family"`
	Revision   string        `json:"revision"`
	Nonce      string        `json:"request_nonce"`
	Commitment string        `json:"request_commitment"`
}

func canonicalDigestReference(text string, nonzero bool) bool {
	if len(text) != 2*core.SHA256DigestBytes {
		return false
	}
	raw, err := hex.DecodeString(text)
	if err != nil || hex.EncodeToString(raw) != text {
		return false
	}
	if !nonzero {
		return true
	}
	for _, b := range raw {
		if b != 0 {
			return true
		}
	}
	return false
}

func policyIDReference(text string) (PolicyRevisionID, bool) {
	var got PolicyRevisionID
	if len(text) != PolicyRevisionTextLength {
		return got, false
	}
	value := new(big.Int)
	for _, symbol := range text {
		digit := strings.IndexRune(policyRevisionAlphabet, symbol)
		if digit < 0 {
			return got, false
		}
		value.Lsh(value, policyRevisionSymbolBits)
		value.Add(value, big.NewInt(int64(digit)))
	}
	if value.Sign() == 0 || value.BitLen() > len(got)*8 {
		return got, false
	}
	value.FillBytes(got[:])
	return got, true
}

func controlwireTextReferenceAccepts(door controlwireTextDoor, text string) bool {
	switch door {
	case controlwireTextDoorRequestNonce, controlwireTextDoorAuthorityNonce, controlwireTextDoorRegistrationToken, controlwireTextDoorRegistrationTokenVerifier:
		return canonicalDigestReference(text, true)
	case controlwireTextDoorRevision:
		return text == Revision2026V1Token
	case controlwireTextDoorPolicyRevisionID:
		_, ok := policyIDReference(text)
		return ok
	case controlwireTextDoorRouteFamily:
		for family := RouteFamilyUnknown + 1; family < routeFamilyLimit; family++ {
			if text == routeFamilyTokens()[family] {
				return true
			}
		}
	}
	return false
}

func controlwireJSONReferenceAccepts(door controlwireJSONDoor, data []byte) bool {
	if len(data) > core.JSONDocumentMaximumBytes {
		return false
	}
	switch door {
	case controlwireJSONDoorPolicyCursor:
		var value cursorReference
		if err := json.Unmarshal(data, &value, json.RejectUnknownMembers(true)); err != nil {
			return false
		}
		_, valid := policyIDReference(value.Revision)
		return valid && value.Activation != 0
	case controlwireJSONDoorReplayIdentity:
		if len(data) > ReplayIdentityJSONMaximumBytes {
			return false
		}
		var value replayReference
		if err := json.Unmarshal(data, &value, json.RejectUnknownMembers(true)); err != nil {
			return false
		}
		return value.Offering.Validate() == nil && controlwireTextReferenceAccepts(controlwireTextDoorRouteFamily, value.Family) && value.Revision == Revision2026V1Token && canonicalDigestReference(value.Nonce, true) && canonicalDigestReference(value.Commitment, false)
	default:
		var text string
		if jsontext.Value(data).Kind() != jsontext.KindString || json.Unmarshal(data, &text) != nil {
			return false
		}
		switch door {
		case controlwireJSONDoorRequestNonce, controlwireJSONDoorAuthorityNonce, controlwireJSONDoorRegistrationToken, controlwireJSONDoorRegistrationTokenVerifier:
			return canonicalDigestReference(text, true)
		case controlwireJSONDoorRequestCommitment:
			return canonicalDigestReference(text, false)
		case controlwireJSONDoorRevision:
			return text == Revision2026V1Token
		case controlwireJSONDoorPolicyRevisionID:
			_, ok := policyIDReference(text)
			return ok
		case controlwireJSONDoorRouteFamily:
			return controlwireTextReferenceAccepts(controlwireTextDoorRouteFamily, text)
		}
	}
	return false
}
