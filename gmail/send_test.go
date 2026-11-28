package gmail

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	fixtureLetterText = "Synthetic letter ✓\n\nSecond paragraph.\n"
	fixtureLetterHTML = "<p>Synthetic letter ✓</p>\n"
	fixtureLetterPDF  = "%PDF-1.7\nSynthetic fixture\n%%EOF\n"
)

func sendFixtureRequest(t testing.TB, pdf bool) MessagePrepareRequest {
	t.Helper()
	from, err := core.ParseOAuthEmail(fixtureMailbox)
	if err != nil {
		t.Fatal(err)
	}
	to, err := core.ParseOAuthEmail("buyer@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	at, err := temporal.ParseRFC3339("2026-11-27T12:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	request := MessagePrepareRequest{
		From: Recipient{Address: from}, Recipients: Recipients{To: []Recipient{{Address: to}}},
		Subject: "Synthetic proposal", RFCID: "<attempt-2@example.invalid>", Text: fixtureLetterText,
		HTML: fixtureLetterHTML, At: at, ContentSHA: core.SHA256Of([]byte("frozen snapshot")),
		Reply: &ReplyReference{Thread: fixtureThread, Parent: fixtureMessage,
			ParentRFCID: "<parent@example.invalid>", ParentSubject: "Synthetic proposal",
			References: []InternetMessageID{"<initial@example.invalid>"}},
	}
	if pdf {
		size, err := core.NewByteLength(uint64(len(fixtureLetterPDF)))
		if err != nil {
			t.Fatal(err)
		}
		request.PDFs = []SendPDF{{Name: "proposal.pdf", Size: size,
			SHA256: core.SHA256Of([]byte(fixtureLetterPDF)), Source: strings.NewReader(fixtureLetterPDF)}}
	}
	return request
}

type sendFixturePDF struct {
	Name PDFName
	Body string
}

func (p sendFixturePDF) source(t testing.TB) SendPDF {
	t.Helper()
	size, err := core.NewByteLength(uint64(len(p.Body)))
	if err != nil {
		t.Fatal(err)
	}
	return SendPDF{Name: p.Name, Size: size, SHA256: core.SHA256Of([]byte(p.Body)), Source: strings.NewReader(p.Body)}
}

var orderedSendPDFs = [...]sendFixturePDF{
	{Name: "response.pdf", Body: "%PDF-1.7\nResponse\n%%EOF\n"},
	{Name: "pricing.pdf", Body: "%PDF-1.7\nPricing\n%%EOF\n"},
	{Name: "forms.pdf", Body: "%PDF-1.7\nForms\n%%EOF\n"},
}

func workspaceOAuthFixture(t testing.TB, r *http.Request, w http.ResponseWriter, token tokenFixture) bool {
	t.Helper()
	switch r.URL.Path {
	case "/token":
		if r.Method != http.MethodPost || r.Header.Get("X-Fixture-Provider") != "oauth2.googleapis.com" ||
			r.Header.Get("Authorization") != "" {
			t.Error("wrong OAuth request")
		}
		nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, token))
	case "/gmail/v1/users/me/profile":
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer "+fixtureToken {
			t.Error("profile did not receive the acquired bearer")
		}
		nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, mailboxWire{Email: fixtureMailbox}))
	default:
		return false
	}
	return true
}

