package loops

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Operation identifies a documented provider HTTP door. It carries no audience,
// scheduling, permission, spend or delivery-completion policy.
// https://app.loops.so/openapi.json
type Operation uint8

const (
	OperationInvalid Operation = iota
	TestAPIKey
	CreateContact
	UpdateContact
	FindContact
	DeleteContact
	ReadSuppression
	RemoveSuppression
	ListContactProperties
	CreateContactProperty
	ListMailingLists
	SendEvent
	SendTransactional
	ListTransactionalEmails
	operationLimit
)

func (o Operation) Validate() error {
	if _, ok := o.door(); !ok {
		return core.ErrLoopsBinding
	}
	return nil
}
func (o Operation) IsValid() bool { return o.Validate() == nil }

// String names the operation for diagnostics; it never selects a route.
func (o Operation) String() string {
	door, ok := o.door()
	if !ok {
		return ""
	}
	return door.name
}
func (Operation) OffWireEnum() {}

// admitsIdempotencyKey reports the two doors whose OpenAPI operation declares
// the Idempotency-Key header (maximum 100 characters, reuse returns 409).
func (o Operation) admitsIdempotencyKey() bool {
	return o == SendEvent || o == SendTransactional
}

// ContactIdentifierKind selects the one query identity Loops admits for find
// and suppression doors. Exactly one kind crosses per request.
type ContactIdentifierKind uint8

const (
	ContactIdentifierKindInvalid ContactIdentifierKind = iota
	ContactIdentifierEmail
	ContactIdentifierUserID
	contactIdentifierKindLimit
)

func (k ContactIdentifierKind) Validate() error {
	if k <= ContactIdentifierKindInvalid || k >= contactIdentifierKindLimit {
		return core.ErrLoopsContract
	}
	return nil
}
func (k ContactIdentifierKind) IsValid() bool { return k.Validate() == nil }
func (k ContactIdentifierKind) String() string {
	if !k.IsValid() {
		return ""
	}
	return [...]string{"", queryEmail, queryUserID}[k]
}
func (ContactIdentifierKind) OffWireEnum() {}

// ContactIdentifier is one bounded, printable provider identity. The SDK does
// not interpret email syntax; it only owns the bytes it projects into a URL.
type ContactIdentifier struct {
	value string
	kind  ContactIdentifierKind
}

func ParseContactIdentifier(kind ContactIdentifierKind, value string) (ContactIdentifier, error) {
	candidate := ContactIdentifier{kind: kind, value: value}
	if err := candidate.Validate(); err != nil {
		return ContactIdentifier{}, err
	}
	return candidate, nil
}

func (c ContactIdentifier) Validate() error {
	if c.kind.Validate() != nil || !validProviderText(c.value, core.LoopsContactIdentifierMaximumBytes) {
		return core.ErrLoopsContract
	}
	return nil
}
func (c ContactIdentifier) Kind() ContactIdentifierKind { return c.kind }
func (c ContactIdentifier) String() string              { return c.value }
func (c ContactIdentifier) IsZero() bool                { return c == ContactIdentifier{} }

// PropertyListFilter is the closed `list` query of contact-property listing.
type PropertyListFilter uint8

const (
	PropertyListInvalid PropertyListFilter = iota
	PropertyListAll
	PropertyListCustom
	propertyListLimit
)

func (f PropertyListFilter) Validate() error {
	if f <= PropertyListInvalid || f >= propertyListLimit {
		return core.ErrLoopsContract
	}
	return nil
}
func (f PropertyListFilter) IsValid() bool { return f.Validate() == nil }
func (f PropertyListFilter) String() string {
	if !f.IsValid() {
		return ""
	}
	return [...]string{"", propertyListAllText, propertyListCustomText}[f]
}
func (PropertyListFilter) OffWireEnum() {}

// PageSize is the documented 10..50 perPage window. Zero in Page means the
// parameter is omitted and Loops applies its documented default.
type PageSize uint8

