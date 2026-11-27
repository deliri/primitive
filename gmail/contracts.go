package gmail

import (
	"errors"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	ThreadPageMaximum       = 25
	ThreadMessageMaximum    = 100
	MessageTextMaximumBytes = 16 * 1024
	idMaximumBytes          = 128
	cursorMaximumBytes      = 512
	subjectMaximumBytes     = 998
)

type ThreadID string
type MessageID string
type PageToken string
type Subject string
type PlainText string

func (id ThreadID) Validate() error  { return validateID(string(id)) }
func (id MessageID) Validate() error { return validateID(string(id)) }
func (t PageToken) Validate() error {
	if len(t) > cursorMaximumBytes {
		return core.ErrGmailContract
	}
	for _, c := range t {
		if c < '!' || c > '~' {
			return core.ErrGmailContract
		}
	}
	return nil
}

func validateID(value string) error {
	if len(value) == 0 || len(value) > idMaximumBytes {
		return core.ErrGmailContract
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return core.ErrGmailContract
		}
	}
	return nil
}

func (s Subject) Validate() error {
	if len(s) > subjectMaximumBytes || !utf8.ValidString(string(s)) {
		return core.ErrGmailResponse
	}
	for _, c := range s {
		if c < ' ' && c != '\t' || c == 127 {
			return core.ErrGmailResponse
		}
	}
	return nil
}

type ThreadFilter uint8

const (
	ThreadFilterInvalid ThreadFilter = iota
	ThreadFilterInbox
	ThreadFilterUnread
)

func (ThreadFilter) OffWireEnum()    {}
func (f ThreadFilter) IsValid() bool { return f == ThreadFilterInbox || f == ThreadFilterUnread }
func (f ThreadFilter) Validate() error {
	if !f.IsValid() {
		return core.ErrGmailContract
	}
	return nil
}

type BodyKind uint8

const (
	BodyInvalid BodyKind = iota
	BodyUnavailable
	BodyPlainText
)

func (BodyKind) OffWireEnum()    {}
func (k BodyKind) IsValid() bool { return k == BodyUnavailable || k == BodyPlainText }
func (k BodyKind) Validate() error {
	if !k.IsValid() {
		return core.ErrGmailResponse
	}
	return nil
}

type ThreadListRequest struct {
	Access Access
	Policy exchange.OperationPolicy
	Filter ThreadFilter
	Limit  uint8
	After  PageToken
}

type ThreadReadRequest struct {
	Access Access
	Policy exchange.OperationPolicy
	Thread ThreadID
}

type MessageReadRequest struct {
	Access  Access
	Policy  exchange.OperationPolicy
	Thread  ThreadID
	Message MessageID
}

func validatePolicy(p exchange.OperationPolicy) error {
	if p.Retry.MaximumAttempts != 1 || p.Redirect.Mode != exchange.RedirectReject {
		return core.ErrGmailContract
	}
	return p.Validate()
}

func (r ThreadListRequest) Validate() error {
	if r.Limit == 0 || r.Limit > ThreadPageMaximum {
		return core.ErrGmailContract
	}
	return errors.Join(r.Access.Validate(), validatePolicy(r.Policy), r.Filter.Validate(), r.After.Validate())
}

func (r ThreadReadRequest) Validate() error {
	return errors.Join(r.Access.Validate(), validatePolicy(r.Policy), r.Thread.Validate())
}

func (r MessageReadRequest) Validate() error {
	return errors.Join(r.Access.Validate(), validatePolicy(r.Policy), r.Thread.Validate(), r.Message.Validate())
}

type ThreadPage struct {
	Threads []ThreadID
	Next    PageToken
}

func (p ThreadPage) Validate() error {
	if len(p.Threads) > ThreadPageMaximum || len(p.Threads) == 0 && p.Next != "" {
		return core.ErrGmailResponse
	}
	for i, id := range p.Threads {
		if err := id.Validate(); err != nil || duplicate(p.Threads[:i], id) {
			return errors.Join(core.ErrGmailResponse, err)
		}
	}
	return p.Next.Validate()
}

type Thread struct {
	ID       ThreadID
	Messages []MessageHeader
}

type MessageHeader struct {
	ID         MessageID
	Sender     core.OAuthEmail
	Subject    Subject
	At         temporal.Instant
	Unread     bool
	Sent       bool
	Recipients Recipients
}

func (m MessageHeader) Validate() error {
	if m.Sender != (core.OAuthEmail{}) {
		if err := m.Sender.Validate(); err != nil {
			return errors.Join(core.ErrGmailResponse, err)
		}
	}
	return errors.Join(m.ID.Validate(), m.Subject.Validate(), m.At.Validate(), m.Recipients.Validate())
}

func (t Thread) Validate() error {
	if len(t.Messages) > ThreadMessageMaximum {
		return core.ErrGmailResponse
	}
	if err := t.ID.Validate(); err != nil {
		return err
	}
	var ids [ThreadMessageMaximum]MessageID
	for i, message := range t.Messages {
		if err := message.Validate(); err != nil || duplicate(ids[:i], message.ID) {
			return errors.Join(core.ErrGmailResponse, err)
		}
		ids[i] = message.ID
	}
	return nil
}

func duplicate[T comparable](seen []T, id T) bool {
	for _, item := range seen {
		if item == id {
			return true
		}
	}
	return false
}

type Message struct {
	ID     MessageID
	Thread ThreadID
	Kind   BodyKind
	Text   PlainText
}

func (m Message) Validate() error {
	if (m.Kind == BodyUnavailable && m.Text != "") || len(m.Text) > MessageTextMaximumBytes || !utf8.ValidString(string(m.Text)) {
		return core.ErrGmailResponse
	}
	return errors.Join(m.ID.Validate(), m.Thread.Validate(), m.Kind.Validate())
}
