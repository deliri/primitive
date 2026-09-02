package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestCapabilityDocumentValidOperations(t *testing.T) {
	t.Parallel()
	claim, trusted := schedulingClaimDocumentFixture(t)

	cases := []struct {
		name string
		run  func() error
		why  string
	}{
		{name: "scheduling payload validates its complete policy closure", run: claim.Capability.Payload.Validate, why: "scheduling policy owner"},
		{name: "member payload validates its admitted run closure", run: claim.Members[0].Payload.Validate, why: "member policy owner"},
		{name: "experiment payload validates its execution closure", run: claim.Direct[0].Payload.Validate, why: "experiment policy owner"},
		{name: "scheduling document verifies its independent signature", run: func() error { return runnercontrol.VerifySchedulingCapability(claim.Capability, trusted) }, why: "scheduling authentication boundary"},
		{name: "member document verifies its independent signature", run: func() error { return runnercontrol.VerifyMemberCapability(claim.Members[0], trusted) }, why: "member authentication boundary"},
		{name: "experiment document verifies its independent signature", run: func() error { return runnercontrol.VerifyExperimentCapability(claim.Direct[0], trusted) }, why: "experiment authentication boundary"},
		{name: "claim validates all signed digest links", run: claim.Validate, why: "chain structure boundary"},
		{name: "claim verifies all signed layers", run: func() error { return runnercontrol.VerifySchedulingClaim(claim, trusted) }, why: "chain authentication boundary"},
		{name: "claim record seals exact canonical bytes", run: func() error {
			record, err := runnercontrol.NewSchedulingClaimRecord(claim)
			return errors.Join(err, record.Validate())
		}, why: "sealed projection boundary"},
		{name: "canonical replay retains the complete authenticated chain", run: func() error {
			encoded, err := claim.MarshalJSON()
			var replay runnercontrol.SchedulingClaim
			return errors.Join(err, replay.UnmarshalJSON(encoded), runnercontrol.VerifySchedulingClaim(replay, trusted))
		}, why: "durable replay boundary"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.run(); err != nil {
				t.Fatalf("capability operation (%s) error = %v, want nil", tc.why, err)
			}
		})
	}
}

func TestCapabilityDocumentExternalIngressHostileBoundaries(t *testing.T) {
	t.Parallel()
	claim, _ := schedulingClaimDocumentFixture(t)
	scheduling := mustCapabilityDocumentJSON(t, claim.Capability)
	member := mustCapabilityDocumentJSON(t, claim.Members[0])
	experiment := mustCapabilityDocumentJSON(t, claim.Direct[0])
	payloadPrefix, attestation := splitCapabilityDocument(t, scheduling)

	type decodeCapabilityDocument func([]byte) error
	decodeScheduling := func(data []byte) error {
		var got runnercontrol.SchedulingCapabilityDocument
		return got.UnmarshalJSON(data)
	}
	decodeMember := func(data []byte) error {
		var got runnercontrol.MemberCapabilityDocument
		return got.UnmarshalJSON(data)
	}
	decodeExperiment := func(data []byte) error {
		var got runnercontrol.ExperimentCapabilityDocument
		return got.UnmarshalJSON(data)
	}

	cases := []struct {
		decode  decodeCapabilityDocument
		name    string
		why     string
		wire    []byte
		wantErr bool
	}{
		{name: "canonical scheduling document is admitted", wire: scheduling, decode: decodeScheduling, why: "canonical framing floor"},
		{name: "surrounding JSON whitespace remains semantically neutral", wire: append(append([]byte(" \n"), scheduling...), '\t'), decode: decodeScheduling, why: "permitted JSON whitespace boundary"},
		{name: "empty document is refused", wire: []byte{}, decode: decodeScheduling, wantErr: true, why: "minimum byte extent"},
		{name: "trailing second document is refused", wire: append(append([]byte{}, scheduling...), []byte(` true`)...), decode: decodeScheduling, wantErr: true, why: "single-document framing"},
		{name: "unknown top-level member is refused", wire: insertCapabilityWire(t, scheduling, []byte(`{"payload":`), []byte(`{"future":true,"payload":`)), decode: decodeScheduling, wantErr: true, why: "closed top-level protocol"},
		{name: "duplicate payload member is refused", wire: insertCapabilityWire(t, scheduling, []byte(`{"payload":`), []byte(`{"payload":null,"payload":`)), decode: decodeScheduling, wantErr: true, why: "duplicate-name refusal"},
		{name: "missing payload is refused", wire: joinCapabilityWire([]byte(`{"attestation":`), attestation, []byte(`}`)), decode: decodeScheduling, wantErr: true, why: "required payload member"},
		{name: "missing attestation is refused", wire: joinCapabilityWire(payloadPrefix, []byte(`}`)), decode: decodeScheduling, wantErr: true, why: "required attestation member"},
		{name: "payload null is refused", wire: joinCapabilityWire([]byte(`{"payload":null,"attestation":`), attestation, []byte(`}`)), decode: decodeScheduling, wantErr: true, why: "payload object type"},
		{name: "attestation null is refused", wire: joinCapabilityWire(payloadPrefix, []byte(`,"attestation":null}`)), decode: decodeScheduling, wantErr: true, why: "attestation object type"},
		{name: "payload array is refused", wire: joinCapabilityWire([]byte(`{"payload":[],"attestation":`), attestation, []byte(`}`)), decode: decodeScheduling, wantErr: true, why: "payload container type"},
		{name: "attestation string is refused", wire: joinCapabilityWire(payloadPrefix, []byte(`,"attestation":"signed"}`)), decode: decodeScheduling, wantErr: true, why: "attestation container type"},
		{name: "unknown payload member is refused", wire: insertCapabilityWire(t, scheduling, []byte(`{"payload":{`), []byte(`{"payload":{"future":true,`)), decode: decodeScheduling, wantErr: true, why: "closed payload protocol"},
		{name: "duplicate payload schema is refused", wire: insertCapabilityWire(t, scheduling, []byte(`{"payload":{`), []byte(`{"payload":{"schema_version":1,`)), decode: decodeScheduling, wantErr: true, why: "nested duplicate-name refusal"},
		{name: "unknown attestation member is refused", wire: insertCapabilityWire(t, scheduling, []byte(`,"attestation":{`), []byte(`,"attestation":{"future":true,`)), decode: decodeScheduling, wantErr: true, why: "closed attestation protocol"},
		{name: "duplicate attestation domain is refused", wire: insertCapabilityWire(t, scheduling, []byte(`,"attestation":{`), []byte(`,"attestation":{"domain":"future",`)), decode: decodeScheduling, wantErr: true, why: "attestation duplicate-name refusal"},
		{name: "lowest signing domain is admitted only by scheduling document", wire: scheduling, decode: decodeScheduling, why: "lowest published domain arm"},
		{name: "highest signing domain is admitted only by experiment document", wire: experiment, decode: decodeExperiment, why: "highest published domain arm"},
		{name: "member signing domain is admitted only by member document", wire: member, decode: decodeMember, why: "middle published domain arm"},
		{name: "unknown-next signing domain is refused", wire: replaceCapabilityWire(t, scheduling, []byte(`"`+runnercontrol.SchedulingCapabilitySigningDomainToken+`"`), []byte(`"primitive-runner-scheduling-capability-2026-2"`)), decode: decodeScheduling, wantErr: true, why: "future domain refusal"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.decode(tc.wire)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrJSONContract) {
					t.Fatalf("capability document decode (%s) error = %v, want %v", tc.why, gotErr, core.ErrJSONContract)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("capability document decode (%s) error = %v, want nil", tc.why, gotErr)
			}
		})
	}
}

