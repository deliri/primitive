package gmail

import (
	"encoding/base64"
	"errors"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzThreadPageIngress(f *testing.F) {
	f.Add(encodeFixture(f, threadPageWire{Threads: []threadReferenceWire{{ID: fixtureThread}}, Next: "cursor"}), uint8(3))
	f.Add(encodeFixture(f, threadPageWire{}), uint8(1))
	f.Add([]byte(`{"threads":[{"id":"probe/path"}]}`), uint8(1))
	f.Add([]byte(`{"threads":null,"threads":[]}`), uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, limit uint8) {
		wire, err := decodeProvider[threadPageWire](data)
		if err != nil {
			if !errors.Is(err, core.ErrGmailResponse) || len(wire.Threads) != 0 || wire.Next != "" {
				t.Fatalf("decoder refusal = (%+v,%v)", wire, err)
			}
			return
		}
		if limit == 0 || limit > ThreadPageMaximum {
			limit = ThreadPageMaximum
		}
		input := ThreadListRequest{Limit: limit, After: "same"}
		page, err := wire.project(input)
		if err != nil {
			if !errors.Is(err, core.ErrGmailResponse) || page.Threads != nil || page.Next != "" {
				t.Fatalf("page refusal = (%+v,%v)", page, err)
			}
			return
		}
		if page.Validate() != nil || len(page.Threads) > int(limit) || page.Next == input.After ||
			len(page.Threads) != len(wire.Threads) || page.Next != wire.Next {
			t.Fatalf("page lost provider facts or escaped bounds = %+v", page)
		}
		seen := make([]ThreadID, 0, len(page.Threads))
		for i, id := range page.Threads {
			if id != wire.Threads[i].ID || slices.Contains(seen, id) {
				t.Fatalf("page lost order or accepted duplicate at %d", i)
			}
			seen = append(seen, id)
		}
		reencoded := encodeFixture(t, wire)
		decoded, err := decodeProvider[threadPageWire](reencoded)
		if err != nil || !slices.Equal(decoded.Threads, wire.Threads) || decoded.Next != wire.Next {
			t.Fatalf("canonical round trip = (%+v,%v), want %+v", decoded, err, wire)
		}
	})
}