func TestGmailWorkspaceSendThroughServer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		status  int
		body    string
		want    error
		pdf     bool
		ordered bool
	}{
		{name: "reply with pinned PDF", status: 200, body: `{"id":"sent_1","threadId":"thread_a","internalDate":"1795782600000"}`, pdf: true},
		{name: "ordered bid PDFs", status: 200, body: `{"id":"sent_1","threadId":"thread_a"}`, ordered: true},
		{name: "link only", status: 200, body: `{"id":"sent_1","threadId":"thread_a"}`},
		{name: "other thread", status: 200, body: `{"id":"sent_1","threadId":"other"}`, want: core.ErrGmailSendUnconfirmed},
		{name: "malformed acceptance", status: 200, body: `{"id":"sent_1"`, want: core.ErrGmailSendUnconfirmed},
		{name: "bearer refused", status: 401, body: `{"credential":"private-marker"}`, want: core.ErrGmailConsent},
		{name: "recipient refused", status: 400, body: `{"error":{"code":400,"errors":[{"reason":"invalidRecipient","message":"private-marker"}]}}`, want: core.ErrGmailRecipient},
		{name: "quota refused", status: 403, body: `{"error":{"code":403,"errors":[{"reason":"userRateLimitExceeded","message":"private-marker"}]}}`, want: core.ErrExchangeRateLimited},
		{name: "content refused", status: 413, body: `{"message":"private-marker"}`, want: core.ErrExchangeBodyLimit},
		{name: "provider failure after dispatch", status: 500, body: `{"message":"private-marker"}`, want: core.ErrGmailSendUnconfirmed},
		{name: "lost response after reading entire request", want: core.ErrGmailSendUnconfirmed},
		{name: "workspace policy", status: 403, body: `{"error":{"code":403,"errors":[{"reason":"domainPolicy"}]}}`, want: core.ErrGmailPolicy},
		{name: "mailbox sending limit", status: 429, body: `{"message":"private-marker"}`, want: core.ErrExchangeRateLimited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refresh := fixtureRefresh(t)
			refresh.Scope = GrantReadAndSend
			token := grantedTokenFixture()
			token.Scope = ReadOnlyScope + " " + SendScope
			request := sendFixtureRequest(t, tc.pdf)
			var files []sendFixturePDF
			if tc.ordered {
				files = orderedSendPDFs[:]
				for _, file := range files {
					request.PDFs = append(request.PDFs, file.source(t))
				}
			}
			if tc.pdf {
				files = []sendFixturePDF{{Name: "proposal.pdf", Body: fixtureLetterPDF}}
			}
			var posts atomic.Int32
			received := make(chan core.SHA256Digest, 1)
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				if workspaceOAuthFixture(t, r, w, token) {
					return
				}
				posts.Add(1)
				raw, err := receiveSendFixture(r)
				if err != nil {
					t.Error(err)
				} else {
					select {
					case received <- core.SHA256Of(raw):
					default:
						t.Error("send repeated a completed POST")
					}
					checkSendMIME(t, raw, files)
				}
				if tc.status == 0 {
					panic(http.ErrAbortHandler)
				}
				nativeFixtureWrite(t, w, tc.status, []byte(tc.body))
			})
			access, err := server.Refresh().Execute(t.Context(), refresh)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := access.Close(); err != nil {
					t.Error(err)
				}
			}()
			send, err := access.Sending()
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := server.PrepareMessage().Execute(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			got, err := server.SendMessage().Execute(t.Context(), MessageSendRequest{Prepared: prepared, Access: send, Policy: refresh.Policy})
			if tc.want != nil && (!errors.Is(err, tc.want) || got != (SendAcceptance{}) || strings.Contains(err.Error(), "private-marker")) {
				t.Errorf("send refusal = (%+v, %v), want typed %v and no partial result", got, err, tc.want)
			}
			if tc.want == nil && (err != nil || got.Message != "sent_1" || got.Thread != fixtureThread || got.At.IsSet() != tc.pdf) {
				t.Errorf("send result = (%+v, %v), want bound Gmail acceptance", got, err)
			}
			if posts.Load() != 1 {
				t.Errorf("send POST count = %d, want 1", posts.Load())
			}
			select {
			case digest := <-received:
				if digest != prepared.Digest() {
					t.Error("send did not POST the exact prepared bytes")
				}
			default:
				t.Error("provider did not receive the prepared message")
			}
		})
	}
}

