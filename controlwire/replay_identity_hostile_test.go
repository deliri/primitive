package controlwire_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

// FuzzReplayIdentityExternalJSONSemanticClosure drives the persisted authority
// record through strict external decoding. Acceptance must close canonically;
// rejection must retain both typed identity and the populated receiver.
func FuzzReplayIdentityExternalJSONSemanticClosure(f *testing.F) {
	fixture := productionSocketFixture(f)
	seed, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		f.Fatalf("CommitReplayIdentity(seed) error = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("ReplayIdentity.MarshalJSON(seed) error = %v, want nil", err)
	}
	conflicting, err := controlwire.CommitReplayIdentity(registrationRequestWithDistinctToken(f, fixture.request))
	if err != nil {
		f.Fatalf("CommitReplayIdentity(conflicting seed) error = %v, want nil", err)
	}
	conflictingCanonical, err := conflicting.MarshalJSON()
	if err != nil {
		f.Fatalf("ReplayIdentity.MarshalJSON(conflicting seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(conflictingCanonical)
	for _, hostile := range replayIdentityHostileDocuments(canonical) {
		f.Add(hostile.document)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrControlWireContract) ||
				!errors.Is(gotErr, core.ErrJSONContract) || got != seed {
				t.Fatalf("ReplayIdentity.UnmarshalJSON(rejected) = (%v, %v), want preserved and errors.Is(..., %v, %v)", got, gotErr, core.ErrControlWireContract, core.ErrJSONContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("ReplayIdentity.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > controlwire.ReplayIdentityJSONMaximumBytes {
			t.Fatalf("ReplayIdentity.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var roundTrip controlwire.ReplayIdentity
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("ReplayIdentity canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("ReplayIdentity second canonical projection = (%d bytes, %v), want exact %d bytes and nil", len(second), err, len(encoded))
		}
	})
}

// TestReplayIdentitySeparatesExactReplayFromConflictingNonceReuse is the
// authority-side decision the HTTP Idempotency-Key alone cannot make. The key
// proves which slot to inspect; the canonical request commitment proves
// whether the bytes admitted into that slot are the same request.
func TestReplayIdentitySeparatesExactReplayFromConflictingNonceReuse(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	original, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		t.Fatalf("CommitReplayIdentity(original) error = %v, want nil", err)
	}
	identical, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		t.Fatalf("CommitReplayIdentity(identical) error = %v, want nil", err)
	}
	if !original.Equal(identical) || !identical.Equal(original) {
		t.Fatalf("ReplayIdentity.Equal(exact pair) = (%t, %t), want (true, true)", original.Equal(identical), identical.Equal(original))
	}
	exact, err := controlwire.CheckReplay(controlwire.ReplayCheck{
		Existing: &original,
		Incoming: identical,
	})
	if err != nil || exact != controlwire.ReplayDispositionExact {
		t.Fatalf("CheckReplay(identical) = (%v, %v), want (%v, nil)", exact, err, controlwire.ReplayDispositionExact)
	}

	conflictingRequest := registrationRequestWithDistinctToken(t, fixture.request)
	conflicting, err := controlwire.CommitReplayIdentity(conflictingRequest)
	if err != nil {
		t.Fatalf("CommitReplayIdentity(conflicting) error = %v, want nil", err)
	}
	if original.Equal(conflicting) || conflicting.Equal(original) ||
		original.Equal(controlwire.ReplayIdentity{}) || (controlwire.ReplayIdentity{}).Equal(original) ||
		(controlwire.ReplayIdentity{}).Equal(controlwire.ReplayIdentity{}) {
		t.Fatalf("ReplayIdentity.Equal(distinct/zero values) admitted a non-exact or invalid identity")
	}
	disposition, err := controlwire.CheckReplay(controlwire.ReplayCheck{
		Existing: &original,
		Incoming: conflicting,
	})
	if disposition != controlwire.ReplayDispositionUnknown ||
		!errors.Is(err, core.ErrControlWireReplayConflict) ||
		!errors.Is(err, core.ErrControlWireContract) {
		t.Fatalf("CheckReplay(conflicting nonce reuse) = (%v, %v), want zero and errors.Is(..., %v, %v)", disposition, err, core.ErrControlWireReplayConflict, core.ErrControlWireContract)
	}
}

// TestRequestCommitmentMatchesAnIndependentSHA256Frame pins the commitment to
// the canonical request bytes with a standard-library oracle. A test that only
// compared two CommitReplayIdentity calls would let identical production bugs
// agree with each other.
func TestRequestCommitmentMatchesAnIndependentSHA256Frame(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	canonical, err := fixture.request.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}
	identity, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		t.Fatalf("CommitReplayIdentity() error = %v, want nil", err)
	}
	encoded, err := identity.MarshalJSON()
	if err != nil {
		t.Fatalf("ReplayIdentity.MarshalJSON() error = %v, want nil", err)
	}
	var persisted struct {
		Commitment core.SHA256Digest `json:"request_commitment"`
	}
	if err := json.Unmarshal(encoded, &persisted); err != nil {
		t.Fatalf("json.Unmarshal(replay identity oracle) error = %v, want nil", err)
	}
	framed := make([]byte, 0, len(controlwire.ReplayIdentityCommitmentDomain)+1+len(canonical))
	framed = append(framed, controlwire.ReplayIdentityCommitmentDomain...)
	framed = append(framed, controlwire.ReplayIdentityFrameSeparator)
	framed = append(framed, canonical...)
	want := core.NewSHA256Digest(sha256.Sum256(framed))
	if persisted.Commitment != want {
		t.Fatalf("persisted request commitment = %v, want independent SHA-256 frame %v", persisted.Commitment, want)
	}
}

