package core

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestOAuthRedirectLocalhostNamespace(t *testing.T) {
	for _, test := range []struct {
		uri     string
		allowed bool
	}{
		{"http://api.dev.localhost:8083/v1/timematch/calendar/callback", true},
		{"http://app.localhost:8080/callback", true},
		{"http://localhost:8083/callback", true},
		{"http://127.0.0.1:8083/callback", true},
		{"http://[::1]:8083/callback", true},
		{"https://api.offgridsoftware.ca/callback", true},
		{"http://api.offgridsoftware.ca/callback", false},
		{"http://evillocalhost/callback", false},
		{"http://api.dev.localhost.attacker.example/callback", false},
		{"http://localhost.attacker.example/callback", false},
	} {
		t.Run(test.uri, func(t *testing.T) {
			got, err := ParseOAuthRedirectURI(test.uri)
			if test.allowed {
				if err != nil || got.String() != test.uri {
					t.Fatalf("redirect=%q err=%v", got.String(), err)
				}
				return
			}
			if !errors.Is(err, ErrOAuthContract) || got != (OAuthRedirectURI{}) {
				t.Fatalf("foreign HTTP redirect admitted: %q %v", got.String(), err)
			}
		})
	}
}

func FuzzOAuthRedirectLoopbackNamespace(f *testing.F) {
	for _, seed := range []string{"http://api.dev.localhost:8083/callback", "http://localhost/callback", "http://localhost.attacker.example/callback", "http://evillocalhost/callback", "https://api.example/callback"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := ParseOAuthRedirectURI(input)
		if err != nil {
			if !errors.Is(err, ErrOAuthContract) || got != (OAuthRedirectURI{}) {
				t.Fatalf("invalid refusal: %v", err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(got.String())
		if err != nil {
			t.Fatal(err)
		}
		host := u.Hostname()
		loopback := host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "127.0.0.1" || host == "::1"
		if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
			t.Fatalf("non-loopback cleartext redirect admitted: %q", got.String())
		}
	})
}
