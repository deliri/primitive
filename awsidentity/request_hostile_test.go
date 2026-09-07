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
			name     string
			mutation awsURLMutation
			wantErr  error
		}{
			{"absent", awsURLDelete, core.ErrAWSIdentityContract},
			{"empty", awsURLSet, core.ErrAWSIdentityContract},
			{"duplicate", awsURLAppend, core.ErrAWSIdentityContract},
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
					t.Fatal("query mutation unchanged, want edited field")
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
		name    string
		field   amazonQueryField
		value   string
		wantErr error
	}{
		{"signature algorithm bound to SigV4", amazonQueryFieldSignatureAlgorithm, "other", core.ErrAWSIdentityContract},
		{"signed headers cannot expand", amazonQueryFieldSignedHeaders, amazonSignedHeadersValue + ";authorization", core.ErrAWSIdentityContract},
		{"credential access ID absent", amazonQueryFieldCredential, strings.TrimPrefix(credential, awsTestAccess), core.ErrAWSIdentityContract},
		{"credential date too short", amazonQueryFieldCredential, strings.Replace(credential, awsTestDate, awsTestDate[:len(awsTestDate)-1], 1), core.ErrAWSIdentityContract},
		{"credential date too long", amazonQueryFieldCredential, strings.Replace(credential, awsTestDate, awsTestDate+"0", 1), core.ErrAWSIdentityContract},
		{"credential date non decimal", amazonQueryFieldCredential, strings.Replace(credential, awsTestDate, "20260x29", 1), core.ErrAWSIdentityContract},
		{"credential region foreign", amazonQueryFieldCredential, strings.Replace(credential, awsTestRegion, "us-west-2", 1), core.ErrAWSIdentityContract},
		{"credential service foreign", amazonQueryFieldCredential, strings.Replace(credential, "/"+amazonCredentialService+"/", "/s3/", 1), core.ErrAWSIdentityContract},
		{"credential terminal foreign", amazonQueryFieldCredential, strings.TrimSuffix(credential, amazonCredentialTerminal) + "other", core.ErrAWSIdentityContract},
		{"credential segment missing", amazonQueryFieldCredential, strings.TrimSuffix(credential, "/"+amazonCredentialTerminal), core.ErrAWSIdentityContract},
		{"credential segment extra", amazonQueryFieldCredential, credential + "/extra", core.ErrAWSIdentityContract},
		{"credential tiny access ID stays opaque", amazonQueryFieldCredential, strings.Replace(credential, awsTestAccess, "x", 1), nil},
		{"credential date disagrees with signed date", amazonQueryFieldDate, "20260730T120000Z", core.ErrAWSIdentityContract},
		{"signed date truncated", amazonQueryFieldDate, "202607", core.ErrAWSIdentityContract},
		{"signed date impossible hour", amazonQueryFieldDate, awsTestDate + "T250000Z", core.ErrAWSIdentityContract},
		{"signed date lowercase zone", amazonQueryFieldDate, awsTestDate + "T120000z", core.ErrAWSIdentityContract},
		{"signed date fractional seconds noncanonical", amazonQueryFieldDate, awsTestDate + "T120000.1Z", core.ErrAWSIdentityContract},
		{"signed date zero hour", amazonQueryFieldDate, awsTestDate + "T000000Z", nil},
		{"signed date last second", amazonQueryFieldDate, awsTestDate + "T235959Z", nil},
		{"expiry one above minimum", amazonQueryFieldExpires, "2", nil},
		{"expiry one below ceiling", amazonQueryFieldExpires, strconv.Itoa(amazonSignedURLMaximumSecs - 1), nil},
		{"expiry one above ceiling", amazonQueryFieldExpires, strconv.Itoa(amazonSignedURLMaximumSecs + 1), core.ErrAWSIdentityContract},
		{"expiry positive sign", amazonQueryFieldExpires, "+1", core.ErrAWSIdentityContract},
		{"expiry negative sign", amazonQueryFieldExpires, "-1", core.ErrAWSIdentityContract},
		{"expiry leading whitespace", amazonQueryFieldExpires, " 1", core.ErrAWSIdentityContract},
		{"expiry fractional", amazonQueryFieldExpires, "1.0", core.ErrAWSIdentityContract},
		{"expiry uint64 overflow", amazonQueryFieldExpires, "18446744073709551616", core.ErrAWSIdentityContract},
		{"security token escaped delimiters", amazonQueryFieldSecurityToken, "a+/=%&", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			audience := mustAWSAudience(t)
			raw := awsSignedURL(audience, awsTestHost, awsTestRegion)
			changed := mutateAWSURL(t, raw, awsURLSet, tc.field, tc.value)
			if changed == raw {
				t.Fatal("binding mutation unchanged, want edited query")
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
		name, host, region, audience string
		wantErr                      error
	}{
		{"China regional endpoint", "sts.cn-north-1.amazonaws.com.cn", "cn-north-1", "a", nil},
		{"China dual stack endpoint", "sts.cn-north-1.api.amazonwebservices.com.cn", "cn-north-1", "a", nil},
		{"China FIPS endpoint", "sts-fips.cn-north-1.amazonaws.com.cn", "cn-north-1", "a", nil},
		{"China FIPS dual stack endpoint", "sts-fips.cn-north-1.api.amazonwebservices.com.cn", "cn-north-1", "a", nil},
		{"hostname case keeps exact capability", "STS.US-EAST-2.AMAZONAWS.COM", awsTestRegion, "a", nil},
		{"future syntactic region remains provider owned", "sts.future-region-9.amazonaws.com", "future-region-9", "a", nil},
		{"audience maximum remains exact", awsTestHost, awsTestRegion, strings.Repeat("a", AudienceMaximumBytes), nil},
		{"audience encoding remains exact", awsTestHost, awsTestRegion, "é +&=%/", nil},
		{"audience embedded zero remains opaque", awsTestHost, awsTestRegion, "a\x00b", nil},
		{"trailing host dot is not contracted shape", awsTestHost + ".", awsTestRegion, "a", core.ErrAWSIdentityContract},
		{"foreign suffix after approved suffix", awsTestHost + ".example.test", awsTestRegion, "a", core.ErrAWSIdentityContract},
		{"underscore cannot hide in region", "sts.us_east-2.amazonaws.com", "us_east-2", "a", core.ErrAWSIdentityContract},
		{"explicit default port is refused", awsTestHost + ":443", awsTestRegion, "a", core.ErrAWSIdentityContract},
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
		name   string
		change func(*Request)
	}{
		{"unset endpoint", func(r *Request) { r.endpoint = nil }},
		{"zero endpoint", func(r *Request) { r.endpoint = &core.HTTPEndpoint{} }},
		{"unset audience", func(r *Request) { r.audience = Audience{} }},
		{"foreign audience", func(r *Request) { r.audience = Audience{value: "foreign"} }},
		{"unset policy", func(r *Request) { r.policy = Policy{} }},
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
