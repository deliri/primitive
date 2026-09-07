package awsidentity

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAWSRequestInputRefusesInvalidOwnedFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		change   func(*RequestInput)
		wantCore bool
	}{
		{"unset audience", func(i *RequestInput) { i.Audience = Audience{} }, false},
		{"oversized audience", func(i *RequestInput) { i.Audience = Audience{value: strings.Repeat("a", AudienceMaximumBytes+1)} }, false},
		{"invalid UTF8 audience", func(i *RequestInput) { i.Audience = Audience{value: "\xff"} }, false},
		{"unset policy", func(i *RequestInput) { i.Policy = Policy{} }, false},
		{"empty URL", func(i *RequestInput) { i.SignedURL = "" }, true},
		{"relative URL", func(i *RequestInput) { i.SignedURL = "/identity" }, true},
		{"opaque URL", func(i *RequestInput) { i.SignedURL = core.SchemeHTTPS + ":opaque" }, true},
		{"credential bearing URL", func(i *RequestInput) {
			u, _ := url.Parse(i.SignedURL)
			u.User = url.UserPassword("user", "secret")
			i.SignedURL = u.String()
		}, true},
		{"fragment URL", func(i *RequestInput) { i.SignedURL += "#secret" }, true},
		{"empty fragment delimiter", func(i *RequestInput) { i.SignedURL += "#" }, true},
		{"invalid host escape", func(i *RequestInput) { i.SignedURL = core.SchemeHTTPS + "://%zz/" }, true},
		{"encoded slash is not literal root", func(i *RequestInput) {
			u, _ := url.Parse(i.SignedURL)
			u.RawPath = "/%2f"
			u.Path = "//"
			i.SignedURL = u.String()
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			audience := mustAWSAudience(t)
			input := RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion), Audience: audience, Policy: mustAWSPolicy(t)}
			original := input
			tc.change(&input)
			if input == original {
				t.Fatal("request ingress mutation unchanged, want edited field")
			}
			got, err := NewRequest(input)
			if got != (Request{}) || !errors.Is(err, core.ErrAWSIdentityContract) || !errors.Is(input.Validate(), core.ErrAWSIdentityContract) {
				t.Fatalf("NewRequest invalid ingress = (%v,%v), want zero typed refusal", got, err)
			}
			if tc.wantCore {
				if !errors.Is(err, core.ErrPrimitiveContract) {
					t.Fatalf("endpoint error = %v, want core identity preserved", err)
				}
				if _, coreErr := core.ParseHTTPEndpoint(input.SignedURL); coreErr == nil {
					t.Fatal("core endpoint fixture error = nil, want invalid endpoint")
				}
			}
			if input.Audience == original.Audience && input.Policy == original.Policy {
				var failure requestError
				if !errors.As(err, &failure) {
					t.Fatalf("request URL refusal = %v, want requestError", err)
				}
				for _, format := range []string{"%v", "%+v", "%#v", "%q"} {
					if text := fmt.Sprintf(format, err); text != requestFailureText {
						t.Fatalf("URL refusal format = %q, want safe diagnostic", text)
					}
				}
			}
		})
	}
}

func TestAWSOpaqueCapabilitiesCannotLeakThroughJSON(t *testing.T) {
	t.Parallel()
	request := awsRequest(t)
	token, err := newToken(awsTestBearer)
	if err != nil {
		t.Fatalf("token fixture error = %v, want nil", err)
	}
	requestJSON, requestErr := json.Marshal(request)
	tokenJSON, tokenErr := json.Marshal(token)
	var requestSemantic, tokenSemantic *json.SemanticError
	if len(requestJSON) != 0 || len(tokenJSON) != 0 || !errors.As(requestErr, &requestSemantic) || !errors.As(tokenErr, &tokenSemantic) {
		t.Fatalf("opaque JSON = (%s,%s,%v,%v), want no bytes and typed stdlib refusal", requestJSON, tokenJSON, requestErr, tokenErr)
	}
	if requestSemantic.GoType != reflect.TypeFor[Request]() || tokenSemantic.GoType != reflect.TypeFor[Token]() {
		t.Fatalf("JSON refusal types = (%v,%v), want Request and Token", requestSemantic.GoType, tokenSemantic.GoType)
	}
}
