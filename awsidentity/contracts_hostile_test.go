package awsidentity

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestAWSAudienceByteAndRepresentationBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input string
		wantErr     error
	}{
		{"empty below minimum", "", core.ErrAWSIdentityContract},
		{"one ASCII byte", "a", nil}, {"two ASCII bytes", "ab", nil},
		{"one below maximum", strings.Repeat("a", AudienceMaximumBytes-1), nil},
		{"exact maximum", strings.Repeat("z", AudienceMaximumBytes), nil},
		{"one above maximum", strings.Repeat("a", AudienceMaximumBytes+1), core.ErrAWSIdentityContract},
		{"multibyte exact byte maximum", awsUTF8Extent("é", AudienceMaximumBytes), nil},
		{"multibyte one byte above maximum", awsUTF8Extent("é", AudienceMaximumBytes+1), core.ErrAWSIdentityContract},
		{"truncated two byte sequence", "\xc3", core.ErrAWSIdentityContract},
		{"truncated three byte sequence", "\xe2\x82", core.ErrAWSIdentityContract},
		{"orphan continuation", "\x80", core.ErrAWSIdentityContract},
		{"overlong representation", "\xc0\xaf", core.ErrAWSIdentityContract},
		{"surrogate representation", "\xed\xa0\x80", core.ErrAWSIdentityContract},
		{"above Unicode scalar ceiling", "\xf4\x90\x80\x80", core.ErrAWSIdentityContract},
		{"maximum Unicode scalar", "\U0010ffff", nil},
		{"embedded zero remains opaque", "a\x00b", nil},
		{"whitespace is not product policy", " \t\n", nil},
		{"combining sequence remains exact", "e\u0301", nil},
		{"query delimiters remain audience data", "a&b=c+%/", nil},
		{"last one-byte scalar", "\u007f", nil},
		{"first two-byte scalar", "\u0080", nil},
		{"last two-byte scalar", "\u07ff", nil},
		{"first three-byte scalar", "\u0800", nil},
		{"last scalar before surrogate range", "\ud7ff", nil},
		{"first scalar after surrogate range", "\ue000", nil},
		{"last three-byte scalar", "\uffff", nil},
		{"first four-byte scalar", "\U00010000", nil},
		{"replacement scalar is valid input data", "\ufffd", nil},
		{"three-byte text one below byte ceiling", awsUTF8Extent("€", AudienceMaximumBytes-1), nil},
		{"three-byte text exactly at byte ceiling", awsUTF8Extent("€", AudienceMaximumBytes), nil},
		{"three-byte text one above byte ceiling", awsUTF8Extent("€", AudienceMaximumBytes+1), core.ErrAWSIdentityContract},
		{"four-byte text one below byte ceiling", awsUTF8Extent("😀", AudienceMaximumBytes-1), nil},
		{"four-byte text exactly at byte ceiling", awsUTF8Extent("😀", AudienceMaximumBytes), nil},
		{"four-byte text one above byte ceiling", awsUTF8Extent("😀", AudienceMaximumBytes+1), core.ErrAWSIdentityContract},
		{"invalid continuation at exact byte ceiling", strings.Repeat("a", AudienceMaximumBytes-3) + "\xe2\x28\xa1", core.ErrAWSIdentityContract},
		{"truncated final four-byte scalar", "\xf0\x90\x80", core.ErrAWSIdentityContract},
		{"overlong three-byte encoding", "\xe0\x80\xaf", core.ErrAWSIdentityContract},
		{"overlong four-byte encoding", "\xf0\x80\x80\xaf", core.ErrAWSIdentityContract},
		{"unsupported five-byte lead", "\xf8\x88\x80\x80\x80", core.ErrAWSIdentityContract},
		{"invalid byte after long ASCII prefix", strings.Repeat("a", AudienceMaximumBytes-2) + "\xff" + "b", core.ErrAWSIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := ParseAudience(tc.input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseAudience error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Audience{}) || got.String() != "" {
					t.Fatalf("rejected audience = %v, want zero", got)
				}
				if err := (Audience{value: tc.input}).Validate(); !errors.Is(err, tc.wantErr) {
					t.Fatalf("Audience.Validate error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if got.String() != tc.input || got.value != tc.input || got.Validate() != nil {
				t.Fatalf("audience = %q, want exact %q", got.String(), tc.input)
			}
		})
	}
}

