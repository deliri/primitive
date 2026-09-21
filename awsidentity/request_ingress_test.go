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
		change   func(*RequestInput)
		name     string
		wantCore bool
	}{
		{name: "unset audience", change: func(i *RequestInput) { i.Audience = Audience{} }, wantCore: false},
		{name: "oversized audience", change: func(i *RequestInput) { i.Audience = Audience{value: strings.Repeat("a", AudienceMaximumBytes+1)} }, wantCore: false},
		{name: "invalid UTF8 audience", change: func(i *RequestInput) { i.Audience = Audience{value: "\xff"} }, wantCore: false},
		{name: "unset policy", change: func(i *RequestInput) { i.Policy = Policy{} }, wantCore: false},
		{name: "empty URL", change: func(i *RequestInput) { i.SignedURL = "" }, wantCore: true},
		{name: "relative URL", change: func(i *RequestInput) { i.SignedURL = "/identity" }, wantCore: true},
		{name: "opaque URL", change: func(i *RequestInput) { i.SignedURL = core.SchemeHTTPS + ":opaque" }, wantCore: true},
		{name: "credential bearing URL", change: func(i *RequestInput) {
			u, _ := url.Parse(i.SignedURL)
			u.User = url.UserPassword("user", "secret")
			i.SignedURL = u.String()
		}, wantCore: true},
		{name: "fragment URL", change: func(i *RequestInput) { i.SignedURL += "#secret" }, wantCore: true},
		{name: "empty fragment delimiter", change: func(i *RequestInput) { i.SignedURL += "#" }, wantCore: true},
		{name: "invalid host escape", change: func(i *RequestInput) { i.SignedURL = core.SchemeHTTPS + "://%zz/" }, wantCore: true},
		{name: "encoded slash is not literal root", change: func(i *RequestInput) {
			u, _ := url.Parse(i.SignedURL)
			u.RawPath = "/%2f"
			u.Path = "//"
			i.SignedURL = u.String()
		}, wantCore: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			audience := mustAWSAudience(t)
			input := RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion), Audience: audience, Policy: mustAWSPolicy(t)}
			original := input
			tc.change(&input)
			if input == original {
				t.Fatalf("request mutation=%+v, want different from %+v", input, original)
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
				if _, ok := errors.AsType[requestError](err); !ok {
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
