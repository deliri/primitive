package gmail

import (
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	SendRecipientsMaximum    = 500
	SendPartsMaximumBytes    = 128 << 10
	SendPDFMaximumBytes      = 20 << 20
	SendPDFMaximumCount      = 6
	SendMediaMaximumBytes    = 36_700_160
	replyReferencesMaximum   = 32
	internetMessageIDMaximum = 254
	sendFilenameMaximum      = 168
)

// InternetMessageID is an RFC 5322 Message-ID using unquoted dot-atoms.
// It is distinct from the opaque Gmail message ID.
type InternetMessageID string

func (id InternetMessageID) Validate() error {
	raw := string(id)
	if len(raw) < 5 || len(raw) > internetMessageIDMaximum || raw[0] != '<' || raw[len(raw)-1] != '>' {
		return core.ErrGmailContract
	}
	local, domain, ok := strings.Cut(raw[1:len(raw)-1], "@")
	if !ok || !messageIDAtom(local) || !messageIDAtom(domain) {
		return core.ErrGmailContract
	}
	return nil
}

// SearchMessageID admits only operators-safe IDs for a new outgoing message.
// Reply headers may carry the broader RFC 5322 dot-atom grammar.
type SearchMessageID InternetMessageID

func (id SearchMessageID) Validate() error {
	if err := InternetMessageID(id).Validate(); err != nil {
		return err
	}
	for _, c := range id[1 : len(id)-1] {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune(".@_-", c)) {
			return core.ErrGmailContract
		}
	}
	return nil
}

func messageIDAtom(value string) bool {
	if value == "" || value[0] == '.' || value[len(value)-1] == '.' || strings.Contains(value, "..") {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' ||
			strings.ContainsRune("!#$%&'*+-/=?^_{|}~.", char) || char == 96) {
			return false
		}
	}
	return true
}

// ReplyReference is the admitted parent of a reply. References is the parent
// chain, without the parent ID itself. Gmail requires the matching Subject.
type ReplyReference struct {
	Thread        ThreadID
	Parent        MessageID
	ParentRFCID   InternetMessageID
	ParentSubject Subject
	References    []InternetMessageID
}

func (r ReplyReference) Validate() error {
	if len(r.References) >= replyReferencesMaximum || r.Thread.Validate() != nil || r.Parent.Validate() != nil ||
		r.ParentRFCID.Validate() != nil || r.ParentSubject.Validate() != nil {
		return core.ErrGmailContract
	}
	for i, id := range r.References {
		if id.Validate() != nil || id == r.ParentRFCID || duplicate(r.References[:i], id) {
			return core.ErrGmailContract
		}
	}
	return nil
}

type PDFName string

// SendPDF borrows exactly one previously verified PDF stream. Prepare consumes
// the declared extent, checks its digest and EOF, and leaves closing to its owner.
type SendPDF struct {
	Name   PDFName
	Size   core.ByteLength
	SHA256 core.SHA256Digest
	Source io.Reader
}

func (a SendPDF) Validate() error {
	name := string(a.Name)
	if a.Size.Uint64() > SendPDFMaximumBytes {
		return core.ErrExchangeBodyLimit
	}
	if len(name) < 5 || len(name) > sendFilenameMaximum || !strings.HasSuffix(strings.ToLower(name), ".pdf") ||
		strings.ContainsAny(name, "/\\") || strings.ContainsFunc(name, unicode.IsControl) || !utf8.ValidString(name) ||
		a.Size.Validate() != nil || a.Size.Uint64() < 5 || a.SHA256.Validate() != nil || a.Source == nil {
		return core.ErrGmailContract
	}
	return nil
}

type HTMLText string

type MessagePrepareRequest struct {
	From       Recipient
	Recipients Recipients
	Subject    Subject
	Text       PlainText
	HTML       HTMLText
	RFCID      SearchMessageID
	At         temporal.Instant
	ContentSHA core.SHA256Digest
	Reply      *ReplyReference
	PDFs       []SendPDF
}

