package gmail

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzGmailSendResponse(f *testing.F) {
	f.Add(uint16(http.StatusOK), encodeFixture(f, sendAcceptanceWire{ID: fixtureMessage, Thread: fixtureThread, Date: "1795708800000"}))
	f.Add(uint16(http.StatusBadRequest), encodeFixture(f, sendFaultWire{Error: sendFaultDetails{
		Code: http.StatusBadRequest, Reasons: []sendFaultItem{{Reason: sendFaultBadRecipient}},
	}}))
	f.Add(uint16(http.StatusOK), []byte(`{"id":"message_b","threadId":"other"}`))
	f.Add(uint16(http.StatusOK), []byte(`{"id":"message_b","id":"other","threadId":"thread_a"}`))
	f.Add(uint16(http.StatusServiceUnavailable), []byte(`{"message":"private-marker"}`))

	f.Fuzz(func(t *testing.T, status uint16, raw []byte) {
		if status != http.StatusOK {
			got := classifySendFailure(int(status), raw, nil)
			if got == nil || strings.Contains(got.Error(), "private-marker") {
				t.Fatal("provider failure escaped the typed error boundary")
			}
			if status >= 500 && !errors.Is(got, core.ErrGmailSendUnconfirmed) {
				t.Fatal("server failure asserted a definite send outcome")
			}
			if status == http.StatusTooManyRequests && !errors.Is(got, core.ErrExchangeRateLimited) {
				t.Fatal("lost provider throttling")
			}
			return
		}
		got, err := decodeSendAcceptance(raw, fixtureThread)
		if err != nil {
			if got != (SendAcceptance{}) || !errors.Is(err, core.ErrGmailSendUnconfirmed) {
				t.Fatal("invalid acceptance produced a partial or definite result")
			}
			return
		}
		var wire sendAcceptanceWire
		if len(raw) > sendResponseMaximumBytes || json.Unmarshal(raw, &wire) != nil ||
			got.Validate() != nil || got.Message != wire.ID || got.Thread != fixtureThread || got.Thread != wire.Thread {
			t.Fatal("accepted send lost the provider's bound routing coordinates")
		}
		if wire.Date == "" {
			if got.At.IsSet() {
				t.Fatal("missing provider time was invented")
			}
			return
		}
		ms, parseErr := strconv.ParseUint(string(wire.Date), 10, 64)
		ns, timeErr := got.At.Nanoseconds()
		if parseErr != nil || timeErr != nil || ms == 0 || ms > math.MaxInt64/1_000_000 || int64(ms)*1_000_000 != ns {
			t.Fatal("accepted provider time did not preserve positive epoch milliseconds")
		}
	})
}

func FuzzGmailMessageListResponse(f *testing.F) {
	f.Add(encodeFixture(f, messageListWire{Messages: []messageReferenceWire{{ID: fixtureMessage, Thread: fixtureThread}}}))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"messages":[{"id":"a","threadId":"thread_a"},{"id":"b","threadId":"thread_a"}]}`))
	f.Add([]byte(`{"nextPageToken":"ambiguous"}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		wire, err := decodeProvider[messageListWire](raw)
		if err != nil {
			return
		}
		if err := wire.validateRecentReplyCandidates(); err == nil {
			if len(wire.Messages) > recentReplySearchMaximum || wire.Next.Validate() != nil {
				t.Fatal("off-thread search admitted invalid extent")
			}
			for i, ref := range wire.Messages {
				if ref.Thread.Validate() != nil || ref.ID.Validate() != nil {
					t.Fatal("off-thread search admitted an invalid message reference")
				}
				for _, prev := range wire.Messages[:i] {
					if prev.ID == ref.ID {
						t.Fatal("off-thread search admitted repeated references")
					}
				}
			}
		}
		got, err := wire.project(fixtureThread)
		if err != nil {
			if got != (messageReferenceWire{}) {
				t.Fatal("rejected list escaped with a selected message")
			}
			return
		}
		if wire.Next != "" || len(wire.Messages) > 1 {
			t.Fatal("unbounded or ambiguous search produced match evidence")
		}
		if len(wire.Messages) == 0 {
			if got != (messageReferenceWire{}) {
				t.Fatal("empty search invented a visible message")
			}
			return
		}
		if got != wire.Messages[0] || got.Thread != fixtureThread || got.ID.Validate() != nil {
			t.Fatal("search did not conserve exact message and thread")
		}
	})
}

