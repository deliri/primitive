package awsidentity

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAWSRequestEveryQueryFieldMultiplicity(t *testing.T) {
	t.Parallel()
	for field := amazonQueryFieldAction; field < amazonQueryFieldLimit; field++ {
		for _, tc := range []struct {
			wantErr  error
			name     string
			mutation awsURLMutation
		}{
			{name: "absent", mutation: awsURLDelete, wantErr: core.ErrAWSIdentityContract},
			{name: "empty", mutation: awsURLSet, wantErr: core.ErrAWSIdentityContract},
			{name: "duplicate", mutation: awsURLAppend, wantErr: core.ErrAWSIdentityContract},
		} {
			t.Run(field.name()+" "+tc.name, func(t *testing.T) {
				t.Parallel()
				audience := mustAWSAudience(t)
				raw := awsSignedURL(audience, awsTestHost, awsTestRegion)
				if field == amazonQueryFieldSecurityToken {
					raw = mutateAWSURL(t, raw, awsURLSet, field, "session-value")
				}
				changed := mutateAWSURL(t, raw, tc.mutation, field, "")
				if changed == raw {
					t.Fatalf("query mutation=%q, want different from %q", changed, raw)
				}
				input := RequestInput{SignedURL: changed, Audience: audience, Policy: mustAWSPolicy(t)}
				wantErr := tc.wantErr
				if field == amazonQueryFieldSecurityToken && tc.mutation == awsURLDelete {
					wantErr = nil
				}
				got, err := NewRequest(input)
				if !errors.Is(err, wantErr) || !errors.Is(input.Validate(), wantErr) {
					t.Fatalf("request errors = (%v,%v), want %v", err, input.Validate(), wantErr)
				}
				if wantErr != nil {
					if got != (Request{}) {
						t.Fatalf("rejected request = %v, want zero", got)
					}
					return
				}
				if got.endpoint.String() != changed || got.audience != audience || got.policy != input.Policy {
					t.Fatalf("accepted request facts = %v, want exact input", got)
				}
			})
		}
	}
}

