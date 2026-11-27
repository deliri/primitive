package gmail

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	HistoryRecordMaximum = 25
	HistoryEventMaximum  = 50
)

// HistoryID is an opaque positive decimal Gmail history coordinate. IDs
// increase over time but may have gaps.
// https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.history/list
type HistoryID string

func (i HistoryID) Validate() error {
	n, err := strconv.ParseUint(string(i), 10, 64)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != string(i) {
		return core.ErrGmailContract
	}
	return nil
}

func (i HistoryID) after(j HistoryID) bool {
	return len(i) > len(j) || len(i) == len(j) && i > j
}

type HistoryCursorRequest struct {
	Access Access
	Policy exchange.OperationPolicy
}

func (r HistoryCursorRequest) Validate() error {
	return errors.Join(r.Access.Validate(), validatePolicy(r.Policy))
}

type HistoryListRequest struct {
	Access Access
	Policy exchange.OperationPolicy
	Since  HistoryID
	Cursor PageToken
	Limit  uint8
}

func (r HistoryListRequest) Validate() error {
	if r.Limit == 0 || r.Limit > HistoryRecordMaximum {
		return core.ErrGmailContract
	}
	return errors.Join(r.Access.Validate(), validatePolicy(r.Policy), r.Since.Validate(), r.Cursor.Validate())
}

type HistoryAddition struct {
	Message MessageID
	Thread  ThreadID
}

func (a HistoryAddition) Validate() error {
	return errors.Join(a.Message.Validate(), a.Thread.Validate())
}

// HistoryPage supplies message additions and either a continuation or a
// completed watermark. A partial page never returns an advanceable watermark.
type HistoryPage struct {
	Added   []HistoryAddition
	Next    PageToken
	Through HistoryID
}

func (p HistoryPage) Validate() error {
	if len(p.Added) > HistoryEventMaximum || (p.Next == "") == (p.Through == "") {
		return core.ErrGmailResponse
	}
	for _, event := range p.Added {
		if err := event.Validate(); err != nil {
			return err
		}
	}
	if p.Next == "" {
		return p.Through.Validate()
	}
	return p.Next.Validate()
}

type historyProfileWire struct {
	Email  mailAddressWire `json:"emailAddress"`
	Cursor HistoryID       `json:"historyId"`
}

type historyPageWire struct {
	History []historyRecordWire `json:"history"`
	ID      HistoryID           `json:"historyId"`
	Next    PageToken           `json:"nextPageToken"`
}

type historyRecordWire struct {
	ID    HistoryID         `json:"id"`
	Added []historyItemWire `json:"messagesAdded"`
}

type historyItemWire struct {
	Message messageReferenceWire `json:"message"`
}

func (c Client) ReadHistoryCursor(ctx context.Context, r HistoryCursorRequest) (HistoryID, error) {
	if err := c.admit(ctx, r); err != nil {
		return "", err
	}
	path := mailboxEndpoint + "/profile?fields=emailAddress,historyId"
	wire, err := readWithAccess[historyProfileWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
	if err != nil {
		return "", err
	}
	return wire.project(r.Access.Mailbox())
}

func (w historyProfileWire) project(expected core.OAuthEmail) (HistoryID, error) {
	mailbox, err := core.ParseOAuthEmail(string(w.Email))
	if err != nil || !strings.EqualFold(mailbox.String(), expected.String()) {
		return "", core.ErrGmailBinding
	}
	if err := w.Cursor.Validate(); err != nil {
		return "", errors.Join(core.ErrGmailResponse, err)
	}
	return w.Cursor, nil
}

func (c Client) ListHistory(ctx context.Context, r HistoryListRequest) (HistoryPage, error) {
	if err := c.admit(ctx, r); err != nil {
		return HistoryPage{}, err
	}
	path := mailboxEndpoint + "/history?historyTypes=messageAdded&fields=history(id,messagesAdded(message(id,threadId))),historyId,nextPageToken&startHistoryId=" +
		string(r.Since) + "&maxResults=" + strconv.Itoa(int(r.Limit))
	if r.Cursor != "" {
		path += "&pageToken=" + url.QueryEscape(string(r.Cursor))
	}
	wire, err := readWithAccess[historyPageWire](ctx, r.Access, mailboxRead{client: c, policy: r.Policy, path: path})
	if err != nil {
		var status exchange.StatusError
		if errors.As(err, &status) {
			code, _ := status.Status().Int()
			if code == http.StatusNotFound {
				return HistoryPage{}, core.ErrGmailHistoryResetRequired
			}
		}
		if errors.Is(err, core.ErrExchangeBodyLimit) {
			return HistoryPage{}, errors.Join(core.ErrGmailHistoryResetRequired, core.ErrExchangeBodyLimit)
		}
		return HistoryPage{}, err
	}
	return wire.project(r)
}

func (w historyPageWire) project(r HistoryListRequest) (HistoryPage, error) {
	if len(w.History) > int(r.Limit) || w.ID.Validate() != nil || r.Since.after(w.ID) || w.Next != "" && r.Cursor == w.Next {
		return HistoryPage{}, core.ErrGmailResponse
	}
	page := HistoryPage{Next: w.Next}
	if w.Next == "" {
		page.Through = w.ID
	}
	last := r.Since
	for _, record := range w.History {
		if record.ID.Validate() != nil || !record.ID.after(last) || record.ID.after(w.ID) {
			return HistoryPage{}, core.ErrGmailResponse
		}
		if len(record.Added) > HistoryEventMaximum-len(page.Added) {
			return HistoryPage{}, errors.Join(core.ErrGmailHistoryResetRequired, core.ErrExchangeBodyLimit)
		}
		last = record.ID
		for _, item := range record.Added {
			page.Added = append(page.Added, HistoryAddition{Message: item.Message.ID, Thread: item.Message.Thread})
		}
	}
	if err := page.Validate(); err != nil {
		return HistoryPage{}, errors.Join(core.ErrGmailResponse, err)
	}
	return page, nil
}

func (HistoryCursorRequest) gmailProtocolFact() {}
func (HistoryListRequest) gmailProtocolFact()   {}
func (HistoryAddition) gmailSealedProjection()  {}
func (HistoryPage) gmailSealedProjection()      {}
func (historyProfileWire) gmailInternalFlow()   {}
func (historyPageWire) gmailInternalFlow()      {}
func (historyRecordWire) gmailInternalFlow()    {}
func (historyItemWire) gmailInternalFlow()      {}
