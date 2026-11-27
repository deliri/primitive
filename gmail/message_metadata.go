package gmail

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const replyMetadataQuery = "?format=metadata&metadataHeaders=From&metadataHeaders=Subject&metadataHeaders=To&metadataHeaders=Cc&metadataHeaders=Bcc&metadataHeaders=Message-ID&metadataHeaders=References&metadataHeaders=In-Reply-To&fields=id,threadId,internalDate,labelIds,payload(headers(name,value))"

// MessageMetadata supplies the exact message header and, when supported by
// the parent's headers, a typed reply reference. No body is fetched.
type MessageMetadata struct {
	Thread ThreadID
	Header MessageHeader
	Reply  *ReplyReference
}

func (m MessageMetadata) Validate() error {
	if m.Reply != nil {
		if m.Reply.Thread != m.Thread || m.Reply.Parent != m.Header.ID || m.Reply.ParentSubject != m.Header.Subject {
			return core.ErrGmailBinding
		}
		if err := m.Reply.Validate(); err != nil {
			return err
		}
	}
	return errors.Join(m.Thread.Validate(), m.Header.Validate())
}

// https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/get
func (c Client) ReadMessageMetadata(ctx context.Context, r MessageReadRequest) (MessageMetadata, error) {
	if err := c.admit(ctx, r); err != nil {
		return MessageMetadata{}, err
	}
	path := mailboxEndpoint + "/messages/" + string(r.Message) + replyMetadataQuery
	wire, err := readWithAccess[messageHeaderWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
	if err != nil {
		return MessageMetadata{}, err
	}
	return wire.messageMetadata(r)
}

func (w messageHeaderWire) messageMetadata(r MessageReadRequest) (MessageMetadata, error) {
	if w.ID != r.Message || w.Thread != r.Thread || slices.Contains(w.Labels, labelDraft) {
		return MessageMetadata{}, core.ErrGmailBinding
	}
	header, err := w.project(r.Access.Mailbox())
	if err != nil {
		return MessageMetadata{}, err
	}
	out := MessageMetadata{Thread: w.Thread, Header: header}
	if reply, ok := w.parentReference(header.Subject); ok {
		out.Reply = &reply
	}
	return out, out.Validate()
}

// RFC 5322 derives References from the parent's References, or from a single
// In-Reply-To when References is absent, then appends the parent's Message-ID.
func (w messageHeaderWire) parentReference(subject Subject) (ReplyReference, bool) {
	rfc, rfcErr := w.oneHeader("Message-ID")
	raw, chainErr := w.oneHeader("References")
	if rfcErr != nil || chainErr != nil {
		return ReplyReference{}, false
	}
	chain, err := unfoldRecipientHeader(string(raw))
	if err != nil {
		return ReplyReference{}, false
	}
	fields := strings.Fields(chain)
	if len(fields) == 0 {
		rawFallback, err := w.oneHeader("In-Reply-To")
		if err != nil {
			return ReplyReference{}, false
		}
		fallback, err := unfoldRecipientHeader(string(rawFallback))
		if err != nil {
			return ReplyReference{}, false
		}
		if ids := strings.Fields(fallback); len(ids) == 1 {
			fields = ids
		}
	}
	if len(fields) >= replyReferencesMaximum {
		return ReplyReference{}, false
	}
	reply := ReplyReference{Thread: w.Thread, Parent: w.ID, ParentRFCID: InternetMessageID(rfc), ParentSubject: subject}
	for _, id := range fields {
		reply.References = append(reply.References, InternetMessageID(id))
	}
	if reply.Validate() != nil {
		return ReplyReference{}, false
	}
	return reply, true
}

func (MessageMetadata) gmailSealedProjection() {}
