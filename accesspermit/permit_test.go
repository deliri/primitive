package accesspermit

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

func permitFixture(t testing.TB) (Document, attest.TrustedKeys, ed25519.PrivateKey) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, ed25519.SeedSize))
	t.Cleanup(func() { clear(key) })
	public, err := core.NewEd25519PublicKey(key.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("public key error = %v, want nil", err)
	}
	keys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("trusted keys error = %v, want nil", err)
	}
	device, err := lease.DeviceIDForPublicKey(public)
	if err != nil {
		t.Fatalf("device error = %v, want nil", err)
	}
	entitlement, err := lease.NewEntitlementID([16]byte{1})
	if err != nil {
		t.Fatalf("entitlement error = %v, want nil", err)
	}
	account, err := receipt.NewPrincipalIdentity([16]byte{2})
	if err != nil {
		t.Fatalf("account error = %v, want nil", err)
	}
	nonce, err := controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{3})
	if err != nil {
		t.Fatalf("nonce error = %v, want nil", err)
	}
	generation, err := lease.NewGeneration(1)
	if err != nil {
		t.Fatalf("generation error = %v, want nil", err)
	}
	terms := Terms{Revision: RevisionV1,
		Binding: Binding{Family: controlwire.RouteFamilyRegistrations, Subject: lease.Subject{Offering: core.Offering{Token: "arbitrary-instrument"}, DeviceID: device, EntitlementID: entitlement}, Account: account, RequestNonce: nonce, Generation: generation},
		Window:  Window{Decision: DecisionAllow, NotBefore: temporal.InstantFromNanoseconds(100), RefreshAfter: temporal.InstantFromNanoseconds(200), NotAfter: temporal.InstantFromNanoseconds(300)},
	}
	document, err := Issue(terms, key)
	if err != nil {
		t.Fatalf("Issue() error = %v, want nil", err)
	}
	return document, keys, key
}

func TestPermitVerificationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		decision   Decision
		tamper     bool
		wantVerify error
		wantAllow  error
	}{
		{name: "authentic scoped grant admits inside its interval", decision: DecisionAllow},
		{name: "unsigned grant substitution cannot upgrade refusal", decision: DecisionRefuse, tamper: true, wantVerify: core.ErrAccessPermitDenied},
		{name: "authentic refusal is retained but admits nothing", decision: DecisionRefuse, wantAllow: core.ErrAccessPermitDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			document, keys, key := permitFixture(t)
			terms := document.Terms
			terms.Window.Decision = tc.decision
			document, err := Issue(terms, key)
			if err != nil {
				t.Fatalf("Issue() error = %v, want nil", err)
			}
			if tc.tamper {
				document.Terms.Window.Decision = DecisionAllow
			}
			got, err := Verify(document, terms.Binding, keys)
			if !errors.Is(err, tc.wantVerify) {
				t.Fatalf("Verify() error = %v, want %v", err, tc.wantVerify)
			}
			if err != nil {
				if got != (Verified{}) {
					t.Fatalf("refused proof = %+v, want zero", got)
				}
				return
			}
			retained, err := got.Terms()
			if err != nil || retained != terms {
				t.Fatalf("retained terms = (%+v,%v), want (%+v,nil)", retained, err, terms)
			}
			if err := got.Allows(temporal.InstantFromNanoseconds(250)); !errors.Is(err, tc.wantAllow) {
				t.Fatalf("Allows() error = %v, want %v", err, tc.wantAllow)
			}
		})
	}
}

func TestPermitHalfOpenWindowBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		at      int64
		wantErr error
	}{
		{"minimum representable observation precedes grant", math.MinInt64, core.ErrAccessPermitDenied},
		{"one before activation cannot start", 99, core.ErrAccessPermitDenied},
		{"exact activation starts", 100, nil},
		{"one after activation remains admitted", 101, nil},
		{"one before refresh remains admitted", 199, nil},
		{"exact refresh is not expiry", 200, nil},
		{"one after refresh remains admitted", 201, nil},
		{"one before expiry remains admitted", 299, nil},
		{"exact expiry refuses", 300, core.ErrAccessPermitDenied},
		{"one after expiry refuses", 301, core.ErrAccessPermitDenied},
		{"maximum observation cannot wrap into the interval", math.MaxInt64, core.ErrAccessPermitDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			document, keys, _ := permitFixture(t)
			proof, err := Verify(document, document.Terms.Binding, keys)
			if err != nil {
				t.Fatalf("Verify() error = %v, want nil", err)
			}
			if got := proof.Allows(temporal.InstantFromNanoseconds(tc.at)); !errors.Is(got, tc.wantErr) {
				t.Fatalf("Allows(%d) = %v, want %v", tc.at, got, tc.wantErr)
			}
		})
	}
}

