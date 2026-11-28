package gmail

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestInboxCandidateMetadataThroughServer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		from       headerValueWire
		headers    []headerWire
		labels     []labelWire
		foreign    bool
		want       bool
		senderName RecipientName
		wantErr    error
	}{
		{name: "incoming named sender", want: true, senderName: "Élodie Example"},
		{name: "unnamed sender", from: "buyer@example.invalid", want: true},
		{name: "long name has no display name", from: headerValueWire(strings.Repeat("A", 257) + " <buyer@example.invalid>"), want: true},
		{name: "automated invitation", headers: []headerWire{{Name: "Auto-Submitted", Value: "auto-generated"}}, want: true, senderName: "Élodie Example"},
		{name: "unknown precedence", headers: []headerWire{{Name: "Precedence", Value: "urgent"}}, want: true, senderName: "Élodie Example"},
		{name: "empty list ID", headers: []headerWire{{Name: "List-ID", Value: " "}}, want: true, senderName: "Élodie Example"},
		{name: "list ID", headers: []headerWire{{Name: "lIsT-iD", Value: "Digest <newsletter.example.invalid>"}}},
		{name: "list unsubscribe", headers: []headerWire{{Name: "List-Unsubscribe", Value: "<https://example.invalid/unsubscribe>"}}},
		{name: "list posting", headers: []headerWire{{Name: "List-Post", Value: "NO"}}},
		{name: "bulk precedence", headers: []headerWire{{Name: "Precedence", Value: " BULK "}}},
		{name: "list precedence", headers: []headerWire{{Name: "Precedence", Value: "list"}}},
		{name: "junk precedence", headers: []headerWire{{Name: "Precedence", Value: "junk"}}},
		{name: "repeated precedence cannot hide bulk", headers: []headerWire{{Name: "Precedence", Value: "urgent"}, {Name: "Precedence", Value: "list"}}},
		{name: "owner inbox copy", from: "Owner <OWNER@example.com>"},
		{name: "invalid sender", from: "no valid address"},
		{name: "archived", labels: []labelWire{labelUnread}},
		{name: "sent", labels: []labelWire{labelInbox, labelSent}},
		{name: "spam", labels: []labelWire{labelInbox, labelSpam}},
		{name: "trash", labels: []labelWire{labelInbox, labelTrash}},
		{name: "draft", labels: []labelWire{labelInbox, labelDraft}, wantErr: core.ErrGmailBinding},
		{name: "foreign coordinate", foreign: true, wantErr: core.ErrGmailBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refresh := fixtureRefresh(t)
			from := tc.from
			if from == "" {
				from = "=?utf-8?q?=C3=89lodie_Example?= <buyer@example.invalid>"
			}
			labels := tc.labels
			if labels == nil {
				labels = []labelWire{labelInbox, labelUnread}
			}
			wire := messageHeaderWire{
				ID: fixtureMessage, Thread: fixtureThread, Date: "1795708800000", Labels: labels,
				Payload: metadataWire{Headers: []headerWire{
					{Name: "From", Value: from},
					{Name: "Subject", Value: "Synthetic invitation"},
					{Name: "Message-ID", Value: "<invitation@example.invalid>"},
				}},
			}
			if tc.foreign {
				wire.Thread = "foreign_thread"
			}
			wire.Payload.Headers = append(wire.Payload.Headers, tc.headers...)
			raw, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				if workspaceOAuthFixture(t, r, w, grantedTokenFixture()) {
					return
				}
				q := r.URL.Query()
				if r.Method != http.MethodGet || r.URL.Path != "/gmail/v1/users/me/messages/"+string(fixtureMessage) ||
					q.Get("format") != "metadata" || q.Get("fields") != "id,threadId,internalDate,labelIds,payload(headers(name,value))" {
					t.Error("candidate lookup requested something beyond exact header metadata")
				}
				for _, header := range []string{"From", "Subject", "List-ID", "List-Unsubscribe", "List-Post", "Precedence"} {
					if !slices.Contains(q["metadataHeaders"], header) {
						t.Errorf("missing requested header %q", header)
					}
				}
				nativeFixtureWrite(t, w, http.StatusOK, raw)
			})
			access := nativeFixtureAccess(t, server, refresh)
			got, err := server.ReadMessageMetadata().Execute(t.Context(), MessageReadRequest{
				Access: access, Policy: refresh.Policy, Thread: fixtureThread, Message: fixtureMessage,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("metadata error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.Thread != "" || got.Reply != nil || got.InboxCandidate != nil {
					t.Fatal("refusal returned partial metadata")
				}
				return
			}
			if got.Thread != fixtureThread || got.Header.ID != fixtureMessage || got.Reply == nil ||
				got.Header.Subject != "Synthetic invitation" || got.Validate() != nil {
				t.Fatal("candidate lookup changed exact reply evidence")
			}
			if (got.InboxCandidate != nil) != tc.want || (tc.want && got.InboxCandidate.SenderName != tc.senderName) {
				t.Fatalf("candidate projection = %+v, want eligible %t with name %q", got.InboxCandidate, tc.want, tc.senderName)
			}
		})
	}
}
