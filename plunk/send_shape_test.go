package plunk

import (
	"encoding/json/jsontext"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"math"
	"slices"
	"testing"
)

func sendDuplicateLabel(labels []string) string {
	for index := 1; index < len(labels); index++ {
		if labels[index] == labels[index-1] {
			return labels[index]
		}
	}
	return ""
}

func TestSendResponseFieldHostileTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		pointer jsontext.Pointer
		want    SendResponseField
	}{
		{name: "data object is the shallowest known path", pointer: "/data", want: SendResponseFieldData},
		{name: "emails collection", pointer: "/data/emails", want: SendResponseFieldEmails},
		{name: "timestamp leaf", pointer: "/data/timestamp", want: SendResponseFieldTimestamp},
		{name: "first email item", pointer: "/data/emails/0", want: SendResponseFieldEmailItem},
		{name: "index beyond uint64 still collapses to item", pointer: "/data/emails/18446744073709551616", want: SendResponseFieldEmailItem},
		{name: "message identity leaf", pointer: "/data/emails/0/email", want: SendResponseFieldEmailItemMessage},
		{name: "send status leaf", pointer: "/data/emails/0/status", want: SendResponseFieldEmailItemStatus},
		{name: "scheduled instant leaf", pointer: "/data/emails/0/scheduledFor", want: SendResponseFieldEmailItemScheduledFor},
		{name: "throttle instant leaf", pointer: "/data/emails/0/nextAllowedAt", want: SendResponseFieldEmailItemNextAllowedAt},
		{name: "case variant status is not the status leaf", pointer: "/data/emails/0/Status", want: SendResponseFieldUnknown},
		{name: "status child below leaf", pointer: "/data/emails/0/status/0", want: SendResponseFieldUnknown},
		{name: "second item contact object", pointer: "/data/emails/1/contact", want: SendResponseFieldEmailItemContact},
		{name: "contact identity leaf at maximum depth", pointer: "/data/emails/0/contact/id", want: SendResponseFieldEmailItemContactID},
		{name: "contact address leaf at maximum depth", pointer: "/data/emails/0/contact/email", want: SendResponseFieldEmailItemContactEmail},
		{name: "leading zero digits are normalized to item", pointer: "/data/emails/007/contact/id", want: SendResponseFieldEmailItemContactID},
		{name: "root pointer is not a receipt field", pointer: "", want: SendResponseFieldUnknown},
		{name: "success sibling outside the receipt paths", pointer: "/success", want: SendResponseFieldUnknown},
		{name: "provider error document is outside the receipt paths", pointer: "/error/code", want: SendResponseFieldUnknown},
		{name: "negative index is not digits", pointer: "/data/emails/-1", want: SendResponseFieldUnknown},
		{name: "letter index is not digits", pointer: "/data/emails/x", want: SendResponseFieldUnknown},
		{name: "empty index token", pointer: "/data/emails/", want: SendResponseFieldUnknown},
		{name: "unicode digit index is not ASCII", pointer: "/data/emails/\u0663", want: SendResponseFieldUnknown},
		{name: "unknown contact sibling leaf", pointer: "/data/emails/0/contact/x", want: SendResponseFieldUnknown},
		{name: "empty contact leaf token", pointer: "/data/emails/0/contact/", want: SendResponseFieldUnknown},
		{name: "one token deeper than contact identity", pointer: "/data/emails/0/contact/id/0", want: SendResponseFieldUnknown},
		{name: "two tokens deeper than the bounded path", pointer: "/data/emails/0/contact/id/0/0", want: SendResponseFieldUnknown},
		{name: "known name at the wrong depth", pointer: "/data/emails/0/timestamp", want: SendResponseFieldUnknown},
		{name: "item child below message leaf", pointer: "/data/emails/0/email/0", want: SendResponseFieldUnknown},
		{name: "timestamp child below leaf", pointer: "/data/timestamp/0", want: SendResponseFieldUnknown},
		{name: "singular emails near name", pointer: "/data/email", want: SendResponseFieldUnknown},
		{name: "case variant of data", pointer: "/Data", want: SendResponseFieldUnknown},
		{name: "escaped slash is one member name", pointer: "/data/emails~1x", want: SendResponseFieldUnknown},
		{name: "missing leading slash is invalid", pointer: "data", want: SendResponseFieldUnknown},
		{name: "invalid escape is invalid", pointer: "/data~2", want: SendResponseFieldUnknown},
		{name: "invalid UTF-8 is invalid", pointer: "/data\xff", want: SendResponseFieldUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifySendResponseField(tc.pointer); got != tc.want {
				t.Fatalf("ClassifySendResponseField(%q) = %v, want %v", tc.pointer, got, tc.want)
			}
		})
	}
}