func TestPermitEverySignedFactRejectsSubstitution(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*Terms)
		binding bool
	}{
		{"foreign route family", func(t *Terms) { t.Binding.Family = controlwire.RouteFamilyCheckIns }, true},
		{"foreign namespace", func(t *Terms) { t.Binding.Subject.Offering = core.Offering{Token: "future-device"} }, true},
		{"foreign account", func(t *Terms) { t.Binding.Account, _ = receipt.NewPrincipalIdentity([16]byte{9}) }, true},
		{"foreign entitlement", func(t *Terms) { t.Binding.Subject.EntitlementID, _ = lease.NewEntitlementID([16]byte{9}) }, true},
		{"foreign installation", func(t *Terms) { t.Binding.Subject.DeviceID, _ = lease.NewDeviceID([16]byte{9}) }, true},
		{"foreign request nonce", func(t *Terms) {
			t.Binding.RequestNonce, _ = controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{9})
		}, true},
		{"foreign generation", func(t *Terms) { t.Binding.Generation, _ = lease.NewGeneration(2) }, true},
		{"earlier activation", func(t *Terms) { t.Window.NotBefore = temporal.InstantFromNanoseconds(99) }, false},
		{"later refresh", func(t *Terms) { t.Window.RefreshAfter = temporal.InstantFromNanoseconds(201) }, false},
		{"extended expiry", func(t *Terms) { t.Window.NotAfter = temporal.InstantFromNanoseconds(301) }, false},
		{"changed decision", func(t *Terms) { t.Window.Decision = DecisionRefuse }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			baseline, keys, key := permitFixture(t)
			changed := baseline
			tc.mutate(&changed.Terms)
			if changed.Terms == baseline.Terms || changed.Terms.Validate() != nil {
				t.Fatalf("mutation = %+v, want distinct admissible typed fact", changed.Terms)
			}
			got, err := Verify(changed, baseline.Terms.Binding, keys)
			want := error(core.ErrAccessPermitDenied)
			if tc.binding {
				want = core.ErrAccessPermitBinding
			}
			if !errors.Is(err, want) || got != (Verified{}) {
				t.Fatalf("unsigned substitution = (%+v,%v), want zero/%v", got, err, want)
			}
			// A genuinely signed foreign agreement still cannot spend our binding.
			if tc.binding {
				foreign, err := Issue(changed.Terms, key)
				if err != nil {
					t.Fatalf("Issue(foreign) error = %v, want nil", err)
				}
				got, err = Verify(foreign, baseline.Terms.Binding, keys)
				if !errors.Is(err, core.ErrAccessPermitBinding) || got != (Verified{}) {
					t.Fatalf("authentic foreign proof = (%+v,%v), want zero/binding refusal", got, err)
				}
			}
		})
	}
}

func TestPermitDecisionExhaustiveByteDomain(t *testing.T) {
	t.Parallel()
	for raw := 0; raw <= math.MaxUint8; raw++ {
		got := Decision(raw).Validate()
		want := raw == int(DecisionAllow) || raw == int(DecisionRefuse)
		if (got == nil) != want || (!want && !errors.Is(got, core.ErrAccessPermitContract)) {
			t.Fatalf("Decision(%d).Validate() = %v, want admitted=%t", raw, got, want)
		}
	}
}

func FuzzPermitAuthenticatedSemanticClosure(f *testing.F) {
	seed, keys, _ := permitFixture(f)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{"terms":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		err := got.UnmarshalJSON(data)
		decoded, decodeErr := Decode(bytes.NewReader(data))
		if err != nil {
			if !errors.Is(err, core.ErrAccessPermitContract) || !errors.Is(decodeErr, core.ErrAccessPermitContract) || got != seed || decoded != (Document{}) {
				t.Fatalf("decode refusal = (%v,%v), want typed refusal and unchanged/zero values", err, decodeErr)
			}
			return
		}
		if decodeErr != nil || decoded != got || got.Validate() != nil {
			t.Fatalf("admitted document parity = (%+v,%v), want %+v and nil", decoded, decodeErr, got)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > DocumentMaximumBytes {
			t.Fatalf("canonical bytes/error = %d/%v, want bounded/nil", len(encoded), err)
		}
		var roundTrip Document
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("canonical round trip = (%+v,%v), want (%+v,nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second projection equal/error = %t/%v, want true/nil", bytes.Equal(second, encoded), err)
		}
		proof, err := Verify(got, seed.Terms.Binding, keys)
		if (got == seed) != (err == nil) {
			t.Fatalf("authentication acceptance = %t, want exact signed-seed membership %t", err == nil, got == seed)
		}
		if err == nil {
			if got != seed || proof.Validate() != nil {
				t.Fatalf("authenticated mutation = %+v, want exact independently signed seed", got)
			}
		} else if proof != (Verified{}) || !(errors.Is(err, core.ErrAccessPermitDenied) || errors.Is(err, core.ErrAccessPermitBinding)) {
			t.Fatalf("authentication refusal = (%+v,%v), want zero/typed refusal", proof, err)
		}
	})
}

func BenchmarkPermitVerification(b *testing.B) {
	document, keys, _ := permitFixture(b)
	if _, err := Verify(document, document.Terms.Binding, keys); err != nil {
		b.Fatalf("fixture Verify() error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		got, err := Verify(document, document.Terms.Binding, keys)
		if err != nil || got.Validate() != nil {
			b.Fatalf("Verify() error = %v, want nil and valid proof", err)
		}
	}
}

func FuzzPermitDecisionSemanticDomain(f *testing.F) {
	for _, decision := range []Decision{DecisionAllow, DecisionRefuse} {
		data, err := decision.MarshalJSON()
		if err != nil {
			f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte(`"future"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := DecisionRefuse
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrAccessPermitContract) || got != DecisionRefuse {
				t.Fatalf("decision refusal = (%v,%v), want preserved/typed", got, err)
			}
			return
		}
		text, err := core.DecodeJSONStringToken(data)
		if err != nil || (got == DecisionAllow && text != DecisionAllowToken) || (got == DecisionRefuse && text != DecisionRefuseToken) || !got.IsValid() {
			t.Fatalf("accepted decision = (%v,%q,%v), want exact declared arm", got, text, err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v, want nil", err)
		}
		var second Decision
		if err := second.UnmarshalJSON(encoded); err != nil || second != got {
			t.Fatalf("round trip = (%v,%v), want (%v,nil)", second, err, got)
		}
	})
}
