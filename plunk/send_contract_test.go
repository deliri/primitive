package plunk

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const sendContactFixture = `"contact":{"id":"contact-fixture","email":"recipient@example.test"}`
const sendTimeFixture = `"timestamp":"2026-09-14T12:00:00.123Z"`

func sendFixture(item string) []byte {
	return []byte(`{"success":true,"data":{` + sendTimeFixture + `,"emails":[{` + sendContactFixture + `,` + item + `}]}}`)
}

func TestSendResponseProviderFactsAndAppBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, item string
		status     SendStatus
	}{
		{"unreported retained provider contract", `"email":"message-fixture"`, SendStatusUnreported},
		{"queued", `"email":"message-fixture","status":"queued"`, SendStatusQueued},
		{"scheduled", `"email":"message-fixture","status":"scheduled","scheduledFor":"2026-09-15T12:00:00Z"`, SendStatusScheduled},
		{"throttled with next instant", `"email":null,"status":"throttled","nextAllowedAt":"2026-09-15T12:00:00Z"`, SendStatusThrottled},
		{"throttled without next instant", `"email":null,"status":"throttled","nextAllowedAt":null`, SendStatusThrottled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeSendResponse(t.Context(), sendFixture(tc.item), core.ExtensibleJSONLimits())
			if err != nil || !got.Success || len(got.Data.Emails) != 1 || got.Data.Emails[0].Status != tc.status || got.Data.Timestamp.Instant().Validate() != nil {
				t.Fatalf("documented provider fact: response=%+v error=%v", got, err)
			}
			wire, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			again, err := DecodeSendResponse(t.Context(), wire, core.ExtensibleJSONLimits())
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatalf("typed wire round trip changed facts: %v", err)
			}
		})
	}
	// The SDK represents facts. Neither zero/multiple recipients, mismatched
	// recipients nor provider-opaque identifiers imply application acceptance.
	for _, wire := range []string{
		`{"success":false,"error":{"code":"refused","message":"provider fixture"}}`,
		`{"success":true,"data":{` + sendTimeFixture + `,"emails":[]}}`,
		`{"success":true,"data":{` + sendTimeFixture + `,"emails":[{"contact":{"id":"opaque","email":"other@example.test"},"email":"opaque"},{"email":"second"}]}}`,
	} {
		if _, err := DecodeSendResponse(t.Context(), []byte(wire), core.ExtensibleJSONLimits()); err != nil {
			t.Fatalf("application policy leaked into SDK: %v", err)
		}
	}
}

