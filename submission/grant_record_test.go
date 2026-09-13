package submission

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestGrantRecordRetainedAgreementLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("real retained transfer"), 0x10)
	document := receiveIssuedCompletion(t, fixture)
	encoded, err := fixture.grantRecord.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{testCapabilityObjectPrefix, testCapabilityQuery, "signed_url", "headers"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("retained grant disclosed %q, want no bearer", forbidden)
		}
	}
	var restarted GrantRecord
	if err := restarted.UnmarshalJSON(encoded); err != nil || restarted != fixture.grantRecord {
		t.Fatalf("restarted grant = (%v, %v), want exact non-secret agreement", restarted, err)
	}
	fromReceiver, err := fixture.grantDocument.Record()
	if err != nil || fromReceiver != restarted {
		t.Fatalf("receiver record = (%v, %v), want issuer record", fromReceiver, err)
	}
	expectation := CompletionExpectation{Request: fixture.request, Document: document, Grant: restarted, Provider: objectstore.ProviderGoogleCloudStorage, GrantKeys: fixture.grantKeys, CompletionKeys: fixture.deviceKeys, Nonce: fixture.nonce}
	got, err := VerifyCompletion(expectation)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := got.Payload()
	if err != nil || payload != document.Payload {
		t.Fatalf("restarted completion = (%v, %v), want exact provider facts", payload, err)
	}
	for _, tc := range []struct {
		name    string
		mutate  func(*CompletionExpectation)
		wantErr error
	}{
		{name: "missing retained grant produces no proof", mutate: func(v *CompletionExpectation) { v.Grant = GrantRecord{} }, wantErr: core.ErrControlPlaneContract},
		{name: "authority provider differs from completion", mutate: func(v *CompletionExpectation) { v.Provider = objectstore.ProviderAmazonS3 }, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "unset authority provider is not inferred from client", mutate: func(v *CompletionExpectation) { v.Provider = objectstore.ProviderUnknown }, wantErr: core.ErrControlPlaneContract},
		{name: "unsigned issue time mutation is rejected", mutate: func(v *CompletionExpectation) {
			v.Grant.Payload.IssuedAt = temporal.InstantFromNanoseconds(testGrantIssuedAt + 1)
		}, wantErr: core.ErrAttestVerification},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := expectation
			tc.mutate(&changed)
			if changed.Grant == expectation.Grant && changed.Provider == expectation.Provider {
				t.Fatalf("mutated grant/provider = (%v, %v), want one changed fact", changed.Grant, changed.Provider)
			}
			got, err := VerifyCompletion(changed)
			if !errors.Is(err, tc.wantErr) || got != (VerifiedCompletion{}) {
				t.Fatalf("completion = (%v, %v), want zero and %v", got, err, tc.wantErr)
			}
		})
	}
}

func FuzzGrantRecordSemanticAndSignatureClosure(f *testing.F) {
	fixture := newGrantFixture(f, grantFixtureRequest{})
	seed, err := fixture.projection.Record()
	if err != nil {
		f.Fatal(err)
	}
	encoded, err := seed.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add(encoded[:len(encoded)-1])
	f.Add(append(bytes.Clone(encoded), encoded...))
	f.Add(append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"payload":null}`)...))
	f.Add(append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"capability":"unowned bearer"}`)...))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) || got != seed {
				t.Fatalf("rejected grant = (%v, %v), want preserved and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		first, err := got.MarshalJSON()
		if err != nil || len(first) > GrantRecordJSONMaximumBytes {
			t.Fatalf("canonical grant = (%d bytes, %v), want bounded output", len(first), err)
		}
		var again GrantRecord
		if err := again.UnmarshalJSON(first); err != nil || again != got {
			t.Fatalf("round trip = (%v, %v), want exact grant", again, err)
		}
		second, err := again.MarshalJSON()
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("canonical second write = (%q, %v), want %q", second, err, first)
		}
		proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{Body: got.Payload, Envelope: got.Attestation, TrustedKeys: fixture.trusted})
		if err != nil {
			if !errors.Is(err, core.ErrAttestVerification) || proof != (attest.Verified[SigningDomain]{}) {
				t.Fatalf("signature refusal = (%v, %v), want zero and typed refusal", proof, err)
			}
			return
		}
		if got != seed {
			t.Fatalf("authenticated grant = %v, want exact genuinely signed seed", got)
		}
	})
}

func TestGrantRecordByteBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newGrantFixture(t, grantFixtureRequest{})
	seed, err := fixture.projection.Record()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := seed.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "one below record ceiling admits padded agreement", size: GrantRecordJSONMaximumBytes - 1},
		{name: "exact record ceiling admits padded agreement", size: GrantRecordJSONMaximumBytes},
		{name: "one above record ceiling preserves receiver", size: GrantRecordJSONMaximumBytes + 1, wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := append(bytes.Repeat([]byte(" "), tc.size-len(encoded)), encoded...)
			got := seed
			err := got.UnmarshalJSON(input)
			if !errors.Is(err, tc.wantErr) || got != seed {
				t.Fatalf("record at %d bytes = (%v, %v), want preserved agreement and %v", len(input), got, err, tc.wantErr)
			}
		})
	}
	if got, err := (GrantProjection{}).Record(); !errors.Is(err, core.ErrControlPlaneContract) || got != (GrantRecord{}) {
		t.Fatalf("absent issuer projection = (%v, %v), want zero and typed refusal", got, err)
	}
	if got, err := (GrantDocument{}).Record(); !errors.Is(err, core.ErrControlPlaneContract) || got != (GrantRecord{}) {
		t.Fatalf("absent received grant = (%v, %v), want zero and typed refusal", got, err)
	}
	if err := (*GrantRecord)(nil).UnmarshalJSON(encoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil receiver = %v, want typed refusal", err)
	}
}

func TestGrantRecordMalformedInputPreservesBothReceiverStates(t *testing.T) {
	t.Parallel()
	fixture := newGrantFixture(t, grantFixtureRequest{})
	seed, err := fixture.projection.Record()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := seed.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := seed.Payload.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := seed.Attestation.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "absent document"},
		{name: "truncated signed record", data: encoded[:len(encoded)-1]},
		{name: "second record outside envelope", data: append(bytes.Clone(encoded), encoded...)},
		{name: "injected bearer member", data: append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"capability":"bearer"}`)...)},
		{name: "duplicate payload member", data: append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"payload":null}`)...)},
		{name: "missing attestation", data: append(append([]byte(`{"payload":`), payload...), '}')},
		{name: "missing payload", data: append(append([]byte(`{"attestation":`), envelope...), '}')},
		{name: "scalar payload replacing typed facts", data: bytes.Replace(encoded, payload, []byte("true"), 1)},
		{name: "scalar attestation replacing signed frame", data: bytes.Replace(encoded, envelope, []byte("true"), 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, before := range []GrantRecord{{}, seed} {
				got := before
				err := got.UnmarshalJSON(tc.data)
				if !errors.Is(err, core.ErrJSONContract) || got != before {
					t.Fatalf("decode = (%v, %v), want unchanged %v and typed JSON refusal", got, err, before)
				}
			}
		})
	}
}

