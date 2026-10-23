package exchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestOAuthSDKRefusalCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	if SanitizeOAuthTokenError(nil) != nil || !errors.Is(SanitizeOAuthTokenError(context.Canceled), context.Canceled) || !errors.Is(SanitizeOAuthTokenError(io.ErrUnexpectedEOF), io.ErrUnexpectedEOF) {
		t.Fatalf("SDK sanitizer native causes = (%v,%v,%v), want nil/canceled/unexpectedEOF", SanitizeOAuthTokenError(nil), SanitizeOAuthTokenError(context.Canceled), SanitizeOAuthTokenError(io.ErrUnexpectedEOF))
	}
	request, err := http.NewRequest("POST", "https://provider.example/token", strings.NewReader("client_secret=private&code=private&code_verifier=private"))
	if err != nil {
		t.Fatal(err)
	}
	original := &oauth2.RetrieveError{Response: &http.Response{StatusCode: 400, Status: "400 untrusted private text", Request: request, Header: http.Header{"Secret": []string{"private"}}}, Body: []byte("private"), ErrorCode: "invalid_grant", ErrorDescription: "private", ErrorURI: "https://provider.example/private"}
	result := SanitizeOAuthTokenError(original)
	var got *oauth2.RetrieveError
	if !errors.As(result, &got) || got == original || got.Response == original.Response || got.Response.Request != nil || got.Response.Header != nil || len(got.Body) != 0 || got.ErrorDescription != "" || got.ErrorURI != "" || got.ErrorCode != "invalid_grant" || got.Response.Status != "400 Bad Request" {
		t.Fatalf("sanitized native refusal = %+v, want preserved safe class/status and severed secret custody", got)
	}
	if original.Response.Request.GetBody == nil || len(original.Body) != 7 {
		t.Fatalf("caller request replay = %t, body bytes = %d, want retained replay and7bytes", original.Response.Request.GetBody != nil, len(original.Body))
	}
}
func FuzzOAuthSDKErrorProjectionSemanticClosure(f *testing.F) {
	f.Add("invalid_grant", 400)
	f.Add("private-client-secret", 401)
	f.Add("", 200)
	f.Fuzz(func(t *testing.T, text string, status int) {
		source := &oauth2.RetrieveError{Response: &http.Response{StatusCode: status, Request: &http.Request{}, Header: http.Header{"Private": []string{text}}}, Body: []byte(text), ErrorCode: text, ErrorDescription: text, ErrorURI: text}
		result := SanitizeOAuthTokenError(source)
		var got *oauth2.RetrieveError
		if !errors.As(result, &got) || got == source || got.Response == nil || got.Response.StatusCode != status || got.Response.Request != nil || got.Response.Header != nil || len(got.Body) != 0 || got.ErrorDescription != "" || got.ErrorURI != "" {
			t.Fatalf("SDK refusal projection retained custody or lost status: %+v", got)
		}
		allowed := false
		for _, value := range []string{"", "invalid_request", "invalid_client", "invalid_grant", "unauthorized_client", "unsupported_grant_type", "invalid_scope", "access_denied", "server_error", "temporarily_unavailable", "bad_verification_code", "incorrect_client_credentials", "redirect_uri_mismatch", "application_suspended"} {
			if got.ErrorCode == value {
				allowed = true
			}
		}
		if !allowed {
			t.Fatalf("SDK refusal retained unknown provider text %q", got.ErrorCode)
		}
	})
}