func TestSendResponseStrictFailureZeroAndCause(t *testing.T) {
	t.Parallel()
	valid := string(sendFixture(`"email":"message-fixture","status":"queued"`))
	for _, tc := range []struct{ name, wire string }{
		{"string item is no union", strings.Replace(valid, `{`+sendContactFixture+`,"email":"message-fixture","status":"queued"}`, `"private-recipient@example.test"`, 1)},
		{"unknown status", strings.Replace(valid, `"queued"`, `"delivered"`, 1)},
		{"empty status", strings.Replace(valid, `"queued"`, `""`, 1)},
		{"null status", strings.Replace(valid, `"queued"`, `null`, 1)},
		{"status spelling", strings.Replace(valid, `"queued"`, `"Queued"`, 1)},
		{"number status", strings.Replace(valid, `"queued"`, `17`, 1)},
		{"unknown provider member", strings.Replace(valid, `"status":"queued"`, `"private-member-fixture":"private-value-fixture"`, 1)},
		{"case folded member", strings.Replace(valid, `"emails"`, `"Emails"`, 1)},
		{"duplicate", strings.Replace(valid, `"success":true`, `"success":true,"success":true`, 1)},
		{"trailing document", valid + `{}`},
		{"invalid RFC3339", strings.Replace(valid, `2026-09-14T12:00:00.123Z`, `private-invalid-time`, 1)},
		{"missing timestamp", strings.Replace(valid, sendTimeFixture+`,`, ``, 1)},
		{"queued without message", string(sendFixture(`"email":null,"status":"queued"`))},
		{"queued with schedule", string(sendFixture(`"email":"m","status":"queued","scheduledFor":"2026-09-15T12:00:00Z"`))},
		{"scheduled without instant", string(sendFixture(`"email":"m","status":"scheduled"`))},
		{"throttled with accepted message", string(sendFixture(`"email":"m","status":"throttled"`))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeSendResponse(t.Context(), []byte(tc.wire), core.ExtensibleJSONLimits())
			if !errors.Is(err, core.ErrPlunkContract) || !reflect.DeepEqual(got, SendResponse{}) {
				t.Fatalf("refusal retained facts: %+v, %v", got, err)
			}
			for _, secret := range []string{"private-recipient", "private-member", "private-value", "private-invalid-time"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("provider data leaked in diagnostic: %s", err)
				}
			}
		})
	}
	unknown := []byte(strings.Replace(valid, `"status":"queued"`, `"private-member-fixture":"private-value-fixture"`, 1))
	_, err := DecodeSendResponse(t.Context(), unknown, core.ExtensibleJSONLimits())
	semantic, native := errors.AsType[*json.SemanticError](err)
	shape, projected := errors.AsType[*SendResponseShapeError](err)
	if !native || semantic == nil || !errors.Is(err, json.ErrUnknownName) || !projected || shape.Field != SendResponseFieldUnknown || shape.Shape != SendResponseStringShapeNotApplicable {
		t.Fatalf("native cause/closed projection lost: %v", err)
	}
	limits := core.ExtensibleJSONLimits()
	limits.DocumentMaximumBytes, err = core.NewByteCount(uint64(len(valid) - 1))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeSendResponse(t.Context(), []byte(valid), limits); err == nil || !reflect.DeepEqual(got, SendResponse{}) {
		t.Fatal("caller byte policy not enforced")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := DecodeSendResponse(ctx, []byte(valid), core.ExtensibleJSONLimits()); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, SendResponse{}) {
		t.Fatal("cancellation cause or zero lost")
	}
	if _, err := DecodeSendResponse(nil, []byte(valid), core.ExtensibleJSONLimits()); !errors.Is(err, core.ErrPlunkContract) {
		t.Fatal("nil context admitted")
	}
}

