package exchange

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/oauth2"
)

// SanitizeOAuthTokenError preserves the SDK's typed refusal and safe status
// while severing request/response custody. RetrieveError.Request.GetBody can
// replay a confidential client secret, grant code and proof-key verifier.
// Provider descriptions, URIs and bodies may echo those same secrets.
// Other native causes pass through unchanged for errors.Is/errors.As.
// Source: https://pkg.go.dev/golang.org/x/oauth2#RetrieveError
func SanitizeOAuthTokenError(err error) error {
	if err == nil {
		return nil
	}
	native, found := errors.AsType[*oauth2.RetrieveError](err)
	if !found {
		return err
	}
	if native == nil {
		return errors.Join(core.ErrOAuthContract, core.ErrExchangeContract)
	}
	status := 0
	if native.Response != nil {
		status = native.Response.StatusCode
	}
	return &oauth2.RetrieveError{Response: &http.Response{StatusCode: status, Status: strconv.Itoa(status) + " " + http.StatusText(status)}, ErrorCode: safeOAuthErrorCode(native.ErrorCode)}
}

// RFC6749 token errors plus GitHub's documented exchange refusals are closed
// diagnostic data. Unknown provider text is never copied into the error graph.
// Sources: https://www.rfc-editor.org/rfc/rfc6749#section-5.2
// https://docs.github.com/en/apps/oauth-apps/maintaining-oauth-apps/troubleshooting-oauth-app-access-token-request-errors
func safeOAuthErrorCode(value string) string {
	switch value {
	case "invalid_request", "invalid_client", "invalid_grant", "unauthorized_client", "unsupported_grant_type", "invalid_scope", "access_denied", "server_error", "temporarily_unavailable", "bad_verification_code", "incorrect_client_credentials", "redirect_uri_mismatch", "application_suspended":
		return value
	default:
		return ""
	}
}
