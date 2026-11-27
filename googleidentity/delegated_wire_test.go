package googleidentity

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	wireFixtureIssuer  = "mail-reader@fixture-project.iam.gserviceaccount.com"
	wireFixtureSubject = "reader@example.invalid"
	wireFixtureToken   = "fixture-provider-bearer"
	wireFixtureKey     = "f0f1cafe"
	wireFixtureTime    = 1795708800000000000
)

type signReplyFixture struct {
	KeyID string `json:"keyId"`
	JWT   string `json:"signedJwt"`
}

type tokenReplyFixture struct {
	Token   string `json:"access_token"`
	Type    string `json:"token_type"`
	Expires uint64 `json:"expires_in"`
	Scope   string `json:"scope,omitempty"`
}

func providerFixtureJSON[T any](t testing.TB, doc T) []byte {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func providerClaimsFixture(t testing.TB) delegationClaims {
	t.Helper()
	signer, signerErr := ParseServiceAccountEmail(wireFixtureIssuer)
	subject, subjectErr := ParseWorkspaceSubject(wireFixtureSubject)
	policy, pErr := DefaultPolicy()
	if err := errors.Join(signerErr, subjectErr, pErr); err != nil {
		t.Fatal(err)
	}
	at := temporal.InstantFromNanoseconds(wireFixtureTime)
	request := DelegatedAccessRequest{Signer: signer, Subject: subject, Scope: DelegatedScopeGmailReadOnly, ObservedAt: at, Policy: policy}
	claims, err := request.claims()
	if err != nil || claims.Issuer != wireFixtureIssuer || claims.Subject != (optionalDelegationText{value: wireFixtureSubject, present: true}) ||
		claims.IssuedAtSeconds != 1795708800 || claims.ExpiresAtSeconds != 1795709100 {
		t.Fatalf("claims = %v/%v, want the configured identities and five-minute assertion", claims, err)
	}
	return claims
}

func providerAssertionFixture(t testing.TB, claims delegationClaims, kid string) signReplyFixture {
	t.Helper()
	header := providerFixtureJSON(t, googleCloudJWTHeader{Algorithm: googleCloudSigningAlgorithmRS256, KeyID: wireFixtureKey})
	payload := providerFixtureJSON(t, claims)
	jwt := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".c3ludGhldGlj"
	return signReplyFixture{KeyID: kid, JWT: jwt}
}

func TestDelegatedTokenReplyBoundaries(t *testing.T) {
	t.Parallel()
	start := temporal.InstantFromNanoseconds(wireFixtureTime)
	canonical := tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 3600, Scope: GmailReadOnlyScope}
	doc := providerFixtureJSON(t, canonical)
	cases := []struct {
		name        string
		body        []byte
		wantSeconds uint64
		wantErr     error
	}{
		{name: "quarter-hour short-lived bearer", body: providerFixtureJSON(t, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 900}), wantSeconds: 900},
		{name: "one second minimum with explicit scope", body: providerFixtureJSON(t, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 1, Scope: GmailReadOnlyScope}), wantSeconds: 1},
		{name: "one second under maximum", body: providerFixtureJSON(t, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 3599}), wantSeconds: 3599},
		{name: "maximum lifetime and exact Gmail scope", body: doc, wantSeconds: 3600},
		{name: "whole response exactly at limit", body: append(bytes.Repeat([]byte{' '}, delegationWireMaximum-len(doc)), doc...), wantSeconds: 3600},
		{name: "whole response one byte below limit", body: append(bytes.Repeat([]byte{' '}, delegationWireMaximum-len(doc)-1), doc...), wantSeconds: 3600},
		{name: "whole response one byte over limit", body: append(bytes.Repeat([]byte{' '}, delegationWireMaximum-len(doc)+1), doc...), wantErr: core.ErrExchangeBodyLimit},
		{name: "bearer extent exactly at limit", body: providerFixtureJSON(t, tokenReplyFixture{Token: strings.Repeat("z", exchange.BearerAuthorizationTokenMaximumBytes), Type: "Bearer", Expires: 60}), wantSeconds: 60},
		{name: "bearer extent one over limit", body: providerFixtureJSON(t, tokenReplyFixture{Token: strings.Repeat("z", exchange.BearerAuthorizationTokenMaximumBytes+1), Type: "Bearer", Expires: 60}), wantErr: core.ErrGoogleDelegationResponse},
		{name: "token68 alphabet and trailing padding", body: providerFixtureJSON(t, tokenReplyFixture{Token: "AZaz09-._~+/==", Type: "Bearer", Expires: 20}), wantSeconds: 20},
		{name: "reordered keys with whitespace", body: []byte("{\n\"expires_in\":55, \"token_type\":\"Bearer\", \"access_token\":\"fixture-provider-bearer\"}"), wantSeconds: 55},
		{name: "bearer syntax with embedded whitespace", body: []byte(`{"access_token":"bad token","token_type":"Bearer","expires_in":1}`), wantErr: core.ErrGoogleDelegationResponse},
		{name: "expired on receipt", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":0}`), wantErr: core.ErrGoogleDelegationResponse},
		{name: "expiry over maximum", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":3601}`), wantErr: core.ErrGoogleDelegationResponse},
		{name: "expiry integer overflow", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":18446744073709551616}`), wantErr: core.ErrJSONContract},
		{name: "expiry negative", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":-1}`), wantErr: core.ErrJSONContract},
		{name: "expiry fractional", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":1.25}`), wantErr: core.ErrJSONContract},
		{name: "unknown authorization type", body: []byte(`{"access_token":"a","token_type":"Basic","expires_in":1}`), wantErr: core.ErrGoogleDelegationResponse},
		{name: "missing token type", body: []byte(`{"access_token":"a","expires_in":1}`), wantErr: core.ErrGoogleDelegationResponse},
		{name: "missing access token", body: []byte(`{"token_type":"Bearer","expires_in":1}`), wantErr: core.ErrGoogleDelegationResponse},
		{name: "scope explicitly empty", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":1,"scope":""}`), wantErr: core.ErrGoogleDelegationBinding},
		{name: "scope explicitly null", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":1,"scope":null}`), wantErr: core.ErrJSONContract},
		{name: "scope widened by Gmail send", body: providerFixtureJSON(t, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 60, Scope: GmailReadOnlyScope + " https://www.googleapis.com/auth/gmail.send"}), wantErr: core.ErrGoogleDelegationBinding},
		{name: "scope switched to Search Console", body: providerFixtureJSON(t, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 60, Scope: "https://www.googleapis.com/auth/webmasters.readonly"}), wantErr: core.ErrGoogleDelegationBinding},
		{name: "unexpected refresh token", body: []byte(`{"access_token":"a","token_type":"Bearer","expires_in":1,"refresh_token":"never-store"}`), wantErr: core.ErrJSONContract},
		{name: "duplicate token keys", body: []byte(`{"access_token":"a","ACCESS_TOKEN":"b","token_type":"Bearer","expires_in":1}`), wantErr: core.ErrJSONContract},
		{name: "member nested instead of scalar", body: []byte(`{"access_token":{"value":"a"},"token_type":"Bearer","expires_in":1}`), wantErr: core.ErrJSONContract},
		{name: "truncated document", body: doc[:len(doc)-1], wantErr: core.ErrJSONContract},
		{name: "trailing data after valid document", body: append(bytes.Clone(doc), []byte("{}")...), wantErr: core.ErrJSONContract},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeDelegatedToken(row.body, start, GmailReadOnlyScope)
			if !errors.Is(err, row.wantErr) {
				t.Fatalf("token decoder error = %v, want %v", err, row.wantErr)
			}
			if row.wantErr != nil {
				if got != (DelegatedAccessToken{}) {
					t.Fatalf("rejected response reported lifetime %v, want zero bearer state", got.Lifetime())
				}
				return
			}
			defer got.Close()
			gotExpiry, expErr := got.ExpiresAt().Nanoseconds()
			wantExpiry := wireFixtureTime + int64(row.wantSeconds)*1000000000
			if got.Validate() != nil || expErr != nil || gotExpiry != wantExpiry {
				t.Fatalf("bearer expiry = %v/%v, want %d", gotExpiry, expErr, wantExpiry)
			}
		})
	}
}

func TestDelegatedAssertionRejectsIndependentSubstitutions(t *testing.T) {
	t.Parallel()
	claims := providerClaimsFixture(t)
	baseline := providerAssertionFixture(t, claims, wireFixtureKey)
	rows := []struct {
		name    string
		edit    func(signReplyFixture, delegationClaims) signReplyFixture
		wantErr error
	}{
		{name: "original assertion", edit: func(f signReplyFixture, _ delegationClaims) signReplyFixture { return f }},
		{name: "foreign issuer", edit: func(_ signReplyFixture, c delegationClaims) signReplyFixture {
			c.Issuer = "foreign@project.iam.gserviceaccount.com"
			return providerAssertionFixture(t, c, wireFixtureKey)
		}, wantErr: core.ErrGoogleDelegationBinding},
		{name: "foreign subject", edit: func(_ signReplyFixture, c delegationClaims) signReplyFixture {
			c.Subject = optionalDelegationText{value: "stranger@example.invalid", present: true}
			return providerAssertionFixture(t, c, wireFixtureKey)
		}, wantErr: core.ErrGoogleDelegationBinding},
		{name: "plain account assertion without subject", edit: func(_ signReplyFixture, c delegationClaims) signReplyFixture {
			c.Subject = optionalDelegationText{}
			return providerAssertionFixture(t, c, wireFixtureKey)
		}, wantErr: core.ErrGoogleDelegationBinding},
		{name: "wrong token audience", edit: func(_ signReplyFixture, c delegationClaims) signReplyFixture {
			c.Audience = "https://example.invalid/token"
			return providerAssertionFixture(t, c, wireFixtureKey)
		}, wantErr: core.ErrGoogleDelegationBinding},
		{name: "changed issue time", edit: func(_ signReplyFixture, c delegationClaims) signReplyFixture {
			c.IssuedAtSeconds++
			return providerAssertionFixture(t, c, wireFixtureKey)
		}, wantErr: core.ErrGoogleDelegationBinding},
		{name: "changed assertion expiry", edit: func(_ signReplyFixture, c delegationClaims) signReplyFixture {
			c.ExpiresAtSeconds--
			return providerAssertionFixture(t, c, wireFixtureKey)
		}, wantErr: core.ErrGoogleDelegationBinding},
		{name: "unexpected signer response key", edit: func(f signReplyFixture, _ delegationClaims) signReplyFixture { f.KeyID = "other-key"; return f }, wantErr: core.ErrGoogleDelegationBinding},
		{name: "missing signature", edit: func(f signReplyFixture, _ delegationClaims) signReplyFixture {
			f.JWT = f.JWT[:strings.LastIndex(f.JWT, ".")+1]
			return f
		}, wantErr: core.ErrGoogleDelegationResponse},
		{name: "signature with framing injected", edit: func(f signReplyFixture, _ delegationClaims) signReplyFixture { f.JWT += ".YWJj"; return f }, wantErr: core.ErrGoogleDelegationResponse},
		{name: "signature with newline accepted by base64 only", edit: func(f signReplyFixture, _ delegationClaims) signReplyFixture { f.JWT += "\n"; return f }, wantErr: core.ErrGoogleDelegationResponse},
		{name: "jwt field exceeds compact credential bound", edit: func(f signReplyFixture, _ delegationClaims) signReplyFixture {
			f.JWT = f.JWT[:strings.LastIndex(f.JWT, ".")+1] + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{17}, 7*1024))
			return f
		}, wantErr: core.ErrGoogleDelegationResponse},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture := row.edit(baseline, claims)
			if row.wantErr != nil && fixture == baseline {
				t.Fatalf("mutation %q left its fixture unchanged, want one changed binding fact", row.name)
			}
			got, err := decodeDelegatedAssertion(providerFixtureJSON(t, fixture), claims)
			defer clear(got.bytes)
			if !errors.Is(err, row.wantErr) {
				t.Fatalf("assertion error = %v, want %v", err, row.wantErr)
			}
			if row.wantErr != nil && got.bytes != nil || row.wantErr == nil && !bytes.Equal(got.bytes, []byte(baseline.JWT)) {
				t.Fatalf("assertion bytes = %d, err %v, want exact bytes on success and zero on refusal", len(got.bytes), err)
			}
		})
	}
}

