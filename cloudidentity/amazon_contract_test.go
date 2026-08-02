package cloudidentity

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAmazonSignedRequestHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	audience := mustAudience(t, "https://api.example.com/release")
	base := amazonSignedURL(audience, amazonTestHost)
	cases := []struct {
		mutate  func(string) string
		name    string
		wantErr bool
	}{
		{name: "commercial regional endpoint is accepted"},
		{name: "commercial dual-stack endpoint is accepted", mutate: replaceHost("sts.us-east-2.api.aws")},
		{name: "commercial FIPS endpoint is accepted", mutate: replaceHost("sts-fips.us-east-2.amazonaws.com")},
		{name: "commercial FIPS dual-stack endpoint is accepted", mutate: replaceHost("sts-fips.us-east-2.api.aws")},
		{name: "GovCloud regional endpoint is accepted", mutate: replaceHostAndRegion("sts.us-gov-west-1.amazonaws.com", "us-gov-west-1")},
		{name: "GovCloud dual-stack endpoint is accepted", mutate: replaceHostAndRegion("sts.us-gov-west-1.api.aws", "us-gov-west-1")},
		{name: "China regional endpoint is accepted", mutate: replaceHostAndRegion("sts.cn-north-1.amazonaws.com.cn", "cn-north-1")},
		{name: "China dual-stack endpoint is accepted", mutate: replaceHostAndRegion("sts.cn-north-1.api.amazonwebservices.com.cn", "cn-north-1")},
		{name: "temporary credential security token is accepted", mutate: setQuery(amazonSecurityTokenQuery, "session-token")},
		{name: "one-second signed capability is accepted", mutate: setQuery(amazonExpiresQuery, "1")},
		{name: "exact five-minute signed capability is accepted", mutate: setQuery(amazonExpiresQuery, "300")},
		{name: "encoded audience delimiters are accepted exactly", mutate: nil},
		{name: "global STS endpoint is rejected", mutate: replaceHost("sts.amazonaws.com"), wantErr: true},
		{name: "plain HTTP endpoint is rejected", mutate: replaceScheme("http"), wantErr: true},
		{name: "non-root STS path is rejected", mutate: replacePath("/identity"), wantErr: true},
		{name: "custom port is rejected", mutate: replaceHost("sts.us-east-2.amazonaws.com:8443"), wantErr: true},
		{name: "unrelated HTTPS host is rejected", mutate: replaceHost("identity.example.com"), wantErr: true},
		{name: "regionless commercial host is rejected", mutate: replaceHost("sts..amazonaws.com"), wantErr: true},
		{name: "dotted region is rejected", mutate: replaceHost("sts.us.east-2.amazonaws.com"), wantErr: true},
		{name: "uppercase region is rejected", mutate: replaceHostAndRegion("sts.US-east-2.amazonaws.com", "US-east-2"), wantErr: true},
		{name: "wrong action is rejected", mutate: setQuery(amazonActionQuery, "GetCallerIdentity"), wantErr: true},
		{name: "wrong API version is rejected", mutate: setQuery(amazonVersionQuery, "2026-01-01"), wantErr: true},
		{name: "contradictory audience is rejected", mutate: setQuery(amazonAudienceQuery, "other"), wantErr: true},
		{name: "second audience is rejected", mutate: setQuery("Audience.member.2", audience.String()), wantErr: true},
		{name: "tag claim is rejected", mutate: setQuery("Tags.member.1.Key", "team"), wantErr: true},
		{name: "ES384 request is outside common RS256 contract", mutate: setQuery(amazonSigningAlgorithmQuery, "ES384"), wantErr: true},
		{name: "longer token duration is rejected", mutate: setQuery(amazonDurationQuery, "3600"), wantErr: true},
		{name: "unknown SigV4 algorithm is rejected", mutate: setQuery(amazonSigAlgorithmQuery, "AWS4-ECDSA-P256-SHA256"), wantErr: true},
		{name: "credential region contradiction is rejected", mutate: setQuery(amazonCredentialQuery, "AKIATEST/20260729/us-west-2/sts/aws4_request"), wantErr: true},
		{name: "credential service contradiction is rejected", mutate: setQuery(amazonCredentialQuery, "AKIATEST/20260729/us-east-2/s3/aws4_request"), wantErr: true},
		{name: "credential terminal contradiction is rejected", mutate: setQuery(amazonCredentialQuery, "AKIATEST/20260729/us-east-2/sts/aws4_requestx"), wantErr: true},
		{name: "credential date contradiction is rejected", mutate: setQuery(amazonCredentialQuery, "AKIATEST/20260728/us-east-2/sts/aws4_request"), wantErr: true},
		{name: "malformed signed date is rejected", mutate: setQuery(amazonDateQuery, "20260729"), wantErr: true},
		{name: "nonexistent signed date is rejected", mutate: setQuery(amazonDateQuery, "20260230T120000Z"), wantErr: true},
		{name: "zero expiry is rejected", mutate: setQuery(amazonExpiresQuery, "0"), wantErr: true},
		{name: "noncanonical expiry is rejected", mutate: setQuery(amazonExpiresQuery, "060"), wantErr: true},
		{name: "one beyond expiry ceiling is rejected", mutate: setQuery(amazonExpiresQuery, "301"), wantErr: true},
		{name: "additional signed header cannot be reconstructed", mutate: setQuery(amazonSignedHeadersQuery, "host;x-amz-date"), wantErr: true},
		{name: "short signature is rejected", mutate: setQuery(amazonSignatureQuery, strings.Repeat("a", 63)), wantErr: true},
		{name: "long signature is rejected", mutate: setQuery(amazonSignatureQuery, strings.Repeat("a", 65)), wantErr: true},
		{name: "uppercase signature is rejected", mutate: setQuery(amazonSignatureQuery, strings.Repeat("A", 64)), wantErr: true},
		{name: "nonhex signature is rejected", mutate: setQuery(amazonSignatureQuery, strings.Repeat("z", 64)), wantErr: true},
		{name: "empty security token is rejected", mutate: setQuery(amazonSecurityTokenQuery, ""), wantErr: true},
		{name: "unknown query field is rejected", mutate: setQuery("FutureParameter", "value"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := base
			if tc.mutate != nil {
				value = tc.mutate(value)
			}
			got, gotErr := NewAmazonWebServicesRequest(
				AmazonWebServicesRequestInput{
					Request: Request{
						Audience: audience,
						Policy:   mustPolicy(t),
					},
					SignedURL: value,
				},
			)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
					t.Fatalf(
						"NewAmazonWebServicesRequest() error = %v, want %v",
						gotErr,
						core.ErrCloudIdentityContract,
					)
				}
				if got != (AmazonWebServicesRequest{}) {
					t.Fatalf(
						"NewAmazonWebServicesRequest() value = %#v, want zero",
						got,
					)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf(
					"NewAmazonWebServicesRequest() = (%v, %v), want validated request",
					got,
					gotErr,
				)
			}
		})
	}
}

