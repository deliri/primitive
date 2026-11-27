package gmail

import (
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// SendAccess borrows a verified OAuth mailbox capability with both of the
// required scopes. It shares the issuing Access handle's secret lifetime.
type SendAccess struct{ access Access }

func (a Access) Sending() (SendAccess, error) {
	send := SendAccess{access: a}
	if err := send.Validate(); err != nil {
		return SendAccess{}, err
	}
	return send, nil
}

func (a SendAccess) Mailbox() core.OAuthEmail { return a.access.Mailbox() }

func (a SendAccess) Validate() error {
	if a.access.delegated != nil || a.access.scope != GrantReadAndSend {
		return core.ErrGmailConsent
	}
	return a.access.Validate()
}

func (SendAccess) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, core.RedactedValueText)
}

func (SendAccess) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(core.ErrGmailConsent, core.ErrJSONContract)
}

func (SendAccess) gmailCapabilityWrapper() {}