func TestDelegatedRequestRefusesWrongScopeAndClock(t *testing.T) {
	t.Parallel()
	signer, signerErr := ParseServiceAccountEmail(wireFixtureIssuer)
	subject, subjectErr := ParseWorkspaceSubject(wireFixtureSubject)
	policy, pErr := DefaultPolicy()
	if err := errors.Join(signerErr, subjectErr, pErr); err != nil {
		t.Fatal(err)
	}
	baseline := DelegatedAccessRequest{Signer: signer, Subject: subject, Scope: DelegatedScopeGmailReadOnly, ObservedAt: temporal.InstantFromNanoseconds(wireFixtureTime), Policy: policy}
	for _, row := range []struct {
		name    string
		edit    func(*DelegatedAccessRequest)
		wantErr error
	}{
		{"configured signer, exact subject and only Gmail-read scope", func(*DelegatedAccessRequest) {}, nil},
		{"zero signer", func(r *DelegatedAccessRequest) { r.Signer = ServiceAccountEmail{} }, core.ErrGoogleIdentityContract},
		{"zero subject", func(r *DelegatedAccessRequest) { r.Subject = WorkspaceSubject{} }, core.ErrGoogleIdentityContract},
		{"invalid scope", func(r *DelegatedAccessRequest) { r.Scope = DelegatedScopeInvalid }, core.ErrGoogleIdentityContract},
		{"unknown next scope", func(r *DelegatedAccessRequest) { r.Scope = DelegatedScope(2) }, core.ErrGoogleIdentityContract},
		{"negative observed wall time", func(r *DelegatedAccessRequest) { r.ObservedAt = temporal.InstantFromNanoseconds(-1) }, core.ErrGoogleIdentityContract},
		{"cannot fit provider lifetime", func(r *DelegatedAccessRequest) { r.ObservedAt = temporal.InstantFromNanoseconds(math.MaxInt64) }, core.ErrGoogleIdentityContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			request := baseline
			row.edit(&request)
			got, err := request.claims()
			if !errors.Is(err, row.wantErr) || row.wantErr != nil && got != (delegationClaims{}) {
				t.Fatalf("claims = %v/%v, want refusal %v and no partial result", got, err, row.wantErr)
			}
		})
	}
}

