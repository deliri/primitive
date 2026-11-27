package gmail

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// RecipientsMaximum is the total number of mailboxes across To, Cc and Bcc.
	RecipientsMaximum            = 25
	RecipientHeaderMaximumBytes  = 4096
	recipientAddressMaximumBytes = 254
	recipientNameMaximumBytes    = 256
)

type RecipientName string

type Recipient struct {
	Address core.OAuthEmail
	Name    RecipientName
}

func (r Recipient) Validate() error {
	if len(r.Address.String()) > recipientAddressMaximumBytes || r.Address.Validate() != nil ||
		len(r.Name) > recipientNameMaximumBytes || !utf8.ValidString(string(r.Name)) ||
		strings.ContainsFunc(string(r.Name), unicode.IsControl) {
		return core.ErrGmailResponse
	}
	return nil
}

type Recipients struct {
	To  []Recipient
	Cc  []Recipient
	Bcc []Recipient
}

func (r Recipients) Validate() error {
	if len(r.To)+len(r.Cc)+len(r.Bcc) > RecipientsMaximum {
		return recipientExtentError()
	}
	for _, list := range [...][]Recipient{r.To, r.Cc, r.Bcc} {
		for _, recipient := range list {
			if err := recipient.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func recipientExtentError() error {
	return errors.Join(core.ErrGmailResponse, core.ErrExchangeBodyLimit)
}

// RFC 5322 §§2.2.3, 3.4 and 3.6.3 define unfolding, address lists and
// destination fields. net/mail owns the grammar and expands groups into
// their listed mailboxes. RFC 2047 encoded words can supply display names.
// https://www.rfc-editor.org/rfc/rfc5322
// https://www.rfc-editor.org/rfc/rfc2047
func parseRecipientHeader(raw headerValueWire, remaining int, allowEmpty bool) ([]Recipient, error) {
	if len(raw) > RecipientHeaderMaximumBytes {
		return nil, recipientExtentError()
	}
	unfolded, err := unfoldRecipientHeader(string(raw))
	if err != nil {
		return nil, err
	}
	if allowEmpty && strings.Trim(unfolded, " \t") == "" {
		return nil, nil
	}
	parsed, err := mail.ParseAddressList(unfolded)
	if err != nil {
		// The standard library can put address text in its diagnostics.
		return nil, core.ErrGmailResponse
	}
	if len(parsed) > remaining {
		return nil, recipientExtentError()
	}
	var recipients []Recipient
	for _, address := range parsed {
		email, err := core.ParseOAuthEmail(address.Address)
		if err != nil {
			return nil, core.ErrGmailResponse
		}
		recipient := Recipient{Address: email, Name: RecipientName(address.Name)}
		if err := recipient.Validate(); err != nil {
			return nil, err
		}
		recipients = append(recipients, recipient)
	}
	return recipients, nil
}

func unfoldRecipientHeader(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", core.ErrGmailResponse
	}
	unfolded := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n ", " "), "\r\n\t", "\t")
	if strings.ContainsFunc(unfolded, func(r rune) bool { return r != '\t' && unicode.IsControl(r) }) {
		return "", core.ErrGmailResponse
	}
	return unfolded, nil
}
