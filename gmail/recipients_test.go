package gmail

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func recipientListFixture(count int) string {
	var addresses []string
	for i := range count {
		addresses = append(addresses, fmt.Sprintf("fixture%d@example.test", i))
	}
	return strings.Join(addresses, ",")
}

func TestGmailRecipientHeaderGrammarAndBounds(t *testing.T) {
	t.Parallel()
	const address = "worker+project@example.test"
	wireMaximum := address + strings.Repeat(" ", RecipientHeaderMaximumBytes-len(address))
	domain := strings.Repeat("d", 63) + "." + strings.Repeat("e", 63) + "." + strings.Repeat("f", 61)
	cases := []struct {
		name       string
		wire       string
		wantEmail  string
		wantName   RecipientName
		wantCount  int
		wantErr    error
		allowEmpty bool
	}{
		{name: "bare exact plus address", wire: address, wantCount: 1, wantEmail: address},
		{name: "ordinary display name", wire: "Example Person <" + address + ">", wantCount: 1, wantEmail: address, wantName: "Example Person"},
		{name: "quoted display comma", wire: `"Name, Example" <` + address + ">", wantCount: 1, wantEmail: address, wantName: "Name, Example"},
		{name: "quoted display escaped quote", wire: `"The \"Example\" Shop" <` + address + ">", wantCount: 1, wantName: `The "Example" Shop`},
		{name: "RFC2047 encoded display", wire: "=?UTF-8?Q?Jos=C3=A9?= <" + address + ">", wantCount: 1, wantName: "José"},
		{name: "adjacent encoded words", wire: "=?UTF-8?Q?Jos=C3=A9?= =?UTF-8?Q?_Fixture?= <" + address + ">", wantCount: 1, wantName: "José Fixture"},
		{name: "malformed encoded display remains literal", wire: "=?UTF-8?B?z?= <" + address + ">", wantCount: 1, wantName: "=?UTF-8?B?z?=", wantEmail: address},
		{name: "UTF8 display", wire: "Zoë Fixture <" + address + ">", wantCount: 1, wantName: "Zoë Fixture"},
		{name: "group yields only its listed members", wire: "Operations: " + address + ", someone@example.test;", wantCount: 2, wantEmail: address},
		{name: "empty group does not invent an address", wire: "Undisclosed recipients:;"},
		{name: "empty group followed by a mailbox", wire: "Undisclosed:;, " + address, wantCount: 1, wantEmail: address},
		{name: "comments with unquoted paths", wire: address + " (Fixture)", wantCount: 1, wantName: "Fixture"},
		{name: "folded lists", wire: address + ",\r\n someone@example.test", wantCount: 2, wantEmail: address},
		{name: "folded tabs", wire: address + ",\r\n\tsomeone@example.test", wantCount: 2, wantEmail: address},
		{name: "empty To field is not an address-list", wantErr: core.ErrGmailResponse},
		{name: "an empty Bcc field is permitted", wire: "\r\n\t", allowEmpty: true},
		{name: "dangling addr-spec", wire: "fixture@", wantErr: core.ErrGmailResponse},
		{name: "surplus junk after valid prefix", wire: address + " [stray]", wantErr: core.ErrGmailResponse},
		{name: "unterminated angle address", wire: "Example <" + address, wantErr: core.ErrGmailResponse},
		{name: "unterminated phrase quote", wire: `"Example <` + address + ">", wantErr: core.ErrGmailResponse},
		{name: "unquoted display comma is not guessed", wire: "Name, Example <" + address + ">", wantErr: core.ErrGmailResponse},
		{name: "group must terminate", wire: "Sales: " + address, wantErr: core.ErrGmailResponse},
		{name: "nested group is refused", wire: "Outer: Inner: " + address + ";;", wantErr: core.ErrGmailResponse},
		{name: "mixed valid and malformed recipients withholds all", wire: address + ", private-provider-text", wantErr: core.ErrGmailResponse},
		{name: "bare LF is not an RFC fold", wire: address + ",\n someone@example.test", wantErr: core.ErrGmailResponse},
		{name: "a CRLF without whitespace cannot start another header", wire: address + "\r\nBcc: other@example.test", wantErr: core.ErrGmailResponse},
		{name: "bare CR refuses even where mail grammar treats it as whitespace", wire: "Example\r <" + address + ">", wantErr: core.ErrGmailResponse},
		{name: "invalid UTF8 inside a trailing comment", wire: "Fixture <" + address + "> (\xff)", wantErr: core.ErrGmailResponse},
		{name: "control inside an ignored trailing comment", wire: "Fixture <" + address + "> (\r)", wantErr: core.ErrGmailResponse},
		{name: "decoded display control is refused", wire: "=?UTF-8?B?YQBi?= <" + address + ">", wantErr: core.ErrGmailResponse},
		{name: "wire one below limit", wire: wireMaximum[:len(wireMaximum)-1], wantCount: 1, wantEmail: address},
		{name: "wire at limit", wire: wireMaximum, wantCount: 1, wantEmail: address},
		{name: "wire one over limit", wire: wireMaximum + " ", wantErr: core.ErrExchangeBodyLimit},
		{name: "extreme wire bound before unfolding", wire: strings.Repeat("\r\n ", RecipientHeaderMaximumBytes), wantErr: core.ErrExchangeBodyLimit},
		{name: "empty Bcc still has a wire bound", wire: strings.Repeat(" ", RecipientHeaderMaximumBytes+1), allowEmpty: true, wantErr: core.ErrExchangeBodyLimit},
		{name: "one below count ceiling", wire: recipientListFixture(RecipientsMaximum - 1), wantCount: RecipientsMaximum - 1},
		{name: "at count ceiling", wire: recipientListFixture(RecipientsMaximum), wantCount: RecipientsMaximum},
		{name: "one beyond count ceiling", wire: recipientListFixture(RecipientsMaximum + 1), wantErr: core.ErrExchangeBodyLimit},
		{name: "compact but very large group count", wire: "Group:" + strings.Repeat("a@b,", 50) + "a@b;", wantErr: core.ErrExchangeBodyLimit},
		{name: "display name one below byte bound", wire: strings.Repeat("x", recipientNameMaximumBytes-1) + " <" + address + ">", wantCount: 1},
		{name: "display name at byte bound", wire: strings.Repeat("é", recipientNameMaximumBytes/2) + " <" + address + ">", wantCount: 1},
		{name: "display name one beyond byte bound", wire: strings.Repeat("x", recipientNameMaximumBytes+1) + " <" + address + ">", wantErr: core.ErrGmailResponse},
		{name: "address one below byte bound", wire: strings.Repeat("x", 63) + "@" + domain, wantCount: 1},
		{name: "address at byte bound", wire: strings.Repeat("x", 64) + "@" + domain, wantCount: 1},
		{name: "address one above byte bound", wire: strings.Repeat("x", 65) + "@" + domain, wantErr: core.ErrGmailResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseRecipientHeader(headerValueWire(tc.wire), RecipientsMaximum, tc.allowEmpty)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != nil || strings.Contains(fmt.Sprint(err), "private-provider-text") {
					t.Fatalf("recipient result = %d rows, %v, want no rows and sanitized %v", len(got), err, tc.wantErr)
				}
				return
			}
			if err != nil || len(got) != tc.wantCount {
				t.Fatalf("recipient result = %d rows, %v, want %d rows and no error", len(got), err, tc.wantCount)
			}
			if len(got) != 0 && (tc.wantEmail != "" && got[0].Address.String() != tc.wantEmail ||
				tc.wantName != "" && got[0].Name != tc.wantName) {
				t.Fatalf("first recipient = %+v, want address %q and display name %q", got[0], tc.wantEmail, tc.wantName)
			}
		})
	}
}

