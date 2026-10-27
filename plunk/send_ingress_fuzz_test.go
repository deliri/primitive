package plunk

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"net/mail"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Changed external ingress inventory: SendStatus.UnmarshalJSON is owned by
// FuzzSendStatusStrictWire; SendTimestamp.UnmarshalJSON by the timestamp target;
// DecodeSendResponse by FuzzSendResponseSemanticClosure; generic request wire
// encoding by FuzzEncodeSendRequestSemanticClosure. Constructor-only methods
// over validated Temporal instants have no new untrusted wire grammar.
func FuzzSendTimestampStrictWire(f *testing.F) {
	for _, text := range []string{"2026-09-14T12:00:00.123Z", "1970-01-01T00:00:00Z", "1969-12-31T23:59:59Z"} {
		value := sendTimestamp(f, text)
		wire, err := value.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(wire)
	}
	for _, raw := range []string{`null`, `""`, `"private-invalid-time"`, `"2026-09-14T12:00:00Z" {}`} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		initial, err := temporal.ParseRFC3339("2026-09-14T12:00:00.123Z")
		if err != nil {
			t.Fatal(err)
		}
		value, err := NewSendTimestamp(initial)
		if err != nil {
			t.Fatal(err)
		}
		before := value
		gotErr := value.UnmarshalJSON(raw)
		var spelling *string
		textErr := json.Unmarshal(raw, &spelling)
		var want temporal.Instant
		parseErr := textErr
		if parseErr == nil && spelling != nil {
			want, parseErr = temporal.ParseRFC3339(*spelling)
		}
		if parseErr != nil {
			if !errors.Is(gotErr, core.ErrPlunkContract) || value != before {
				t.Fatal("refused timestamp mutated receiver or lost contract cause")
			}
			return
		}
		if gotErr != nil || value.Instant() != want {
			t.Fatalf("provider timestamp changed instant: %v", gotErr)
		}
		canonical, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var again SendTimestamp
		if err := again.UnmarshalJSON(canonical); err != nil || again != value {
			t.Fatal("timestamp not closed under canonical provider wire")
		}
	})
}

func FuzzEncodeSendRequestSemanticClosure(f *testing.F) {
	f.Add("recipient@example.test", "sender@example.test", "reply@example.test", "", "subject", "<p>body</p>", "signup")
	f.Add("recipient@example.test", "", "", "template-fixture", "", "", "metadata")
	f.Add("bad", "", "", "", "", "", "")
	f.Fuzz(func(t *testing.T, to, from, reply, template, subject, body, purpose string) {
		type metadata struct {
			Purpose string `json:"purpose"`
		}
		request := SendRequest[metadata]{To: to, From: from, Reply: reply, Template: template, Subject: subject, Body: body, Data: &metadata{Purpose: purpose}}
		_, toErr := mail.ParseAddress(to)
		valid := toErr == nil && (template != "" || strings.TrimSpace(subject) != "" && strings.TrimSpace(body) != "")
		for _, address := range [...]string{from, reply} {
			if address != "" {
				if _, err := mail.ParseAddress(address); err != nil {
					valid = false
				}
			}
		}
		native, nativeErr := json.Marshal(request)
		wire, err := EncodeSendRequest(request)
		if !valid || nativeErr != nil {
			if !errors.Is(err, core.ErrPlunkContract) || wire != nil {
				t.Fatal("invalid request emitted wire facts")
			}
			return
		}
		if err != nil || !bytes.Equal(wire, native) {
			t.Fatal("SDK changed provider request bytes")
		}
		var again SendRequest[metadata]
		if err := json.Unmarshal(wire, &again); err != nil || !sameSendRequest(again, request) {
			t.Fatal("generic metadata or send fields changed on wire")
		}
	})
}