func TestAWSRequestCredentialAndCanonicalQueryBindings(t *testing.T) {
	t.Parallel()
	credential := awsTestAccess + "/" + awsTestDate + "/" + awsTestRegion + "/" + amazonCredentialService + "/" + amazonCredentialTerminal
	for _, tc := range []struct {
		wantErr error
		name    string
		value   string
		field   amazonQueryField
	}{
		{name: "signature algorithm bound to SigV4", field: amazonQueryFieldSignatureAlgorithm, value: "other", wantErr: core.ErrAWSIdentityContract},
		{name: "signed headers cannot expand", field: amazonQueryFieldSignedHeaders, value: amazonSignedHeadersValue + ";authorization", wantErr: core.ErrAWSIdentityContract},
		{name: "credential access ID absent", field: amazonQueryFieldCredential, value: strings.TrimPrefix(credential, awsTestAccess), wantErr: core.ErrAWSIdentityContract},
		{name: "credential date too short", field: amazonQueryFieldCredential, value: strings.Replace(credential, awsTestDate, awsTestDate[:len(awsTestDate)-1], 1), wantErr: core.ErrAWSIdentityContract},
		{name: "credential date too long", field: amazonQueryFieldCredential, value: strings.Replace(credential, awsTestDate, awsTestDate+"0", 1), wantErr: core.ErrAWSIdentityContract},
		{name: "credential date non decimal", field: amazonQueryFieldCredential, value: strings.Replace(credential, awsTestDate, "20260x29", 1), wantErr: core.ErrAWSIdentityContract},
		{name: "credential region foreign", field: amazonQueryFieldCredential, value: strings.Replace(credential, awsTestRegion, "us-west-2", 1), wantErr: core.ErrAWSIdentityContract},
		{name: "credential service foreign", field: amazonQueryFieldCredential, value: strings.Replace(credential, "/"+amazonCredentialService+"/", "/s3/", 1), wantErr: core.ErrAWSIdentityContract},
		{name: "credential terminal foreign", field: amazonQueryFieldCredential, value: strings.TrimSuffix(credential, amazonCredentialTerminal) + "other", wantErr: core.ErrAWSIdentityContract},
		{name: "credential segment missing", field: amazonQueryFieldCredential, value: strings.TrimSuffix(credential, "/"+amazonCredentialTerminal), wantErr: core.ErrAWSIdentityContract},
		{name: "credential segment extra", field: amazonQueryFieldCredential, value: credential + "/extra", wantErr: core.ErrAWSIdentityContract},
		{name: "credential tiny access ID stays opaque", field: amazonQueryFieldCredential, value: strings.Replace(credential, awsTestAccess, "x", 1), wantErr: nil},
		{name: "credential date disagrees with signed date", field: amazonQueryFieldDate, value: "20260730T120000Z", wantErr: core.ErrAWSIdentityContract},
		{name: "signed date truncated", field: amazonQueryFieldDate, value: "202607", wantErr: core.ErrAWSIdentityContract},
		{name: "signed date impossible hour", field: amazonQueryFieldDate, value: awsTestDate + "T250000Z", wantErr: core.ErrAWSIdentityContract},
		{name: "signed date lowercase zone", field: amazonQueryFieldDate, value: awsTestDate + "T120000z", wantErr: core.ErrAWSIdentityContract},
		{name: "signed date fractional seconds noncanonical", field: amazonQueryFieldDate, value: awsTestDate + "T120000.1Z", wantErr: core.ErrAWSIdentityContract},
		{name: "signed date zero hour", field: amazonQueryFieldDate, value: awsTestDate + "T000000Z", wantErr: nil},
		{name: "signed date last second", field: amazonQueryFieldDate, value: awsTestDate + "T235959Z", wantErr: nil},
		{name: "expiry one above minimum", field: amazonQueryFieldExpires, value: "2", wantErr: nil},
		{name: "expiry one below ceiling", field: amazonQueryFieldExpires, value: strconv.Itoa(amazonSignedURLMaximumSecs - 1), wantErr: nil},
		{name: "expiry one above ceiling", field: amazonQueryFieldExpires, value: strconv.Itoa(amazonSignedURLMaximumSecs + 1), wantErr: core.ErrAWSIdentityContract},
		{name: "expiry positive sign", field: amazonQueryFieldExpires, value: "+1", wantErr: core.ErrAWSIdentityContract},
		{name: "expiry negative sign", field: amazonQueryFieldExpires, value: "-1", wantErr: core.ErrAWSIdentityContract},
		{name: "expiry leading whitespace", field: amazonQueryFieldExpires, value: " 1", wantErr: core.ErrAWSIdentityContract},
		{name: "expiry fractional", field: amazonQueryFieldExpires, value: "1.0", wantErr: core.ErrAWSIdentityContract},
		{name: "expiry uint64 overflow", field: amazonQueryFieldExpires, value: "18446744073709551616", wantErr: core.ErrAWSIdentityContract},
		{name: "security token escaped delimiters", field: amazonQueryFieldSecurityToken, value: "a+/=%&", wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			audience := mustAWSAudience(t)
			raw := awsSignedURL(audience, awsTestHost, awsTestRegion)
			changed := mutateAWSURL(t, raw, awsURLSet, tc.field, tc.value)
			if changed == raw {
				t.Fatalf("binding mutation=%q, want different from %q", changed, raw)
			}
			input := RequestInput{SignedURL: changed, Audience: audience, Policy: mustAWSPolicy(t)}
			got, err := NewRequest(input)
			if !errors.Is(err, tc.wantErr) || !errors.Is(input.Validate(), tc.wantErr) {
				t.Fatalf("NewRequest/Input.Validate errors = (%v,%v), want %v", err, input.Validate(), tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Request{}) {
					t.Fatalf("rejected request = %v, want zero", got)
				}
				return
			}
			if got.endpoint.String() != changed || got.audience != audience || got.policy != input.Policy {
				t.Fatalf("accepted request = %v, want exact source facts", got)
			}
		})
	}
}

