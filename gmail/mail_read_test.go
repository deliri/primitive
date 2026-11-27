package gmail

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestGmailSentEvidenceThroughServer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		editPage func(*messageListWire)
		editHead func(*messageHeaderWire)
		want     error
		exact    bool
		empty    bool
	}{
		{name: "exact acceptance read", exact: true},
		{name: "one exact RFC ID match"},
		{name: "empty visible result", empty: true, editPage: func(p *messageListWire) { p.Messages = nil }},
		{name: "multiple matches", editPage: func(p *messageListWire) {
			p.Messages = append(p.Messages, messageReferenceWire{ID: "another_id", Thread: fixtureThread})
		}, want: core.ErrGmailSendUnconfirmed},
		{name: "more results", editPage: func(p *messageListWire) { p.Next = "other" }, want: core.ErrGmailSendUnconfirmed},
		{name: "wrong list thread", editPage: func(p *messageListWire) { p.Messages[0].Thread = "foreign_thread" }, want: core.ErrGmailBinding},
		{name: "wrong metadata thread", editHead: func(w *messageHeaderWire) { w.Thread = "foreign_thread" }, want: core.ErrGmailBinding},
		{name: "wrong metadata ID", editHead: func(w *messageHeaderWire) { w.ID = "other_id" }, want: core.ErrGmailBinding},
		{name: "not Sent", editHead: func(w *messageHeaderWire) { w.Labels = []labelWire{"INBOX"} }, want: core.ErrGmailBinding},
		{name: "draft", editHead: func(w *messageHeaderWire) { w.Labels = append(w.Labels, labelDraft) }, want: core.ErrGmailBinding},
		{name: "unexpected From", editHead: func(w *messageHeaderWire) { w.Payload.Headers[0].Value = "other@example.invalid" }, want: core.ErrGmailBinding},
		{name: "unexpected recipient", editHead: func(w *messageHeaderWire) { w.Payload.Headers[1].Value = "other@example.invalid" }, want: core.ErrGmailBinding},
		{name: "unexpected RFC ID", editHead: func(w *messageHeaderWire) { w.Payload.Headers[2].Value = "<other@example.invalid>" }, want: core.ErrGmailBinding},
		{name: "unexpected content hash", editHead: func(w *messageHeaderWire) { w.Payload.Headers[3].Value = "different" }, want: core.ErrGmailBinding},
		{name: "duplicate digest", editHead: func(w *messageHeaderWire) { w.Payload.Headers = append(w.Payload.Headers, w.Payload.Headers[3]) }, want: core.ErrGmailResponse},
		{name: "invalid provider date", editHead: func(w *messageHeaderWire) { w.Date = "not-milliseconds" }, want: core.ErrGmailResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refresh := fixtureRefresh(t)
			prepared := sendFixtureRequest(t, false)
			page := messageListWire{Messages: []messageReferenceWire{{ID: fixtureMessage, Thread: fixtureThread}}}
			header := sentHeaderFixture(t, prepared)
			if tc.editPage != nil {
				tc.editPage(&page)
			}
			if tc.editHead != nil {
				tc.editHead(&header)
			}
			var lists, metadata atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gmail/v1/users/me/messages" {
					lists.Add(1)
					values := r.URL.Query()
					if values.Get("q") != "rfc822msgid:"+string(prepared.RFCID) || values.Get("labelIds") != "SENT" || values.Get("maxResults") != "2" {
						t.Error("lookup escaped exact Sent search")
					}
					nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, page))
					return
				}
				if r.URL.Path == "/gmail/v1/users/me/messages/"+string(fixtureMessage) {
					metadata.Add(1)
					values := r.URL.Query()
					wantHeaders := []string{"From", "To", "Cc", "Bcc", "Message-ID", sendDigestHeader}
					if values.Get("format") != "metadata" || !slices.Equal(values["metadataHeaders"], wantHeaders) || strings.Contains(values.Get("fields"), "body") {
						t.Error("lookup escaped bounded metadata")
					}
					nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, header))
					return
				}
				if !workspaceOAuthFixture(t, r, w, grantedTokenFixture()) {
					t.Error("unexpected provider call")
					w.WriteHeader(http.StatusNotFound)
				}
			})
			access := nativeFixtureAccess(t, server, refresh)
			request := SentEvidenceRequest{Access: access, Policy: refresh.Policy, Thread: fixtureThread,
				RFCID: prepared.RFCID, ContentSHA: prepared.ContentSHA, Recipients: prepared.Recipients}
			if tc.exact {
				request.Message = fixtureMessage
			}
			got, err := server.ReadSentEvidence().Execute(t.Context(), request)
			if !errors.Is(err, tc.want) {
				t.Fatalf("evidence error = %v, want %v", err, tc.want)
			}
			wantLists, wantMetadata := int32(1), int32(1)
			switch {
			case tc.exact:
				wantLists = 0
			case tc.editPage != nil:
				wantMetadata = 0
			}
			if lists.Load() != wantLists || metadata.Load() != wantMetadata {
				t.Errorf("lookups = %d list, %d metadata, want %d/%d", lists.Load(), metadata.Load(), wantLists, wantMetadata)
			}
			if tc.want != nil || tc.empty {
				if got != (SentEvidence{}) {
					t.Fatal("unconfirmed attempt leaked partial proof")
				}
				return
			}
			wantAt := temporal.InstantFromNanoseconds(1795708800000000000)
			if got.Match.Message != fixtureMessage || got.Match.Thread != fixtureThread || got.Match.At != wantAt {
				t.Errorf("evidence = %v, want exact provider coordinates and time", got)
			}
		})
	}
}

