package jsonio

import "github.com/deliri/primitive/v2026/core"

// MemberPolicy controls ownership of fields in a typed object. The zero value
// keeps owned protocols closed. Foreign formats must explicitly opt into
// ignoring members outside the declared Go struct; known fields, duplicate
// names, UTF-8, null admission and Document.Validate remain checked.
// Source: https://pkg.go.dev/encoding/json/v2#RejectUnknownMembers
type MemberPolicy uint8

const (
	MembersDeclaredOnly MemberPolicy = iota
	MembersIgnoreExtra
	memberPolicyLimit
)

func (p MemberPolicy) Validate() error {
	if p >= memberPolicyLimit {
		return core.ErrJSONContract
	}
	return nil
}

func (p MemberPolicy) IsValid() bool { return p.Validate() == nil }
func (MemberPolicy) OffWireEnum()    {}
func (p MemberPolicy) String() string {
	if !p.IsValid() {
		return "unknown"
	}
	return [...]string{"declared only", "ignore extra"}[p]
}
