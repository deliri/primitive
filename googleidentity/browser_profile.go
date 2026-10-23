package googleidentity

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

const (
	BrowserPhoneScope        = "https://www.googleapis.com/auth/user.phonenumbers.read"
	BrowserBirthdayScope     = "https://www.googleapis.com/auth/user.birthday.read"
	peopleSelfResource       = "people/me"
	peopleProfileSource      = "READ_SOURCE_TYPE_PROFILE"
	peopleFieldSourceProfile = "PROFILE"
)

// BrowserProfileRequest is application-selected optional profile access. Basic
// identity never implies consent to read a private phone number or birthday.
type BrowserProfileRequest struct {
	Name        bool
	PhoneNumber bool
	Birthday    bool
}

func (BrowserProfileRequest) Validate() error { return nil }

// PhoneE164 preserves Google's canonical international phone observation.
// It proves syntax, not current possession or application account ownership.
type PhoneE164 struct{ value string }

func ParsePhoneE164(text string) (PhoneE164, error) {
	phone := PhoneE164{value: text}
	if err := phone.Validate(); err != nil {
		return PhoneE164{}, err
	}
	return phone, nil
}

func (p PhoneE164) Validate() error {
	// E.164 permits at most fifteen decimal digits, including the country code.
	// Source: https://www.itu.int/rec/T-REC-E.164
	if len(p.value) < 2 || len(p.value) > 16 || p.value[0] != '+' || p.value[1] < '1' || p.value[1] > '9' {
		return core.ErrGoogleIdentityContract
	}
	for i := 2; i < len(p.value); i++ {
		if p.value[i] < '0' || p.value[i] > '9' {
			return core.ErrGoogleIdentityContract
		}
	}
	return nil
}

func (p PhoneE164) String() string { return p.value }

// BrowserProfile contains optional provider observations, never authorization
// to replace an existing account's profile. A partial birthday has no guessed
// year. The owning product decides which missing details need completion.
type BrowserProfile struct {
	FullName        core.OAuthProfileName
	GivenName       core.OAuthProfileName
	FamilyName      core.OAuthProfileName
	PhoneNumber     PhoneE164
	PictureURL      core.HTTPEndpoint
	PictureOutcome  core.OAuthProfileOutcome
	Birthday        temporal.CivilDate
	NameOutcome     core.OAuthProfileOutcome
	PhoneOutcome    core.OAuthProfileOutcome
	BirthdayOutcome core.OAuthProfileOutcome
	PhoneVerified   bool
	refusal         error
	nameIssues      browserNameIssues
}

// browserProfileCredential binds the optional authenticated-user response to
// the already signature-verified subject. The bearer never leaves this SDK.
type browserProfileCredential struct {
	Token   *oauth2.Token
	Subject core.OAuthSubject
}

// ObservationError preserves a sanitized native optional-import failure for the
// auth boundary. Optional metadata failure does not revoke a verified subject.
func (p BrowserProfile) ObservationError() error { return p.refusal }

