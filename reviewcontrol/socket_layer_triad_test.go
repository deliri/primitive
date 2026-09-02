package reviewcontrol

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/proofledger"
)

type socketFixtures struct {
	issueRequest        IssueReviewRequest
	issueResponse       IssueReviewResponse
	readRequest         ReadReviewRequest
	readResponse        ReadReviewResponse
	observationRequest  RecordObservationRequest
	observationResponse RecordObservationResponse
	decisionRequest     RecordDecisionRequest
	decisionResponse    RecordDecisionResponse
	eventsRequest       ReadEventsRequest
	eventsResponse      ReadEventsResponse
	projectionRequest   ReadProjectionRequest
	projectionResponse  ReadProjectionResponse
}

func newSocketFixtures(t testing.TB) socketFixtures {
	t.Helper()
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	intent := reviewDecision(t, packet, observation, DecisionAccept)
	issueReceipt := reviewReceipt(t, packet)
	observationRequest := reviewNonce(t, 3)
	observationReceipt := reviewReceiptForRequest(t, packet, observationRequest)
	decisionReceipt := reviewReceiptForRequest(t, packet, intent.Request)
	genesis, err := proofledger.NewGenesisHead(issueReceipt.Receipt.Ledger)
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	limit, err := proofledger.NewPageLimit(proofledger.PageEventMaximum)
	if err != nil {
		t.Fatalf("NewPageLimit() error = %v, want nil", err)
	}
	issued := reviewEvent(t, genesis, 0, EventPayload{Kind: EventReviewIssued, Review: &packet})
	return socketFixtures{
		issueRequest: IssueReviewRequest{Request: issueReceipt.Receipt.Request, Packet: packet}, issueResponse: IssueReviewResponse{Receipt: issueReceipt},
		readRequest: ReadReviewRequest{Identity: packet.Identity}, readResponse: ReadReviewResponse{Packet: packet},
		observationRequest: RecordObservationRequest{Request: observationRequest, Observation: observation}, observationResponse: RecordObservationResponse{Receipt: observationReceipt},
		decisionRequest: RecordDecisionRequest{Intent: intent}, decisionResponse: RecordDecisionResponse{Receipt: decisionReceipt},
		eventsRequest: ReadEventsRequest{Page: proofledger.PageRequest{Ledger: issueReceipt.Receipt.Ledger, After: genesis, Limit: limit}}, eventsResponse: ReadEventsResponse{Page: proofledger.Page[EventPayload]{After: genesis, Limit: limit, Events: []proofledger.Envelope[EventPayload]{issued}, Next: issued.Head()}},
		projectionRequest: ReadProjectionRequest{Review: packet.Identity}, projectionResponse: ReadProjectionResponse{Projection: socketProjection(packet, observation, decisionReceipt)},
	}
}

func socketProjection(packet Packet, observation Observation, receipt proofledger.ReceiptDocument) Projection {
	observationIdentity := observation.Identity
	eventIdentity := receipt.Receipt.Event
	verdict := observation.Verdict
	decision := DecisionAccept
	return Projection{
		Review: packet.Identity, Subject: packet.Subject, LatestVerdict: &verdict,
		LatestDecision: &decision, Observation: &observationIdentity,
		DecisionEvent: &eventIdentity, Current: true,
	}
}

func proveSocketJSONClosure[D core.ValidatedJSONMarshaler, P interface {
	*D
	json.Unmarshaler
}](data []byte) ([]byte, error) {
	var decoded D
	if err := P(&decoded).UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return decoded.MarshalJSON()
}