func receiveSendFixture(r *http.Request) ([]byte, error) {
	const path = "/upload/gmail/v1/users/me/messages/send"
	if r.URL.Path != path || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+fixtureToken ||
		r.Header.Get("X-Fixture-Provider") != "gmail.googleapis.com" || r.URL.Query().Get("uploadType") != "multipart" ||
		r.URL.Query().Get("fields") != "id,threadId,internalDate" || r.ContentLength <= 0 {
		return nil, errors.New("wire send route, method, credentials, query or extent did not match")
	}
	kind, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "multipart/related" {
		return nil, errors.New("send upload must be multipart/related")
	}
	envelope := multipart.NewReader(r.Body, params["boundary"])
	metadata, err := envelope.NextPart()
	if err != nil {
		return nil, err
	}
	var doc sendMetadata
	if err := json.NewDecoder(metadata).Decode(&doc); err != nil || doc.Thread != fixtureThread ||
		!strings.HasPrefix(metadata.Header.Get("Content-Type"), "application/json") {
		return nil, errors.New("send metadata did not bind requested thread")
	}
	message, err := envelope.NextPart()
	if err != nil {
		return nil, err
	}
	if message.Header.Get("Content-Type") != "message/rfc822" {
		return nil, errors.New("missing RFC media part")
	}
	raw, err := io.ReadAll(message)
	_, end := envelope.NextPart()
	if err != nil || end != io.EOF {
		return nil, errors.New("send request must contain exactly two upload parts")
	}
	return raw, nil
}

func checkSendMIME(t *testing.T, raw []byte, files []sendFixturePDF) {
	t.Helper()
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Error(err)
		return
	}
	if message.Header.Get("From") != "<"+fixtureMailbox+">" ||
		message.Header.Get("Message-ID") != "<attempt-2@example.invalid>" ||
		message.Header.Get("In-Reply-To") != "<parent@example.invalid>" ||
		message.Header.Get("References") != "<initial@example.invalid> <parent@example.invalid>" ||
		message.Header.Get("Subject") != "Synthetic proposal" ||
		!strings.Contains(message.Header.Get("To"), "buyer@example.invalid") {
		t.Error("reply and envelope headers did not match")
	}
	kind, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Error(err)
		return
	}
	body := multipart.NewReader(message.Body, params["boundary"])
	if len(files) != 0 {
		if kind != "multipart/mixed" {
			t.Error("expected multipart mixed for PDF")
			return
		}
		part, err := body.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		kind, params, err = mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Error(err)
			return
		}
		checkAlternative(t, kind, multipart.NewReader(part, params["boundary"]))
		for _, expected := range files {
			file, err := body.NextPart()
			if err != nil {
				t.Error(err)
				return
			}
			if file.FileName() != string(expected.Name) {
				t.Error("incorrect PDF filename or order")
			}
			checkMIMEPart(t, file, "application/pdf", expected.Body)
		}
		if _, err := body.NextPart(); err != io.EOF {
			t.Error("unexpected extra attachment")
		}
		return
	}
	checkAlternative(t, kind, body)
}

func checkAlternative(t *testing.T, kind string, reader *multipart.Reader) {
	t.Helper()
	if kind != "multipart/alternative" {
		t.Errorf("body kind = %q, want alternative", kind)
		return
	}
	for _, want := range [...]struct{ media, text string }{{"text/plain", fixtureLetterText}, {"text/html", fixtureLetterHTML}} {
		part, err := reader.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		checkMIMEPart(t, part, want.media, want.text)
	}
	if _, err := reader.NextPart(); err != io.EOF {
		t.Error("expected exactly the two rendered alternatives")
	}
}

func checkMIMEPart(t *testing.T, part *multipart.Part, media, want string) {
	t.Helper()
	kind, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil || kind != media || part.Header.Get("Content-Transfer-Encoding") != "base64" {
		t.Error("incorrect MIME part headers")
		return
	}
	body, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
	if err != nil || string(body) != want {
		t.Error("MIME part did not preserve approved bytes")
	}
}

