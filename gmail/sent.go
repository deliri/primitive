package gmail

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	sentSearchMaximum = 2
	sentMetadataQuery = "?format=metadata&metadataHeaders=From&metadataHeaders=To&metadataHeaders=Cc&metadataHeaders=Bcc&metadataHeaders=Message-ID&metadataHeaders=" + sendDigestHeader +
		"&fields=id,threadId,internalDate,labelIds,payload(headers(name,value))"
)

// SentEvidenceRequest is exact sent evidence for a frozen, known attempt.
// Use Message after an accepted response, or omit it to search by RFCID.
type SentEvidenceRequest struct {
	Access     Access
	Policy     exchange.OperationPolicy
	Message    MessageID
	Thread     ThreadID
	RFCID      SearchMessageID
	ContentSHA core.SHA256Digest
	Recipients Recipients
}

func (r SentEvidenceRequest) Validate() error {
	if r.Message != "" && (r.Message.Validate() != nil || r.Thread == "") || r.Thread != "" && r.Thread.Validate() != nil {
		return core.ErrGmailContract
	}
	if len(r.Recipients.To)+len(r.Recipients.Cc)+len(r.Recipients.Bcc) == 0 {
		return core.ErrGmailRecipient
	}
	return errors.Join(r.Access.Validate(), validatePolicy(r.Policy), r.RFCID.Validate(), r.ContentSHA.Validate(), r.Recipients.Validate())
}

// SentEvidence distinguishes a positive match from no visible match in Sent.
// A missing match is not evidence that Gmail did not accept the send.
type SentEvidence struct {
	Match   SendAcceptance
	Headers SentHeaders
}

// SentHeaderFact reports the provider's corroboration independently of the
// exact sent message coordinate. A rewritten RFC ID is usable only with the
// ID and thread returned by the send call.
type SentHeaderFact uint8

const (
	SentHeaderUnknown SentHeaderFact = iota
	SentHeaderMatched
	SentHeaderAbsent
	SentHeaderGmailRewritten
)

type SentHeaders struct {
	MessageID     SentHeaderFact `json:"message_id"`
	ContentDigest SentHeaderFact `json:"content_digest"`
}

func (h SentHeaders) Validate() error {
	if h.MessageID < SentHeaderMatched || h.MessageID > SentHeaderGmailRewritten ||
		h.ContentDigest < SentHeaderMatched || h.ContentDigest > SentHeaderAbsent {
		return core.ErrGmailResponse
	}
	return nil
}

func (e SentEvidence) Validate() error {
	if e.Match == (SendAcceptance{}) && e.Headers == (SentHeaders{}) {
		return nil
	}
	return errors.Join(e.Match.Validate(), e.Match.At.Validate(), e.Headers.Validate())
}

type messageReferenceWire struct {
	ID     MessageID `json:"id"`
	Thread ThreadID  `json:"threadId"`
}

type messageListWire struct {
	Messages []messageReferenceWire `json:"messages"`
	Next     PageToken              `json:"nextPageToken"`
}

func (c Client) ReadSentEvidence(ctx context.Context, r SentEvidenceRequest) (SentEvidence, error) {
	if err := c.admit(ctx, r); err != nil {
		return SentEvidence{}, err
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return SentEvidence{}, errors.Join(core.ErrGmailContract, err)
	}
	defer cancel()
	ref := messageReferenceWire{ID: r.Message, Thread: r.Thread}
	if ref.ID == "" {
		ref, err = c.findSent(ctx, r)
		if err != nil {
			return SentEvidence{}, sentEvidenceError{class: SentEvidenceSearch, cause: err}
		}
		if ref.ID == "" {
			return SentEvidence{}, nil
		}
	}
	path := mailboxEndpoint + "/messages/" + string(ref.ID) + sentMetadataQuery
	wire, err := readWithAccess[messageHeaderWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
	if err != nil {
		if errors.Is(err, core.ErrGmailConsent) {
			return SentEvidence{}, sentEvidenceError{class: SentEvidenceAccess, cause: err}
		}
		if errors.Is(err, core.ErrJSONContract) {
			return SentEvidence{}, sentEvidenceError{class: SentEvidenceMetadata, cause: err}
		}
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceFetch, cause: err}
	}
	return wire.sentEvidence(r, ref)
}

