package cloudidentity

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestProviderExhaustiveClosedDomain(t *testing.T) {
	t.Parallel()

	for value := uint64(0); value <= math.MaxUint8; value++ {
		provider := Provider(value)
		wantValid := provider == ProviderGoogleCloud ||
			provider == ProviderAmazonWebServices
		if got := provider.IsValid(); got != wantValid {
			t.Fatalf(
				"Provider(%d).IsValid() = %t, want %t",
				value,
				got,
				wantValid,
			)
		}
		if (provider.String() != "") != wantValid {
			t.Fatalf(
				"Provider(%d).String() nonempty = %t, want %t",
				value,
				provider.String() != "",
				wantValid,
			)
		}
		if gotErr := provider.Validate(); wantValid != (gotErr == nil) {
			t.Fatalf(
				"Provider(%d).Validate() = %v, want valid %t",
				value,
				gotErr,
				wantValid,
			)
		} else if !wantValid &&
			!errors.Is(gotErr, core.ErrCloudIdentityContract) {
			t.Fatalf(
				"Provider(%d).Validate() error = %v, want %v",
				value,
				gotErr,
				core.ErrCloudIdentityContract,
			)
		}
	}
	names := make(map[string]Provider, providerLimit)
	for provider := ProviderGoogleCloud; provider < providerLimit; provider++ {
		name := provider.String()
		if previous, duplicated := names[name]; duplicated {
			t.Fatalf(
				"Provider(%d) and Provider(%d) share the name %q, want distinct names",
				previous,
				provider,
				name,
			)
		}
		names[name] = provider
	}
	if len(names) != int(providerLimit)-1 {
		t.Fatalf(
			"named providers = %d, want %d",
			len(names),
			int(providerLimit)-1,
		)
	}
}

// TestTokenCannotBeForgedOutsideTheAcquisitionPath keeps a zero Token from
// projecting a bearer. The token's only legitimate origin is one provider
// response, so a value that never crossed that path must refuse disclosure.
func TestTokenCannotBeForgedOutsideTheAcquisitionPath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		token Token
	}{
		{name: "zero token", token: Token{}},
		{name: "value without provenance", token: Token{value: testIdentityToken}},
		{name: "provenance without value", token: Token{provider: ProviderGoogleCloud}},
		{
			name: "out-of-domain provenance",
			token: Token{
				value:    testIdentityToken,
				provider: providerLimit,
			},
		},
		{
			name: "value outside the token syntax",
			token: Token{
				value:    "not a token",
				provider: ProviderGoogleCloud,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.token.Validate(); !errors.Is(
				gotErr,
				core.ErrCloudIdentityContract,
			) {
				t.Fatalf(
					"Token.Validate() error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			gotBearer, gotErr := tc.token.BearerValue()
			if gotBearer != "" || !errors.Is(
				gotErr,
				core.ErrCloudIdentityContract,
			) {
				t.Fatalf(
					"Token.BearerValue() = (%q, %v), want (\"\", %v)",
					gotBearer,
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			if got := fmt.Sprintf("%v", tc.token); got !=
				core.RedactedValueText {
				t.Fatalf(
					"fmt.Sprintf(%%v, unusable token) = %q, want %q",
					got,
					core.RedactedValueText,
				)
			}
		})
	}
}

func TestParseAudienceHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "one ASCII byte reaches the minimum", value: "a"},
		{name: "service URL remains exact", value: "https://api.example.com"},
		{name: "configured custom audience remains exact", value: "release-broker"},
		{name: "OAuth client identifier remains exact", value: "123.apps.googleusercontent.com"},
		{name: "query delimiters remain audience data", value: "service?tenant=one&role=writer"},
		{name: "space remains audience data", value: "service audience"},
		{name: "plus remains audience data", value: "service+audience"},
		{name: "Unicode remains exact UTF-8", value: "服务"},
		{name: "one below maximum is accepted", value: strings.Repeat("a", AudienceMaximumBytes-1)},
		{name: "exact maximum is accepted", value: strings.Repeat("a", AudienceMaximumBytes)},
		{name: "empty audience is rejected", wantErr: true},
		{name: "one above maximum is rejected", value: strings.Repeat("a", AudienceMaximumBytes+1), wantErr: true},
		{name: "far above maximum is rejected", value: strings.Repeat("a", 4*AudienceMaximumBytes), wantErr: true},
		{name: "single invalid UTF-8 byte is rejected", value: string([]byte{0xff}), wantErr: true},
		{name: "truncated two-byte UTF-8 is rejected", value: string([]byte{0xc2}), wantErr: true},
		{name: "truncated three-byte UTF-8 is rejected", value: string([]byte{0xe2, 0x82}), wantErr: true},
		{name: "surrogate UTF-8 is rejected", value: string([]byte{0xed, 0xa0, 0x80}), wantErr: true},
		{name: "overlong UTF-8 is rejected", value: string([]byte{0xc0, 0xaf}), wantErr: true},
		{name: "maximum prefix with invalid suffix is rejected", value: strings.Repeat("a", AudienceMaximumBytes-1) + string([]byte{0xff}), wantErr: true},
		{name: "multi-byte audience exceeding byte bound is rejected", value: strings.Repeat("界", AudienceMaximumBytes/2), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseAudience(tc.value)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
					t.Fatalf(
						"ParseAudience() error = %v, want %v",
						gotErr,
						core.ErrCloudIdentityContract,
					)
				}
				if got != (Audience{}) {
					t.Fatalf("ParseAudience() value = %#v, want zero", got)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ParseAudience() error = %v, want nil", gotErr)
			}
			if got.String() != tc.value || got.Validate() != nil {
				t.Fatalf(
					"audience round trip = (%q, %v), want (%q, nil)",
					got.String(),
					got.Validate(),
					tc.value,
				)
			}
		})
	}
}

func TestTokenBearerBoundaryHostileTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "JWT-shaped bearer remains opaque", value: testIdentityToken},
		{name: "lexical three-segment material is accepted without JWT claim", value: "a.b.c"},
		{name: "one token byte reaches the minimum", value: "a"},
		{name: "standard Base64 alphabet is accepted", value: "abc+/"},
		{name: "URL-safe Base64 alphabet is accepted", value: "abc-_"},
		{name: "tilde bearer byte is accepted", value: "abc~"},
		{name: "one padding byte is accepted at end", value: "abc="},
		{name: "two padding bytes are accepted at end", value: "abc=="},
		{name: "one below maximum is accepted", value: strings.Repeat("a", TokenMaximumBytes-1)},
		{name: "exact maximum is accepted", value: strings.Repeat("a", TokenMaximumBytes)},
		{name: "empty bearer is rejected", wantErr: true},
		{name: "leading padding is rejected", value: "=abc", wantErr: true},
		{name: "interior padding is rejected", value: "ab=c", wantErr: true},
		{name: "space is rejected", value: "ab c", wantErr: true},
		{name: "tab is rejected", value: "ab\tc", wantErr: true},
		{name: "newline is rejected", value: "ab\nc", wantErr: true},
		{name: "carriage return is rejected", value: "ab\rc", wantErr: true},
		{name: "comma is rejected", value: "ab,c", wantErr: true},
		{name: "non-ASCII is rejected", value: "ab界c", wantErr: true},
		{name: "one above maximum is rejected", value: strings.Repeat("a", TokenMaximumBytes+1), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := newToken(
				ProviderGoogleCloud,
				tc.value,
			)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
					t.Fatalf(
						"newToken() error = %v, want %v",
						gotErr,
						core.ErrCloudIdentityContract,
					)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("newToken() error = %v, want nil", gotErr)
			}
			gotBearer, gotBearerErr := got.BearerValue()
			wantBearer := bearerPrefix + tc.value
			if gotBearerErr != nil || gotBearer != wantBearer {
				t.Fatalf(
					"Token.BearerValue() = (%q, %v), want (%q, nil)",
					gotBearer,
					gotBearerErr,
					wantBearer,
				)
			}
		})
	}
}