func TestGmailWorkspaceGrantExactScopes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		mode      GrantScope
		wire      scopeWire
		refreshOK bool
		sendOK    bool
	}{
		{name: "read provider grant", wire: ReadOnlyScope, refreshOK: true},
		{name: "send provider grant", mode: GrantReadAndSend, wire: ReadOnlyScope + " " + SendScope, refreshOK: true, sendOK: true},
		{name: "reverse scope order", mode: GrantReadAndSend, wire: SendScope + " " + ReadOnlyScope, refreshOK: true, sendOK: true},
		{name: "read mode cannot adopt both", wire: ReadOnlyScope + " " + SendScope},
		{name: "refuse modify", mode: GrantReadAndSend, wire: "https://www.googleapis.com/auth/gmail.modify"},
		{name: "missing write", mode: GrantReadAndSend, wire: ReadOnlyScope},
		{name: "missing read", mode: GrantReadAndSend, wire: SendScope},
		{name: "over-scoped", mode: GrantReadAndSend, wire: ReadOnlyScope + " " + SendScope + " https://mail.google.com/"},
		{name: "repeated scope", mode: GrantReadAndSend, wire: ReadOnlyScope + " " + SendScope + " " + SendScope},
		{name: "empty scope", mode: GrantReadAndSend},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refresh := fixtureRefresh(t)
			refresh.Scope = tc.mode
			token := grantedTokenFixture()
			token.Scope = tc.wire
			var profiles atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gmail/v1/users/me/profile" {
					profiles.Add(1)
				}
				if !workspaceOAuthFixture(t, r, w, token) {
					t.Error("unexpected provider call")
					w.WriteHeader(http.StatusNotFound)
				}
			})
			got, err := server.Refresh().Execute(t.Context(), refresh)
			if !tc.refreshOK {
				if !errors.Is(err, core.ErrGmailConsent) || got != (Access{}) || profiles.Load() != 0 {
					t.Fatal("wrong scope reached profile lookup or escaped admission")
				}
				return
			}
			if err != nil || profiles.Load() != 1 {
				t.Fatalf("verified grant missing after the profile read: %v", err)
			}
			defer func() {
				if err := got.Close(); err != nil {
					t.Error(err)
				}
			}()
			send, sendErr := got.Sending()
			if tc.sendOK {
				if sendErr != nil || send.Mailbox() != refresh.Mailbox {
					t.Fatal("exact grant did not yield an account-bound send capability")
				}
			} else if !errors.Is(sendErr, core.ErrGmailConsent) || send != (SendAccess{}) {
				t.Fatal("read-only grant issued a send capability")
			}
		})
	}
}

func TestGmailPrepareRejectsHostileParts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		edit func(*MessagePrepareRequest)
		want error
	}{
		{name: "header injection", edit: func(r *MessagePrepareRequest) { r.Subject = "Synthetic\r\nCc: forged" }, want: core.ErrGmailContract},
		{name: "changed reply subject", edit: func(r *MessagePrepareRequest) { r.Subject = "Another subject" }, want: core.ErrGmailBinding},
		{name: "repeated message ID", edit: func(r *MessagePrepareRequest) { r.Reply.References = append(r.Reply.References, r.Reply.ParentRFCID) }, want: core.ErrGmailBinding},
		{name: "combined rendered extent", edit: func(r *MessagePrepareRequest) { r.HTML = HTMLText(strings.Repeat("x", SendPartsMaximumBytes)) }, want: core.ErrExchangeBodyLimit},
		{name: "duplicate recipient", edit: func(r *MessagePrepareRequest) { r.Recipients.Bcc = r.Recipients.To }, want: core.ErrGmailRecipient},
		{name: "too many recipients", edit: func(r *MessagePrepareRequest) { r.Recipients.To = make([]Recipient, SendRecipientsMaximum+1) }, want: core.ErrGmailRecipient},
		{name: "PDF digest mismatch", edit: func(r *MessagePrepareRequest) { r.PDFs[0].SHA256 = core.SHA256Of([]byte("other")) }, want: core.ErrGmailBinding},
		{name: "PDF short read", edit: func(r *MessagePrepareRequest) { r.PDFs[0].Source = strings.NewReader("%PDF-") }, want: core.ErrGmailBinding},
		{name: "PDF trailing bytes", edit: func(r *MessagePrepareRequest) { r.PDFs[0].Source = strings.NewReader(fixtureLetterPDF + "extra") }, want: core.ErrGmailBinding},
		{name: "non PDF header", edit: func(r *MessagePrepareRequest) { r.PDFs[0].Source = strings.NewReader("other" + fixtureLetterPDF[5:]) }, want: core.ErrGmailContract},
		{name: "PDF outside bound", edit: func(r *MessagePrepareRequest) { r.PDFs[0].Size, _ = core.NewByteLength(SendPDFMaximumBytes + 1) }, want: core.ErrExchangeBodyLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := sendFixtureRequest(t, true)
			tc.edit(&req)
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("preparation must not contact the provider")
				w.WriteHeader(http.StatusNotFound)
			})
			got, err := server.PrepareMessage().Execute(t.Context(), req)
			if !errors.Is(err, tc.want) || got.Validate() == nil || got.Extent() != 0 {
				t.Fatalf("preparation = (%v, %v), want typed refusal %v", got, err, tc.want)
			}
		})
	}
}

