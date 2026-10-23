package github

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The independent native cursor retains one provider record and the one unique
// primary fact; it never builds an email-page collection for the oracle.
func browserEmailOracle(data []byte) (core.OAuthEmail, bool, bool) {
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	token, err := decoder.ReadToken()
	if err != nil || token.Kind() != jsontext.KindBeginArray {
		return core.OAuthEmail{}, false, false
	}
	var selected core.OAuthEmail
	verified := false
	for decoder.PeekKind() != jsontext.KindEndArray {
		var record browserEmailWire
		if err := json.UnmarshalDecode(decoder, &record); err != nil || record.Email == nil || record.Primary == nil || record.Verified == nil {
			return core.OAuthEmail{}, false, false
		}
		email, err := core.ParseOAuthEmail(*record.Email)
		if err != nil {
			return core.OAuthEmail{}, false, false
		}
		if *record.Primary {
			if selected != (core.OAuthEmail{}) && (selected != email || verified != *record.Verified) {
				return core.OAuthEmail{}, false, false
			}
			selected = email
			verified = *record.Verified
		}
	}
	if _, err := decoder.ReadToken(); err != nil {
		return core.OAuthEmail{}, false, false
	}
	if _, err := decoder.ReadToken(); err != io.EOF {
		return core.OAuthEmail{}, false, false
	}
	return selected, verified, true
}
func FuzzBrowserOAuthEmailStreamSemanticClosure(f *testing.F) {
	address, err := core.ParseOAuthEmail("athlete@example.test")
	if err != nil {
		f.Fatal(err)
	}
	email := address.String()
	yes := true
	wire := []browserEmailWire{{Email: &email, Primary: &yes, Verified: &yes}}
	seed, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte("[]"))
	f.Add([]byte("null"))
	f.Add([]byte("[{"))
	f.Fuzz(func(t *testing.T, data []byte) {
		want, verified, admitted := browserEmailOracle(data)
		var got browserPrimaryEmail
		err := decodeBrowserEmails(t.Context(), bytes.NewReader(data), &got)
		if (err == nil) != admitted {
			t.Fatalf("email stream admitted=%t error=%v, want %t", err == nil, err, admitted)
		}
		if err != nil {
			if !errors.Is(err, core.ErrGitHubResponse) {
				t.Fatalf("email refusal = %v, want provider response identity", err)
			}
			return
		}
		if got.email != want || got.verified != verified {
			t.Fatalf("email stream projection = %+v, want exact %q/verified%t", got, want.String(), verified)
		}
	})
}
func FuzzBrowserOAuthUserSemanticClosure(f *testing.F) {
	id, err := NewBrowserUserID(42)
	if err != nil {
		f.Fatal(err)
	}
	value, err := id.Uint64()
	if err != nil {
		f.Fatal(err)
	}
	seed, err := core.MarshalCanonicalJSONDocument(browserUserWire{ID: &value})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte("null"))
	f.Add([]byte(`{"id":0}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var reference browserUserWire
		parseErr := json.Unmarshal(data, &reference)
		trimmed := bytes.TrimSpace(data)
		admitted := parseErr == nil && len(trimmed) > 0 && trimmed[0] == '{' && reference.ID != nil && *reference.ID > 0
		got, err := decodeBrowserUser(context.Background(), bytes.NewReader(data))
		if (err == nil) != admitted {
			t.Fatalf("user identity admitted=%t error=%v, want %t", err == nil, err, admitted)
		}
		if err != nil {
			if got != (BrowserUserID{}) || !errors.Is(err, core.ErrGitHubResponse) {
				t.Fatalf("user refusal = (%+v,%v), want zero and provider identity", got, err)
			}
			return
		}
		value, err := got.Uint64()
		if err != nil || value != *reference.ID {
			t.Fatalf("user identity = (%d,%v), want exact %d", value, err, *reference.ID)
		}
	})
}
