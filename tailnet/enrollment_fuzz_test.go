package tailnet

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	tailscale "tailscale.com/client/tailscale/v2"
)

func FuzzProviderAuthKeyAdmission(f *testing.F) {
	configuration := fixtureConfiguration(f)
	key := tailscale.Key{Key: fixtureAuthKey}
	key.Capabilities.Devices.Create.Ephemeral = true
	key.Capabilities.Devices.Create.Tags = []string{configuration.Tag.String()}
	seed, err := core.MarshalCanonicalJSONDocument(key)
	if err != nil {
		f.Fatalf("key encoding = %v, want nil", err)
	}
	if err := validateAuthKey(key.Key); err != nil {
		f.Fatalf("seed key admission = %v, want nil", err)
	}
	f.Add(seed)
	for _, value := range []string{authKeyPrefix + "a", authKeyPrefix + strings.Repeat("a", authKeyMaximumBytes-len(authKeyPrefix))} {
		extreme := key
		extreme.Key = value
		if err := validateAuthKey(extreme.Key); err != nil {
			f.Fatalf("extreme seed admission = %v, want nil", err)
		}
		encoded, err := core.MarshalCanonicalJSONDocument(extreme)
		if err != nil {
			f.Fatalf("extreme seed encoding = %v, want nil", err)
		}
		f.Add(encoded)
	}

	f.Add([]byte("null"))
	f.Add([]byte("{broken"))
	f.Add([]byte(strings.Repeat(" ", enrollmentResponseMaximumBytes+1)))
	grammar := regexp.MustCompile("^" + regexp.QuoteMeta(authKeyPrefix) + `[A-Za-z0-9-]+$`)
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > enrollmentResponseMaximumBytes+1 {
			data = data[:enrollmentResponseMaximumBytes+1]
		}
		token := fixtureIdentityToken(t)
		access, err := core.MarshalCanonicalJSONDocument(fixtureAccessResponse{AccessToken: fixtureAccessToken, TokenType: exchange.BearerAuthorizationScheme, ExpiresIn: 60})
		if err != nil {
			t.Fatalf("access response encoding = %v, want nil", err)
		}
		client := localEnrollmentClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := data
			if r.URL.Path == tokenExchangePath {
				response = access
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(response); err != nil {
				t.Errorf("provider Write() = %v, want nil", err)
			}
		}))
		got, err := acquireAuthKey(t.Context(), client, configuration, token)
		if err != nil {
			if got != "" || !errors.Is(err, core.ErrTailnetEnrollment) {
				t.Fatalf("key refusal = (empty=%t,%v), want zero and Tailnet enrollment", got == "", err)
			}
			return
		}
		// Go decodes an independent observation. Acceptance must preserve exactly
		// the key and requested authority; grammar, extent and flag checks do not
		// call the production key validator.
		var observed tailscale.Key
		decodeErr := json.Unmarshal(data, &observed)
		create := observed.Capabilities.Devices.Create
		if decodeErr != nil || len(data) > enrollmentResponseMaximumBytes || observed.Key != got || len(got) > authKeyMaximumBytes || !grammar.MatchString(got) || observed.Invalid || !observed.Revoked.IsZero() || create.Reusable || !create.Ephemeral || create.Preauthorized || len(create.Tags) != 1 {
			t.Fatalf("accepted provider key = decode=%v, bytes=%d, keyMatches=%t, flags=%+v, want bounded exact single-use ephemeral key", decodeErr, len(data), observed.Key == got, create)
		}
		if create.Tags[0] != configuration.Tag.String() {
			t.Fatalf("accepted tag = %q, want %q", create.Tags[0], configuration.Tag.String())
		}
	})
}
