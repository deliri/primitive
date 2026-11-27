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
	if r.Message != "" && r.Message.Validate() != nil || r.Thread != "" && r.Thread.Validate() != nil {
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
	Match SendAcceptance
}

func (e SentEvidence) Validate() error {
	if e.Match == (SendAcceptance{}) {
		return nil
	}
	return errors.Join(e.Match.Validate(), e.Match.At.Validate())
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
		if err != nil || ref.ID == "" {
			return SentEvidence{}, err
		}
	}
	path := mailboxEndpoint + "/messages/" + string(ref.ID) + sentMetadataQuery
	wire, err := readWithAccess[messageHeaderWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
	if err != nil {
		return SentEvidence{}, err
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
	if w.ID != ref.ID || ref.Thread != "" && w.Thread != ref.Thread || slices.Contains(w.Labels, labelDraft) {
		return SentEvidence{}, core.ErrGmailBinding
	}
	header, err := w.project(r.Access.Mailbox())
	if err != nil {
		return SentEvidence{}, err
	}
	rfc, rfcErr := w.oneHeader("Message-ID")
	digest, digestErr := w.oneHeader(sendDigestHeader)
	wantDigest, _ := r.ContentSHA.Hex()
	if err := errors.Join(rfcErr, digestErr); err != nil {
		return SentEvidence{}, err
	}
	if !header.Sent || !strings.EqualFold(header.Sender.String(), r.Access.Mailbox().String()) ||
		rfc != headerValueWire(r.RFCID) || digest != headerValueWire(wantDigest) || !equalDestinations(header.Recipients, r.Recipients) {
		return SentEvidence{}, core.ErrGmailBinding
	}
	out := SentEvidence{Match: SendAcceptance{Message: w.ID, Thread: w.Thread, At: header.At}}
	if err := out.Validate(); err != nil {
		return SentEvidence{}, errors.Join(core.ErrGmailResponse, err)
	}
	return out, nil
}

func (w messageHeaderWire) oneHeader(name headerNameWire) (headerValueWire, error) {
	var value headerValueWire
	found := false
	for _, kv := range w.Payload.Headers {
		if !strings.EqualFold(string(kv.Name), string(name)) {
			continue
		}
		if found {
			return "", core.ErrGmailResponse
		}
		found, value = true, kv.Value
	}
	return value, nil
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