// TestAmazonQueryFieldDomainIsExhaustivelyEnforced derives its cases from the
// closed query domain instead of naming fields by hand. Admitting a new field
// therefore extends this table automatically: the field must be enforced as
// required or declared optional, and neither can be forgotten.
func TestAmazonQueryFieldDomainIsExhaustivelyEnforced(t *testing.T) {
	t.Parallel()

	audience := mustAudience(t, "https://api.example.com/release")
	// complete carries every member of the domain, including the optional
	// session token, so duplicating a field means duplicating a field that was
	// already present rather than introducing one.
	complete := setQuery(amazonSecurityTokenQuery, "session-token")(
		amazonSignedURL(audience, amazonTestHost),
	)
	fields := 0
	for field := amazonQueryFieldAction; field < amazonQueryFieldLimit; field++ {
		fields++
		name := field.name()
		if name == "" {
			t.Fatalf(
				"amazonQueryField(%d).name() is empty, want the published name",
				field,
			)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			proveFieldRemovalContract(t, audience, complete, field)
			proveRejected(
				t,
				audience,
				appendQuery(name, "duplicate")(complete),
				"a duplicated "+name,
			)
		})
	}
	if fields != amazonQueryFieldCount {
		t.Fatalf(
			"enumerated query fields = %d, want %d",
			fields,
			amazonQueryFieldCount,
		)
	}
}

// proveFieldRemovalContract requires a required field's absence to be refused
// and an optional field's absence to be accepted. A required field the parser
// forgot to check would pass removal, which is exactly the drift the retired
// copied field count could not detect.
func proveFieldRemovalContract(
	t *testing.T,
	audience Audience,
	base string,
	field amazonQueryField,
) {
	t.Helper()

	removed := deleteQuery(field.name())(base)
	if field.optional() {
		got, gotErr := NewAmazonWebServicesRequest(
			AmazonWebServicesRequestInput{
				Request: Request{
					Audience: audience,
					Policy:   mustPolicy(t),
				},
				SignedURL: removed,
			},
		)
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf(
				"omitting optional %s = (%v, %v), want a validated capability",
				field.name(),
				got,
				gotErr,
			)
		}
		return
	}
	proveRejected(t, audience, removed, "an absent "+field.name())
}