func TestReviewControlSocketDocumentsLayerTriad(t *testing.T) {
	t.Parallel()
	fixtures := newSocketFixtures(t)
	t.Run("positive decision request preserves exact intent", func(t *testing.T) {
		t.Parallel()
		encoded, err := fixtures.decisionRequest.MarshalJSON()
		if err != nil {
			t.Fatalf("RecordDecisionRequest.MarshalJSON() error = %v, want nil", err)
		}
		second, err := proveSocketJSONClosure[RecordDecisionRequest, *RecordDecisionRequest](encoded)
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("RecordDecisionRequest round trip = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
	t.Run("negative unknown member preserves admitted request", func(t *testing.T) {
		t.Parallel()
		encoded, err := fixtures.issueRequest.MarshalJSON()
		if err != nil {
			t.Fatalf("IssueReviewRequest.MarshalJSON() error = %v, want nil", err)
		}
		hostile := append(append([]byte(nil), encoded[:len(encoded)-1]...), []byte(`,"unknown":1}`)...)
		preserved := fixtures.issueRequest
		gotErr := preserved.UnmarshalJSON(hostile)
		after, afterErr := preserved.MarshalJSON()
		if !errors.Is(gotErr, core.ErrJSONContract) || afterErr != nil || !bytes.Equal(after, encoded) {
			t.Fatalf("IssueReviewRequest.UnmarshalJSON(unknown) = (after=%q, decode=%v, encode=%v), want preserved %q and %v", after, gotErr, afterErr, encoded, core.ErrJSONContract)
		}
	})
	t.Run("neutral empty event page preserves genesis and emits no event", func(t *testing.T) {
		t.Parallel()
		empty := fixtures.eventsResponse
		empty.Page.Events = nil
		empty.Page.Next = empty.Page.After
		encoded, err := empty.MarshalJSON()
		if err != nil {
			t.Fatalf("ReadEventsResponse.MarshalJSON(empty) error = %v, want nil", err)
		}
		var got ReadEventsResponse
		if err := got.UnmarshalJSON(encoded); err != nil || len(got.Page.Events) != 0 || got.Page.More || got.Page.Next != empty.Page.Next {
			t.Fatalf("ReadEventsResponse.UnmarshalJSON(empty) = (events=%d, more=%t, next=%+v, error=%v), want (0, false, %+v, nil)", len(got.Page.Events), got.Page.More, got.Page.Next, err, empty.Page.Next)
		}
	})
	t.Run("positive event page preserves the complete nested envelope", func(t *testing.T) {
		t.Parallel()
		encoded, err := fixtures.eventsResponse.MarshalJSON()
		if err != nil {
			t.Fatalf("ReadEventsResponse.MarshalJSON(nonempty) error = %v, want nil", err)
		}
		var got ReadEventsResponse
		if err := got.UnmarshalJSON(encoded); err != nil || len(got.Page.Events) != 1 || got.Page.Events[0].Hash != fixtures.eventsResponse.Page.Events[0].Hash || got.Page.Next != fixtures.eventsResponse.Page.Next {
			t.Fatalf("ReadEventsResponse.UnmarshalJSON(nonempty) = (%+v, %v), want exact nested envelope and next head", got, err)
		}
	})
}

func TestReviewControlEverySocketDocumentSharesCanonicalAgreement(t *testing.T) {
	t.Parallel()
	fixtures := newSocketFixtures(t)
	cases := []struct {
		name      string
		encode    func() ([]byte, error)
		roundTrip func([]byte) ([]byte, error)
	}{
		{name: "issue-review request", encode: fixtures.issueRequest.MarshalJSON, roundTrip: proveSocketJSONClosure[IssueReviewRequest, *IssueReviewRequest]},
		{name: "issue-review response", encode: fixtures.issueResponse.MarshalJSON, roundTrip: proveSocketJSONClosure[IssueReviewResponse, *IssueReviewResponse]},
		{name: "read-review request", encode: fixtures.readRequest.MarshalJSON, roundTrip: proveSocketJSONClosure[ReadReviewRequest, *ReadReviewRequest]},
		{name: "read-review response", encode: fixtures.readResponse.MarshalJSON, roundTrip: proveSocketJSONClosure[ReadReviewResponse, *ReadReviewResponse]},
		{name: "record-observation request", encode: fixtures.observationRequest.MarshalJSON, roundTrip: proveSocketJSONClosure[RecordObservationRequest, *RecordObservationRequest]},
		{name: "record-observation response", encode: fixtures.observationResponse.MarshalJSON, roundTrip: proveSocketJSONClosure[RecordObservationResponse, *RecordObservationResponse]},
		{name: "record-decision request", encode: fixtures.decisionRequest.MarshalJSON, roundTrip: proveSocketJSONClosure[RecordDecisionRequest, *RecordDecisionRequest]},
		{name: "record-decision response", encode: fixtures.decisionResponse.MarshalJSON, roundTrip: proveSocketJSONClosure[RecordDecisionResponse, *RecordDecisionResponse]},
		{name: "read-events request", encode: fixtures.eventsRequest.MarshalJSON, roundTrip: proveSocketJSONClosure[ReadEventsRequest, *ReadEventsRequest]},
		{name: "read-events response", encode: fixtures.eventsResponse.MarshalJSON, roundTrip: proveSocketJSONClosure[ReadEventsResponse, *ReadEventsResponse]},
		{name: "read-projection request", encode: fixtures.projectionRequest.MarshalJSON, roundTrip: proveSocketJSONClosure[ReadProjectionRequest, *ReadProjectionRequest]},
		{name: "read-projection response", encode: fixtures.projectionResponse.MarshalJSON, roundTrip: proveSocketJSONClosure[ReadProjectionResponse, *ReadProjectionResponse]},
	}
	for _, tc := range cases {
		t.Run(tc.name+" canonical closure", func(t *testing.T) {
			t.Parallel()
			encoded, err := tc.encode()
			if err != nil {
				t.Fatalf("%s MarshalJSON() error = %v, want nil", tc.name, err)
			}
			second, err := tc.roundTrip(encoded)
			if err != nil || !bytes.Equal(second, encoded) {
				t.Fatalf("%s round trip = (%q, %v), want (%q, nil)", tc.name, second, err, encoded)
			}
		})
	}
}