func (r MessagePrepareRequest) Validate() error {
	if len(r.Text) > SendPartsMaximumBytes-len(r.HTML) {
		return core.ErrExchangeBodyLimit
	}
	if strings.TrimSpace(string(r.Text)) == "" || strings.TrimSpace(string(r.HTML)) == "" ||
		!utf8.ValidString(string(r.Text)) || !utf8.ValidString(string(r.HTML)) ||
		r.Subject == "" || r.Subject.Validate() != nil || r.At.Validate() != nil {
		return core.ErrGmailContract
	}
	if err := errors.Join(r.RFCID.Validate(), r.ContentSHA.Validate(), validateSendRecipients(r.From, r.Recipients)); err != nil {
		return err
	}
	if r.Reply != nil && (r.Reply.Validate() != nil || r.Subject != r.Reply.ParentSubject) {
		return core.ErrGmailBinding
	}
	return r.validatePDFs()
}

func (r MessagePrepareRequest) validatePDFs() error {
	pdfs := r.PDFs
	if len(pdfs) > SendPDFMaximumCount {
		return core.ErrExchangeBodyLimit
	}
	var total uint64
	for index, pdf := range pdfs {
		if err := pdf.Validate(); err != nil {
			return err
		}
		total += pdf.Size.Uint64()
		if total > SendPDFMaximumBytes {
			return core.ErrExchangeBodyLimit
		}
		for _, previous := range pdfs[:index] {
			if strings.EqualFold(string(previous.Name), string(pdf.Name)) {
				return core.ErrGmailContract
			}
		}
	}
	return nil
}

func validateSendRecipients(from Recipient, recipients Recipients) error {
	count := len(recipients.To) + len(recipients.Cc) + len(recipients.Bcc)
	if count == 0 || count > SendRecipientsMaximum || !validSendRecipient(from) {
		return core.ErrGmailRecipient
	}
	seen := make(map[string]struct{}, count)
	for _, list := range [...][]Recipient{recipients.To, recipients.Cc, recipients.Bcc} {
		for _, recipient := range list {
			if !validSendRecipient(recipient) {
				return core.ErrGmailRecipient
			}
			key := strings.ToLower(recipient.Address.String())
			if _, found := seen[key]; found {
				return core.ErrGmailRecipient
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func validSendRecipient(value Recipient) bool {
	if value.Validate() != nil {
		return false
	}
	addr := value.Address.String()
	for _, char := range addr {
		if char <= ' ' || char >= 127 {
			return false
		}
	}
	parsed, err := mail.ParseAddress(addr)
	return err == nil && parsed.Address == addr && parsed.Name == ""
}

// PreparedMessage can only be made by the typed MIME preparation operation.
type PreparedMessage struct {
	raw    []byte
	from   core.OAuthEmail
	thread ThreadID
	digest core.SHA256Digest
}

func (m PreparedMessage) Digest() core.SHA256Digest { return m.digest }

func (m PreparedMessage) Extent() int { return len(m.raw) }

func (PreparedMessage) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, core.RedactedValueText)
}

func (PreparedMessage) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(core.ErrGmailContract, core.ErrJSONContract)
}

func (m PreparedMessage) Validate() error {
	if len(m.raw) == 0 || len(m.raw) > SendMediaMaximumBytes {
		return core.ErrGmailContract
	}
	return errors.Join(m.from.Validate(), m.digest.Validate())
}

type MessageSendRequest struct {
	Access   SendAccess
	Policy   exchange.OperationPolicy
	Prepared PreparedMessage
}

func (r MessageSendRequest) Validate() error {
	if err := errors.Join(r.Access.Validate(), validatePolicy(r.Policy), r.Prepared.Validate()); err != nil {
		return err
	}
	if !strings.EqualFold(r.Access.Mailbox().String(), r.Prepared.from.String()) {
		return core.ErrGmailBinding
	}
	return nil
}

// SendAcceptance is the observed Gmail reply. Gmail can omit internalDate on
// send; an unset At then requires an exact read before recording sent time.
type SendAcceptance struct {
	Message MessageID
	Thread  ThreadID
	At      temporal.Instant
}

func (m SendAcceptance) Validate() error {
	return errors.Join(m.Message.Validate(), m.Thread.Validate())
}

func (ReplyReference) gmailProtocolFact()        {}
func (SendPDF) gmailProtocolFact()               {}
func (MessagePrepareRequest) gmailProtocolFact() {}
func (MessageSendRequest) gmailProtocolFact()    {}
func (PreparedMessage) gmailCapabilityWrapper()  {}
func (SendAcceptance) gmailSealedProjection()    {}