// The native decoder, not a copied switch, names each JSON kind here.
func TestSendResponseJSONKindExhaustive(t *testing.T) {
	t.Parallel()
	native := []struct {
		value jsontext.Value
		want  SendResponseJSONKind
	}{
		{value: jsontext.Value("null"), want: SendResponseJSONKindNull},
		{value: jsontext.Value("false"), want: SendResponseJSONKindFalse},
		{value: jsontext.Value("true"), want: SendResponseJSONKindTrue},
		{value: jsontext.Value(`"s"`), want: SendResponseJSONKindString},
		{value: jsontext.Value("-0.5e3"), want: SendResponseJSONKindNumber},
		{value: jsontext.Value("{}"), want: SendResponseJSONKindObject},
		{value: jsontext.Value("[]"), want: SendResponseJSONKindArray},
	}
	known := make([]jsontext.Kind, 0, len(native))
	for _, tc := range native {
		kind := tc.value.Kind()
		known = append(known, kind)
		if got := ClassifySendResponseJSONKind(kind); got != tc.want {
			t.Fatalf("ClassifySendResponseJSONKind(native %s) = %v, want %v", tc.value, got, tc.want)
		}
	}
	for raw := 0; raw <= math.MaxUint8; raw++ {
		kind := jsontext.Kind(raw)
		if slices.Contains(known, kind) {
			continue
		}
		if got := ClassifySendResponseJSONKind(kind); got != SendResponseJSONKindUnknown {
			t.Fatalf("ClassifySendResponseJSONKind(byte %d) = %v, want unknown", raw, got)
		}
	}
}

func TestSendResponseShapeEnumDomains(t *testing.T) {
	t.Parallel()
	for _, domain := range []struct {
		name      string
		label     func(uint8) (string, error)
		wantCount int
	}{
		{name: "field", wantCount: int(SendResponseFieldEmailItemNextAllowedAt) + 1, label: func(v uint8) (string, error) {
			return SendResponseField(v).String(), SendResponseField(v).Validate()
		}},
		{name: "kind", wantCount: int(SendResponseJSONKindArray) + 1, label: func(v uint8) (string, error) {
			return SendResponseJSONKind(v).String(), SendResponseJSONKind(v).Validate()
		}},
		{name: "shape", wantCount: int(SendResponseStringShapeOpaque) + 1, label: func(v uint8) (string, error) {
			return SendResponseStringShape(v).String(), SendResponseStringShape(v).Validate()
		}},
		{name: "send status", wantCount: int(SendStatusThrottled) + 1, label: func(v uint8) (string, error) {
			return SendStatus(v).String(), SendStatus(v).Validate()
		}},
	} {
		t.Run(domain.name, func(t *testing.T) {
			t.Parallel()
			var labels []string
			for raw := 0; raw <= math.MaxUint8; raw++ {
				label, err := domain.label(uint8(raw))
				valid := raw < domain.wantCount
				if (err == nil) != valid || err != nil && !errors.Is(err, core.ErrPlunkContract) || (label != "invalid") != valid {
					t.Fatalf("%s value %d = (%q, %v), want valid=%t", domain.name, raw, label, err, valid)
				}
				if valid {
					labels = append(labels, label)
				}
			}
			slices.Sort(labels)
			if duplicate := sendDuplicateLabel(labels); duplicate != "" {
				t.Fatalf("%s labels duplicate %q", domain.name, duplicate)
			}
		})
	}
}
