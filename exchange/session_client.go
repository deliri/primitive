package exchange

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
)

// SessionClientRequest supplies Go's cookie-domain authority. The caller owns
// the list's correctness, updates, and concurrent-use safety, as required by
// cookiejar.PublicSuffixList. Exchange neither embeds a domain database nor
// substitutes an insecure nil list. Validate checks capability presence; it
// cannot establish the truth of a caller's domain authority.
type SessionClientRequest struct {
	PublicSuffixList cookiejar.PublicSuffixList
}

func (r SessionClientRequest) Validate() error {
	if r.PublicSuffixList == nil {
		return core.ErrExchangeContract
	}
	value := reflect.ValueOf(r.PublicSuffixList)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return core.ErrExchangeContract
		}
	}
	return nil
}

// NewSessionClient creates an independent Go cookie jar and a standard HTTP
// client. Go owns cookie matching, expiration, storage, and synchronization;
// Exchange owns the operation's timing, replay, redirect, and body limits.
func NewSessionClient(request SessionClientRequest) (Client, error) {
	if err := request.Validate(); err != nil {
		return Client{}, err
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: request.PublicSuffixList})
	if err != nil {
		return Client{}, errors.Join(core.ErrExchangeContract, err)
	}
	return NewClient(&http.Client{Jar: jar})
}

var _ core.Validatable = SessionClientRequest{}