func proveRejected(
	t *testing.T,
	audience Audience,
	signedURL string,
	description string,
) {
	t.Helper()

	got, gotErr := NewAmazonWebServicesRequest(
		AmazonWebServicesRequestInput{
			Request: Request{
				Audience: audience,
				Policy:   mustPolicy(t),
			},
			SignedURL: signedURL,
		},
	)
	if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
		t.Fatalf(
			"NewAmazonWebServicesRequest() with %s error = %v, want %v",
			description,
			gotErr,
			core.ErrCloudIdentityContract,
		)
	}
	if got != (AmazonWebServicesRequest{}) {
		t.Fatalf(
			"NewAmazonWebServicesRequest() with %s value = %#v, want zero",
			description,
			got,
		)
	}
}

// TestAmazonQueryFieldNamesAreUniqueAndClosed keeps the domain's projection
// injective. Two fields sharing a wire name would let one satisfy the other's
// presence check while the real field went unvalidated.
func TestAmazonQueryFieldNamesAreUniqueAndClosed(t *testing.T) {
	t.Parallel()

	seen := make(map[string]amazonQueryField, amazonQueryFieldCount)
	optional := make([]amazonQueryField, 0)
	for field := amazonQueryFieldAction; field < amazonQueryFieldLimit; field++ {
		name := field.name()
		if previous, duplicated := seen[name]; duplicated {
			t.Fatalf(
				"amazonQueryField(%d) and amazonQueryField(%d) share the name %q, want unique names",
				previous,
				field,
				name,
			)
		}
		seen[name] = field
		if field.optional() {
			optional = append(optional, field)
		}
		parsed, parseErr := parseAmazonQueryField(name)
		if parseErr != nil || parsed != field {
			t.Fatalf(
				"parseAmazonQueryField(%q) = (%d, %v), want (%d, nil)",
				name,
				parsed,
				parseErr,
				field,
			)
		}
	}
	wantOptional := []amazonQueryField{amazonQueryFieldSecurityToken}
	if !slices.Equal(optional, wantOptional) {
		t.Fatalf(
			"optional query fields = %v, want %v",
			optional,
			wantOptional,
		)
	}
	for _, outside := range []string{
		"",
		"action",
		"ACTION",
		"Action ",
		" Action",
		"X-Amz-Signature2",
		"Tags.member.1.Key",
		"Audience.member.2",
	} {
		got, gotErr := parseAmazonQueryField(outside)
		if !errors.Is(gotErr, core.ErrCloudIdentityContract) ||
			got != amazonQueryFieldUnknown {
			t.Fatalf(
				"parseAmazonQueryField(%q) = (%d, %v), want (unknown, %v)",
				outside,
				got,
				gotErr,
				core.ErrCloudIdentityContract,
			)
		}
	}
}

// TestAmazonQueryFieldClosedDomainExhaustsItsIntegerSpace sweeps every uint8 so
// no value outside the declared members can name a field.
func TestAmazonQueryFieldClosedDomainExhaustsItsIntegerSpace(t *testing.T) {
	t.Parallel()

	for value := uint64(0); value <= math.MaxUint8; value++ {
		field := amazonQueryField(value)
		wantValid := field > amazonQueryFieldUnknown &&
			field < amazonQueryFieldLimit
		gotErr := field.validate()
		if wantValid != (gotErr == nil) {
			t.Fatalf(
				"amazonQueryField(%d).validate() = %v, want valid %t",
				value,
				gotErr,
				wantValid,
			)
		}
		if wantValid != (field.name() != "") {
			t.Fatalf(
				"amazonQueryField(%d).name() nonempty = %t, want %t",
				value,
				field.name() != "",
				wantValid,
			)
		}
	}
}