// TestRequestCommitmentChangesAcrossEveryRegistrationFact proves the closure
// is not merely a nonce hash. Every independently mutable accepted fact moves
// the commitment, while independently invalid device bindings are refused
// before a replay identity can escape.
func TestRequestCommitmentChangesAcrossEveryRegistrationFact(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	baseline, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		t.Fatalf("CommitReplayIdentity(baseline) error = %v, want nil", err)
	}
	distinctNonceBytes := [core.SHA256DigestBytes]byte{2}
	distinctNonce, err := controlwire.NewRequestNonce(distinctNonceBytes)
	if err != nil {
		t.Fatalf("NewRequestNonce(distinct) error = %v, want nil", err)
	}
	distinctBuild, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Version:  core.NewReleaseVersion(2026, 0, 54),
		Commit:   fixture.request.Build.Commit(),
		Platform: fixture.request.Build.Platform(),
		Offering: fixture.request.Build.Offering(),
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity(distinct version) error = %v, want nil", err)
	}
	distinctOfferingBuild, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Version:  fixture.request.Build.Version(),
		Commit:   fixture.request.Build.Commit(),
		Platform: fixture.request.Build.Platform(),
		Offering: controlwireExternalOfferingFixture(t, 2),
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity(distinct offering) error = %v, want nil", err)
	}
	distinctInstallation := replayInstallation(t, 0x61, 0x71)
	cases := []struct {
		mutate func(*controlplane.RegistrationRequest)
		name   string
	}{
		{name: "registration token changes", mutate: func(request *controlplane.RegistrationRequest) {
			*request = registrationRequestWithDistinctToken(t, *request)
		}},
		{name: "request nonce changes", mutate: func(request *controlplane.RegistrationRequest) { request.RequestNonce = distinctNonce }},
		{name: "build version changes", mutate: func(request *controlplane.RegistrationRequest) { request.Build = distinctBuild }},
		{name: "build offering and route change", mutate: func(request *controlplane.RegistrationRequest) { request.Build = distinctOfferingBuild }},
		{name: "device key and derived installation change", mutate: func(request *controlplane.RegistrationRequest) {
			request.DeviceKey = distinctInstallation.DevicePublic
			request.Installation = distinctInstallation.Certificate.Body.Subject.DeviceID
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := fixture.request
			tc.mutate(&request)
			got, gotErr := controlwire.CommitReplayIdentity(request)
			if gotErr != nil || got.Validate() != nil || got == baseline {
				t.Fatalf("CommitReplayIdentity(%s) = (%v, %v), want distinct validated identity and nil", tc.name, got, gotErr)
			}
		})
	}

	invalid := []struct {
		mutate func(*controlplane.RegistrationRequest)
		name   string
	}{
		{name: "device key changes without installation", mutate: func(request *controlplane.RegistrationRequest) { request.DeviceKey = distinctInstallation.DevicePublic }},
		{name: "installation changes without device key", mutate: func(request *controlplane.RegistrationRequest) {
			request.Installation = distinctInstallation.Certificate.Body.Subject.DeviceID
		}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := fixture.request
			tc.mutate(&request)
			got, gotErr := controlwire.CommitReplayIdentity(request)
			if got != (controlwire.ReplayIdentity{}) ||
				!errors.Is(gotErr, core.ErrControlWireContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneInstallationBinding) {
				t.Fatalf("CommitReplayIdentity(%s) = (%v, %v), want zero and errors.Is(..., %v, %v)", tc.name, got, gotErr, core.ErrControlWireContract, core.ErrControlPlaneInstallationBinding)
			}
		})
	}
}

