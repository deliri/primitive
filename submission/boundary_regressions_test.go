package submission

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

const submissionWhitespaceFixtureBytes = 1 << 20

type submissionJSONReceiver[T any] interface {
	*T
	UnmarshalJSON([]byte) error
}

func submissionPreservingDecode[T comparable, P submissionJSONReceiver[T]](want T) func([]byte) (bool, error) {
	return func(data []byte) (bool, error) {
		got := want
		err := P(&got).UnmarshalJSON(data)
		return got == want, err
	}
}

func TestSubmissionJSONExtentLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := submissionFixturesForFuzz(t)
	doors := []struct {
		decode func([]byte) (bool, error)
		name   string
		value  []byte
	}{
		{name: "request_payload", value: mustSubmissionJSON(t, fixture.requestPayload), decode: submissionPreservingDecode(fixture.requestPayload)},
		{name: "request_document", value: mustSubmissionJSON(t, fixture.requestDocument), decode: submissionPreservingDecode(fixture.requestDocument)},
		{name: "grant_payload", value: mustSubmissionJSON(t, fixture.grantPayload), decode: submissionPreservingDecode(fixture.grantPayload)},
		{name: "completion_payload", value: mustSubmissionJSON(t, fixture.completionPayload), decode: submissionPreservingDecode(fixture.completionPayload)},
		{name: "completion_document", value: mustSubmissionJSON(t, fixture.completionDocument), decode: submissionPreservingDecode(fixture.completionDocument)},
		{name: "grant_document", value: fixture.grantWire, decode: func(data []byte) (bool, error) {
			got := fixture.grantDocument
			err := got.UnmarshalJSON(data)
			return sameGrantDocument(got, fixture.grantDocument), err
		}},
		{name: "decision_document", value: fixture.decisionWire, decode: func(data []byte) (bool, error) {
			got := fixture.decisionDocument
			err := got.UnmarshalJSON(data)
			return sameReuseDecision(got, fixture.decisionDocument), err
		}},
	}
	for _, door := range doors {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			gap := bytes.Repeat([]byte(" "), submissionWhitespaceFixtureBytes)
			cases := []struct {
				wantErr error
				name    string
				data    []byte
			}{
				{name: "canonical", data: door.value, wantErr: nil},
				{name: "large_prefix", data: append(bytes.Clone(gap), door.value...), wantErr: nil},
				{name: "large_interior", data: append(append([]byte{'{'}, gap...), door.value[1:]...), wantErr: nil},
				{name: "large_suffix", data: append(bytes.Clone(door.value), gap...), wantErr: nil},
				{name: "neutral_whitespace", data: gap, wantErr: core.ErrJSONContract},
				{name: "second_value_after_gap", data: append(append(bytes.Clone(door.value), gap...), []byte("{}")...), wantErr: core.ErrJSONContract},
				{name: "truncated_after_large_prefix", data: append(bytes.Clone(gap), door.value[:len(door.value)-1]...), wantErr: core.ErrJSONContract},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					same, err := door.decode(tc.data)
					if !same || !errors.Is(err, tc.wantErr) || tc.wantErr != nil && !errors.Is(err, core.ErrControlPlaneContract) {
						t.Fatalf("preserved=%t error=%v, want exact facts and %v", same, err, tc.wantErr)
					}
				})
			}
		})
	}
}

func mustSubmissionJSON(t testing.TB, value submissionJSONValue) []byte {
	t.Helper()
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type submissionCanonicalJSON interface {
	attest.CanonicalBody[SigningDomain]
	submissionJSONValue
}

type submissionNilWriter struct{}

func (*submissionNilWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSubmissionCanonicalDestinationLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("canonical typed nil boundary"), 0x10)
	projection, err := IssueCompletion(CompletionIssuance{Signer: fixture.deviceSigner, Transfer: fixture.transfer, Request: fixture.request, Grant: fixture.grant, Nonce: fixture.nonce})
	if err != nil {
		t.Fatal(err)
	}
	document := receiveCompletionProjection(t, projection)
	bodies := []struct {
		valid submissionCanonicalJSON
		zero  submissionCanonicalJSON
		name  string
	}{
		{name: "request", valid: fixture.request, zero: RequestPayload{}},
		{name: "grant", valid: fixture.grantDocument.Payload, zero: GrantPayload{}},
		{name: "completion", valid: document.Payload, zero: CompletionPayload{}},
		{name: "projection", valid: projection.payload, zero: completionProjectionPayload{}},
	}
	for _, body := range bodies {
		t.Run(body.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				wantErr   error
				name      string
				nilWriter bool
				typedNil  bool
				invalid   bool
			}{
				{name: "valid", nilWriter: false, typedNil: false, invalid: false, wantErr: nil},
				{name: "absent_destination", nilWriter: true, typedNil: false, invalid: false, wantErr: core.ErrControlPlaneContract},
				{name: "typed_nil_destination", nilWriter: false, typedNil: true, invalid: false, wantErr: core.ErrControlPlaneContract},
				{name: "invalid_body_no_output", nilWriter: false, typedNil: false, invalid: true, wantErr: core.ErrJSONContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					var output bytes.Buffer
					var destination io.Writer = &output
					if tc.nilWriter {
						destination = nil
					}
					if tc.typedNil {
						destination = (*submissionNilWriter)(nil)
					}
					selected := body.valid
					if tc.invalid {
						selected = body.zero
					}
					err := selected.WriteCanonical(destination)
					if !errors.Is(err, tc.wantErr) || errors.Is(err, io.ErrClosedPipe) || tc.wantErr != nil && !errors.Is(err, core.ErrControlPlaneContract) {
						t.Fatalf("writer error=%v, want %v without calling absent writer", err, tc.wantErr)
					}
					if tc.wantErr != nil {
						if output.Len() != 0 {
							t.Fatalf("rejected body wrote %d bytes", output.Len())
						}
						return
					}
					want := mustSubmissionJSON(t, body.valid)
					if !bytes.Equal(output.Bytes(), want) {
						t.Fatalf("got canonical bytes=%d, want exact %d-byte payload", output.Len(), len(want))
					}
				})
			}
		})
	}
}