func TestGmailRecipientsThroughLoopback(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name        string
		from        headerValueWire
		labels      []labelWire
		destination []headerWire
		wantCounts  [3]int
		wantErr     error
	}{
		{name: "own sent message projects each supplied destination", from: fixtureMailbox, labels: []labelWire{labelSent}, destination: []headerWire{
			{Name: "tO", Value: `"First, Example" <worker+fixture@example.test>`}, {Name: "CC", Value: "Others: cc@example.test;"}, {Name: "bCC", Value: "private@example.test"},
		}, wantCounts: [3]int{1, 1, 1}},
		{name: "own incoming message withholds even a malformed Bcc", from: fixtureMailbox, destination: []headerWire{{Name: "Bcc", Value: "private-provider-text"}}},
		{name: "foreign sender with SENT label has no Bcc disclosure", from: "outside@example.test", labels: []labelWire{labelSent}, destination: []headerWire{{Name: "Bcc", Value: "private@example.test"}}},
		{name: "own sent message with explicitly empty Bcc", from: fixtureMailbox, labels: []labelWire{labelSent}, destination: []headerWire{{Name: "Bcc", Value: ""}}},
		{name: "same To twice is ambiguous", from: fixtureMailbox, destination: []headerWire{{Name: "To", Value: "one@example.test"}, {Name: "tO", Value: "two@example.test"}}, wantErr: core.ErrGmailResponse},
		{name: "same Cc twice is ambiguous", from: fixtureMailbox, destination: []headerWire{{Name: "Cc", Value: "one@example.test"}, {Name: "cC", Value: "two@example.test"}}, wantErr: core.ErrGmailResponse},
		{name: "own sent message rejects repeated Bcc", from: fixtureMailbox, labels: []labelWire{labelSent}, destination: []headerWire{
			{Name: "Bcc", Value: "one@example.test"}, {Name: "bCC", Value: "two@example.test"},
		}, wantErr: core.ErrGmailResponse},
		{name: "combined To and Cc can reach ceiling", from: fixtureMailbox, destination: []headerWire{
			{Name: "To", Value: headerValueWire(recipientListFixture(RecipientsMaximum - 1))}, {Name: "Cc", Value: "last@example.test"},
		}, wantCounts: [3]int{RecipientsMaximum - 1, 1, 0}},
		{name: "combined destinations cannot exceed ceiling", from: fixtureMailbox, labels: []labelWire{labelSent}, destination: []headerWire{
			{Name: "To", Value: headerValueWire(recipientListFixture(RecipientsMaximum - 1))}, {Name: "Cc", Value: "last@example.test"}, {Name: "Bcc", Value: "extra@example.test"},
		}, wantErr: core.ErrExchangeBodyLimit},
		{name: "aggregate budget beats individual surplus validation", from: fixtureMailbox, labels: []labelWire{labelSent}, destination: []headerWire{
			{Name: "To", Value: headerValueWire(recipientListFixture(RecipientsMaximum))},
			{Name: "Bcc", Value: headerValueWire(strings.Repeat("x", recipientNameMaximumBytes+1) + " <extra@example.test>")},
		}, wantErr: core.ErrExchangeBodyLimit},
		{name: "reply-to never supplies recipients", from: fixtureMailbox, destination: []headerWire{{Name: "Reply-To", Value: "redirect@example.test"}}},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := fixtureThreadWire()
			wire.Messages = wire.Messages[:1]
			wire.Messages[0].Labels = tc.labels
			wire.Messages[0].Payload.Headers = append([]headerWire{{Name: "From", Value: tc.from}}, tc.destination...)
			body := encodeFixture(t, wire)
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				if r.URL.Path != "/gmail/v1/users/me/threads/thread_a" || query.Get("format") != "metadata" ||
					!slices.Equal(query["metadataHeaders"], []string{"From", "Subject", "To", "Cc", "Bcc"}) {
					t.Error("recipient metadata request differs")
				}
				nativeFixtureWrite(t, w, http.StatusOK, body)
			})
			got, err := server.ReadThread().Execute(t.Context(), ThreadReadRequest{Access: fixtureAccess(t), Policy: fixturePolicy(t), Thread: fixtureThread})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got.ID != "" || got.Messages != nil {
					t.Fatalf("thread error = %v, admitted rows = %d, want %v and no rows", err, len(got.Messages), tc.wantErr)
				}
				return
			}
			if err != nil || len(got.Messages) != 1 {
				t.Fatalf("thread error = %v, rows = %d, want one row", err, len(got.Messages))
			}
			h := got.Messages[0]
			counts := [3]int{len(h.Recipients.To), len(h.Recipients.Cc), len(h.Recipients.Bcc)}
			if counts != tc.wantCounts || h.Sent != slices.Contains(tc.labels, labelSent) {
				t.Fatalf("recipient counts = %v, sent = %t, want %v and the provider label", counts, h.Sent, tc.wantCounts)
			}
			if tc.wantCounts == [3]int{1, 1, 1} &&
				(h.Recipients.To[0].Name != "First, Example" || h.Recipients.To[0].Address.String() != "worker+fixture@example.test" ||
					h.Recipients.Cc[0].Address.String() != "cc@example.test" || h.Recipients.Bcc[0].Address.String() != "private@example.test") {
				t.Fatal("typed recipient identities changed")
			}
		})
	}
}