// TestReplayDispositionExhaustsItsDomainAndReplayCheckRefusals ensures no zero
// or future enum can masquerade as a decision and every non-success check
// returns a zero disposition.
func TestReplayDispositionExhaustsItsDomainAndReplayCheckRefusals(t *testing.T) {
	t.Parallel()

	for raw := range math.MaxUint8 + 1 {
		disposition := controlwire.ReplayDisposition(raw)
		wantValid := disposition == controlwire.ReplayDispositionFresh ||
			disposition == controlwire.ReplayDispositionExact
		gotValid := disposition.Validate() == nil
		if gotValid != wantValid || disposition.IsValid() != wantValid {
			t.Fatalf("ReplayDisposition(%d) Validate/IsValid = (%t, %t), want (%t, %t)", raw, gotValid, disposition.IsValid(), wantValid, wantValid)
		}
		if (disposition.String() != "") != wantValid {
			t.Fatalf("ReplayDisposition(%d).String() = %q, want diagnostic present %t", raw, disposition.String(), wantValid)
		}
	}

	fixture := productionSocketFixture(t)
	incoming, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		t.Fatalf("CommitReplayIdentity(incoming) error = %v, want nil", err)
	}
	otherRequest := fixture.request
	otherNonceBytes := [core.SHA256DigestBytes]byte{3}
	otherRequest.RequestNonce, err = controlwire.NewRequestNonce(otherNonceBytes)
	if err != nil {
		t.Fatalf("NewRequestNonce(other slot) error = %v, want nil", err)
	}
	otherSlot, err := controlwire.CommitReplayIdentity(otherRequest)
	if err != nil {
		t.Fatalf("CommitReplayIdentity(other slot) error = %v, want nil", err)
	}
	cases := []struct {
		name  string
		want  []error
		check controlwire.ReplayCheck
	}{
		{name: "zero incoming is refused", check: controlwire.ReplayCheck{}, want: []error{core.ErrControlWireContract}},
		{name: "zero existing is refused", check: controlwire.ReplayCheck{Existing: new(controlwire.ReplayIdentity), Incoming: incoming}, want: []error{core.ErrControlWireContract}},
		{name: "existing record from a different nonce slot is refused", check: controlwire.ReplayCheck{Existing: &otherSlot, Incoming: incoming}, want: []error{core.ErrControlWireContract, core.ErrControlWireNonce}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := controlwire.CheckReplay(tc.check)
			if got != controlwire.ReplayDispositionUnknown {
				t.Fatalf("CheckReplay(%s) disposition = %v, want %v", tc.name, got, controlwire.ReplayDispositionUnknown)
			}
			for _, wantErr := range tc.want {
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("CheckReplay(%s) error = %v, want errors.Is %v", tc.name, gotErr, wantErr)
				}
			}
		})
	}
}

func replayInstallation(t testing.TB, authorityByte, deviceByte byte) controlplanetest.Installation {
	t.Helper()

	authoritySeed := [ed25519.SeedSize]byte{}
	deviceSeed := [ed25519.SeedSize]byte{}
	for index := range authoritySeed {
		authoritySeed[index] = authorityByte
		deviceSeed[index] = deviceByte
	}
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: authoritySeed, DeviceSeed: deviceSeed, Offering: controlwireExternalOfferingFixture(t, 3),
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	return installation
}