type submissionReadProbe struct{ reads int }

func (r *submissionReadProbe) Read([]byte) (int, error) {
	if r != nil {
		r.reads++
	}
	return 0, io.ErrClosedPipe
}

func TestUploadSourceAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newUploadCallFixture(t, []byte("source must stay unread"))
	for _, tc := range []struct {
		wantErr        error
		name           string
		nilSource      bool
		typedNil       bool
		invalidRequest bool
	}{
		{name: "owned_unread_source", nilSource: false, typedNil: false, invalidRequest: false, wantErr: nil},
		{name: "absent_source", nilSource: true, typedNil: false, invalidRequest: false, wantErr: core.ErrControlPlaneContract},
		{name: "typed_nil_source", nilSource: false, typedNil: true, invalidRequest: false, wantErr: core.ErrControlPlaneContract},
		{name: "invalid_request_no_read", nilSource: false, typedNil: false, invalidRequest: true, wantErr: core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &submissionReadProbe{}
			var reader io.Reader = source
			if tc.nilSource {
				reader = nil
			}
			if tc.typedNil {
				reader = (*submissionReadProbe)(nil)
			}
			request := UploadCallRequest{Source: reader, Request: fixture.request, Policy: providerPolicy(t)}
			if tc.invalidRequest {
				request.Request = RequestPayload{}
			}
			err := request.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate error=%v, want %v", err, tc.wantErr)
			}
			got, err := fixture.decision.UploadCall(request)
			if !errors.Is(err, tc.wantErr) || source.reads != 0 {
				t.Fatalf("UploadCall error=%v reads=%d, want %v and zero", err, source.reads, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !uploadCallIsZero(got) {
					t.Fatalf("got rejected call zero=%t, want true", uploadCallIsZero(got))
				}
				return
			}
			if got.Source != reader || got.Policy != request.Policy || got.Integrity != request.Request.Declaration.Integrity() {
				t.Fatalf("got source identity=%t policy=%v integrity=%v, want unchanged caller input", got.Source == reader, got.Policy, got.Integrity)
			}
		})
	}
}

func TestUploadDecisionConstructionLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newGrantFixture(t, grantFixtureRequest{})
	for _, tc := range []struct {
		wantErr error
		mutate  func(*GrantProjection)
		name    string
	}{
		{name: "exact_grant", mutate: func(*GrantProjection) {}, wantErr: nil},
		{name: "neutral_zero", mutate: func(g *GrantProjection) { *g = GrantProjection{} }, wantErr: core.ErrControlPlaneContract},
		{name: "missing_bearer", mutate: func(g *GrantProjection) { g.Capability = objectstore.UploadCapabilityProjection{} }, wantErr: core.ErrControlPlaneContract},
		{name: "missing_attestation", mutate: func(g *GrantProjection) { g.Attestation = attest.Envelope[SigningDomain]{} }, wantErr: core.ErrControlPlaneContract},
		{name: "wrong_signing_domain", mutate: func(g *GrantProjection) { g.Attestation.Domain = SigningDomainRequestV1 }, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "foreign_capability_commitment", mutate: func(g *GrantProjection) { g.Payload.Capability = objectstore.UploadCapabilityCommitment{} }, wantErr: core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			grant := fixture.projection
			tc.mutate(&grant)
			got, err := UploadDecision(grant)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UploadDecision error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (DecisionProjection{}) {
					t.Fatalf("got refused decision zero=%t, want true", got == (DecisionProjection{}))
				}
				data, err := got.MarshalJSON()
				if data != nil || !errors.Is(err, core.ErrJSONContract) {
					t.Fatalf("refused decision bytes=%d error=%v", len(data), err)
				}
				return
			}
			document := decodeDecisionProjection(t, got)
			if document.Kind != DecisionUpload || document.Evidence != nil || document.Grant == nil || !sameGrantDocument(*document.Grant, fixture.document) {
				t.Fatalf("got kind=%v grant present=%t evidence present=%t, want exact upload grant alone", document.Kind, document.Grant != nil, document.Evidence != nil)
			}
		})
	}
}
