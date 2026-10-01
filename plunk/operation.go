package plunk

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"strings"
)

// Operation identifies a documented provider HTTP door. It carries no audience,
// scheduling, permission, spend or delivery-completion policy.
type Operation uint8

const (
	OperationInvalid Operation = iota
	SendEmail
	UpsertContact
	ReadContact
	ListContacts
	UpdateContact
	DeleteContact
	CreateCampaign
	ReadCampaign
	ListCampaigns
	UpdateCampaign
	DeleteCampaign
	SendCampaign
	CancelCampaign
	ReadCampaignStats
	TestCampaign
)

// Route binds one operation to an optional provider-owned resource ID. The SDK
// owns host/path/method facts; the caller owns the typed body and stream bounds.
type Route struct {
	Operation  Operation
	ResourceID string
}

func (r Route) Validate() error {
	switch r.Operation {
	case SendEmail, UpsertContact, ListContacts, CreateCampaign, ListCampaigns:
		if r.ResourceID != "" {
			return core.ErrPlunkBinding
		}
		return nil
	case ReadContact, UpdateContact, DeleteContact, ReadCampaign, UpdateCampaign, DeleteCampaign, SendCampaign, CancelCampaign, ReadCampaignStats, TestCampaign:
		if !validResourceID(r.ResourceID) {
			return core.ErrPlunkBinding
		}
		return nil
	default:
		return core.ErrPlunkBinding
	}
}
func validResourceID(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func (r Route) Method() (exchange.Method, error) {
	if e := r.Validate(); e != nil {
		return exchange.Method(0), e
	}
	switch r.Operation {
	case ReadContact, ListContacts, ReadCampaign, ListCampaigns, ReadCampaignStats:
		return exchange.MethodGet, nil
	case UpdateContact, UpdateCampaign:
		return exchange.MethodPatch, nil
	case DeleteContact, DeleteCampaign:
		return exchange.MethodDelete, nil
	case SendEmail, UpsertContact, CreateCampaign, SendCampaign, CancelCampaign, TestCampaign:
		return exchange.MethodPost, nil
	default:
		return exchange.Method(0), core.ErrPlunkBinding
	}
}
func (r Route) Endpoint() (core.HTTPEndpoint, error) {
	if e := r.Validate(); e != nil {
		return core.HTTPEndpoint{}, e
	}
	var path string
	switch r.Operation {
	case SendEmail:
		path = "/v1/send"
	case UpsertContact, ListContacts:
		path = "/contacts"
	case ReadContact, UpdateContact, DeleteContact:
		path = "/contacts/" + r.ResourceID
	case CreateCampaign, ListCampaigns:
		path = "/campaigns"
	case ReadCampaign, UpdateCampaign, DeleteCampaign:
		path = "/campaigns/" + r.ResourceID
	case SendCampaign:
		path = "/campaigns/" + r.ResourceID + "/send"
	case CancelCampaign:
		path = "/campaigns/" + r.ResourceID + "/cancel"
	case ReadCampaignStats:
		path = "/campaigns/" + r.ResourceID + "/stats"
	case TestCampaign:
		path = "/campaigns/" + r.ResourceID + "/test"
	default:
		return core.HTTPEndpoint{}, core.ErrPlunkBinding
	}
	return core.ParseHTTPEndpoint(core.SchemeHTTPS + "://" + core.PlunkAPIHost + path)
}
func validOperationPath(path string, method exchange.Method) bool {
	// Keep the published v1 transport door for send/track/verify; the resource
	// API is deliberately closed to the documented contacts/campaign operations.
	if strings.HasPrefix(path, apiPathPrefix) {
		return method == exchange.MethodPost || method == exchange.MethodGet
	}
	rest, contact := strings.CutPrefix(path, "/contacts")
	if !contact {
		var campaign bool
		rest, campaign = strings.CutPrefix(path, "/campaigns")
		if !campaign {
			return false
		}
	}
	if rest == "" {
		return method == exchange.MethodPost || method == exchange.MethodGet
	}
	if !strings.HasPrefix(rest, "/") {
		return false
	}
	id, action, hasAction := strings.Cut(rest[1:], "/")
	if !validResourceID(id) {
		return false
	}
	if !hasAction {
		return method == exchange.MethodGet || method == exchange.MethodPatch || method == exchange.MethodDelete
	}
	if contact {
		return false
	}
	switch action {
	case "send", "cancel", "test":
		return method == exchange.MethodPost
	case "stats":
		return method == exchange.MethodGet
	default:
		return false
	}
}
func (Route) plunkProtocolFact() {}
