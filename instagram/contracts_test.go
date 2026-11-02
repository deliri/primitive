package instagram

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// UserID admits Meta's numeric Graph identity and nothing that could change
// the request path. Families: length, digit range, separators, encoding, case.
func TestParseUserIDContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		// Valid.
		{"the real Off Grid account", "17841432921003907", true},
		{"one digit", "7", true},
		{"a Facebook page sized id", "102938475610293", true},
		{"leading zeros are digits", "000123", true},
		{"every digit once", "0123456789", true},
		{"all zeros", "0000", true},
		{"all nines", "9999", true},
		{"thirty one digits", strings.Repeat("1", 31), true},
		{"sixteen digits", "1234567890123456", true},
		{"two digits", "42", true},
		// Rejected.
		{"empty", "", false},
		{"letter suffix", "17841432921003907a", false},
		{"negative", "-1", false},
		{"decimal point", "1.5", false},
		{"path traversal", "1/../2", false},
		{"query injection", "1?fields=access_token", false},
		{"percent encoding", "1%2F2", false},
		{"inner space", "123 456", false},
		{"scientific notation", "1e10", false},
		{"hex prefix", "0x1F", false},
		// Boundary: length.
		{"thirty one digits again at the edge below", strings.Repeat("2", 31), true},
		{"thirty two digits, the maximum", strings.Repeat("9", 32), true},
		{"thirty three digits", strings.Repeat("9", 33), false},
		{"64 KiB of digits", strings.Repeat("1", 64<<10), false},
		// Boundary: the ASCII neighbours of the digit range.
		{"slash, one below zero", "1/", false},
		{"colon, one above nine", "1:", false},
		{"zero, lowest digit", "10", true},
		{"nine, highest digit", "19", true},
		// Boundary: whitespace placement.
		{"leading space", " 1", false},
		{"trailing space", "1 ", false},
		{"trailing newline", "1\n", false},
		{"tab", "1\t", false},
		// Boundary: non-ASCII digits that look like digits.
		{"fullwidth one", "１", false},
		{"Arabic-Indic one", "١", false},
		{"superscript one", "¹", false},
		{"NUL byte", "1\x00", false},
		// Boundary: sign and separator characters.
		{"plus sign", "+1", false},
		{"underscore", "1_2", false},
		{"comma", "1,2", false},
		{"ampersand", "1&limit=50", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseUserID(tc.raw)
			if tc.want != (err == nil) {
				t.Fatalf("ParseUserID(%q) error = %v, want valid=%v", tc.raw, err, tc.want)
			}
			if err != nil {
				if !errors.Is(err, core.ErrInstagramContract) || got != (UserID{}) {
					t.Fatalf("refusal kept %q or lost its family: %v", got.String(), err)
				}
				return
			}
			if got.String() != tc.raw || got.Validate() != nil {
				t.Fatalf("ParseUserID(%q) changed the identity to %q", tc.raw, got.String())
			}
		})
	}
	if !errors.Is((UserID{}).Validate(), core.ErrInstagramContract) {
		t.Fatal("the zero UserID validated")
	}
}