func FuzzDelegatedProviderDocuments(f *testing.F) {
	gmail := providerClaimsFixture(f)
	signer, err := ParseServiceAccountEmail(wireFixtureIssuer)
	if err != nil {
		f.Fatal(err)
	}
	search, err := makeAssertionClaims(assertionScope{Signer: signer, At: temporal.InstantFromNanoseconds(wireFixtureTime), Scope: SearchConsoleReadOnlyScope})
	if err != nil {
		f.Fatal(err)
	}
	seed := providerAssertionFixture(f, gmail, wireFixtureKey)
	f.Add(uint8(0), providerFixtureJSON(f, seed))
	f.Add(uint8(1), providerFixtureJSON(f, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 3600, Scope: GmailReadOnlyScope}))
	f.Add(uint8(2), providerFixtureJSON(f, delegatedFaultReply{Code: delegationFaultUnauthorizedClient}))
	f.Add(uint8(3), providerFixtureJSON(f, providerAssertionFixture(f, search, wireFixtureKey)))
	f.Add(uint8(4), providerFixtureJSON(f, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 3600, Scope: SearchConsoleReadOnlyScope}))
	f.Add(uint8(0), []byte(`{"keyId":"x","signedJwt":"bad"}`))
	f.Add(uint8(1), []byte(`{"access_token":"x","access_token":"y"}`))
	f.Add(uint8(2), []byte(`{"error":null}`))
	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		want := gmail
		if selector%5 > 2 {
			want = search
		}
		switch selector % 5 {
		case 0, 3:
			got, err := decodeDelegatedAssertion(data, want)
			defer clear(got.bytes)
			if err != nil {
				if got.bytes != nil || !errors.Is(err, core.ErrGoogleDelegationResponse) && !errors.Is(err, core.ErrGoogleDelegationBinding) {
					t.Fatalf("assertion refusal = %d bytes/%v", len(got.bytes), err)
				}
				return
			}
			parts := strings.Split(string(got.bytes), ".")
			if len(parts) != 3 || len(got.bytes) > delegationJWTMaximum {
				t.Fatalf("compact parts = %d, extent %d, want three parts and at most %d bytes", len(parts), len(got.bytes), delegationJWTMaximum)
			}
			raw, decErr := base64.RawURLEncoding.DecodeString(parts[1])
			var gotClaims delegationClaims
			if decErr != nil || json.Unmarshal(raw, &gotClaims) != nil || gotClaims != want {
				t.Fatalf("admitted claims match = %t, decode error %v, want exact signed claims", gotClaims == want, decErr)
			}
			var wire signReplyFixture
			if json.Unmarshal(data, &wire) != nil || !bytes.Equal(got.bytes, []byte(wire.JWT)) {
				t.Fatalf("provider assertion equality = %t, want true", bytes.Equal(got.bytes, []byte(wire.JWT)))
			}
		case 1, 4:
			got, err := decodeDelegatedToken(data, temporal.InstantFromNanoseconds(wireFixtureTime), want.Scope)
			if err != nil {
				if got != (DelegatedAccessToken{}) || !errors.Is(err, core.ErrGoogleDelegationResponse) && !errors.Is(err, core.ErrGoogleDelegationBinding) {
					t.Fatalf("token refusal = %v/%v", got, err)
				}
				return
			}
			defer got.Close()
			var raw tokenReplyFixture
			if json.Unmarshal(data, &raw) != nil || raw.Type != "Bearer" || raw.Expires < 1 || raw.Expires > 3600 || raw.Scope != "" && raw.Scope != want.Scope || got.Validate() != nil {
				t.Fatalf("provider expiry = %d, type %q, want bounded bearer bound to the request", raw.Expires, raw.Type)
			}
			bearer, copyErr := got.CopyBearer()
			gotBytes, readErr := bearer.CopyBytes()
			defer clear(gotBytes)
			defer bearer.Close()
			expires, expiryErr := got.ExpiresAt().Nanoseconds()
			if errors.Join(copyErr, readErr, expiryErr) != nil || !bytes.Equal(gotBytes, []byte(raw.Token)) || expires != wireFixtureTime+int64(raw.Expires)*1000000000 {
				t.Fatalf("provider bearer match = %t, expiry %d, want exact bearer and expiry", bytes.Equal(gotBytes, []byte(raw.Token)), expires)
			}
		case 2:
			got, err := decodeDelegationDocument[delegatedFaultReply](data, 2)
			if err != nil {
				if got != (delegatedFaultReply{}) || !errors.Is(err, core.ErrGoogleDelegationResponse) {
					t.Fatalf("fault refusal = %v/%v", got, err)
				}
				return
			}
			encoded := providerFixtureJSON(t, got)
			again, err := decodeDelegationDocument[delegatedFaultReply](encoded, 2)
			if err != nil || again != got || !bytes.Equal(encoded, providerFixtureJSON(t, again)) {
				t.Fatalf("normalized fault equality = %t, decode error %v, want canonical closure", again == got, err)
			}
		}
	})
}