func TestCapabilitySigningDomainExhaustive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		want    string
		domain  runnercontrol.CapabilitySigningDomain
		wantErr bool
	}{
		{name: "scheduling domain is the lowest valid arm", domain: runnercontrol.CapabilitySigningDomainSchedulingV1, want: runnercontrol.SchedulingCapabilitySigningDomainToken},
		{name: "member domain is the middle valid arm", domain: runnercontrol.CapabilitySigningDomainMemberV1, want: runnercontrol.MemberCapabilitySigningDomainToken},
		{name: "experiment domain is the highest valid arm", domain: runnercontrol.CapabilitySigningDomainExperimentV1, want: runnercontrol.ExperimentCapabilitySigningDomainToken},
		{name: "zero domain is refused", domain: runnercontrol.CapabilitySigningDomainUnknown, wantErr: true},
		{name: "unknown-next domain is refused", domain: runnercontrol.CapabilitySigningDomainExperimentV1 + 1, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.domain.Validate()
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrPrimitiveContract) || tc.domain.IsValid() {
					t.Fatalf("CapabilitySigningDomain.Validate() = (%v, valid %t), want (%v, false)", gotErr, tc.domain.IsValid(), core.ErrPrimitiveContract)
				}
				return
			}
			if gotErr != nil || !tc.domain.IsValid() || tc.domain.String() != tc.want {
				t.Fatalf("CapabilitySigningDomain = (error %v, valid %t, text %q), want (nil, true, %q)", gotErr, tc.domain.IsValid(), tc.domain.String(), tc.want)
			}
		})
	}
}

func mustCapabilityDocumentJSON[T interface{ MarshalJSON() ([]byte, error) }](t testing.TB, value T) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil || len(got) == 0 {
		t.Fatalf("capability document MarshalJSON() = (%q, %v), want non-empty bytes and nil", got, err)
	}
	return got
}

func splitCapabilityDocument(t testing.TB, document []byte) ([]byte, []byte) {
	t.Helper()
	delimiter := []byte(`,"attestation":`)
	index := bytes.Index(document, delimiter)
	if index < 0 || len(document) == 0 || document[len(document)-1] != '}' {
		t.Fatalf("capability document split index = %d for %q, want a canonical object", index, document)
	}
	payload := append([]byte{}, document[:index]...)
	attestation := append([]byte{}, document[index+len(delimiter):len(document)-1]...)
	return payload, attestation
}

func insertCapabilityWire(t testing.TB, document, old, replacement []byte) []byte {
	t.Helper()
	return replaceCapabilityWire(t, document, old, replacement)
}

func replaceCapabilityWire(t testing.TB, document, old, replacement []byte) []byte {
	t.Helper()
	got := bytes.Replace(document, old, replacement, 1)
	if bytes.Equal(got, document) {
		t.Fatalf("capability wire mutation changed bytes = false for %q, want true", old)
	}
	return got
}

func joinCapabilityWire(parts ...[]byte) []byte {
	var got []byte
	for _, part := range parts {
		got = append(got, part...)
	}
	return got
}