func TestTokenAndSignedRequestRedactEveryFormattingSurface(t *testing.T) {
	t.Parallel()

	token, err := newToken(
		ProviderGoogleCloud,
		testIdentityToken,
	)
	if err != nil {
		t.Fatalf("newToken() setup error = %v, want nil", err)
	}
	audience := mustAudience(t, "https://api.example.com")
	signed, err := NewAmazonWebServicesRequest(
		AmazonWebServicesRequestInput{
			Request: Request{
				Audience: audience,
				Policy:   mustPolicy(t),
			},
			SignedURL: amazonSignedURL(
				audience,
				"sts.us-east-2.amazonaws.com",
			),
		},
	)
	if err != nil {
		t.Fatalf(
			"NewAmazonWebServicesRequest() setup error = %v, want nil",
			err,
		)
	}
	for _, tc := range []struct {
		name   string
		format string
	}{
		{name: "default value formatting redacts", format: "%v"},
		{name: "detailed value formatting redacts", format: "%+v"},
		{name: "Go syntax formatting redacts", format: "%#v"},
		{name: "string formatting redacts", format: "%s"},
		{name: "quoted formatting redacts", format: "%q"},
		{name: "hex formatting redacts", format: "%x"},
		{name: "uppercase hex formatting redacts", format: "%X"},
		{name: "integer formatting redacts", format: "%d"},
		{name: "character formatting redacts", format: "%c"},
		{name: "floating formatting redacts", format: "%f"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, value := range []fmt.Formatter{token, signed} {
				got := fmt.Sprintf(tc.format, value)
				if got != core.RedactedValueText {
					t.Fatalf(
						"fmt.Sprintf(%q) = %q, want %q",
						tc.format,
						got,
						core.RedactedValueText,
					)
				}
			}
		})
	}
}

func TestPolicyRejectsInvalidTimeoutLattice(t *testing.T) {
	t.Parallel()

	one, err := temporal.DurationFromNanoseconds(1)
	if err != nil {
		t.Fatalf("DurationFromNanoseconds(1) setup error = %v, want nil", err)
	}
	two, err := temporal.DurationFromNanoseconds(2)
	if err != nil {
		t.Fatalf("DurationFromNanoseconds(2) setup error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		policy  Policy
		wantErr bool
	}{
		{name: "equal positive bounds are accepted", policy: Policy{OperationTimeout: one, AttemptTimeout: one}},
		{name: "smaller attempt bound is accepted", policy: Policy{OperationTimeout: two, AttemptTimeout: one}},
		{name: "zero operation bound is rejected", policy: Policy{AttemptTimeout: one}, wantErr: true},
		{name: "zero attempt bound is rejected", policy: Policy{OperationTimeout: one}, wantErr: true},
		{name: "attempt beyond operation is rejected", policy: Policy{OperationTimeout: one, AttemptTimeout: two}, wantErr: true},
		{name: "complete zero policy is rejected", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.policy.Validate()
			if tc.wantErr != errors.Is(
				gotErr,
				core.ErrCloudIdentityContract,
			) {
				t.Fatalf(
					"Policy.Validate() error = %v, want contract error %t",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func FuzzParseAudience(f *testing.F) {
	for _, seed := range []string{
		"",
		"a",
		"https://api.example.com",
		strings.Repeat("a", AudienceMaximumBytes),
		strings.Repeat("a", AudienceMaximumBytes+1),
		string([]byte{0xff}),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseAudience(value)
		wantValid := len(value) > 0 &&
			len(value) <= AudienceMaximumBytes &&
			utf8.ValidString(value)
		if !wantValid {
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf(
					"ParseAudience() error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			if got != (Audience{}) {
				t.Fatalf("ParseAudience() rejected value = %#v, want zero", got)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil || got.String() != value {
			t.Fatalf(
				"ParseAudience() accepted value = (%q, %v, %v), want exact validated round trip",
				got.String(),
				gotErr,
				got.Validate(),
			)
		}
	})
}