func TestSendNativeTLSProviderDoor(t *testing.T) {
	t.Parallel()
	type metadata struct {
		Purpose string `json:"purpose"`
	}
	request := SendRequest[metadata]{Data: &metadata{Purpose: "signup"}, From: "Lift <sender@example.test>", Reply: "reply@example.test", To: "recipient@example.test", Subject: "Verify fixture", Body: "<p>fixture</p>"}
	wire, err := EncodeSendRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		actual, readErr := io.ReadAll(r.Body)
		if readErr != nil || !bytes.Equal(actual, wire) || r.Method != http.MethodPost || r.URL.Path != "/v1/send" || r.Header.Get("Authorization") != "Bearer sk_local_fixture" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("send did not preserve typed authenticated wire: %v", readErr)
		}
		var decoded SendRequest[metadata]
		if err := json.Unmarshal(actual, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
			t.Errorf("request fields lost: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(sendFixture(`"email":"message-fixture","status":"queued"`)); err != nil {
			t.Errorf("provider write: %v", err)
		}
	}))
	defer peer.Close()
	redirect, err := url.Parse(peer.URL)
	if err != nil {
		t.Fatal(err)
	}
	native, err := exchange.NewClient(&http.Client{Transport: resourceRedirect{target: redirect, transport: peer.Client().Transport}})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := ParseCredential([]byte("sk_local_fixture"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(native, credential)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := errors.Join(client.Close(), credential.Close()); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()
	endpoint, err := (Route{Operation: SendEmail}).Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	length, err := core.NewByteLength(uint64(len(wire)))
	if err != nil {
		t.Fatal(err)
	}
	var destination bytes.Buffer
	transport, err := client.RoundTrip(t.Context(), Request{Stream: exchange.StreamRoundTripRequest{Target: endpoint, Source: bytes.NewReader(wire), Destination: &destination, RequestContentLength: &length, Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, RequestContentType: core.HTTPMediaTypeJSON(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}}, plunkStreamPolicy(t))
	if err != nil || transport.Validate() != nil || calls.Load() != 1 {
		t.Fatalf("native send: calls=%d error=%v", calls.Load(), err)
	}
	response, err := DecodeSendResponse(t.Context(), destination.Bytes(), core.ExtensibleJSONLimits())
	if err != nil || len(response.Data.Emails) != 1 || response.Data.Emails[0].Status != SendStatusQueued {
		t.Fatalf("native response ownership: %v", err)
	}
}

func TestSendRequestWireRefusalsAndTemplate(t *testing.T) {
	t.Parallel()
	type metadata struct {
		Purpose string `json:"purpose"`
	}
	for _, request := range []SendRequest[metadata]{
		{To: "recipient@example.test", Template: "template-fixture"},
		{To: "recipient@example.test", Subject: "subject", Body: "body"},
	} {
		if wire, err := EncodeSendRequest(request); err != nil || !bytes.Contains(wire, []byte(`"to":"recipient@example.test"`)) {
			t.Fatalf("documented request refused: %v", err)
		}
	}
	for _, request := range []SendRequest[metadata]{
		{}, {To: "bad", Subject: "s", Body: "b"}, {To: "recipient@example.test", From: "bad", Subject: "s", Body: "b"}, {To: "recipient@example.test", Reply: "bad", Subject: "s", Body: "b"}, {To: "recipient@example.test", Subject: "s"}, {To: "recipient@example.test", Body: "b"},
	} {
		if wire, err := EncodeSendRequest(request); !errors.Is(err, core.ErrPlunkContract) || wire != nil {
			t.Fatal("invalid request emitted provider bytes")
		}
	}
	badMetadata := SendRequest[chan int]{To: "recipient@example.test", Template: "template-fixture", Data: new(make(chan int))}
	if wire, err := EncodeSendRequest(badMetadata); !errors.Is(err, core.ErrPlunkContract) || wire != nil {
		t.Fatal("metadata encoding failure emitted bytes")
	}
}

func FuzzSendResponseSemanticClosure(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add(sendFixture(`"email":"message-fixture","status":"queued"`))
	f.Add(sendFixture(`"email":null,"status":"throttled","nextAllowedAt":null`))
	f.Add([]byte(`{"success":true,"data":{"emails":["private@example.test"]}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		got, err := DecodeSendResponse(t.Context(), raw, core.ExtensibleJSONLimits())
		if err != nil {
			if !reflect.DeepEqual(got, SendResponse{}) {
				t.Fatal("refusal retained facts")
			}
			return
		}
		if got.Validate() != nil {
			t.Fatal("decoder admitted invalid provider fact")
		}
		wire, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		again, err := DecodeSendResponse(t.Context(), wire, core.ExtensibleJSONLimits())
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatalf("admitted fact not closed under provider wire: %v", err)
		}
	})
}

func FuzzSendStatusStrictWire(f *testing.F) {
	for _, value := range []string{`"queued"`, `"scheduled"`, `"throttled"`, `null`, `""`, `"delivered"`} {
		f.Add([]byte(value))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		value := SendStatusScheduled
		err := value.UnmarshalJSON(raw)
		if err != nil {
			if value != SendStatusScheduled {
				t.Fatal("refused status mutated destination")
			}
			return
		}
		if value == SendStatusUnreported || value.Validate() != nil {
			t.Fatal("present status escaped closed domain")
		}
		wire, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var again SendStatus
		if err := json.Unmarshal(wire, &again); err != nil || again != value {
			t.Fatal("status wire not closed")
		}
	})
}