// ParsePageSize admits one documented perPage value.
func ParsePageSize(value uint8) (PageSize, error) {
	candidate := PageSize(value)
	if err := candidate.Validate(); err != nil {
		return 0, err
	}
	return candidate, nil
}

func (s PageSize) Validate() error {
	if s < core.LoopsPageSizeMinimum || s > core.LoopsPageSizeMaximum {
		return core.ErrLoopsContract
	}
	return nil
}
func (s PageSize) IsValid() bool { return s.Validate() == nil }

// String is the canonical decimal perPage spelling of an admitted size.
func (s PageSize) String() string {
	if !s.IsValid() {
		return ""
	}
	return strconv.FormatUint(uint64(s), 10)
}
func (PageSize) OffWireEnum() {}

// Cursor is one opaque provider pagination cursor from pagination.nextCursor.
type Cursor struct{ value string }

func ParseCursor(value string) (Cursor, error) {
	candidate := Cursor{value: value}
	if err := candidate.Validate(); err != nil {
		return Cursor{}, err
	}
	return candidate, nil
}

func (c Cursor) Validate() error {
	if !validVisibleASCII([]byte(c.value), 1, core.LoopsCursorMaximumBytes) {
		return core.ErrLoopsContract
	}
	return nil
}
func (c Cursor) String() string { return c.value }
func (c Cursor) IsZero() bool   { return c == Cursor{} }

// Page carries the optional perPage and cursor of a paginated listing.
type Page struct {
	Cursor Cursor
	Size   PageSize
}

func (p Page) Validate() error {
	if p.Size != 0 && p.Size.Validate() != nil || !p.Cursor.IsZero() && p.Cursor.Validate() != nil {
		return core.ErrLoopsContract
	}
	return nil
}
func (p Page) IsZero() bool { return p == Page{} }

// Route binds one operation to exactly the typed query facts it documents.
// The SDK owns host/path/method/query facts; the caller owns the typed body and
// stream bounds. Facts foreign to the operation must be zero.
type Route struct {
	Contact      ContactIdentifier
	Page         Page
	Operation    Operation
	PropertyList PropertyListFilter
}

// queryShape is the one typed query family an operation documents.
type queryShape uint8

const (
	queryShapeNone queryShape = iota
	queryShapeContact
	queryShapePropertyList
	queryShapePage
	queryShapeLimit
)

// operationDoor is the compiler-owned OpenAPI fact for one operation.
type operationDoor struct {
	name     string
	resource string
	method   exchange.Method
	query    queryShape
}

// operationDoors returns the closed operation table by value; no caller can
// mutate the provider facts another call observes.
func operationDoors() [operationLimit]operationDoor {
	return [operationLimit]operationDoor{
		TestAPIKey:              {name: "TestAPIKey", resource: "api-key", method: exchange.MethodGet},
		CreateContact:           {name: "CreateContact", resource: "contacts/create", method: exchange.MethodPost},
		UpdateContact:           {name: "UpdateContact", resource: "contacts/update", method: exchange.MethodPut},
		FindContact:             {name: "FindContact", resource: "contacts/find", method: exchange.MethodGet, query: queryShapeContact},
		DeleteContact:           {name: "DeleteContact", resource: "contacts/delete", method: exchange.MethodPost},
		ReadSuppression:         {name: "ReadSuppression", resource: "contacts/suppression", method: exchange.MethodGet, query: queryShapeContact},
		RemoveSuppression:       {name: "RemoveSuppression", resource: "contacts/suppression", method: exchange.MethodDelete, query: queryShapeContact},
		ListContactProperties:   {name: "ListContactProperties", resource: "contacts/properties", method: exchange.MethodGet, query: queryShapePropertyList},
		CreateContactProperty:   {name: "CreateContactProperty", resource: "contacts/properties", method: exchange.MethodPost},
		ListMailingLists:        {name: "ListMailingLists", resource: "lists", method: exchange.MethodGet},
		SendEvent:               {name: "SendEvent", resource: "events/send", method: exchange.MethodPost},
		SendTransactional:       {name: "SendTransactional", resource: "transactional", method: exchange.MethodPost},
		ListTransactionalEmails: {name: "ListTransactionalEmails", resource: "transactional-emails", method: exchange.MethodGet, query: queryShapePage},
	}
}

