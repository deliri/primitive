package awsidentity

import (
	"errors"
	"net/url"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAWSRequestRejectsMalformedQueryWithoutDiscardingFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		suffix string
	}{
		{name: "invalid escape in additional name", suffix: "&%zz=value"},
		{name: "invalid escape in additional value", suffix: "&FutureParameter=%zz"},
		{name: "semicolon in additional pair", suffix: "&FutureParameter=value;hidden=value"},
		{name: "malformed duplicate required field", suffix: "&" + amazonActionQuery + "=%zz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			audience := mustAWSAudience(t)
			input := RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion) + tc.suffix, Audience: audience, Policy: mustAWSPolicy(t)}
			parsed, err := url.Parse(input.SignedURL)
			if err != nil {
				t.Fatalf("url.Parse fixture error = %v, want query-level failure", err)
			}
			_, parseErr := url.ParseQuery(parsed.RawQuery)
			if parseErr == nil {
				t.Fatal("url.ParseQuery fixture error = nil, want malformed query")
			}
			got, gotErr := NewRequest(input)
			if !errors.Is(gotErr, core.ErrAWSIdentityContract) || got != (Request{}) {
				t.Fatalf("NewRequest(malformed query) = (%v, %v), want zero and %v", got, gotErr, core.ErrAWSIdentityContract)
			}
			var wantEscape, gotEscape url.EscapeError
			if errors.As(parseErr, &wantEscape) && (!errors.As(gotErr, &gotEscape) || gotEscape != wantEscape) {
				t.Fatalf("query escape cause = %v, want typed stdlib %v", gotErr, wantEscape)
			}
			if err := input.Validate(); !errors.Is(err, core.ErrAWSIdentityContract) {
				t.Fatalf("RequestInput.Validate(malformed query) error = %v, want %v", err, core.ErrAWSIdentityContract)
			}
		})
	}
}
