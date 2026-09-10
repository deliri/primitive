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
		name   string
		value  []byte
		decode func([]byte) (bool, error)
	}{
		{"request_payload", mustSubmissionJSON(t, fixture.requestPayload), submissionPreservingDecode(fixture.requestPayload)},
		{"request_document", mustSubmissionJSON(t, fixture.requestDocument), submissionPreservingDecode(fixture.requestDocument)},
		{"grant_payload", mustSubmissionJSON(t, fixture.grantPayload), submissionPreservingDecode(fixture.grantPayload)},
		{"completion_payload", mustSubmissionJSON(t, fixture.completionPayload), submissionPreservingDecode(fixture.completionPayload)},
		{"completion_document", mustSubmissionJSON(t, fixture.completionDocument), submissionPreservingDecode(fixture.completionDocument)},
		{"grant_document", fixture.grantWire, func(data []byte) (bool, error) {
			got := fixture.grantDocument
			err := got.UnmarshalJSON(data)
			return sameGrantDocument(got, fixture.grantDocument), err
		}},
		{"decision_document", fixture.decisionWire, func(data []byte) (bool, error) {
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
				name    string
				data    []byte
				wantErr error
			}{
				{"canonical", door.value, nil},
				{"large_prefix", append(bytes.Clone(gap), door.value...), nil},
				{"large_interior", append(append([]byte{'{'}, gap...), door.value[1:]...), nil},
				{"large_suffix", append(bytes.Clone(door.value), gap...), nil},
				{"neutral_whitespace", gap, core.ErrJSONContract},
				{"second_value_after_gap", append(append(bytes.Clone(door.value), gap...), []byte("{}")...), core.ErrJSONContract},
				{"truncated_after_large_prefix", append(bytes.Clone(gap), door.value[:len(door.value)-1]...), core.ErrJSONContract},
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
		name  string
		valid submissionCanonicalJSON
		zero  submissionCanonicalJSON
	}{
		{"request", fixture.request, RequestPayload{}},
		{"grant", fixture.grantDocument.Payload, GrantPayload{}},
		{"completion", document.Payload, CompletionPayload{}},
		{"projection", projection.payload, completionProjectionPayload{}},
	}
	for _, body := range bodies {
		t.Run(body.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name      string
				nilWriter bool
				typedNil  bool
				invalid   bool
				wantErr   error
			}{
				{"valid", false, false, false, nil},
				{"absent_destination", true, false, false, core.ErrControlPlaneContract},
				{"typed_nil_destination", false, true, false, core.ErrControlPlaneContract},
				{"invalid_body_no_output", false, false, true, core.ErrJSONContract},
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
		name           string
		nilSource      bool
		typedNil       bool
		invalidRequest bool
		wantErr        error
	}{
		{"owned_unread_source", false, false, false, nil},
		{"absent_source", true, false, false, core.ErrControlPlaneContract},
		{"typed_nil_source", false, true, false, core.ErrControlPlaneContract},
		{"invalid_request_no_read", false, false, true, core.ErrControlPlaneContract},
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
		name    string
		mutate  func(*GrantProjection)
		wantErr error
	}{
		{"exact_grant", func(*GrantProjection) {}, nil},
		{"neutral_zero", func(g *GrantProjection) { *g = GrantProjection{} }, core.ErrControlPlaneContract},
		{"missing_bearer", func(g *GrantProjection) { g.Capability = objectstore.UploadCapabilityProjection{} }, core.ErrControlPlaneContract},
		{"missing_attestation", func(g *GrantProjection) { g.Attestation = attest.Envelope[SigningDomain]{} }, core.ErrControlPlaneContract},
		{"wrong_signing_domain", func(g *GrantProjection) { g.Attestation.Domain = SigningDomainRequestV1 }, core.ErrControlPlaneResponseBinding},
		{"foreign_capability_commitment", func(g *GrantProjection) { g.Payload.Capability = objectstore.UploadCapabilityCommitment{} }, core.ErrControlPlaneContract},
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