func FuzzAWSAudienceExactUTF8(f *testing.F) {
	for _, input := range []string{"a", "\U0010ffff", strings.Repeat("é", AudienceMaximumBytes/2)} {
		seed, err := ParseAudience(input)
		if err != nil || seed.Validate() != nil {
			f.Fatalf("audience seed error = %v, want nil", err)
		}
		f.Add(seed.String())
	}
	for _, input := range []string{"", "\xff", strings.Repeat("a", AudienceMaximumBytes+1)} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseAudience(input)
		wantValid := len(input) > 0 && len(input) <= AudienceMaximumBytes && utf8.ValidString(input)
		if !wantValid {
			if !errors.Is(gotErr, core.ErrAWSIdentityContract) || got != (Audience{}) {
				t.Fatalf("ParseAudience rejected = (%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseAudience valid = (%q,%v), want exact input", got.String(), gotErr)
		}
	})
}

func TestAWSTokenExactAlphabetAndExtent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input string
		wantErr     error
	}{
		{"empty below minimum", "", core.ErrAWSIdentityContract},
		{"smallest uppercase byte", "A", nil}, {"largest uppercase byte", "Z", nil},
		{"smallest lowercase byte", "a", nil}, {"largest lowercase byte", "z", nil},
		{"smallest digit", "0", nil}, {"largest digit", "9", nil},
		{"dash is data", "-", nil}, {"dot is data", ".", nil}, {"underscore is data", "_", nil},
		{"tilde is data", "~", nil}, {"plus is data", "+", nil}, {"slash is data", "/", nil},
		{"single trailing padding", "a=", nil}, {"multiple trailing padding", "a===", nil},
		{"one below byte maximum", strings.Repeat("a", TokenMaximumBytes-1), nil},
		{"at byte maximum", strings.Repeat("a", TokenMaximumBytes), nil},
		{"padding at byte maximum", "a" + strings.Repeat("=", TokenMaximumBytes-1), nil},
		{"one above byte maximum", strings.Repeat("a", TokenMaximumBytes+1), core.ErrAWSIdentityContract},
		{"padding exceeds byte maximum", "a" + strings.Repeat("=", TokenMaximumBytes), core.ErrAWSIdentityContract},
		{"padding cannot start", "=a", core.ErrAWSIdentityContract},
		{"padding cannot be whole token", "=", core.ErrAWSIdentityContract},
		{"data cannot follow padding", "a=b", core.ErrAWSIdentityContract},
		{"punctuation cannot follow padding", "a=+", core.ErrAWSIdentityContract},
		{"space cannot prefix", " a", core.ErrAWSIdentityContract},
		{"space cannot suffix", "a ", core.ErrAWSIdentityContract},
		{"CR header injection", "a\rb", core.ErrAWSIdentityContract},
		{"LF header injection", "a\nb", core.ErrAWSIdentityContract},
		{"tab separator", "a\tb", core.ErrAWSIdentityContract},
		{"embedded zero", "a\x00b", core.ErrAWSIdentityContract},
		{"Unicode lookalike", "а", core.ErrAWSIdentityContract},
		{"invalid UTF8", "\xff", core.ErrAWSIdentityContract},
		{"before uppercase range", "@", core.ErrAWSIdentityContract},
		{"after uppercase range", "[", core.ErrAWSIdentityContract},
		{"before lowercase range", "`", core.ErrAWSIdentityContract},
		{"after lowercase range", "{", core.ErrAWSIdentityContract},
		{"after digit range", ":", core.ErrAWSIdentityContract},
		{"quoted token", "\"a\"", core.ErrAWSIdentityContract},
		{"comma separated tokens", "a,b", core.ErrAWSIdentityContract},
		{"opaque non JWT token is admitted", "not-a-jwt", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := newToken(tc.input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("newToken error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Token{}) {
					t.Fatalf("newToken rejection = %v, want zero", got)
				}
				invalid := Token{value: &tc.input}
				disclosed, err := invalid.BearerValue()
				if !errors.Is(err, tc.wantErr) || disclosed != "" {
					t.Fatalf("invalid BearerValue = (%q,%v), want empty typed refusal", disclosed, err)
				}
				return
			}
			disclosed, err := got.BearerValue()
			if err != nil || got.Validate() != nil || disclosed != bearerPrefix+tc.input {
				t.Fatalf("BearerValue = (%q,%v), want exact token", disclosed, err)
			}
		})
	}
	// Exhaust the byte domain independently of the production range predicate.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~+/"
	for value := 0; value <= math.MaxUint8; value++ {
		t.Run(fmt.Sprintf("single byte %02x", value), func(t *testing.T) {
			t.Parallel()
			input := string([]byte{byte(value)})
			got, err := newToken(input)
			wantValid := strings.ContainsRune(alphabet, rune(value))
			if wantValid {
				disclosed, discloseErr := got.BearerValue()
				if err != nil || discloseErr != nil || disclosed != bearerPrefix+input {
					t.Fatalf("byte %x = (%q,%v,%v), want exact disclosure", value, disclosed, err, discloseErr)
				}
				return
			}
			if !errors.Is(err, core.ErrAWSIdentityContract) || got != (Token{}) {
				t.Fatalf("byte %x = (%v,%v), want zero typed refusal", value, got, err)
			}
		})
	}
}

