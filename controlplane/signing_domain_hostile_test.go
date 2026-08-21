package controlplane_test

import (
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// TestParseSigningDomainAcceptsExactlyTheClosedSet is the admission table.
//
// A domain names the namespace a signature covers. A parser that accepted a
// near miss would let a verifier report that a signature is valid for something
// the signer never signed, so every row that is not an exact published text is
// a refusal.
func TestParseSigningDomainAcceptsExactlyTheClosedSet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		text  string
		want  controlplane.SigningDomain
		valid bool
	}{
		{
			name: "the installation certificate domain",
			text: controlplane.SigningDomainInstallationCertificateV1Token,
			want: controlplane.SigningDomainInstallationCertificateV1, valid: true,
		},
		{
			name: "the registration domain",
			text: controlplane.SigningDomainRegistrationV1Token,
			want: controlplane.SigningDomainRegistrationV1, valid: true,
		},
		{
			name: "the check-in domain",
			text: controlplane.SigningDomainCheckInV1Token,
			want: controlplane.SigningDomainCheckInV1, valid: true,
		},
		{name: "empty text names no domain", text: ""},
		{name: "an unknown domain is refused rather than carried", text: "ogs-control-something-2026-1"},
		{
			name: "upper case is not the canonical text",
			text: strings.ToUpper(controlplane.SigningDomainRegistrationV1Token),
		},
		{
			name: "a leading space is not the canonical text",
			text: " " + controlplane.SigningDomainRegistrationV1Token,
		},
		{
			name: "a trailing space is not the canonical text",
			text: controlplane.SigningDomainRegistrationV1Token + " ",
		},
		{
			name: "a prefix of a published domain is not that domain",
			text: strings.TrimSuffix(controlplane.SigningDomainRegistrationV1Token, "1"),
		},
		{
			name: "a published domain with a suffix is not that domain",
			text: controlplane.SigningDomainRegistrationV1Token + "0",
		},
		{
			name: "an older revision of a domain is not the published one",
			text: "ogs-control-registration-2025-1",
		},
		{name: "text past the attest bound is refused", text: strings.Repeat("a", 4096)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlplane.ParseSigningDomain(testCase.text)
			if !testCase.valid {
				if !errors.Is(err, core.ErrControlPlaneSigningDomain) {
					t.Fatalf("ParseSigningDomain(%q) error = %v, want %v",
						testCase.text, err, core.ErrControlPlaneSigningDomain)
				}
				if got != controlplane.SigningDomainUnknown {
					t.Fatalf("ParseSigningDomain(%q) = %v, want %v",
						testCase.text, got, controlplane.SigningDomainUnknown)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSigningDomain(%q) error = %v, want nil", testCase.text, err)
			}
			if got != testCase.want {
				t.Fatalf("ParseSigningDomain(%q) = %v, want %v", testCase.text, got, testCase.want)
			}
			if !got.IsValid() {
				t.Fatalf("%v.IsValid() = false, want true", got)
			}
		})
	}
}

// TestSigningDomainTextAndJSONAgreeAndRoundTrip proves the two spellings are the
// same bytes. Attest covers the text form inside the signature while documents
// carry the JSON form, so a domain that rendered differently in each could
// verify under one name and be read under another.
func TestSigningDomainTextAndJSONAgreeAndRoundTrip(t *testing.T) {
	t.Parallel()

	domains := []controlplane.SigningDomain{
		controlplane.SigningDomainInstallationCertificateV1,
		controlplane.SigningDomainRegistrationV1,
		controlplane.SigningDomainCheckInV1,
	}
	for _, domain := range domains {
		t.Run(domain.String(), func(t *testing.T) {
			t.Parallel()

			text, err := domain.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error = %v, want nil", err)
			}
			encoded, err := json.Marshal(domain)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v, want nil", err)
			}
			if want := `"` + string(text) + `"`; string(encoded) != want {
				t.Fatalf("json.Marshal() = %s, want %s", encoded, want)
			}
			var decoded controlplane.SigningDomain
			if err := decoded.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
			}
			if decoded != domain {
				t.Fatalf("decoded domain = %v, want %v", decoded, domain)
			}
			parsed, err := controlplane.ParseSigningDomain(string(text))
			if err != nil {
				t.Fatalf("ParseSigningDomain() error = %v, want nil", err)
			}
			if parsed != domain {
				t.Fatalf("ParseSigningDomain(MarshalText()) = %v, want %v", parsed, domain)
			}
		})
	}
}

// TestSigningDomainRefusesTheUnsetDomainAtEveryBoundary keeps the zero value
// inert. An unset domain that emitted anything would sign a document into a
// namespace nobody named.
func TestSigningDomainRefusesTheUnsetDomainAtEveryBoundary(t *testing.T) {
	t.Parallel()

	unset := controlplane.SigningDomainUnknown
	if err := unset.Validate(); !errors.Is(err, core.ErrControlPlaneSigningDomain) {
		t.Fatalf("Validate() error = %v, want %v", err, core.ErrControlPlaneSigningDomain)
	}
	if got := unset.IsValid(); got {
		t.Fatalf("SigningDomainUnknown.IsValid() = %t, want false", got)
	}
	if got, err := unset.MarshalText(); !errors.Is(err, core.ErrControlPlaneSigningDomain) || got != nil {
		t.Fatalf("MarshalText() = (%q, %v), want nil and errors.Is %v", got, err, core.ErrControlPlaneSigningDomain)
	}
	if got, err := unset.MarshalJSON(); !errors.Is(err, core.ErrControlPlaneSigningDomain) ||
		!errors.Is(err, core.ErrJSONContract) || got != nil {
		t.Fatalf("MarshalJSON() = (%s, %v), want nil and errors.Is %v and %v", got, err,
			core.ErrControlPlaneSigningDomain, core.ErrJSONContract)
	}
	// A rejected decode must leave the receiver alone, so a partially decoded
	// document cannot end up holding a domain it never carried.
	held := controlplane.SigningDomainRegistrationV1
	if err := held.UnmarshalJSON([]byte(`"ogs-control-not-a-domain"`)); !errors.Is(err, core.ErrControlPlaneSigningDomain) || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("UnmarshalJSON() error = %v, want errors.Is %v and %v", err,
			core.ErrControlPlaneSigningDomain, core.ErrJSONContract)
	}
	if held != controlplane.SigningDomainRegistrationV1 {
		t.Fatalf("receiver after a rejected decode = %v, want the unchanged %v",
			held, controlplane.SigningDomainRegistrationV1)
	}
}