type sendUnreadPDF struct{ t *testing.T }

func (r sendUnreadPDF) Read([]byte) (int, error) {
	r.t.Fatal("oversized attachment list started reading file bytes")
	return 0, io.EOF
}

func TestGmailPrepareRejectsInvalidPDFList(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		edit func(*testing.T, *MessagePrepareRequest)
		want error
	}{
		{name: "too many before reading", want: core.ErrExchangeBodyLimit, edit: func(t *testing.T, r *MessagePrepareRequest) {
			r.PDFs = make([]SendPDF, SendPDFMaximumCount+1)
			for index := range r.PDFs {
				r.PDFs[index].Source = sendUnreadPDF{t: t}
			}
		}},
		{name: "combined extent before reading", want: core.ErrExchangeBodyLimit, edit: func(t *testing.T, r *MessagePrepareRequest) {
			r.PDFs[0].Size, _ = core.NewByteLength(SendPDFMaximumBytes - 20)
			r.PDFs[1].Size, _ = core.NewByteLength(21)
			r.PDFs[0].Source = sendUnreadPDF{t: t}
		}},
		{name: "duplicate name", want: core.ErrGmailContract, edit: func(t *testing.T, r *MessagePrepareRequest) {
			r.PDFs[1].Name = "RESPONSE.PDF"
			r.PDFs[0].Source = sendUnreadPDF{t: t}
		}},
		{name: "corrupt second file", want: core.ErrGmailBinding, edit: func(t *testing.T, r *MessagePrepareRequest) {
			r.PDFs[1].SHA256 = r.PDFs[0].SHA256
			r.PDFs[2].Source = sendUnreadPDF{t: t}
		}},
		{name: "trailing bytes on last file", want: core.ErrGmailBinding, edit: func(t *testing.T, r *MessagePrepareRequest) {
			r.PDFs[2].Source = strings.NewReader(orderedSendPDFs[2].Body + "extra bytes")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := sendFixtureRequest(t, false)
			for _, pdf := range orderedSendPDFs {
				request.PDFs = append(request.PDFs, pdf.source(t))
			}
			tc.edit(t, &request)
			server := fixtureServer(t, func(w http.ResponseWriter, _ *http.Request) {
				t.Error("invalid attachment list contacted Gmail")
				w.WriteHeader(http.StatusNotFound)
			})
			prepared, err := server.PrepareMessage().Execute(t.Context(), request)
			if !errors.Is(err, tc.want) || prepared.Validate() == nil || prepared.Extent() != 0 {
				t.Fatalf("preparation = (%v, %v), want blank value and %v", prepared, err, tc.want)
			}
		})
	}
}

func TestGmailPreparePreservesReleasedMIME(t *testing.T) {
	t.Parallel()
	// Captured by encoding these fixtures with Primitive v2026.1.517.
	for _, tc := range []struct {
		name   string
		pdf    bool
		bytes  int
		digest string
	}{
		{name: "without attachments", bytes: 848, digest: "16479ffa85a29e53b21d53d4d86cdb59a64624ceec1b73fc9cf64c735592c226"},
		{name: "one attachment", pdf: true, bytes: 1232, digest: "c5df8b066469592ec8fb877fcf2db0354bdab6ef271d0f1619496df12176ccc7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, err := encodeMessage(sendFixtureRequest(t, tc.pdf))
			if err != nil {
				t.Fatal(err)
			}
			digest, err := core.SHA256Of(raw).Hex()
			if err != nil || len(raw) != tc.bytes || digest != tc.digest {
				t.Fatalf("prepared MIME changed: extent %d, digest %s, error %v", len(raw), digest, err)
			}
		})
	}
}