// Require the SENT label on both list and exact message metadata. The Gmail
// API's query example supplies the Message-ID with its angle brackets.
// https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/list
// https://support.google.com/mail/answer/7190
func (c Client) findSent(ctx context.Context, r SentEvidenceRequest) (messageReferenceWire, error) {
	q := "rfc822msgid:" + string(r.RFCID)
	path := mailboxEndpoint + "/messages?labelIds=SENT&maxResults=2&fields=messages(id,threadId),nextPageToken&q=" + url.QueryEscape(q)
	page, err := readWithAccess[messageListWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
	if err != nil {
		return messageReferenceWire{}, err
	}
	return page.project(r.Thread)
}

func (w messageListWire) project(expected ThreadID) (messageReferenceWire, error) {
	if w.Next != "" || len(w.Messages) > sentSearchMaximum {
		return messageReferenceWire{}, core.ErrGmailSendUnconfirmed
	}
	switch len(w.Messages) {
	case 0:
		return messageReferenceWire{}, nil
	case 1:
		ref := w.Messages[0]
		if ref.ID.Validate() != nil || ref.Thread.Validate() != nil || expected != "" && ref.Thread != expected {
			return messageReferenceWire{}, errors.Join(core.ErrGmailResponse, core.ErrGmailBinding)
		}
		return ref, nil
	default:
		return messageReferenceWire{}, core.ErrGmailSendUnconfirmed
	}
}

func (w messageHeaderWire) sentEvidence(r SentEvidenceRequest, ref messageReferenceWire) (SentEvidence, error) {
	if w.ID != ref.ID {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceMessage, cause: core.ErrGmailBinding}
	}
	if w.Thread != ref.Thread {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceThread, cause: core.ErrGmailBinding}
	}
	if !slices.Contains(w.Labels, labelSent) || slices.Contains(w.Labels, labelDraft) {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceSentLabel, cause: core.ErrGmailBinding}
	}
	header, err := w.project(r.Access.Mailbox())
	if err != nil {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceMetadata, cause: err}
	}
	if !strings.EqualFold(header.Sender.String(), r.Access.Mailbox().String()) {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceFrom, cause: core.ErrGmailBinding}
	}
	if !equalDestinations(header.Recipients, r.Recipients) {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceRecipients, cause: core.ErrGmailBinding}
	}
	messageID, err := w.sentMessageID(r)
	if err != nil {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceMessageIDHeader, cause: err}
	}
	digest, err := w.sentContentDigest(r)
	if err != nil {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceDigestHeader, cause: err}
	}
	out := SentEvidence{Match: SendAcceptance{Message: w.ID, Thread: w.Thread, At: header.At},
		Headers: SentHeaders{MessageID: messageID, ContentDigest: digest}}
	if err := out.Validate(); err != nil {
		return SentEvidence{}, sentEvidenceError{class: SentEvidenceMetadata, cause: errors.Join(core.ErrGmailResponse, err)}
	}
	return out, nil
}

func (w messageHeaderWire) sentMessageID(r SentEvidenceRequest) (SentHeaderFact, error) {
	value, present, err := w.findHeader("Message-ID")
	if err != nil {
		return SentHeaderUnknown, err
	}
	id := InternetMessageID(strings.TrimSpace(string(value)))
	if present && id == InternetMessageID(r.RFCID) {
		return SentHeaderMatched, nil
	}
	if r.Message != "" {
		if !present {
			return SentHeaderAbsent, nil
		}
		// The exact accepted provider coordinate already fixes identity. Only
		// recognize the known Gmail-generated form as a rewrite. An unrelated
		// RFC ID is a mismatch even on that coordinate.
		if id.Validate() == nil && strings.HasSuffix(strings.ToLower(string(id)), "@mail.gmail.com>") {
			return SentHeaderGmailRewritten, nil
		}
	}
	return SentHeaderUnknown, core.ErrGmailBinding
}

func (w messageHeaderWire) sentContentDigest(r SentEvidenceRequest) (SentHeaderFact, error) {
	value, present, err := w.findHeader(sendDigestHeader)
	if err != nil {
		return SentHeaderUnknown, err
	}
	if !present {
		return SentHeaderAbsent, nil
	}
	want, err := r.ContentSHA.Hex()
	if err != nil || strings.TrimSpace(string(value)) != want {
		return SentHeaderUnknown, core.ErrGmailBinding
	}
	return SentHeaderMatched, nil
}

func (w messageHeaderWire) oneHeader(name headerNameWire) (headerValueWire, error) {
	value, _, err := w.findHeader(name)
	return value, err
}

func (w messageHeaderWire) findHeader(name headerNameWire) (headerValueWire, bool, error) {
	var value headerValueWire
	found := false
	for _, kv := range w.Payload.Headers {
		if !strings.EqualFold(string(kv.Name), string(name)) {
			continue
		}
		if found {
			return "", false, core.ErrGmailResponse
		}
		found, value = true, kv.Value
	}
	return value, found, nil
}

func equalDestinations(got, want Recipients) bool {
	for _, pair := range [...][2][]Recipient{{got.To, want.To}, {got.Cc, want.Cc}, {got.Bcc, want.Bcc}} {
		if len(pair[0]) != len(pair[1]) {
			return false
		}
		for i, got := range pair[0] {
			if !strings.EqualFold(got.Address.String(), pair[1][i].Address.String()) {
				return false
			}
		}
	}
	return true
}

func (SentEvidenceRequest) gmailProtocolFact()  {}
func (SentEvidence) gmailSealedProjection()     {}
func (messageReferenceWire) gmailInternalFlow() {}
func (messageListWire) gmailInternalFlow()      {}

func (SentHeaders) gmailSealedProjection() {}