func FuzzDelegatedPrincipalText(f *testing.F) {
	f.Add(wireFixtureIssuer)
	f.Add(wireFixtureSubject)
	f.Add("123456789-compute@developer.gserviceaccount.com")
	f.Add("fixture-project@appspot.gserviceaccount.com")
	f.Add("a/b@fixture-project.iam.gserviceaccount.com")
	f.Add("Example <reader@example.invalid>")
	f.Fuzz(func(t *testing.T, text string) {
		subject, subErr := ParseWorkspaceSubject(text)
		signer, signErr := ParseServiceAccountEmail(text)
		if subErr == nil {
			if subject.Validate() != nil || subject.Email().String() != text || len(text) > workspaceEmailMaximum {
				t.Fatalf("subject identity equality = %t, extent %d, want the original bounded input", subject.Email().String() == text, len(text))
			}
		} else if !errors.Is(subErr, core.ErrGoogleIdentityContract) || subject != (WorkspaceSubject{}) {
			t.Fatalf("subject refusal = %v/%v", subject, subErr)
		}
		if signErr == nil {
			if subErr != nil || signer.Validate() != nil || signer.String() != text || !strings.HasSuffix(text, ".gserviceaccount.com") {
				t.Fatalf("signer identity equality = %t, subject error %v, want the same valid account email", signer.String() == text, subErr)
			}
		} else if !errors.Is(signErr, core.ErrGoogleIdentityContract) || signer != (ServiceAccountEmail{}) {
			t.Fatalf("signer refusal = %v/%v", signer, signErr)
		}
	})
}