func FuzzGmailSentMetadataResponse(f *testing.F) {
	prepared := sendFixtureRequest(f, false)
	header := sentHeaderFixture(f, prepared)
	f.Add(encodeFixture(f, header))
	header.Labels = append(header.Labels, labelDraft)
	f.Add(encodeFixture(f, header))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		wire, err := decodeProvider[messageHeaderWire](raw)
		if err != nil {
			return
		}
		request := SentEvidenceRequest{
			Access: Access{mailbox: prepared.From.Address}, Policy: fixturePolicy(t), Thread: fixtureThread,
			RFCID: prepared.RFCID, ContentSHA: prepared.ContentSHA, Recipients: prepared.Recipients,
		}
		got, err := wire.sentEvidence(request, messageReferenceWire{ID: fixtureMessage, Thread: fixtureThread})
		if err != nil {
			if got != (SentEvidence{}) {
				t.Fatal("invalid metadata yielded partial send evidence")
			}
			return
		}
		hash, _ := prepared.ContentSHA.Hex()
		wantHeaders := []headerWire{{Name: "Message-ID", Value: headerValueWire(prepared.RFCID)}, {Name: sendDigestHeader, Value: headerValueWire(hash)}}
		for _, match := range wantHeaders {
			count := 0
			for _, actual := range wire.Payload.Headers {
				if strings.EqualFold(string(actual.Name), string(match.Name)) && actual.Value == match.Value {
					count++
				}
			}
			if count != 1 {
				t.Fatal("Sent evidence admitted without the exact frozen identifying headers")
			}
		}
		if got.Validate() != nil || got.Match.Message != fixtureMessage || got.Match.Thread != fixtureThread || !got.Match.At.IsSet() ||
			!slices.Contains(wire.Labels, labelSent) || slices.Contains(wire.Labels, labelDraft) {
			t.Fatal("metadata was accepted without verified Sent coordinates and time")
		}
	})
}

