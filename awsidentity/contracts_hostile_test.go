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
		wantErr     error
		name, input string
	}{
		{name: "empty below minimum", input: "", wantErr: core.ErrAWSIdentityContract},
		{name: "one ASCII byte", input: "a", wantErr: nil}, {name: "two ASCII bytes", input: "ab", wantErr: nil},
		{name: "one below maximum", input: strings.Repeat("a", AudienceMaximumBytes-1), wantErr: nil},
		{name: "exact maximum", input: strings.Repeat("z", AudienceMaximumBytes), wantErr: nil},
		{name: "one above maximum", input: strings.Repeat("a", AudienceMaximumBytes+1), wantErr: core.ErrAWSIdentityContract},
		{name: "multibyte exact byte maximum", input: awsUTF8Extent("é", AudienceMaximumBytes), wantErr: nil},
		{name: "multibyte one byte above maximum", input: awsUTF8Extent("é", AudienceMaximumBytes+1), wantErr: core.ErrAWSIdentityContract},
		{name: "truncated two byte sequence", input: "\xc3", wantErr: core.ErrAWSIdentityContract},
		{name: "truncated three byte sequence", input: "\xe2\x82", wantErr: core.ErrAWSIdentityContract},
		{name: "orphan continuation", input: "\x80", wantErr: core.ErrAWSIdentityContract},
		{name: "overlong representation", input: "\xc0\xaf", wantErr: core.ErrAWSIdentityContract},
		{name: "surrogate representation", input: "\xed\xa0\x80", wantErr: core.ErrAWSIdentityContract},
		{name: "above Unicode scalar ceiling", input: "\xf4\x90\x80\x80", wantErr: core.ErrAWSIdentityContract},
		{name: "maximum Unicode scalar", input: "\U0010ffff", wantErr: nil},
		{name: "embedded zero remains opaque", input: "a\x00b", wantErr: nil},
		{name: "whitespace is not product policy", input: " \t\n", wantErr: nil},
		{name: "combining sequence remains exact", input: "e\u0301", wantErr: nil},
		{name: "query delimiters remain audience data", input: "a&b=c+%/", wantErr: nil},
		{name: "last one-byte scalar", input: "\u007f", wantErr: nil},
		{name: "first two-byte scalar", input: "\u0080", wantErr: nil},
		{name: "last two-byte scalar", input: "\u07ff", wantErr: nil},
		{name: "first three-byte scalar", input: "\u0800", wantErr: nil},
		{name: "last scalar before surrogate range", input: "\ud7ff", wantErr: nil},
		{name: "first scalar after surrogate range", input: "\ue000", wantErr: nil},
		{name: "last three-byte scalar", input: "\uffff", wantErr: nil},
		{name: "first four-byte scalar", input: "\U00010000", wantErr: nil},
		{name: "replacement scalar is valid input data", input: "\ufffd", wantErr: nil},
		{name: "three-byte text one below byte ceiling", input: awsUTF8Extent("€", AudienceMaximumBytes-1), wantErr: nil},
		{name: "three-byte text exactly at byte ceiling", input: awsUTF8Extent("€", AudienceMaximumBytes), wantErr: nil},
		{name: "three-byte text one above byte ceiling", input: awsUTF8Extent("€", AudienceMaximumBytes+1), wantErr: core.ErrAWSIdentityContract},
		{name: "four-byte text one below byte ceiling", input: awsUTF8Extent("😀", AudienceMaximumBytes-1), wantErr: nil},
		{name: "four-byte text exactly at byte ceiling", input: awsUTF8Extent("😀", AudienceMaximumBytes), wantErr: nil},
		{name: "four-byte text one above byte ceiling", input: awsUTF8Extent("😀", AudienceMaximumBytes+1), wantErr: core.ErrAWSIdentityContract},
		{name: "invalid continuation at exact byte ceiling", input: strings.Repeat("a", AudienceMaximumBytes-3) + "\xe2\x28\xa1", wantErr: core.ErrAWSIdentityContract},
		{name: "truncated final four-byte scalar", input: "\xf0\x90\x80", wantErr: core.ErrAWSIdentityContract},
		{name: "overlong three-byte encoding", input: "\xe0\x80\xaf", wantErr: core.ErrAWSIdentityContract},
		{name: "overlong four-byte encoding", input: "\xf0\x80\x80\xaf", wantErr: core.ErrAWSIdentityContract},
		{name: "unsupported five-byte lead", input: "\xf8\x88\x80\x80\x80", wantErr: core.ErrAWSIdentityContract},
		{name: "invalid byte after long ASCII prefix", input: strings.Repeat("a", AudienceMaximumBytes-2) + "\xff" + "b", wantErr: core.ErrAWSIdentityContract},
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
		wantErr     error
		name, input string
	}{
		{name: "empty below minimum", input: "", wantErr: core.ErrAWSIdentityContract},
		{name: "smallest uppercase byte", input: "A", wantErr: nil}, {name: "largest uppercase byte", input: "Z", wantErr: nil},
		{name: "smallest lowercase byte", input: "a", wantErr: nil}, {name: "largest lowercase byte", input: "z", wantErr: nil},
		{name: "smallest digit", input: "0", wantErr: nil}, {name: "largest digit", input: "9", wantErr: nil},
		{name: "dash is data", input: "-", wantErr: nil}, {name: "dot is data", input: ".", wantErr: nil}, {name: "underscore is data", input: "_", wantErr: nil},
		{name: "tilde is data", input: "~", wantErr: nil}, {name: "plus is data", input: "+", wantErr: nil}, {name: "slash is data", input: "/", wantErr: nil},
		{name: "single trailing padding", input: "a=", wantErr: nil}, {name: "multiple trailing padding", input: "a===", wantErr: nil},
		{name: "one below byte maximum", input: strings.Repeat("a", TokenMaximumBytes-1), wantErr: nil},
		{name: "at byte maximum", input: strings.Repeat("a", TokenMaximumBytes), wantErr: nil},
		{name: "padding at byte maximum", input: "a" + strings.Repeat("=", TokenMaximumBytes-1), wantErr: nil},
		{name: "one above byte maximum", input: strings.Repeat("a", TokenMaximumBytes+1), wantErr: core.ErrAWSIdentityContract},
		{name: "padding exceeds byte maximum", input: "a" + strings.Repeat("=", TokenMaximumBytes), wantErr: core.ErrAWSIdentityContract},
		{name: "padding cannot start", input: "=a", wantErr: core.ErrAWSIdentityContract},
		{name: "padding cannot be whole token", input: "=", wantErr: core.ErrAWSIdentityContract},
		{name: "data cannot follow padding", input: "a=b", wantErr: core.ErrAWSIdentityContract},
		{name: "punctuation cannot follow padding", input: "a=+", wantErr: core.ErrAWSIdentityContract},
		{name: "space cannot prefix", input: " a", wantErr: core.ErrAWSIdentityContract},
		{name: "space cannot suffix", input: "a ", wantErr: core.ErrAWSIdentityContract},
		{name: "CR header injection", input: "a\rb", wantErr: core.ErrAWSIdentityContract},
		{name: "LF header injection", input: "a\nb", wantErr: core.ErrAWSIdentityContract},
		{name: "tab separator", input: "a\tb", wantErr: core.ErrAWSIdentityContract},
		{name: "embedded zero", input: "a\x00b", wantErr: core.ErrAWSIdentityContract},
		{name: "Unicode lookalike", input: "а", wantErr: core.ErrAWSIdentityContract},
		{name: "invalid UTF8", input: "\xff", wantErr: core.ErrAWSIdentityContract},
		{name: "before uppercase range", input: "@", wantErr: core.ErrAWSIdentityContract},
		{name: "after uppercase range", input: "[", wantErr: core.ErrAWSIdentityContract},
		{name: "before lowercase range", input: "`", wantErr: core.ErrAWSIdentityContract},
		{name: "after lowercase range", input: "{", wantErr: core.ErrAWSIdentityContract},
		{name: "after digit range", input: ":", wantErr: core.ErrAWSIdentityContract},
		{name: "quoted token", input: "\"a\"", wantErr: core.ErrAWSIdentityContract},
		{name: "comma separated tokens", input: "a,b", wantErr: core.ErrAWSIdentityContract},
		{name: "opaque non JWT token is admitted", input: "not-a-jwt", wantErr: nil},
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
		wantErr            error
		name               string
		operation, attempt int64
	}{
		{name: "both zero", operation: 0, attempt: 0, wantErr: core.ErrAWSIdentityContract},
		{name: "missing operation", operation: 0, attempt: 1, wantErr: core.ErrAWSIdentityContract},
		{name: "missing attempt", operation: 1, attempt: 0, wantErr: core.ErrAWSIdentityContract},
		{name: "minimum equal budgets", operation: 1, attempt: 1, wantErr: nil},
		{name: "attempt below operation", operation: 2, attempt: 1, wantErr: nil},
		{name: "attempt above operation", operation: 1, attempt: 2, wantErr: core.ErrAWSIdentityContract},
		{name: "maximum equal budgets", operation: math.MaxInt64, attempt: math.MaxInt64, wantErr: nil},
		{name: "attempt one below maximum", operation: math.MaxInt64, attempt: math.MaxInt64 - 1, wantErr: nil},
		{name: "attempt one above allowed maximum", operation: math.MaxInt64 - 1, attempt: math.MaxInt64, wantErr: core.ErrAWSIdentityContract},
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