func (p BrowserProfile) Validate() error {
	if err := errors.Join(p.NameOutcome.Validate(), p.PhoneOutcome.Validate(), p.BirthdayOutcome.Validate(), p.PictureOutcome.Validate()); err != nil {
		return contractError(err)
	}
	for _, name := range [...]core.OAuthProfileName{p.FullName, p.GivenName, p.FamilyName} {
		if name != (core.OAuthProfileName{}) {
			if err := name.Validate(); err != nil {
				return contractError(err)
			}
		}
	}
	switch p.NameOutcome {
	case core.OAuthProfileAvailable:
		if p.FullName == (core.OAuthProfileName{}) {
			return core.ErrGoogleIdentityContract
		}
	case core.OAuthProfileNotRequested, core.OAuthProfileMissing, core.OAuthProfileDenied, core.OAuthProfileUnavailable:
		if p.FullName != (core.OAuthProfileName{}) || p.GivenName != (core.OAuthProfileName{}) || p.FamilyName != (core.OAuthProfileName{}) {
			return core.ErrGoogleIdentityContract
		}
	case core.OAuthProfilePartial, core.OAuthProfileInvalid:
		// Safe partial name components remain data; absence is also possible.
	default:
		return core.ErrGoogleIdentityContract
	}
	if p.PictureOutcome == core.OAuthProfileAvailable {
		if err := p.PictureURL.Validate(); err != nil {
			return contractError(err)
		}
		if p.PictureURL.HTTPURL().Scheme != core.SchemeHTTPS {
			return core.ErrGoogleIdentityContract
		}
	} else if p.PictureURL != (core.HTTPEndpoint{}) {
		return core.ErrGoogleIdentityContract
	}
	if p.PhoneOutcome == core.OAuthProfileAvailable {
		if err := p.PhoneNumber.Validate(); err != nil {
			return err
		}
	} else if p.PhoneNumber != (PhoneE164{}) || p.PhoneVerified {
		return core.ErrGoogleIdentityContract
	}
	if p.BirthdayOutcome == core.OAuthProfileAvailable {
		date, err := p.Birthday.Coordinates()
		if err != nil || date.Year == 0 {
			return contractError(err)
		}
	} else if p.Birthday != (temporal.CivilDate{}) {
		return core.ErrGoogleIdentityContract
	}
	return nil
}

// browserPeopleProfile uses the official People SDK and its typed Person reply.
// This is one native SDK whole-value response, not a collection-streaming claim.
// Request only the authenticated profile's selected fields, never contacts.
// Sources: https://developers.google.com/people/api/rest/v1/people/get
// https://developers.google.com/identity/protocols/oauth2/policies
func browserPeopleProfile(ctx context.Context, r BrowserAuthorizationCompleteRequest, credential browserProfileCredential) (BrowserProfile, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return BrowserProfile{}, contractError(err)
	}
	token := credential.Token
	if token == nil || credential.Subject.Validate() != nil {
		return BrowserProfile{}, core.ErrGoogleIdentityContract
	}
	result, requested, err := admittedPeopleFields(r.Profile, token)
	if err != nil || requested == "" {
		return result, err
	}
	client, err := peopleBrowserClient(ctx, r.Client)
	if err != nil {
		return BrowserProfile{}, err
	}
	call := client.People.Get(peopleSelfResource).PersonFields(requested).Sources(peopleProfileSource).Context(ctx)
	call.Header().Set("Authorization", "Bearer "+token.AccessToken)
	person, err := call.Do()
	if canceled := contextstate.Validate(ctx); canceled != nil {
		return BrowserProfile{}, contractError(canceled)
	}
	if err != nil {
		return refusedPeopleProfile(result, err), nil
	}
	if person == nil {
		return refusedPeopleProfile(result, core.ErrGoogleIdentityContract), nil
	}
	if person.ResourceName != "people/"+credential.Subject.String() {
		result = refusedPeopleProfile(result, core.ErrGoogleIdentityContract)
		if result.NameOutcome == core.OAuthProfileUnavailable {
			result.NameOutcome = core.OAuthProfileInvalid
		}
		if result.PhoneOutcome == core.OAuthProfileUnavailable {
			result.PhoneOutcome = core.OAuthProfileInvalid
		}
		if result.BirthdayOutcome == core.OAuthProfileUnavailable {
			result.BirthdayOutcome = core.OAuthProfileInvalid
		}
		return result, nil
	}
	if result.NameOutcome == core.OAuthProfileMissing {
		result.observeName(person.Names)
	}
	if result.PhoneOutcome == core.OAuthProfileMissing {
		result.observePhone(person.PhoneNumbers)
	}
	if result.BirthdayOutcome == core.OAuthProfileMissing {
		result.observeBirthday(person.Birthdays)
	}
	return result, result.Validate()
}

