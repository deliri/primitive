package reviewcontrol

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	socketDoorIssueRequest uint8 = iota + 1
	socketDoorIssueResponse
	socketDoorReadRequest
	socketDoorReadResponse
	socketDoorObservationRequest
	socketDoorObservationResponse
	socketDoorDecisionRequest
	socketDoorDecisionResponse
	socketDoorEventsRequest
	socketDoorEventsResponse
	socketDoorProjectionRequest
	socketDoorProjectionResponse
)

func addSocketSeed[D core.ValidatedJSONMarshaler](f *testing.F, door uint8, document D) {
	f.Helper()
	encoded, err := document.MarshalJSON()
	if err != nil {
		f.Fatalf("socket document %d MarshalJSON(seed) error = %v, want nil", door, err)
	}
	f.Add(door, encoded)
}

func proveSocketFuzzClosure[D core.ValidatedJSONMarshaler, P interface {
	*D
	json.Unmarshaler
}](t *testing.T, data []byte, admitted D) {
	t.Helper()
	want, err := admitted.MarshalJSON()
	if err != nil {
		t.Fatalf("socket admitted fixture MarshalJSON() error = %v, want nil", err)
	}
	got := admitted
	gotErr := P(&got).UnmarshalJSON(data)
	if gotErr != nil {
		after, afterErr := got.MarshalJSON()
		if !errors.Is(gotErr, core.ErrJSONContract) || afterErr != nil || !bytes.Equal(after, want) {
			t.Fatalf("socket UnmarshalJSON(rejected) = (after=%q, decode=%v, encode=%v), want preserved %q and %v", after, gotErr, afterErr, want, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("socket MarshalJSON(accepted) error = %v, want nil", err)
	}
	var roundTrip D
	if err := P(&roundTrip).UnmarshalJSON(encoded); err != nil {
		t.Fatalf("socket UnmarshalJSON(round trip) error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("socket canonical closure = (%q, %v), want (%q, nil)", second, err, encoded)
	}
}

func FuzzEveryReviewControlSocketDocumentSemanticClosure(f *testing.F) {
	fixtures := newSocketFixtures(f)
	addSocketSeed(f, socketDoorIssueRequest, fixtures.issueRequest)
	addSocketSeed(f, socketDoorIssueResponse, fixtures.issueResponse)
	addSocketSeed(f, socketDoorReadRequest, fixtures.readRequest)
	addSocketSeed(f, socketDoorReadResponse, fixtures.readResponse)
	addSocketSeed(f, socketDoorObservationRequest, fixtures.observationRequest)
	addSocketSeed(f, socketDoorObservationResponse, fixtures.observationResponse)
	addSocketSeed(f, socketDoorDecisionRequest, fixtures.decisionRequest)
	addSocketSeed(f, socketDoorDecisionResponse, fixtures.decisionResponse)
	addSocketSeed(f, socketDoorEventsRequest, fixtures.eventsRequest)
	addSocketSeed(f, socketDoorEventsResponse, fixtures.eventsResponse)
	addSocketSeed(f, socketDoorProjectionRequest, fixtures.projectionRequest)
	addSocketSeed(f, socketDoorProjectionResponse, fixtures.projectionResponse)
	f.Add(socketDoorIssueRequest, []byte{})
	f.Add(socketDoorEventsResponse, []byte(`{}`))
	f.Fuzz(func(t *testing.T, door uint8, data []byte) {
		switch door {
		case socketDoorIssueRequest:
			proveSocketFuzzClosure[IssueReviewRequest, *IssueReviewRequest](t, data, fixtures.issueRequest)
		case socketDoorIssueResponse:
			proveSocketFuzzClosure[IssueReviewResponse, *IssueReviewResponse](t, data, fixtures.issueResponse)
		case socketDoorReadRequest:
			proveSocketFuzzClosure[ReadReviewRequest, *ReadReviewRequest](t, data, fixtures.readRequest)
		case socketDoorReadResponse:
			proveSocketFuzzClosure[ReadReviewResponse, *ReadReviewResponse](t, data, fixtures.readResponse)
		case socketDoorObservationRequest:
			proveSocketFuzzClosure[RecordObservationRequest, *RecordObservationRequest](t, data, fixtures.observationRequest)
		case socketDoorObservationResponse:
			proveSocketFuzzClosure[RecordObservationResponse, *RecordObservationResponse](t, data, fixtures.observationResponse)
		case socketDoorDecisionRequest:
			proveSocketFuzzClosure[RecordDecisionRequest, *RecordDecisionRequest](t, data, fixtures.decisionRequest)
		case socketDoorDecisionResponse:
			proveSocketFuzzClosure[RecordDecisionResponse, *RecordDecisionResponse](t, data, fixtures.decisionResponse)
		case socketDoorEventsRequest:
			proveSocketFuzzClosure[ReadEventsRequest, *ReadEventsRequest](t, data, fixtures.eventsRequest)
		case socketDoorEventsResponse:
			proveSocketFuzzClosure[ReadEventsResponse, *ReadEventsResponse](t, data, fixtures.eventsResponse)
		case socketDoorProjectionRequest:
			proveSocketFuzzClosure[ReadProjectionRequest, *ReadProjectionRequest](t, data, fixtures.projectionRequest)
		case socketDoorProjectionResponse:
			proveSocketFuzzClosure[ReadProjectionResponse, *ReadProjectionResponse](t, data, fixtures.projectionResponse)
		}
	})
}