// TestRejectedSignedCapabilityIsNeverDisclosed proves construction itself owns
// redaction. A malformed signed capability must remain safe even if a wrapped
// parser starts quoting its input in a later revision.
func TestRejectedSignedCapabilityIsNeverDisclosed(t *testing.T) {
	t.Parallel()

	audience := mustAudience(t, "https://api.example.com/release")
	const signature = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	const sessionToken = "session-token-must-never-be-disclosed"
	valid := setQuery(amazonSecurityTokenQuery, sessionToken)(
		setQuery(amazonSignatureQuery, signature)(
			amazonSignedURL(audience, amazonTestHost),
		),
	)
	for _, tc := range []struct {
		name      string
		signedURL string
	}{
		{name: "unparsable capability", signedURL: "https://sts.us-east-2.amazonaws.com/?" + "X-Amz-Signature=" + signature + "&X-Amz-Security-Token=" + sessionToken + "&%zz"},
		{name: "control byte in capability", signedURL: valid + "\x7f"},
		{name: "relative capability", signedURL: strings.TrimPrefix(valid, "https://")},
		{name: "credential-bearing capability", signedURL: strings.Replace(valid, "https://", "https://user:pass@", 1)},
		{name: "fragmented capability", signedURL: valid + "#fragment"},
		{name: "wrong scheme capability", signedURL: strings.Replace(valid, "https://", "ftp://", 1)},
		{name: "unrelated host capability", signedURL: strings.Replace(valid, amazonTestHost, "identity.example.com", 1)},
		{name: "oversized capability", signedURL: valid + "&Action=" + strings.Repeat("a", 1<<20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, gotErr := NewAmazonWebServicesRequest(
				AmazonWebServicesRequestInput{
					Request: Request{
						Audience: audience,
						Policy:   mustPolicy(t),
					},
					SignedURL: tc.signedURL,
				},
			)
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf(
					"NewAmazonWebServicesRequest() error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			proveRedactedError(t, gotErr)
		})
	}
}

// TestAmazonResponseNamespaceIsBoundToTheRequestedAPIVersion keeps the response
// namespace and the signed Version query naming one API version. Two independent
// literals could drift, and a drifted pair would accept a document from an API
// version the request never asked for.
func TestAmazonResponseNamespaceIsBoundToTheRequestedAPIVersion(t *testing.T) {
	t.Parallel()

	if !strings.Contains(amazonResponseNamespace, amazonVersionValue) {
		t.Fatalf(
			"amazonResponseNamespace = %q, want it to carry the requested version %q",
			amazonResponseNamespace,
			amazonVersionValue,
		)
	}
	want := "https://sts.amazonaws.com/doc/" + amazonVersionValue + "/"
	if amazonResponseNamespace != want {
		t.Fatalf(
			"amazonResponseNamespace = %q, want %q",
			amazonResponseNamespace,
			want,
		)
	}
}

func replaceHost(host string) func(string) string {
	return func(value string) string {
		parsed, _ := url.Parse(value)
		parsed.Host = host
		return parsed.String()
	}
}

func replaceHostAndRegion(
	host string,
	region string,
) func(string) string {
	return func(value string) string {
		parsed, _ := url.Parse(value)
		parsed.Host = host
		query := parsed.Query()
		query.Set(
			amazonCredentialQuery,
			fmt.Sprintf(
				"AKIATEST/20260729/%s/sts/aws4_request",
				region,
			),
		)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
}

func replaceScheme(scheme string) func(string) string {
	return func(value string) string {
		parsed, _ := url.Parse(value)
		parsed.Scheme = scheme
		return parsed.String()
	}
}

func replacePath(path string) func(string) string {
	return func(value string) string {
		parsed, _ := url.Parse(value)
		parsed.Path = path
		return parsed.String()
	}
}

func setQuery(name, value string) func(string) string {
	return func(target string) string {
		parsed, _ := url.Parse(target)
		query := parsed.Query()
		query.Set(name, value)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
}

func deleteQuery(name string) func(string) string {
	return func(target string) string {
		parsed, _ := url.Parse(target)
		query := parsed.Query()
		query.Del(name)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
}

func appendQuery(name, value string) func(string) string {
	return func(target string) string {
		parsed, _ := url.Parse(target)
		query := parsed.Query()
		query.Add(name, value)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
}

func FuzzNewAmazonWebServicesRequest(f *testing.F) {
	audience, err := ParseAudience("https://api.example.com/release")
	if err != nil {
		f.Fatalf("ParseAudience() setup error = %v, want nil", err)
	}
	policy, err := DefaultPolicy()
	if err != nil {
		f.Fatalf("DefaultPolicy() setup error = %v, want nil", err)
	}
	f.Add(amazonSignedURL(audience, amazonTestHost))
	f.Add("")
	f.Add("https://sts.amazonaws.com/")
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := NewAmazonWebServicesRequest(
			AmazonWebServicesRequestInput{
				Request: Request{
					Audience: audience,
					Policy:   policy,
				},
				SignedURL: value,
			},
		)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf(
					"NewAmazonWebServicesRequest() error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			if got != (AmazonWebServicesRequest{}) {
				t.Fatalf(
					"NewAmazonWebServicesRequest() rejected value = %#v, want zero",
					got,
				)
			}
			return
		}
		if got.Validate() != nil ||
			fmt.Sprintf("%v", got) != core.RedactedValueText {
			t.Fatalf(
				"NewAmazonWebServicesRequest() accepted invalid or disclosed request",
			)
		}
	})
}
