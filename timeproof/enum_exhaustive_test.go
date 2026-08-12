package timeproof

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAuthorityAndTimestampPolicyExhaustBackingDomains(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		authority := Authority(raw)
		wantAuthority := authority == AuthorityFreeTSA || authority == AuthorityDigiCert
		gotAuthorityErr := authority.Validate()
		if authority.IsValid() != wantAuthority || (gotAuthorityErr == nil) != wantAuthority ||
			(authority.String() != "") != wantAuthority {
			t.Fatalf(
				"Authority(%d) validity/string = (%t, %v, %q), want (%t, matching error state, matching token state)",
				raw, authority.IsValid(), gotAuthorityErr, authority.String(), wantAuthority,
			)
		}
		if !wantAuthority {
			if !errors.Is(gotAuthorityErr, core.ErrTimeProofContract) {
				t.Fatalf("Authority(%d).Validate() error = %v, want %v", raw, gotAuthorityErr, core.ErrTimeProofContract)
			}
		} else {
			proveAuthorityWireClosure(t, authority)
		}

		policy := TimestampPolicy(raw)
		wantPolicy := policy == TimestampPolicyFreeTSA || policy == TimestampPolicyDigiCert
		gotPolicyErr := policy.Validate()
		if policy.IsValid() != wantPolicy || (gotPolicyErr == nil) != wantPolicy ||
			(policy.String() != "") != wantPolicy {
			t.Fatalf(
				"TimestampPolicy(%d) validity/string = (%t, %v, %q), want (%t, matching error state, matching token state)",
				raw, policy.IsValid(), gotPolicyErr, policy.String(), wantPolicy,
			)
		}
		if !wantPolicy {
			if !errors.Is(gotPolicyErr, core.ErrTimeProofContract) {
				t.Fatalf("TimestampPolicy(%d).Validate() error = %v, want %v", raw, gotPolicyErr, core.ErrTimeProofContract)
			}
		} else {
			proveTimestampPolicyWireClosure(t, policy)
		}
	}
}

func proveAuthorityWireClosure(t *testing.T, authority Authority) {
	t.Helper()

	authority.WireEnum()
	encoded, marshalErr := authority.MarshalJSON()
	var roundTrip Authority
	unmarshalErr := roundTrip.UnmarshalJSON(encoded)
	second, secondErr := roundTrip.MarshalJSON()
	if marshalErr != nil || unmarshalErr != nil || secondErr != nil || roundTrip != authority || !bytes.Equal(second, encoded) {
		t.Fatalf(
			"Authority(%d) wire closure = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
			authority, roundTrip, second, marshalErr, unmarshalErr, secondErr, authority, encoded,
		)
	}
}

func proveTimestampPolicyWireClosure(t *testing.T, policy TimestampPolicy) {
	t.Helper()

	policy.WireEnum()
	encoded, marshalErr := policy.MarshalJSON()
	var roundTrip TimestampPolicy
	unmarshalErr := roundTrip.UnmarshalJSON(encoded)
	second, secondErr := roundTrip.MarshalJSON()
	if marshalErr != nil || unmarshalErr != nil || secondErr != nil || roundTrip != policy || !bytes.Equal(second, encoded) {
		t.Fatalf(
			"TimestampPolicy(%d) wire closure = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
			policy, roundTrip, second, marshalErr, unmarshalErr, secondErr, policy, encoded,
		)
	}
}