func admittedPeopleFields(request BrowserProfileRequest, token *oauth2.Token) (BrowserProfile, string, error) {
	var result BrowserProfile
	raw := token.Extra("scope")
	scope, hasScope := raw.(string)
	if raw != nil && !hasScope {
		return BrowserProfile{}, "", core.ErrGoogleIdentityContract
	}
	// RFC6749 section5.1: omitted scope means the granted scope is identical
	// to the originally requested scope. Complete receives that sealed request.
	// Source: https://www.rfc-editor.org/rfc/rfc6749#section-5.1
	phone := request.PhoneNumber && (!hasScope || peopleScopeGranted(scope, BrowserPhoneScope))
	birthday := request.Birthday && (!hasScope || peopleScopeGranted(scope, BrowserBirthdayScope))
	fields := ""
	if request.Name {
		result.NameOutcome, fields = core.OAuthProfileMissing, "names"
	}
	if request.PhoneNumber {
		result.PhoneOutcome = core.OAuthProfileDenied
		if phone {
			result.PhoneOutcome = core.OAuthProfileMissing
			if fields != "" {
				fields += ","
			}
			fields += "phoneNumbers"
		}
	}
	if request.Birthday {
		result.BirthdayOutcome = core.OAuthProfileDenied
		if birthday {
			result.BirthdayOutcome = core.OAuthProfileMissing
			if fields != "" {
				fields += ","
			}
			fields += "birthdays"
		}
	}
	return result, fields, nil
}

func peopleScopeGranted(granted, wanted string) bool {
	for scope := range strings.FieldsSeq(granted) {
		if scope == wanted {
			return true
		}
	}
	return false
}

func peopleBrowserClient(ctx context.Context, client exchange.Client) (*people.Service, error) {
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationJSON})
	if err != nil {
		return nil, contractError(err)
	}
	transport, err := client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		return nil, contractError(err)
	}
	native, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		return nil, contractError(err)
	}
	service, err := people.NewService(ctx, option.WithHTTPClient(native))
	if err != nil {
		return nil, contractError(err)
	}
	return service, nil
}

func refusedPeopleProfile(result BrowserProfile, cause error) BrowserProfile {
	status := core.OAuthProfileUnavailable
	if native, ok := errors.AsType[*googleapi.Error](cause); ok && native != nil {
		// The provider body, header map and prose can contain account data.
		// Retain the native SDK error class/status, not that unowned payload.
		cause = &googleapi.Error{Code: native.Code, Message: http.StatusText(native.Code)}
		if native.Code == http.StatusForbidden || native.Code == http.StatusUnauthorized {
			status = core.OAuthProfileDenied
		}
	}
	if result.NameOutcome == core.OAuthProfileMissing {
		result.NameOutcome = status
	}
	if result.PhoneOutcome == core.OAuthProfileMissing {
		result.PhoneOutcome = status
	}
	if result.BirthdayOutcome == core.OAuthProfileMissing {
		result.BirthdayOutcome = status
	}
	result.refusal = contractError(cause)
	return result
}

func (profile *BrowserProfile) observePhone(values []*people.PhoneNumber) {
	seen := false
	for _, phone := range values {
		if phone == nil || phone.Metadata == nil || !phone.Metadata.Primary {
			continue
		}
		if foreignPeopleFieldSource(phone.Metadata) {
			profile.PhoneNumber, profile.PhoneVerified, profile.PhoneOutcome = PhoneE164{}, false, core.OAuthProfileInvalid
			return
		}
		if seen {
			profile.PhoneNumber, profile.PhoneVerified, profile.PhoneOutcome = PhoneE164{}, false, core.OAuthProfileInvalid
			return
		}
		seen = true
		if phone.CanonicalForm == "" {
			profile.PhoneOutcome = core.OAuthProfilePartial
			continue
		}
		observed, err := ParsePhoneE164(phone.CanonicalForm)
		if err != nil {
			profile.PhoneOutcome = core.OAuthProfileInvalid
			continue
		}
		profile.PhoneNumber, profile.PhoneVerified, profile.PhoneOutcome = observed, phone.Metadata.Verified, core.OAuthProfileAvailable
	}
	if !seen && len(values) != 0 {
		profile.PhoneOutcome = core.OAuthProfilePartial
	}
}