// Exact conversion is the compiler-visible no-bearer storage contract.
type grantRecordDisclosureContract struct {
	Payload     GrantPayload                   `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

var _ = grantRecordDisclosureContract(GrantRecord{})

func TestGrantRecordEverySignedFactMutationRefusesCompletion(t *testing.T) {
	t.Parallel()
	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("signed fact binding"), 0x10)
	document := receiveIssuedCompletion(t, fixture)
	foreign := newGrantFixture(t, grantFixtureRequest{objectName: "foreign.json", content: []byte("foreign content"), authorityByte: 0x61, authorizationByte: 0x62})
	other, err := foreign.projection.Record()
	if err != nil {
		t.Fatal(err)
	}
	base := CompletionExpectation{Request: fixture.request, Document: document, Grant: fixture.grantRecord, Provider: objectstore.ProviderGoogleCloudStorage, GrantKeys: fixture.grantKeys, CompletionKeys: fixture.deviceKeys, Nonce: fixture.nonce}
	if got, err := VerifyCompletion(base); err != nil || got == (VerifiedCompletion{}) {
		t.Fatalf("baseline = (%v, %v), want authentic proof", got, err)
	}
	for index, name := range []string{"request commitment", "authorization nonce", "capability commitment", "issue instant", "expiry instant", "retention instant", "envelope signer", "envelope body extent", "envelope body digest", "envelope signature"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			changed := base
			changed.Grant = mutateRetainedGrantFact(t, base.Grant, other, uint8(index), 1)
			got, err := VerifyCompletion(changed)
			if !errors.Is(err, core.ErrAttestVerification) || got != (VerifiedCompletion{}) {
				t.Fatalf("changed %s = (%v, %v), want zero and authentication refusal", name, got, err)
			}
		})
	}
}

func FuzzGrantRecordSignedFactRecombination(f *testing.F) {
	fixture := newGrantFixture(f, grantFixtureRequest{})
	foreign := newGrantFixture(f, grantFixtureRequest{objectName: "foreign.json", content: []byte("foreign content"), authorityByte: 0x61, authorizationByte: 0x62})
	seed, err := fixture.projection.Record()
	if err != nil {
		f.Fatal(err)
	}
	other, err := foreign.projection.Record()
	if err != nil {
		f.Fatal(err)
	}
	for selector := uint8(0); selector < 10; selector++ {
		f.Add(selector, uint32(1))
	}
	f.Fuzz(func(t *testing.T, selector uint8, delta uint32) {
		changed := mutateRetainedGrantFact(t, seed, other, selector%10, int64(delta%1000)+1)
		encoded, err := changed.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var got GrantRecord
		if err := got.UnmarshalJSON(encoded); err != nil || got != changed {
			t.Fatalf("admitted changed grant = (%v, %v), want exact structurally valid mutation", got, err)
		}
		proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{Body: got.Payload, Envelope: got.Attestation, TrustedKeys: fixture.trusted})
		if !errors.Is(err, core.ErrAttestVerification) || proof != (attest.Verified[SigningDomain]{}) {
			t.Fatalf("recombined grant = (%v, %v), want zero and authentication refusal", proof, err)
		}
	})
}

// Each selector changes exactly one independently named, nominally valid fact.
func mutateRetainedGrantFact(t testing.TB, seed, other GrantRecord, selector uint8, delta int64) GrantRecord {
	t.Helper()
	got := seed
	switch selector {
	case 0:
		got.Payload.Request = other.Payload.Request
	case 1:
		got.Payload.Authorization = other.Payload.Authorization
	case 2:
		got.Payload.Capability = other.Payload.Capability
	case 3:
		got.Payload.IssuedAt = temporal.InstantFromNanoseconds(testGrantIssuedAt + delta)
	case 4:
		got.Payload.ExpiresAt = temporal.InstantFromNanoseconds(testGrantExpiresAt + delta)
	case 5:
		got.Payload.RetainUntil = temporal.InstantFromNanoseconds(testGrantRetainUntil + delta)
	case 6:
		got.Attestation.Signer = other.Attestation.Signer
	case 7:
		length, err := seed.Attestation.BodyLength.Uint64()
		if err != nil {
			t.Fatal(err)
		}
		got.Attestation.BodyLength, err = core.NewByteCount(length + 1)
		if err != nil {
			t.Fatal(err)
		}
	case 8:
		got.Attestation.BodySHA256 = other.Attestation.BodySHA256
	case 9:
		got.Attestation.Signature = other.Attestation.Signature
	default:
		t.Fatalf("mutation selector = %d, want named signed fact", selector)
	}
	if got == seed {
		t.Fatalf("mutated grant = %v, want one changed fact", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("changed record Validate = %v, want nominally valid unauthenticated record", err)
	}
	return got
}

// This is a test-owned disk fixture, not the API's durable writer or manifest.
// The child has no original upload bearer, private signer, or in-memory proof.
type retainedCompletionFixture struct {
	Grant      GrantRecord        `json:"grant"`
	Request    RequestPayload     `json:"request"`
	Completion CompletionDocument `json:"completion"`
}

func TestGrantRecordFreshProcessLayerTriad(t *testing.T) {
	t.Parallel()
	const marker = "primitive-retained-completion-child"
	const maximum = 4 * GrantRecordJSONMaximumBytes
	if len(os.Args) >= 4 && os.Args[len(os.Args)-3] == marker {
		path, mode := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
		if err := errors.Join(readErr, file.Close()); err != nil {
			t.Fatal(err)
		}
		if len(data) > maximum {
			t.Fatalf("fixture bytes = %d, want at most %d", len(data), maximum)
		}
		stored, err := core.DecodeStrictJSONStructure[retainedCompletionFixture](data, core.DefaultStrictJSONLimits())
		if mode == "absent-grant" || mode == "truncated-record" {
			if !errors.Is(err, core.ErrJSONContract) || stored != (retainedCompletionFixture{}) {
				t.Fatalf("disk refusal = (%v, %v), want zero and JSON rejection", stored, err)
			}
			fmt.Fprintln(os.Stdout, marker+":"+mode)
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		// Trust is pinned independently of the persisted envelope's signer.
		grantPublic, _ := testSigningKey(t, 0x50)
		devicePublic, _ := testSigningKey(t, 0x80)
		grantKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{grantPublic}})
		if err != nil {
			t.Fatal(err)
		}
		deviceKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{devicePublic}})
		if err != nil {
			t.Fatal(err)
		}
		nonce, err := controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{0x31})
		if err != nil {
			t.Fatal(err)
		}
		got, err := VerifyCompletion(CompletionExpectation{Request: stored.Request, Document: stored.Completion, Grant: stored.Grant, Provider: objectstore.ProviderGoogleCloudStorage, GrantKeys: grantKeys, CompletionKeys: deviceKeys, Nonce: nonce})
		switch mode {
		case "authentic":
			payload, payloadErr := got.Payload()
			if err != nil || payloadErr != nil || payload != stored.Completion.Payload {
				t.Fatalf("fresh process proof = (%v, %v, %v), want exact signed completion", payload, err, payloadErr)
			}
		case "unsigned-time":
			if !errors.Is(err, core.ErrAttestVerification) || got != (VerifiedCompletion{}) {
				t.Fatalf("fresh process refusal = (%v, %v), want zero and signature rejection", got, err)
			}
		default:
			t.Fatalf("child mode = %q, want explicit contract", mode)
		}
		fmt.Fprintln(os.Stdout, marker+":"+mode)
		return
	}
	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("fresh process retained transfer"), 0x10)
	document := receiveIssuedCompletion(t, fixture)
	for _, mode := range []string{"authentic", "unsigned-time", "absent-grant", "truncated-record"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			stored := retainedCompletionFixture{Grant: fixture.grantRecord, Request: fixture.request, Completion: document}
			if mode == "unsigned-time" {
				stored.Grant.Payload.IssuedAt = temporal.InstantFromNanoseconds(testGrantIssuedAt + 1)
			}
			data, err := core.MarshalCanonicalJSONDocument(stored)
			if err != nil || len(data) == 0 || len(data) > maximum {
				t.Fatalf("fixture bytes = (%d, %v), want bounded nonempty document", len(data), err)
			}
			if mode == "absent-grant" {
				grantBytes, err := stored.Grant.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				changed := bytes.Replace(data, grantBytes, []byte("null"), 1)
				if bytes.Equal(changed, data) {
					t.Fatalf("absent grant mutation = %q, want removed grant", changed)
				}
				data = changed
			}
			if mode == "truncated-record" {
				data = data[:len(data)-1]
			}
			path := filepath.Join(dir, "non-secret-completion.json")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestGrantRecordFreshProcessLayerTriad$", "--", marker, path, mode)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil {
				t.Fatalf("child = (%v, stdout %q, stderr %q), want verified exit", err, stdout.String(), stderr.String())
			}
			if !bytes.Contains(stdout.Bytes(), []byte(marker+":"+mode+"\n")) {
				t.Fatalf("child stdout = %q, want completed contract marker", stdout.String())
			}
		})
	}
}