func (o Operation) door() (operationDoor, bool) {
	if o <= OperationInvalid || o >= operationLimit {
		return operationDoor{}, false
	}
	return operationDoors()[o], true
}

func (r Route) Validate() error {
	door, ok := r.Operation.door()
	if !ok {
		return core.ErrLoopsBinding
	}
	present := [queryShapeLimit]bool{
		queryShapeContact:      !r.Contact.IsZero(),
		queryShapePropertyList: r.PropertyList != PropertyListInvalid,
		queryShapePage:         !r.Page.IsZero(),
	}
	for shape := queryShapeContact; shape < queryShapeLimit; shape++ {
		if shape != door.query && present[shape] {
			return core.ErrLoopsBinding
		}
	}
	if err := door.query.validate(r); err != nil {
		return core.ErrLoopsBinding
	}
	return nil
}

// validate checks the one query family the operation documents.
func (q queryShape) validate(r Route) error {
	switch q {
	case queryShapeNone:
		return nil
	case queryShapeContact:
		return r.Contact.Validate()
	case queryShapePropertyList:
		return r.PropertyList.Validate()
	case queryShapePage:
		return r.Page.Validate()
	default:
		return core.ErrLoopsBinding
	}
}

func (r Route) Method() (exchange.Method, error) {
	if e := r.Validate(); e != nil {
		return exchange.Method(0), e
	}
	door, _ := r.Operation.door()
	return door.method, nil
}

func (r Route) Endpoint() (core.HTTPEndpoint, error) {
	if e := r.Validate(); e != nil {
		return core.HTTPEndpoint{}, e
	}
	door, _ := r.Operation.door()
	target := core.SchemeHTTPS + "://" + core.LoopsAPIHost + apiPathPrefix + door.resource
	separator := "?"
	field := func(name, value string) {
		target += separator + name + queryValueSeparator + url.QueryEscape(value)
		separator = queryFieldSeparator
	}
	switch door.query {
	case queryShapeNone:
	case queryShapeContact:
		field(r.Contact.kind.String(), r.Contact.value)
	case queryShapePropertyList:
		field(queryPropertyList, r.PropertyList.String())
	case queryShapePage:
		if r.Page.Size != 0 {
			field(queryPerPage, r.Page.Size.String())
		}
		if !r.Page.Cursor.IsZero() {
			field(queryCursor, r.Page.Cursor.value)
		}
	default:
		return core.HTTPEndpoint{}, core.ErrLoopsBinding
	}
	endpoint, err := core.ParseHTTPEndpoint(target)
	if err != nil {
		return core.HTTPEndpoint{}, bindingError(err)
	}
	return endpoint, nil
}

// routeFromTarget recovers the one Route whose canonical endpoint and method
// are exactly the caller's target. Any other spelling, query order, escape,
// duplicate or foreign field is refused rather than normalized.
func routeFromTarget(target exchange.Target, method exchange.Method) (Route, error) {
	if target == nil || target.Validate() != nil {
		return Route{}, core.ErrLoopsBinding
	}
	value := target.HTTPURL()
	resource, ok := strings.CutPrefix(value.Path, apiPathPrefix)
	if !validURL(value) || !ok {
		return Route{}, core.ErrLoopsBinding
	}
	route := Route{Operation: operationForDoor(resource, method)}
	if route.Operation == OperationInvalid {
		return Route{}, core.ErrLoopsBinding
	}
	if err := route.admitQuery(value.RawQuery); err != nil {
		return Route{}, err
	}
	canonical, err := route.Endpoint()
	if err != nil || canonical.String() != value.String() {
		return Route{}, core.ErrLoopsBinding
	}
	return route, nil
}