func (profile *BrowserProfile) observeBirthday(values []*people.Birthday) {
	seen := false
	for _, birthday := range values {
		if birthday == nil || birthday.Metadata == nil || !birthday.Metadata.Primary {
			continue
		}
		if foreignPeopleFieldSource(birthday.Metadata) {
			profile.Birthday, profile.BirthdayOutcome = temporal.CivilDate{}, core.OAuthProfileInvalid
			return
		}
		if seen {
			profile.Birthday, profile.BirthdayOutcome = temporal.CivilDate{}, core.OAuthProfileInvalid
			return
		}
		seen = true
		date := birthday.Date
		if date == nil || date.Year == 0 || date.Month == 0 || date.Day == 0 {
			profile.BirthdayOutcome = core.OAuthProfilePartial
			continue
		}
		if date.Year < 1 || date.Year > 9999 || date.Month < 1 || date.Month > 12 || date.Day < 1 || date.Day > 31 {
			profile.BirthdayOutcome = core.OAuthProfileInvalid
			continue
		}
		observed, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: int32(date.Year), Month: temporal.Month(date.Month), Day: uint8(date.Day)})
		if err != nil {
			profile.BirthdayOutcome = core.OAuthProfileInvalid
			continue
		}
		profile.Birthday, profile.BirthdayOutcome = observed, core.OAuthProfileAvailable
	}
	if !seen && len(values) != 0 {
		profile.BirthdayOutcome = core.OAuthProfilePartial
	}
}

func (BrowserProfileRequest) googleIdentityProtocolFact()    {}
func (BrowserProfile) googleIdentitySealedProjection()       {}
func (PhoneE164) googleIdentityProtocolFact()                {}
func (browserProfileCredential) googleIdentityInternalFlow() {}

// observeName admits only the primary PROFILE name belonging to the verified
// subject. Each malformed sibling remains a refusal without hiding safe parts.
func (profile *BrowserProfile) observeName(values []*people.Name) {
	seen := false
	for _, name := range values {
		if name == nil || name.Metadata == nil || !name.Metadata.Primary {
			continue
		}
		if foreignPeopleFieldSource(name.Metadata) {
			profile.FullName, profile.GivenName, profile.FamilyName = core.OAuthProfileName{}, core.OAuthProfileName{}, core.OAuthProfileName{}
			profile.nameIssues = browserNameIssues{Full: true, Given: true, Family: true}
			profile.NameOutcome = core.OAuthProfileInvalid
			return
		}
		if seen {
			profile.FullName, profile.GivenName, profile.FamilyName = core.OAuthProfileName{}, core.OAuthProfileName{}, core.OAuthProfileName{}
			profile.nameIssues = browserNameIssues{Full: true, Given: true, Family: true}
			profile.NameOutcome = core.OAuthProfileInvalid
			return
		}
		seen = true
		for _, part := range [...]struct {
			text    string
			value   *core.OAuthProfileName
			invalid *bool
		}{{name.DisplayName, &profile.FullName, &profile.nameIssues.Full}, {name.GivenName, &profile.GivenName, &profile.nameIssues.Given}, {name.FamilyName, &profile.FamilyName, &profile.nameIssues.Family}} {
			if part.text == "" {
				continue
			}
			observed, err := core.ParseOAuthProfileName(part.text)
			if err != nil {
				*part.invalid = true
				continue
			}
			*part.value = observed
		}
		switch {
		case profile.nameIssues.Full || profile.nameIssues.Given || profile.nameIssues.Family:
			profile.NameOutcome = core.OAuthProfileInvalid
		case profile.FullName != (core.OAuthProfileName{}):
			profile.NameOutcome = core.OAuthProfileAvailable
		case profile.GivenName != (core.OAuthProfileName{}) || profile.FamilyName != (core.OAuthProfileName{}):
			profile.NameOutcome = core.OAuthProfilePartial
		default:
			profile.NameOutcome = core.OAuthProfileMissing
		}
	}
	if !seen && len(values) != 0 {
		profile.NameOutcome = core.OAuthProfilePartial
	}
}

func foreignPeopleFieldSource(metadata *people.FieldMetadata) bool {
	return metadata.Source != nil && metadata.Source.Type != "" && metadata.Source.Type != peopleFieldSourceProfile
}