func FuzzGmailHistoryResponse(f *testing.F) {
	f.Add(encodeFixture(f, historyPageWire{ID: "207", History: []historyRecordWire{{
		ID: "203", Added: []historyItemWire{{Message: messageReferenceWire{ID: fixtureMessage, Thread: fixtureThread}}},
	}}}))
	f.Add(encodeFixture(f, historyPageWire{ID: "205", Next: "next"}))
	f.Add([]byte(`{"historyId":"9"}`))
	f.Add([]byte(`{"historyId":"18446744073709551616"}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		wire, err := decodeProvider[historyPageWire](raw)
		if err != nil {
			return
		}
		const since = 200
		got, err := wire.project(HistoryListRequest{Since: "200", Cursor: "same", Limit: HistoryRecordMaximum})
		if err != nil {
			if len(got.Added) != 0 || got.Through != "" || got.Next != "" {
				t.Fatal("invalid history escaped with a partial cursor")
			}
			return
		}
		if got.Validate() != nil || len(wire.History) > HistoryRecordMaximum {
			t.Fatal("history escaped record and event limits")
		}
		tip, err := strconv.ParseUint(string(wire.ID), 10, 64)
		if err != nil || tip < since || got.Next != wire.Next || got.Next == "same" ||
			got.Next != "" && got.Through != "" || got.Next == "" && got.Through != wire.ID {
			t.Fatal("history returned an incorrect or advanceable partial cursor")
		}
		i := 0
		last := uint64(since)
		for _, history := range wire.History {
			n, err := strconv.ParseUint(string(history.ID), 10, 64)
			if err != nil || n <= last || n > tip {
				t.Fatal("history admitted a coordinate outside the ordered interval")
			}
			last = n
			for _, item := range history.Added {
				if i >= len(got.Added) || got.Added[i].Message != item.Message.ID || got.Added[i].Thread != item.Message.Thread {
					t.Fatal("history lost provider message coordinates")
				}
				i++
			}
		}
		if i != len(got.Added) {
			t.Fatal("history invented an addition")
		}
	})
}

func FuzzGmailReplyMetadataResponse(f *testing.F) {
	request := sendFixtureRequest(f, false)
	message := sentHeaderFixture(f, request)
	for _, h := range []headerWire{
		{Name: "Subject", Value: headerValueWire(request.Subject)},
		{Name: "References", Value: "<first@example.invalid>\r\n <second@example.invalid>"},
	} {
		message.Payload.Headers = append(message.Payload.Headers, h)
	}
	f.Add(encodeFixture(f, message))
	message.Payload.Headers = nil
	f.Add(encodeFixture(f, message))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		wire, err := decodeProvider[messageHeaderWire](raw)
		if err != nil {
			return
		}
		got, err := wire.messageMetadata(MessageReadRequest{
			Access: Access{mailbox: request.From.Address}, Thread: fixtureThread, Message: fixtureMessage,
		})
		if err != nil {
			if got.Thread != "" || got.Header.ID != "" || got.Reply != nil {
				t.Fatal("rejected reply contained partial metadata")
			}
			return
		}
		if got.Thread != fixtureThread || got.Header.ID != fixtureMessage || got.Validate() != nil || slices.Contains(wire.Labels, labelDraft) {
			t.Fatal("message metadata lost its exact parent binding")
		}
		if got.Reply != nil {
			id, err := wire.oneHeader("Message-ID")
			if err != nil || string(got.Reply.ParentRFCID) != string(id) || got.Reply.Parent != wire.ID ||
				got.Reply.ParentSubject != got.Header.Subject || slices.Contains(got.Reply.References, got.Reply.ParentRFCID) {
				t.Fatal("reply reference invented or duplicated its parent")
			}
		}
	})
}

func FuzzGmailRecentReplyMetadataResponse(f *testing.F) {
	request := sendFixtureRequest(f, false)
	message := sentHeaderFixture(f, request)
	message.Labels = nil
	message.Payload.Headers[0].Value = "buyer@example.invalid"
	f.Add(encodeFixture(f, message))
	message.Labels = []labelWire{labelSent}
	f.Add(encodeFixture(f, message))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		wire, err := decodeProvider[messageHeaderWire](raw)
		if err != nil {
			return
		}
		contact := request.Recipients.To[0].Address
		after := temporal.InstantFromNanoseconds(1795708799000000000)
		got, err := wire.recentReply(RecentReplyRequest{Contact: contact, After: after},
			messageReferenceWire{ID: fixtureMessage, Thread: fixtureThread})
		if err != nil {
			if got {
				t.Fatal("rejected metadata claimed a candidate")
			}
			return
		}
		if got {
			date, dateErr := strconv.ParseInt(string(wire.Date), 10, 64)
			rawSender, headerErr := wire.oneHeader("From")
			sender, senderErr := projectSender(rawSender)
			if dateErr != nil || headerErr != nil || senderErr != nil || date <= 1795708799000 ||
				wire.Thread != fixtureThread || wire.ID != fixtureMessage || !strings.EqualFold(sender.String(), contact.String()) ||
				slices.Contains(wire.Labels, labelDraft) || slices.Contains(wire.Labels, labelSent) {
				t.Fatal("admitted off-thread reply lacks exact sender, date or direction")
			}
		}
	})
}

func FuzzGmailHistoryCursorResponse(f *testing.F) {
	mailbox := sendFixtureRequest(f, false).From.Address
	f.Add(encodeFixture(f, historyProfileWire{Email: fixtureMailbox, Cursor: "200"}))
	f.Add(encodeFixture(f, historyProfileWire{Email: "other@example.invalid", Cursor: "200"}))
	f.Add([]byte(`{"emailAddress":"owner@example.com","historyId":"000"}`))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		wire, err := decodeProvider[historyProfileWire](raw)
		if err != nil {
			return
		}
		got, err := wire.project(mailbox)
		if err != nil {
			if got != "" || !errors.Is(err, core.ErrGmailContract) {
				t.Fatal("unbound profile returned a usable cursor")
			}
			return
		}
		n, err := strconv.ParseUint(string(wire.Cursor), 10, 64)
		if err != nil || n == 0 || got != wire.Cursor || strconv.FormatUint(n, 10) != string(got) ||
			!strings.EqualFold(string(wire.Email), mailbox.String()) {
			t.Fatal("cursor did not preserve the exact admitted profile")
		}
	})
}

func FuzzGmailConsentTokenResponse(f *testing.F) {
	fixture := workspaceConsentFixture()
	f.Add(encodeFixture(f, fixture))
	fixture.RefreshExpiry = 3600
	f.Add(encodeFixture(f, fixture))
	fixture.RefreshExpiry, fixture.Offline = 0, ""
	f.Add(encodeFixture(f, fixture))
	f.Add([]byte(`{"refresh_token":"fake","access_token":"fake","scope":[]}`))
	f.Add([]byte("null"))

	mailbox := sendFixtureRequest(f, false).From.Address
	f.Fuzz(func(t *testing.T, raw []byte) {
		access, offline, err := admitWorkspaceGrant(raw, mailbox)
		if err != nil {
			if access != (Access{}) || offline.Validate() == nil || !errors.Is(err, core.ErrGmailContract) {
				t.Fatal("refused consent produced a partial credential")
			}
			return
		}
		defer func() {
			if err := errors.Join(access.Close(), offline.Close()); err != nil {
				t.Error("accepted credentials did not successfully close")
			}
		}()
		var expected consentTokenFixture
		if len(raw) > tokenResponseMaximum || json.Unmarshal(raw, &expected) != nil || expected.RefreshExpiry != 0 ||
			expected.Offline == "" || len(expected.Offline) > credentialMaximumBytes {
			t.Fatal("accepted consent lost the offline lifetime or extent bounds")
		}
		fields := strings.Fields(string(expected.Scope))
		slices.Sort(fields)
		if !slices.Equal(fields, []string{ReadOnlyScope, SendScope}) || access.Mailbox() != mailbox || access.Validate() != nil {
			t.Fatal("accepted consent admitted an incorrect scope or mailbox coordinate")
		}
		got, err := offline.CopyBytes()
		defer clear(got)
		if err != nil || string(got) != expected.Offline {
			t.Fatal("accepted consent changed the confidential custody payload")
		}
		if data, err := offline.MarshalJSON(); err == nil || len(data) != 0 {
			t.Fatal("accepted consent can escape through JSON")
		}
	})
}