func TestAWSPolicyTimeoutPairAndFixedExecution(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		operation, attempt int64
		wantErr            error
	}{
		{"both zero", 0, 0, core.ErrAWSIdentityContract},
		{"missing operation", 0, 1, core.ErrAWSIdentityContract},
		{"missing attempt", 1, 0, core.ErrAWSIdentityContract},
		{"minimum equal budgets", 1, 1, nil},
		{"attempt below operation", 2, 1, nil},
		{"attempt above operation", 1, 2, core.ErrAWSIdentityContract},
		{"maximum equal budgets", math.MaxInt64, math.MaxInt64, nil},
		{"attempt one below maximum", math.MaxInt64, math.MaxInt64 - 1, nil},
		{"attempt one above allowed maximum", math.MaxInt64 - 1, math.MaxInt64, core.ErrAWSIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			operation, opErr := temporal.DurationFromNanoseconds(tc.operation)
			attempt, attemptErr := temporal.DurationFromNanoseconds(tc.attempt)
			if opErr != nil || attemptErr != nil {
				t.Fatalf("duration fixture errors = (%v,%v), want nil", opErr, attemptErr)
			}
			policy := Policy{OperationTimeout: operation, AttemptTimeout: attempt}
			if err := policy.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Policy.Validate = %v, want %v", err, tc.wantErr)
			}
			projected := policy.exchange()
			want := exchange.OperationPolicy{OperationTimeout: operation, AttemptTimeout: attempt, Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
			if projected != want {
				t.Fatalf("policy projection = %+v, want %+v", projected, want)
			}
		})
	}
	got := mustAWSPolicy(t)
	want, err := temporal.DurationFromSeconds(DefaultTimeoutSeconds)
	if err != nil || got.OperationTimeout != want || got.AttemptTimeout != want {
		t.Fatalf("DefaultPolicy = %+v, want both %v (error %v)", got, want, err)
	}
}

func TestAWSZeroCapabilitiesAndRedaction(t *testing.T) {
	t.Parallel()
	client, err := NewClient(exchange.Client{})
	if client != (Client{}) || !errors.Is(err, core.ErrAWSIdentityContract) || !errors.Is(err, core.ErrExchangeContract) {
		t.Fatalf("NewClient zero = (%v,%v), want zero and both typed identities", client, err)
	}
	if err := (Request{}).Validate(); !errors.Is(err, core.ErrAWSIdentityContract) {
		t.Fatalf("zero Request.Validate = %v, want AWS contract", err)
	}
	if value, err := (Token{}).BearerValue(); value != "" || !errors.Is(err, core.ErrAWSIdentityContract) {
		t.Fatalf("zero BearerValue = (%q,%v), want empty typed refusal", value, err)
	}
	token, err := newToken(awsTestBearer)
	if err != nil {
		t.Fatalf("newToken fixture error = %v, want nil", err)
	}
	audience := mustAWSAudience(t)
	request, err := NewRequest(RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion), Audience: audience, Policy: mustAWSPolicy(t)})
	if err != nil {
		t.Fatalf("NewRequest fixture error = %v, want nil", err)
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%20s", "%.3s"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			for _, got := range []string{fmt.Sprintf(format, token), fmt.Sprintf(format, Token{}), fmt.Sprintf(format, request), fmt.Sprintf(format, Request{})} {
				if got != core.RedactedValueText {
					t.Fatalf("format %q = %q, want %q", format, got, core.RedactedValueText)
				}
			}
		})
	}
}

// Fill an exact byte extent with whole UTF-8 scalars and an ASCII remainder.
// Boundary rows remain exact if the owning byte ceiling changes.
func awsUTF8Extent(scalar string, extent int) string {
	return strings.Repeat(scalar, extent/len(scalar)) + strings.Repeat("a", extent%len(scalar))
}