func FuzzThreadMetadataIngress(f *testing.F) {
	f.Add(encodeFixture(f, fixtureThreadWire()))
	f.Add(encodeFixture(f, threadWire{ID: fixtureThread}))
	f.Add([]byte("null"))
	f.Add([]byte(`{"id":"thread_a","messages":[{"id":"foreign","threadId":"other"}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		wire, err := decodeProvider[threadWire](data)
		if err != nil {
			if !errors.Is(err, core.ErrGmailResponse) || wire.ID != "" || wire.Messages != nil {
				t.Fatalf("metadata decode refusal = (%+v,%v)", wire, err)
			}
			return
		}
		thread, err := wire.project(fixtureThread)
		if err != nil {
			if !errors.Is(err, core.ErrGmailContract) || thread.ID != "" || thread.Messages != nil {
				t.Fatalf("metadata refusal = (%+v,%v)", thread, err)
			}
			return
		}
		if thread.Validate() != nil || thread.ID != fixtureThread || thread.ID != wire.ID {
			t.Fatalf("metadata escapes thread binding = %+v", thread)
		}
		want := make([]MessageID, 0, len(wire.Messages))
		for _, message := range wire.Messages {
			if message.Thread != fixtureThread {
				t.Fatalf("substituted member was admitted")
			}
			if !slices.Contains(message.Labels, labelDraft) {
				want = append(want, message.ID)
			}
		}
		got := make([]MessageID, 0, len(thread.Messages))
		for _, message := range thread.Messages {
			got = append(got, message.ID)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("visible metadata IDs = %q, want %q", got, want)
		}
	})
}

func FuzzMessageBodyIngress(f *testing.F) {
	f.Add(encodeFixture(f, fixtureMessageWire()))
	f.Add(encodeFixture(f, messageWire{ID: fixtureMessage, Thread: fixtureThread, Payload: mimePartWire{Type: "text/html"}}))
	f.Add([]byte(`{"id":"message_b","threadId":"thread_a","payload":{"mimeType":"text/plain","body":{"data":"***"}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		wire, err := decodeProvider[messageWire](data)
		if err != nil {
			if !errors.Is(err, core.ErrGmailResponse) || wire.ID != "" || wire.Thread != "" {
				t.Fatalf("message decode refusal = (%+v,%v)", wire, err)
			}
			return
		}
		got, err := wire.project(MessageReadRequest{Thread: fixtureThread, Message: fixtureMessage})
		if err != nil {
			if !errors.Is(err, core.ErrGmailContract) || got != (Message{}) {
				t.Fatalf("message refusal = (%+v,%v)", got, err)
			}
			return
		}
		if got.Validate() != nil || got.ID != fixtureMessage || got.Thread != fixtureThread || slices.Contains(wire.Labels, labelDraft) {
			t.Fatalf("message escaped ownership = %+v", got)
		}
		if wire.Payload.Type == "text/plain" && wire.Payload.Filename == "" && wire.Payload.Body.Data != "" {
			raw := string(wire.Payload.Body.Data)
			want, err := base64.RawURLEncoding.DecodeString(raw)
			if err != nil {
				want, err = base64.URLEncoding.DecodeString(raw)
			}
			if err != nil || got.Kind != BodyPlainText || got.Text != PlainText(want) {
				t.Fatalf("plain body = %+v, want exact admitted bytes", got)
			}
		}
		if wire.Payload.Type == "text/html" && (got.Kind != BodyUnavailable || got.Text != "") {
			t.Fatalf("HTML escaped the plain text boundary = %+v", got)
		}
	})
}

func FuzzRefreshResponseIngress(f *testing.F) {
	f.Add(encodeFixture(f, grantedTokenFixture()))
	f.Add([]byte(`{"access_token":null,"expires_in":3600,"token_type":"Bearer"}`))
	f.Add([]byte(`{"access_token":"x","access_token":"y","expires_in":3600}`))
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		mailbox, err := core.ParseOAuthEmail(fixtureMailbox)
		if err != nil {
			t.Fatal(err)
		}
		got, err := admitAccess(data, mailbox)
		if err != nil {
			if !errors.Is(err, core.ErrGmailContract) || got != (Access{}) {
				t.Fatalf("access refusal = (%v,%v)", got, err)
			}
			return
		}
		defer func() {
			if err := got.Close(); err != nil {
				t.Error(err)
			}
		}()
		wire, err := decodeProvider[tokenFixture](data)
		if err != nil || wire.Scope != ReadOnlyScope {
			t.Fatalf("admitted grant without exact read-only scope")
		}
		token, err := got.token.CopyBytes()
		defer clear(token)
		if err != nil || string(token) != wire.Token || got.Validate() != nil || got.Mailbox() != mailbox {
			t.Fatalf("admitted access lost the expected credential or mailbox")
		}
		lifetime, err := temporal.DurationFromSeconds(wire.Expires)
		if err != nil || lifetime != got.Lifetime() {
			t.Fatalf("admitted access lost the provider lifetime")
		}
	})
}

func FuzzRequestNominals(f *testing.F) {
	f.Add(string(fixtureThread), string(fixtureMessage), "page+token=", uint8(ThreadFilterUnread))
	f.Add("..", "a/b", "line\nbreak", uint8(255))
	f.Fuzz(func(t *testing.T, thread, message, cursor string, filter uint8) {
		for _, id := range []string{thread, message} {
			threadErr := ThreadID(id).Validate()
			messageErr := MessageID(id).Validate()
			if (threadErr == nil) != (messageErr == nil) {
				t.Fatalf("same Gmail ID domain disagrees")
			}
			if threadErr == nil && (len(id) == 0 || len(id) > idMaximumBytes || !portableSegment(id)) {
				t.Fatalf("ID escaped URL segment")
			}
		}
		if PageToken(cursor).Validate() == nil {
			if len(cursor) > cursorMaximumBytes {
				t.Fatal("cursor escaped bound")
			}
			for _, c := range []byte(cursor) {
				if c <= 32 || c > 126 {
					t.Fatal("cursor contains non-visible unit")
				}
			}
		}
		if ThreadFilter(filter).IsValid() != (filter == 1 || filter == 2) {
			t.Fatal("filter enum widened")
		}
	})
}

func portableSegment(value string) bool {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, c := range []byte(value) {
		if !slices.Contains([]byte(alphabet), c) {
			return false
		}
	}
	return true
}