func sentHeaderFixture(t testing.TB, req MessagePrepareRequest) messageHeaderWire {
	t.Helper()
	digest, err := req.ContentSHA.Hex()
	if err != nil {
		t.Fatal(err)
	}
	return messageHeaderWire{ID: fixtureMessage, Thread: fixtureThread, Date: "1795708800000",
		Labels: []labelWire{labelSent}, Payload: metadataWire{Headers: []headerWire{
			{Name: "From", Value: fixtureMailbox},
			{Name: "To", Value: headerValueWire(req.Recipients.To[0].Address.String())},
			{Name: "Message-ID", Value: headerValueWire(req.RFCID)},
			{Name: sendDigestHeader, Value: headerValueWire(digest)},
		}}}
}

func nativeFixtureAccess(t *testing.T, server Server, refresh RefreshRequest) Access {
	t.Helper()
	access, err := server.Refresh().Execute(t.Context(), refresh)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	return access
}

func TestGmailHistoryAndReplyThroughServer(t *testing.T) {
	t.Parallel()
	refresh := fixtureRefresh(t)
	var call atomic.Int32
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch {
		case r.URL.Path == "/gmail/v1/users/me/profile" && query.Get("fields") == "emailAddress,historyId":
			nativeFixtureWrite(t, w, http.StatusOK, []byte(`{"emailAddress":"owner@example.com","historyId":"200"}`))
		case r.URL.Path == "/gmail/v1/users/me/history":
			if query.Get("historyTypes") != "messageAdded" || query.Get("startHistoryId") != "200" ||
				query.Get("maxResults") != "25" || strings.Contains(query.Get("fields"), "payload") {
				t.Error("history query is missing its admission bounds")
			}
			switch call.Add(1) {
			case 1:
				nativeFixtureWrite(t, w, http.StatusOK, []byte(`{"history":[{"id":"201","messagesAdded":[{"message":{"id":"message_b","threadId":"thread_a"}}]}],"nextPageToken":"next&safe","historyId":"204"}`))
			case 2:
				if query.Get("pageToken") != "next&safe" {
					t.Error("history continuation lost its outer coordinate")
				}
				nativeFixtureWrite(t, w, http.StatusOK, []byte(`{"history":[{"id":"203","messagesAdded":[]}],"historyId":"204"}`))
			default:
				nativeFixtureWrite(t, w, http.StatusNotFound, []byte(`{"message":"private-failure"}`))
			}
		case r.URL.Path == "/gmail/v1/users/me/messages/"+string(fixtureMessage):
			if query.Get("format") != "metadata" || !slices.Contains(query["metadataHeaders"], "References") {
				t.Error("reply lookup must request metadata and references")
			}
			nativeFixtureWrite(t, w, http.StatusOK, []byte(`{"id":"message_b","threadId":"thread_a","internalDate":"1795708800000","payload":{"headers":[{"name":"From","value":"sender@example.invalid"},{"name":"Subject","value":"Synthetic proposal"},{"name":"Message-ID","value":"<parent@example.invalid>"},{"name":"References","value":"<first@example.invalid>\r\n <second@example.invalid>"}]}}`))
		default:
			if !workspaceOAuthFixture(t, r, w, grantedTokenFixture()) {
				t.Error("unexpected provider call")
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})
	access := nativeFixtureAccess(t, server, refresh)
	checkpoint, err := server.ReadHistoryCursor().Execute(t.Context(), HistoryCursorRequest{Access: access, Policy: refresh.Policy})
	if err != nil || checkpoint != "200" {
		t.Fatalf("cursor = %s, %v", checkpoint, err)
	}
	request := HistoryListRequest{Access: access, Policy: refresh.Policy, Since: checkpoint, Limit: 25}
	first, err := server.ListHistory().Execute(t.Context(), request)
	addition := HistoryAddition{Message: fixtureMessage, Thread: fixtureThread}
	if err != nil || first.Next != "next&safe" || first.Through != "" || !slices.Equal(first.Added, []HistoryAddition{addition}) {
		t.Fatalf("first history page = %+v, %v", first, err)
	}
	request.Cursor = first.Next
	last, err := server.ListHistory().Execute(t.Context(), request)
	if err != nil || last.Next != "" || last.Through != "204" || len(last.Added) != 0 {
		t.Fatalf("last history page = %+v, %v", last, err)
	}
	stale, err := server.ListHistory().Execute(t.Context(), request)
	if !errors.Is(err, core.ErrGmailHistoryResetRequired) || stale.Through != "" || strings.Contains(err.Error(), "private-failure") {
		t.Fatalf("stale history = %+v, %v", stale, err)
	}
	reply, err := server.ReadMessageMetadata().Execute(t.Context(), MessageReadRequest{Access: access, Policy: refresh.Policy, Thread: addition.Thread, Message: addition.Message})
	if err != nil || reply.Reply == nil || reply.Reply.Parent != fixtureMessage || reply.Reply.ParentRFCID != "<parent@example.invalid>" ||
		!slices.Equal(reply.Reply.References, []InternetMessageID{"<first@example.invalid>", "<second@example.invalid>"}) {
		t.Fatalf("message metadata did not yield the exact parent and folded references: %v", err)
	}
}