func operationForDoor(resource string, method exchange.Method) Operation {
	doors := operationDoors()
	for operation := TestAPIKey; operation < operationLimit; operation++ {
		if doors[operation].resource == resource && doors[operation].method == method {
			return operation
		}
	}
	return OperationInvalid
}

func (r *Route) admitQuery(raw string) error {
	if raw == "" {
		return nil
	}
	for member := range strings.SplitSeq(raw, queryFieldSeparator) {
		name, escaped, ok := strings.Cut(member, queryValueSeparator)
		if !ok {
			return core.ErrLoopsBinding
		}
		value, err := url.QueryUnescape(escaped)
		if err != nil {
			return bindingError(err)
		}
		if err := r.admitQueryField(name, value); err != nil {
			return err
		}
	}
	return nil
}

// admitQueryField decodes one field into its typed home. A repeated field is
// refused here; a field foreign to the operation is refused by Validate.
func (r *Route) admitQueryField(name, value string) error {
	switch name {
	case queryEmail:
		return r.admitContact(ContactIdentifierEmail, value)
	case queryUserID:
		return r.admitContact(ContactIdentifierUserID, value)
	case queryPropertyList:
		return r.admitPropertyList(value)
	case queryPerPage:
		return r.admitPageSize(value)
	case queryCursor:
		return r.admitCursor(value)
	default:
		return core.ErrLoopsBinding
	}
}

func (r *Route) admitContact(kind ContactIdentifierKind, value string) error {
	if !r.Contact.IsZero() {
		return core.ErrLoopsBinding
	}
	contact, err := ParseContactIdentifier(kind, value)
	if err != nil {
		return bindingError(err)
	}
	r.Contact = contact
	return nil
}

func (r *Route) admitPropertyList(value string) error {
	if r.PropertyList != PropertyListInvalid {
		return core.ErrLoopsBinding
	}
	for filter := PropertyListAll; filter < propertyListLimit; filter++ {
		if filter.String() == value {
			r.PropertyList = filter
			return nil
		}
	}
	return core.ErrLoopsBinding
}

func (r *Route) admitPageSize(value string) error {
	size, err := strconv.ParseUint(value, 10, 8)
	if r.Page.Size != 0 || err != nil || PageSize(size).Validate() != nil {
		return core.ErrLoopsBinding
	}
	r.Page.Size = PageSize(size)
	return nil
}

func (r *Route) admitCursor(value string) error {
	if !r.Page.Cursor.IsZero() {
		return core.ErrLoopsBinding
	}
	cursor, err := ParseCursor(value)
	if err != nil {
		return bindingError(err)
	}
	r.Page.Cursor = cursor
	return nil
}

// validProviderText admits nonempty, bounded UTF-8 without control characters.
func validProviderText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func (Route) loopsProtocolFact()             {}
func (ContactIdentifier) loopsProtocolFact() {}
func (Cursor) loopsProtocolFact()            {}
func (Page) loopsProtocolFact()              {}
func (operationDoor) loopsInternalFlow()     {}

var (
	_ core.Validatable = Route{}
	_ core.Validatable = ContactIdentifier{}
	_ core.Validatable = Cursor{}
	_ core.Validatable = Page{}
	_ core.Validatable = PageSize(0)
	_ core.OffWireEnum = PageSize(0)
	_ core.Validatable = Operation(0)
	_ core.OffWireEnum = Operation(0)
	_ core.Validatable = ContactIdentifierKind(0)
	_ core.OffWireEnum = ContactIdentifierKind(0)
	_ core.Validatable = PropertyListFilter(0)
	_ core.OffWireEnum = PropertyListFilter(0)
	_ protocolFact     = Route{}
	_ protocolFact     = ContactIdentifier{}
	_ protocolFact     = Cursor{}
	_ protocolFact     = Page{}
	_ internalFlow     = operationDoor{}
)