// Credential custody admits a token-shaped value and never shows it.
// Families: length, custody maximum, character range, whitespace, encoding.
func TestCredentialContract(t *testing.T) {
	t.Parallel()
	minimum, maximum := core.InstagramCredentialMinimumBytes, core.InstagramCredentialCustodyMaximumBytes
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		// Valid.
		{"a real-shaped Facebook-login token", testToken, true},
		{"an Instagram-login shaped token", "IGAA" + strings.Repeat("Zx9", 60), true},
		{"punctuation Meta uses", strings.Repeat("a", minimum) + "-_.~", true},
		{"all digits", strings.Repeat("7", minimum), true},
		{"all upper case", strings.Repeat("Q", 200), true},
		{"symbols across the range", strings.Repeat("!#$%&*+=?@^|", 4), true},
		{"a 1 KiB token", strings.Repeat("k", 1024), true},
		{"mixed case and digits", strings.Repeat("aB3", 40), true},
		{"quote characters", strings.Repeat("a", minimum) + "\"'`", true},
		{"brackets", strings.Repeat("a", minimum) + "[]{}()<>", true},
		// Rejected.
		{"empty", "", false},
		{"a short password-like value", "hunter2", false},
		{"a space inside", testToken[:20] + " " + testToken[21:], false},
		{"a trailing newline from a pasted secret", testToken + "\n", false},
		{"a carriage return", testToken + "\r", false},
		{"a tab", testToken + "\t", false},
		{"a non-ASCII letter", testToken + "é", false},
		{"a NUL byte", testToken + "\x00", false},
		{"a DEL byte", testToken + "\x7f", false},
		{"a bearer prefix pasted with the token", "Bearer " + testToken, false},
		// Boundary: minimum length.
		{"one byte under the minimum", strings.Repeat("a", minimum-1), false},
		{"exactly the minimum", strings.Repeat("a", minimum), true},
		{"one byte over the minimum", strings.Repeat("a", minimum+1), true},
		{"a single byte", "a", false},
		// Boundary: custody maximum.
		{"one byte under the maximum", strings.Repeat("a", maximum-1), true},
		{"exactly the maximum", strings.Repeat("a", maximum), true},
		{"one byte over the maximum", strings.Repeat("a", maximum+1), false},
		{"one MiB", strings.Repeat("a", 1<<20), false},
		// Boundary: the printable ASCII range.
		{"exclamation, lowest visible", strings.Repeat("!", minimum), true},
		{"tilde, highest visible", strings.Repeat("~", minimum), true},
		{"space, one below visible", strings.Repeat("a", minimum) + " ", false},
		{"DEL, one above visible", strings.Repeat("a", minimum) + "\x7f", false},
		// Boundary: whitespace placement.
		{"leading space", " " + testToken, false},
		{"only spaces", strings.Repeat(" ", minimum), false},
		{"vertical tab", testToken + "\v", false},
		{"form feed", testToken + "\f", false},
		// Boundary: encodings that are not ASCII.
		{"invalid UTF-8", testToken + "\xff", false},
		{"a zero-width space", testToken + "​", false},
		{"a fullwidth letter", testToken + "ａ", false},
		{"a high control byte", testToken + "\x85", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			credential, err := ParseCredential([]byte(tc.raw))
			if tc.want != (err == nil) {
				t.Fatalf("ParseCredential error = %v, want valid=%v", err, tc.want)
			}
			if err != nil {
				if !errors.Is(err, core.ErrInstagramContract) || credential.Validate() == nil {
					t.Fatalf("refusal kept material or lost its family: %v", err)
				}
				return
			}
			if visible := fmt.Sprintf("%v|%+v|%s|%q|%#v", credential, credential, credential, credential, credential); strings.Contains(visible, tc.raw[:minimum]) {
				t.Fatal("the credential formatted its value")
			}
		})
	}
}

func TestCredentialCustodyLifecycle(t *testing.T) {
	t.Parallel()
	source := []byte(testToken)
	credential, err := ParseCredential(source)
	if err != nil {
		t.Fatal(err)
	}
	clear(source)
	if credential.Validate() != nil {
		t.Fatal("the credential shares the caller's buffer; clearing the source broke it")
	}
	if err := credential.Close(); err != nil || credential.Validate() == nil {
		t.Fatalf("Close left a usable credential: %v", err)
	}
	if err := credential.Close(); err != nil {
		t.Fatalf("closing twice must stay safe: %v", err)
	}
	var missing *Credential
	if !errors.Is(missing.Close(), core.ErrInstagramContract) {
		t.Fatal("closing a nil credential must refuse")
	}
}

// The request owns the only caller-chosen facts: the account and the policy.
// Meta documents no page-size parameter, so the URL carries none.
func TestMediaListRequestContract(t *testing.T) {
	t.Parallel()
	user, policy := testUser(t), testPolicy(t)
	one, err := ParseUserID("1")
	if err != nil {
		t.Fatal(err)
	}
	longest, err := ParseUserID(strings.Repeat("9", 32))
	if err != nil {
		t.Fatal(err)
	}
	reversed := policy
	reversed.AttemptTimeout, reversed.OperationTimeout = policy.OperationTimeout, policy.AttemptTimeout
	unredirected := policy
	unredirected.Redirect = exchange.RedirectPolicy{}
	cases := []struct {
		name    string
		request MediaListRequest
		want    string
	}{
		{"the real account", MediaListRequest{User: user, Policy: policy}, "https://graph.facebook.com/v26.0/17841432921003907/media?fields=id,media_type,media_url,thumbnail_url,permalink,caption,timestamp"},
		{"the shortest account", MediaListRequest{User: one, Policy: policy}, "https://graph.facebook.com/v26.0/1/media?fields=id,media_type,media_url,thumbnail_url,permalink,caption,timestamp"},
		{"the longest account", MediaListRequest{User: longest, Policy: policy}, "https://graph.facebook.com/v26.0/" + strings.Repeat("9", 32) + "/media?fields=id,media_type,media_url,thumbnail_url,permalink,caption,timestamp"},
		{"missing account", MediaListRequest{Policy: policy}, ""},
		{"missing policy", MediaListRequest{User: user}, ""},
		{"an attempt longer than the operation", MediaListRequest{User: user, Policy: reversed}, ""},
		{"no redirect decision", MediaListRequest{User: user, Policy: unredirected}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			endpoint, err := tc.request.Endpoint()
			if tc.want == "" {
				if !errors.Is(err, core.ErrInstagramContract) || tc.request.Validate() == nil {
					t.Fatalf("admitted: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			address := endpoint.HTTPURL()
			if address.String() != tc.want || strings.Contains(address.RawQuery, "access_token") || strings.Contains(address.RawQuery, "limit") {
				t.Fatalf("endpoint %q, want %q", address.String(), tc.want)
			}
		})
	}
}