func TestGmailSearchRecentReplyThroughServer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		thread   ThreadID
		sender   headerValueWire
		date     internalDateWire
		label    labelWire
		next     PageToken
		mismatch bool
		want     RecentReply
		wantErr  error
	}{
		{name: "exact unlinked reply", want: RecentReplyFound},
		{name: "already linked", thread: fixtureThread, want: RecentReplyNone},
		{name: "other sender", sender: "someone@example.invalid", want: RecentReplyNone},
		{name: "at previous touch", date: "1795708799999", want: RecentReplyNone},
		{name: "sent copy", label: labelSent, want: RecentReplyNone},
		{name: "saturated candidate search", sender: "someone@example.invalid", next: "opaque", wantErr: core.ErrExchangeBodyLimit},
		{name: "wrong message membership", mismatch: true, wantErr: core.ErrGmailBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refresh := fixtureRefresh(t)
			contact, err := core.ParseOAuthEmail("buyer@example.invalid")
			if err != nil {
				t.Fatal(err)
			}
			thread, sender, at := tc.thread, tc.sender, tc.date
			if thread == "" {
				thread = "candidate_thread"
			}
			if sender == "" {
				sender = headerValueWire(contact.String())
			}
			if at == "" {
				at = "1795708800000"
			}
			ref := messageReferenceWire{ID: fixtureMessage, Thread: thread}
			head := messageHeaderWire{ID: ref.ID, Thread: ref.Thread, Date: at,
				Payload: metadataWire{Headers: []headerWire{{Name: "From", Value: sender}}}}
			if tc.mismatch {
				head.ID = "unexpected"
			}
			if tc.label != "" {
				head.Labels = []labelWire{tc.label}
			}
			var searches, meta atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/gmail/v1/users/me/messages":
					searches.Add(1)
					values := r.URL.Query()
					if values.Get("q") != "from:buyer@example.invalid after:1795708798" || values.Get("maxResults") != "5" {
						t.Error("search was not bound to the contact and prior send time")
					}
					nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, messageListWire{Messages: []messageReferenceWire{ref}, Next: tc.next}))
				case "/gmail/v1/users/me/messages/" + string(fixtureMessage):
					meta.Add(1)
					query := r.URL.Query()
					if query.Get("format") != "metadata" || !slices.Equal(query["metadataHeaders"], []string{"From"}) ||
						strings.Contains(query.Get("fields"), "body") {
						t.Error("unlinked candidate fetched additional private fields")
					}
					nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, head))
				default:
					if !workspaceOAuthFixture(t, r, w, grantedTokenFixture()) {
						t.Error("unexpected provider call")
						w.WriteHeader(http.StatusNotFound)
					}
				}
			})
			access := nativeFixtureAccess(t, server, refresh)
			got, err := server.SearchRecentReply().Execute(t.Context(), RecentReplyRequest{
				Access: access, Policy: refresh.Policy, Contact: contact, Thread: fixtureThread,
				After: temporal.InstantFromNanoseconds(1795708799999000000),
			})
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("recent reply = %v, %v, want %v, %v", got, err, tc.want, tc.wantErr)
			}
			wantMeta := int32(1)
			if thread == fixtureThread {
				wantMeta = 0
			}
			if searches.Load() != 1 || meta.Load() != wantMeta {
				t.Fatal("candidate fetched more mailbox data than its bound")
			}
		})
	}
}