// TestReplayIdentityCanonicalJSONPressure proves the persisted authority fact
// is strict, canonical, bounded, and receiver preserving. Accepted inputs are
// derived from one compiler-built request; rejected inputs attack framing and
// schema rather than hand-writing a second valid protocol.
func TestReplayIdentityCanonicalJSONPressure(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	want, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		t.Fatalf("CommitReplayIdentity() error = %v, want nil", err)
	}
	canonical, err := want.MarshalJSON()
	if err != nil {
		t.Fatalf("ReplayIdentity.MarshalJSON() error = %v, want nil", err)
	}
	if got, gotErr := (controlwire.ReplayIdentity{}).MarshalJSON(); got != nil ||
		!errors.Is(gotErr, core.ErrControlWireContract) || !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("zero ReplayIdentity.MarshalJSON() = (%v, %v), want nil and errors.Is(..., %v, %v)", got, gotErr, core.ErrControlWireContract, core.ErrJSONContract)
	}
	if gotErr := (*controlwire.ReplayIdentity)(nil).UnmarshalJSON(canonical); !errors.Is(gotErr, core.ErrControlWireContract) || !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("nil ReplayIdentity.UnmarshalJSON() error = %v, want errors.Is(..., %v, %v)", gotErr, core.ErrControlWireContract, core.ErrJSONContract)
	}
	if gotErr := (*controlwire.RequestCommitment)(nil).UnmarshalJSON(nil); !errors.Is(gotErr, core.ErrControlWireContract) || !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("nil RequestCommitment.UnmarshalJSON() error = %v, want errors.Is(..., %v, %v)", gotErr, core.ErrControlWireContract, core.ErrJSONContract)
	}

	valid := [][]byte{
		canonical,
		append([]byte(" \n\t"), canonical...),
		append(append([]byte{}, canonical...), ' ', '\n', '\t'),
		append(append([]byte(" \n"), canonical...), ' ', '\n'),
		append([]byte("\t"), canonical...),
		append(append([]byte{}, canonical...), '\r'),
		append([]byte("\r\n"), canonical...),
		append(append([]byte{}, canonical...), '\r', '\n'),
		append([]byte(" \t\r\n"), canonical...),
		append(append([]byte("\n\t"), canonical...), '\t', '\n'),
	}
	for index, document := range valid {
		var got controlwire.ReplayIdentity
		gotErr := got.UnmarshalJSON(document)
		if gotErr != nil || got != want {
			t.Fatalf("ReplayIdentity.UnmarshalJSON(valid representation %d) = (%v, %v), want (%v, nil)", index, got, gotErr, want)
		}
	}

	hostile := replayIdentityHostileDocuments(canonical)
	if len(hostile) < 20 {
		t.Fatalf("replay identity hostile inventory = %d, want at least 20", len(hostile))
	}
	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := want
			gotErr := got.UnmarshalJSON(tc.document)
			if !errors.Is(gotErr, core.ErrControlWireContract) ||
				!errors.Is(gotErr, core.ErrJSONContract) || got != want {
				t.Fatalf("ReplayIdentity.UnmarshalJSON(%s) = (%v, %v), want preserved %v and errors.Is(..., %v, %v)", tc.name, got, gotErr, want, core.ErrControlWireContract, core.ErrJSONContract)
			}
		})
	}
}

func registrationRequestWithDistinctToken(t testing.TB, request controlplane.RegistrationRequest) controlplane.RegistrationRequest {
	t.Helper()

	tokenBytes := [controlwire.RegistrationTokenBytes]byte{2}
	token, err := controlwire.NewRegistrationToken(tokenBytes)
	if err != nil {
		t.Fatalf("NewRegistrationToken(distinct) error = %v, want nil", err)
	}
	request.Token = token
	if err := request.Validate(); err != nil {
		t.Fatalf("distinct-token RegistrationRequest.Validate() error = %v, want nil", err)
	}
	return request
}

type replayIdentityHostileDocument struct {
	name     string
	document []byte
}

func replayIdentityHostileDocuments(canonical []byte) []replayIdentityHostileDocument {
	last := len(canonical) - 1
	return []replayIdentityHostileDocument{
		{name: "empty input", document: nil},
		{name: "whitespace only", document: []byte(" \n\t")},
		{name: "JSON null", document: []byte("null")},
		{name: "empty object", document: []byte("{}")},
		{name: "empty array", document: []byte("[]")},
		{name: "string instead of object", document: []byte(`"replay"`)},
		{name: "number instead of object", document: []byte("1")},
		{name: "boolean instead of object", document: []byte("true")},
		{name: "truncated before first member", document: canonical[:1]},
		{name: "truncated at midpoint", document: canonical[:len(canonical)/2]},
		{name: "truncated before closing brace", document: canonical[:last]},
		{name: "trailing second document", document: append(append([]byte{}, canonical...), canonical...)},
		{name: "trailing object", document: append(append([]byte{}, canonical...), '{', '}')},
		{name: "trailing scalar", document: append(append([]byte{}, canonical...), '0')},
		{name: "unknown member", document: append(append([]byte{}, canonical[:last]...), []byte(`,"unknown":0}`)...)},
		{name: "duplicate complete object member", document: append(append([]byte{}, canonical[:last]...), canonical[1:]...)},
		{name: "one byte above document ceiling", document: bytes.Repeat([]byte{' '}, controlwire.ReplayIdentityJSONMaximumBytes+1)},
		{name: "maximum integer token", document: []byte("18446744073709551615")},
		{name: "negative integer token", document: []byte("-1")},
		{name: "floating point token", document: []byte("1.5")},
		{name: "unterminated string", document: []byte(`"`)},
		{name: "unterminated array", document: []byte("[")},
		{name: "unterminated object", document: []byte("{")},
		{name: "invalid UTF-8", document: []byte{0xff}},
	}
}