func TestAWSRequestEndpointAndAudienceOwnership(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr                      error
		name, host, region, audience string
	}{
		{name: "China regional endpoint", host: "sts.cn-north-1.amazonaws.com.cn", region: "cn-north-1", audience: "a", wantErr: nil},
		{name: "China dual stack endpoint", host: "sts.cn-north-1.api.amazonwebservices.com.cn", region: "cn-north-1", audience: "a", wantErr: nil},
		{name: "China FIPS endpoint", host: "sts-fips.cn-north-1.amazonaws.com.cn", region: "cn-north-1", audience: "a", wantErr: nil},
		{name: "China FIPS dual stack endpoint", host: "sts-fips.cn-north-1.api.amazonwebservices.com.cn", region: "cn-north-1", audience: "a", wantErr: nil},
		{name: "hostname case keeps exact capability", host: "STS.US-EAST-2.AMAZONAWS.COM", region: awsTestRegion, audience: "a", wantErr: nil},
		{name: "future syntactic region remains provider owned", host: "sts.future-region-9.amazonaws.com", region: "future-region-9", audience: "a", wantErr: nil},
		{name: "audience maximum remains exact", host: awsTestHost, region: awsTestRegion, audience: strings.Repeat("a", AudienceMaximumBytes), wantErr: nil},
		{name: "audience encoding remains exact", host: awsTestHost, region: awsTestRegion, audience: "é +&=%/", wantErr: nil},
		{name: "audience embedded zero remains opaque", host: awsTestHost, region: awsTestRegion, audience: "a\x00b", wantErr: nil},
		{name: "trailing host dot is not contracted shape", host: awsTestHost + ".", region: awsTestRegion, audience: "a", wantErr: core.ErrAWSIdentityContract},
		{name: "foreign suffix after approved suffix", host: awsTestHost + ".example.test", region: awsTestRegion, audience: "a", wantErr: core.ErrAWSIdentityContract},
		{name: "underscore cannot hide in region", host: "sts.us_east-2.amazonaws.com", region: "us_east-2", audience: "a", wantErr: core.ErrAWSIdentityContract},
		{name: "explicit default port is refused", host: awsTestHost + ":443", region: awsTestRegion, audience: "a", wantErr: core.ErrAWSIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			audience, err := ParseAudience(tc.audience)
			if err != nil {
				t.Fatalf("audience fixture error = %v, want nil", err)
			}
			input := RequestInput{SignedURL: awsSignedURL(audience, tc.host, tc.region), Audience: audience, Policy: mustAWSPolicy(t)}
			got, err := NewRequest(input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NewRequest error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Request{}) {
					t.Fatalf("rejected request = %v, want zero", got)
				}
				return
			}
			if got.endpoint.String() != input.SignedURL || got.audience.String() != tc.audience {
				t.Fatalf("request retained facts = %v, want exact URL and audience", got)
			}
		})
	}
}

func TestAWSRequestRevalidatesOwnedFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		change func(*Request)
		name   string
	}{
		{name: "unset endpoint", change: func(r *Request) { r.endpoint = nil }},
		{name: "zero endpoint", change: func(r *Request) { r.endpoint = &core.HTTPEndpoint{} }},
		{name: "unset audience", change: func(r *Request) { r.audience = Audience{} }},
		{name: "foreign audience", change: func(r *Request) { r.audience = Audience{value: "foreign"} }},
		{name: "unset policy", change: func(r *Request) { r.policy = Policy{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := awsRequest(t)
			tc.change(&request)
			if err := request.Validate(); !errors.Is(err, core.ErrAWSIdentityContract) {
				t.Fatalf("Request.Validate error = %v, want AWS refusal", err)
			}
		})
	}
}