func FuzzGmailRecipientHeaderDecoder(f *testing.F) {
	email, err := core.ParseOAuthEmail("fixture+word@example.test")
	if err != nil {
		f.Fatal(err)
	}
	seed := Recipient{Address: email, Name: "Fixture, ✓"}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add((&mail.Address{Address: seed.Address.String(), Name: string(seed.Name)}).String(), uint8(25), false)
	f.Add("Dispatch: fixture@example.test;", uint8(1), false)
	f.Add("private-provider-text", uint8(2), false)
	f.Add("\r\n\t", uint8(0), true)
	f.Add(strings.Repeat(" ", RecipientHeaderMaximumBytes+1), uint8(25), true)
	f.Fuzz(func(t *testing.T, raw string, cap uint8, allowEmpty bool) {
		remaining := int(cap % (RecipientsMaximum + 1))
		got, err := parseRecipientHeader(headerValueWire(raw), remaining, allowEmpty)
		if err != nil {
			if !errors.Is(err, core.ErrGmailResponse) || got != nil {
				t.Fatalf("refusal = %v with %d rows, want typed refusal and no rows", err, len(got))
			}
			return
		}
		if len(raw) > RecipientHeaderMaximumBytes || len(got) > remaining {
			t.Fatal("accepted header exceeded a bound")
		}
		for _, row := range got {
			if err := row.Validate(); err != nil {
				t.Fatal(err)
			}
			// Format and read the canonical mailbox alone. RFC 2047 display
			// word encoding may normalize spacing inside a phrase.
			canonical := (&mail.Address{Address: row.Address.String()}).String()
			next, err := parseRecipientHeader(headerValueWire(canonical), 1, false)
			if err != nil || len(next) != 1 || next[0].Address != row.Address {
				t.Fatalf("canonical address round trip = %d rows, %v, want the original address", len(next), err)
			}
		}
	})
}