func TestDelegatedAccessFreshnessAndCustody(t *testing.T) {
	t.Parallel()
	start := temporal.InstantFromNanoseconds(wireFixtureTime)
	raw := providerFixtureJSON(t, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 10})
	token, err := decodeDelegatedToken(raw, start, GmailReadOnlyScope)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := token.Close(); err != nil {
			t.Error(err)
		}
	})
	const second int64 = 1000000000
	for _, row := range []struct {
		name  string
		delta int64
		want  bool
	}{
		{"rollback one nanosecond", -1, false},
		{"mint observation", 0, true},
		{"one nanosecond before ten percent margin", 9*second - 1, true},
		{"at ten percent margin", 9 * second, false},
		{"exact provider expiry", 10 * second, false},
		{"past provider expiry", 10*second + 1, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := token.FreshAt(temporal.InstantFromNanoseconds(wireFixtureTime + row.delta)); got != row.want {
				t.Fatalf("freshness at %d = %t, want %t", row.delta, got, row.want)
			}
		})
	}
}

func TestDelegatedAccessBearerCopySurvivesSourceClose(t *testing.T) {
	t.Parallel()
	raw := providerFixtureJSON(t, tokenReplyFixture{Token: wireFixtureToken, Type: "Bearer", Expires: 40})
	token, err := decodeDelegatedToken(raw, temporal.InstantFromNanoseconds(wireFixtureTime), GmailReadOnlyScope)
	if err != nil {
		t.Fatal(err)
	}
	copyOfToken := token
	bearer, err := token.CopyBearer()
	if err != nil {
		t.Fatal(err)
	}
	defer bearer.Close()
	if err := token.Close(); err != nil {
		t.Fatal(err)
	}
	if copyOfToken.FreshAt(temporal.InstantFromNanoseconds(wireFixtureTime)) || copyOfToken.Validate() == nil {
		t.Fatalf("closed handle validates as %v, want a refused token", copyOfToken.Validate())
	}
	if raw, err := json.Marshal(copyOfToken); !errors.Is(err, core.ErrJSONContract) || len(raw) != 0 {
		t.Fatalf("secret encoding = %d bytes/%v, want zero and typed refusal", len(raw), err)
	}
	got, err := bearer.CopyBytes()
	defer clear(got)
	if err != nil || !bytes.Equal(got, []byte(wireFixtureToken)) {
		t.Fatalf("independent bearer no longer works after cache close, got err %v", err)
	}
}