func TestAWSQueryFieldClosedDomain(t *testing.T) {
	t.Parallel()
	names := amazonQueryFieldNames()
	for value := 0; value <= 255; value++ {
		t.Run(fmt.Sprintf("query enum %d", value), func(t *testing.T) {
			t.Parallel()
			field := amazonQueryField(value)
			valid := field > amazonQueryFieldUnknown && field < amazonQueryFieldLimit
			if !valid {
				if err := field.validate(); !errors.Is(err, core.ErrAWSIdentityContract) || field.name() != "" {
					t.Fatalf("invalid enum = (%q,%v), want empty typed refusal", field.name(), err)
				}
				return
			}
			name := field.name()
			parsed, err := parseAmazonQueryField(name)
			if field.validate() != nil || name != names[field] || name == "" || parsed != field || err != nil {
				t.Fatalf("query enum projection = (%q,%d,%v), want exact %d", name, parsed, err, field)
			}
			for other := amazonQueryFieldAction; other < field; other++ {
				if names[other] == name {
					t.Fatalf("query name %q duplicated for %d and %d", name, other, field)
				}
			}
			if field.optional() != (field == amazonQueryFieldSecurityToken) {
				t.Fatalf("query optional = %v, want only security token", field.optional())
			}
		})
	}
}

func FuzzAWSRequestAudienceBinding(f *testing.F) {
	for _, value := range []string{"a", "é +&=%/", strings.Repeat("a", AudienceMaximumBytes)} {
		audience, err := ParseAudience(value)
		if err != nil {
			f.Fatalf("audience seed = %v, want nil", err)
		}
		request, err := NewRequest(RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion), Audience: audience, Policy: mustAWSPolicy(f)})
		if err != nil || request.Validate() != nil {
			f.Fatalf("request seed = %v, want nil", err)
		}
		f.Add(request.audience.String(), false)
		f.Add(request.audience.String(), true)
	}
	f.Add("", false)
	f.Add("\xff", false)
	f.Add(strings.Repeat("a", AudienceMaximumBytes+1), false)
	f.Fuzz(func(t *testing.T, value string, foreign bool) {
		// Raw mutated text reaches RequestInput/Request validation directly. An
		// invalid Audience is intentionally not filtered out by fixture setup.
		audience := Audience{value: value}
		queryAudience := audience
		if foreign {
			queryAudience = mustAWSAudience(t)
		}
		raw := awsSignedURL(queryAudience, awsTestHost, awsTestRegion)
		// awsSignedURL projects validated Audience.String. Replace the value at the
		// stdlib wire seam so invalid fuzz bytes cannot silently become a default.
		u, parseErr := url.Parse(raw)
		if parseErr != nil {
			t.Fatalf("fixture URL error = %v, want nil", parseErr)
		}
		query := u.Query()
		wireAudience := value
		if foreign {
			wireAudience = queryAudience.String()
		}
		query.Set(amazonAudienceQuery, wireAudience)
		u.RawQuery = query.Encode()
		input := RequestInput{SignedURL: u.String(), Audience: audience, Policy: mustAWSPolicy(t)}
		got, err := NewRequest(input)
		_, audienceErr := ParseAudience(value)
		wantValid := audienceErr == nil && wireAudience == value
		if !wantValid {
			if !errors.Is(err, core.ErrAWSIdentityContract) || got != (Request{}) || !errors.Is(input.Validate(), core.ErrAWSIdentityContract) {
				t.Fatalf("request rejected = (%v,%v), want zero typed refusal", got, err)
			}
			return
		}
		if err != nil || input.Validate() != nil || got.Validate() != nil || got.endpoint.String() != u.String() || got.audience.value != value {
			t.Fatalf("request accepted = (%v,%v), want exact bound audience and URL", got, err)
		}
	})
}
