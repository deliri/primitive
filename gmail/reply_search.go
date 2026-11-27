package gmail

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	recentReplySearchMaximum = 5
	recentReplyFields        = "?format=metadata&metadataHeaders=From&fields=id,threadId,internalDate,labelIds,payload(headers(name,value))"
)

// RecentReplyRequest checks for an exact contact's off-thread reply after a
// verified earlier touch. It does not disclose candidate IDs or message data.
type RecentReplyRequest struct {
	Access  Access
	Policy  exchange.OperationPolicy
	Contact core.OAuthEmail
	Thread  ThreadID
	After   temporal.Instant
}

func (r RecentReplyRequest) Validate() error {
	if err := errors.Join(r.Access.Validate(), validatePolicy(r.Policy), r.Contact.Validate(), r.Thread.Validate(), r.After.Validate()); err != nil {
		return err
	}
	for _, c := range r.Contact.String() {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune(".@_+-", c)) {
			return core.ErrGmailRecipient
		}
	}
	return nil
}

type RecentReply uint8

const (
	RecentReplyNone RecentReply = iota + 1
	RecentReplyFound
)

func (r RecentReply) Validate() error {
	if r != RecentReplyNone && r != RecentReplyFound {
		return core.ErrGmailContract
	}
	return nil
}

// https://developers.google.com/workspace/gmail/api/guides/filtering
func (c Client) SearchRecentReply(ctx context.Context, r RecentReplyRequest) (RecentReply, error) {
	if err := c.admit(ctx, r); err != nil {
		return 0, err
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return 0, errors.Join(core.ErrGmailContract, err)
	}
	defer cancel()
	after, err := r.After.Time()
	if err != nil {
		return 0, errors.Join(core.ErrGmailContract, err)
	}
	// Search back one second to include messages in the same second as After.
	// The exact metadata date then decides whether a message is newer.
	q := "from:" + r.Contact.String() + " after:" + strconv.FormatInt(after.Unix()-1, 10)
	path := mailboxEndpoint + "/messages?maxResults=" + strconv.Itoa(recentReplySearchMaximum) +
		"&fields=messages(id,threadId),nextPageToken&q=" + url.QueryEscape(q)
	page, err := readWithAccess[messageListWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
	if err != nil {
		return 0, err
	}
	if err := page.validateRecentReplyCandidates(); err != nil {
		return 0, err
	}
	for _, ref := range page.Messages {
		if ref.Thread == r.Thread {
			continue
		}
		path := mailboxEndpoint + "/messages/" + string(ref.ID) + recentReplyFields
		header, err := readWithAccess[messageHeaderWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
		if err != nil {
			return 0, err
		}
		matches, err := header.recentReply(r, ref)
		if err != nil {
			return 0, err
		}
		if matches {
			return RecentReplyFound, nil
		}
	}
	if page.Next != "" {
		return 0, errors.Join(core.ErrGmailResponse, core.ErrExchangeBodyLimit)
	}
	return RecentReplyNone, nil
}

func (w messageListWire) validateRecentReplyCandidates() error {
	if len(w.Messages) > recentReplySearchMaximum || w.Next.Validate() != nil {
		return core.ErrGmailResponse
	}
	var seen [recentReplySearchMaximum]MessageID
	for i, ref := range w.Messages {
		if ref.Thread.Validate() != nil || ref.ID.Validate() != nil || duplicate(seen[:i], ref.ID) {
			return core.ErrGmailResponse
		}
		seen[i] = ref.ID
	}
	return nil
}

func (w messageHeaderWire) recentReply(r RecentReplyRequest, ref messageReferenceWire) (bool, error) {
	if w.Thread != ref.Thread || w.ID != ref.ID {
		return false, core.ErrGmailBinding
	}
	if slices.Contains(w.Labels, labelDraft) || slices.Contains(w.Labels, labelSent) {
		return false, nil
	}
	given, err := w.project(r.Access.Mailbox())
	if err != nil {
		return false, err
	}
	order, err := given.At.Compare(r.After)
	return order == core.ComparisonGreater && strings.EqualFold(given.Sender.String(), r.Contact.String()), err
}

func (RecentReplyRequest) gmailProtocolFact() {}