func TestDelegationPrincipalPreservesRoleAndPathBoundaries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, text      string
		signer, subject bool
	}{
		{"dedicated account", "reader@project.iam.gserviceaccount.com", true, true},
		{"App Engine default account", "project@appspot.gserviceaccount.com", true, true},
		{"Compute default account", "123456789-compute@developer.gserviceaccount.com", true, true},
		{"mailbox only", "reader@example.invalid", false, true},
		{"spoof suffix", "reader@project.iam.gserviceaccount.com.example.invalid", false, true},
		{"mailbox plus and apostrophe", "r'obert+inbox@example.invalid", false, true},
		{"path separator", "reader/escape@project.iam.gserviceaccount.com", false, false},
		{"display name", "Guest <reader@project.iam.gserviceaccount.com>", false, false},
		{"missing domain", "reader@", false, false},
		{"address one byte past scope maximum", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63), false, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			signer, sErr := ParseServiceAccountEmail(row.text)
			subject, uErr := ParseWorkspaceSubject(row.text)
			if (sErr == nil) != row.signer || (uErr == nil) != row.subject ||
				sErr != nil && !errors.Is(sErr, core.ErrGoogleIdentityContract) || uErr != nil && !errors.Is(uErr, core.ErrGoogleIdentityContract) {
				t.Fatalf("signer/subject admission = %v/%v, want %t/%t", sErr, uErr, row.signer, row.subject)
			}
			if row.signer && signer.String() != row.text || row.subject && subject.Email().String() != row.text {
				t.Fatalf("signer/subject equal the input = %t/%t, want admitted spellings preserved", signer.String() == row.text, subject.Email().String() == row.text)
			}
		})
	}
}

func TestDelegationResponseBufferStopsBeforeRetainingExcess(t *testing.T) {
	t.Parallel()
	var buffer delegationBuffer
	n, err := buffer.Write(bytes.Repeat([]byte{'a'}, delegationWireMaximum))
	if n != delegationWireMaximum || err != nil {
		t.Fatalf("exact bound = %d/%v, want %d/nil", n, err, delegationWireMaximum)
	}
	n, err = buffer.Write([]byte{'b'})
	if n != 0 || !errors.Is(err, core.ErrExchangeBodyLimit) || len(buffer.bytes) != delegationWireMaximum {
		t.Fatalf("overflow retained %d bytes, write %d/%v, want unchanged extent and typed refusal", len(buffer.bytes), n, err)
	}
}
