package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestConfigurationDocumentAdmitsExactTypedCredentialReferences(t *testing.T) {
	t.Parallel()
	projectID := commandUUIDFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6")
	cases := []struct {
		name      string
		authority string
		username  string
		project   string
		secret    string
		withID    bool
	}{
		{name: "ordinary authority", authority: "https://admin.example.com", username: "agent", project: "example-task-project", secret: "admin-password", withID: true},
		{name: "explicit TLS port", authority: "https://admin.example.com:8443", username: "agent-1", project: "abcde1", secret: "admin_password", withID: true},
		{name: "IPv4 authority", authority: "https://192.0.2.1", username: "agent.2", project: "abcde2", secret: "admin-password-2", withID: true},
		{name: "IPv6 authority", authority: "https://[2001:db8::1]", username: "agent_3", project: "abcde3", secret: "A", withID: true},
		{name: "Unicode username", authority: "https://tasks.example.net", username: "équipe", project: "abcde4", secret: "B", withID: true},
		{name: "project creation omits default project", authority: "https://tasks.example.org", username: "creator", project: "abcde5", secret: "C"},
		{name: "hyphenated project", authority: "https://tasks-1.example.org", username: "operator", project: "a-b-c1", secret: "D", withID: true},
		{name: "numeric project suffix", authority: "https://tasks-2.example.org", username: "operator2", project: "project9", secret: "E", withID: true},
		{name: "secret identifier ceiling shape", authority: "https://tasks-3.example.org", username: "operator3", project: "project8", secret: "proof_secret-v1", withID: true},
		{name: "single character secret identifier", authority: "https://tasks-4.example.org", username: "operator4", project: "project7", secret: "Z", withID: true},
	}
	limits, err := documentLimits(configurationMaxBytes)
	if err != nil {
		t.Fatalf("documentLimits() error = %v, want nil", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authority, parseErr := core.ParseHTTPEndpoint(tc.authority)
			if parseErr != nil {
				t.Fatalf("ParseHTTPEndpoint() error = %v, want nil", parseErr)
			}
			username, parseErr := exchange.ParseBasicAuthorizationIdentity(tc.username)
			if parseErr != nil {
				t.Fatalf("ParseBasicAuthorizationIdentity() error = %v, want nil", parseErr)
			}
			configuration := configurationDocument{
				Revision: commandDocumentRevisionV2, Authority: authority, Username: username,
				PasswordSecret: googleSecretReference{Project: tc.project, Secret: tc.secret},
			}
			if tc.withID {
				configuration.ProjectID = &projectID
			}
			encoded, encodeErr := configuration.MarshalJSON()
			if encodeErr != nil {
				t.Fatalf("configuration MarshalJSON() error = %v, want nil", encodeErr)
			}
			got, decodeErr := core.DecodeStrictJSON[configurationDocument](bytes.NewReader(encoded), limits)
			if decodeErr != nil {
				t.Fatalf("DecodeStrictJSON(configuration) error = %v, want nil", decodeErr)
			}
			second, secondErr := got.MarshalJSON()
			if secondErr != nil || !bytes.Equal(second, encoded) {
				t.Fatalf("configuration canonical round trip = (%q, %v), want (%q, nil)", second, secondErr, encoded)
			}
		})
	}
}

func TestConfigurationDocumentRejectsEmbeddedSecretsAndMalformedOwnership(t *testing.T) {
	t.Parallel()
	limits, err := documentLimits(configurationMaxBytes)
	if err != nil {
		t.Fatalf("documentLimits() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data string
	}{
		{name: "password field is forbidden", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent","password":"never","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "empty object", data: `{}`},
		{name: "null", data: `null`},
		{name: "superseded revision is refused", data: `{"revision":1,"authority":"https://admin.example.com","username":"agent","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "HTTP authority", data: `{"revision":2,"authority":"http://admin.example.com","username":"agent","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "authority path", data: `{"revision":2,"authority":"https://admin.example.com/tasks","username":"agent","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "authority query", data: `{"revision":2,"authority":"https://admin.example.com?x=1","username":"agent","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "username delimiter", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent:secret","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "username leading whitespace", data: `{"revision":2,"authority":"https://admin.example.com","username":" agent","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "invalid Google project", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent","password_secret":{"project":"UPPERCASE","secret":"admin-password"}}`},
		{name: "invalid Google secret", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent","password_secret":{"project":"example-task-project","secret":"secret/path"}}`},
		{name: "missing secret reference", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent"}`},
		{name: "invalid default project UUID", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent","password_secret":{"project":"example-task-project","secret":"admin-password"},"project_id":"not-a-uuid"}`},
		{name: "duplicate username", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent","username":"agent","password_secret":{"project":"example-task-project","secret":"admin-password"}}`},
		{name: "trailing document", data: `{"revision":2,"authority":"https://admin.example.com","username":"agent","password_secret":{"project":"example-task-project","secret":"admin-password"}} {}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := core.DecodeStrictJSON[configurationDocument](bytes.NewBufferString(tc.data), limits)
			if gotErr == nil || (!errors.Is(gotErr, core.ErrJSONContract) && !errors.Is(gotErr, core.ErrTaskManagerContract)) {
				t.Fatalf("DecodeStrictJSON(rejected configuration) error = %v, want typed refusal", gotErr)
			}
			if got.Authority.String() != "" || got.Username.String() != "" || got.ProjectID != nil {
				t.Fatalf("DecodeStrictJSON(rejected configuration) = %+v, want zero", got)
			}
		})
	}
}
