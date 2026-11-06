package timematch

import (
	"context"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
)

// VisitorContact is the identity and available contact returned by the consenting provider.
// It is not an application account, an email-keyed principal or proof of phone
// possession. Missing provider data never becomes an invented contact.
type VisitorContact struct {
	Subject    core.OAuthSubject
	GivenName  core.OAuthProfileName
	FamilyName core.OAuthProfileName
	Email      core.OAuthEmail
	Phone      googleidentity.PhoneE164
}

func (v VisitorContact) Validate() error {
	if err := errors.Join(v.Subject.Validate(), v.GivenName.Validate(), v.FamilyName.Validate(), v.Email.Validate()); err != nil {
		return errors.Join(core.ErrTimeMatchConsent, err)
	}
	if v.Phone != (googleidentity.PhoneE164{}) {
		if err := v.Phone.Validate(); err != nil {
			return errors.Join(core.ErrTimeMatchConsent, err)
		}
	}
	return nil
}

// visitorContact reads only the consenting account's own profile. Calendar
// contents are never part of these requests. The code was already redeemed
// once by ExchangeCode; both reads use that same online-only access token.
func (c Client) visitorContact(ctx context.Context, grant Grant, policy exchange.OperationPolicy) (VisitorContact, error) {
	secrets, err := bearer(grant.AccessToken)
	if err != nil {
		return VisitorContact{}, err
	}
	address := "https://www.googleapis.com/oauth2/v3/userinfo"
	if grant.Provider == ProviderMicrosoft {
		address = graphBase + "/me?$select=id,givenName,surname,mail,mobilePhone"
	}
	target, err := endpoint(address)
	if err != nil {
		return VisitorContact{}, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodGet, secrets: secrets, responseType: core.HTTPMediaTypeJSON(), policy: policy, expected: http.StatusOK})
	defer clear(result.body)
	if err != nil {
		return VisitorContact{}, readFailure(result, err)
	}
	var contact VisitorContact
	if grant.Provider == ProviderGoogle {
		var document struct {
			Subject       string `json:"sub"`
			GivenName     string `json:"given_name"`
			FamilyName    string `json:"family_name"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
		}
		if err := unmarshalExtensible(result.body, &document); err != nil {
			return VisitorContact{}, err
		}
		if !document.EmailVerified {
			return VisitorContact{}, core.ErrTimeMatchConsent
		}
		contact, err = visitorNames(document.Subject, document.GivenName, document.FamilyName, document.Email)
		if err != nil {
			return VisitorContact{}, err
		}
		profile, err := googleidentity.ReadBrowserProfile(ctx, googleidentity.BrowserProfileReadRequest{Client: c.network, AccessToken: secrets[0], Subject: contact.Subject, Profile: googleidentity.BrowserProfileRequest{PhoneNumber: true}, Policy: policy})
		if err != nil {
			return VisitorContact{}, errors.Join(core.ErrTimeMatchConsent, err)
		}
		switch profile.PhoneOutcome {
		case core.OAuthProfileAvailable:
			contact.Phone = profile.PhoneNumber
		case core.OAuthProfileMissing, core.OAuthProfilePartial:
			// The caller completes only this absent contact before booking.
		case core.OAuthProfileDenied:
			return VisitorContact{}, core.ErrTimeMatchPhoneDenied
		case core.OAuthProfileInvalid:
			return VisitorContact{}, core.ErrTimeMatchPhoneInvalid
		case core.OAuthProfileUnavailable:
			return VisitorContact{}, core.ErrTimeMatchPhoneUnavailable
		default:
			return VisitorContact{}, core.ErrTimeMatchContract
		}
	} else {
		var document struct {
			ID          string `json:"id"`
			GivenName   string `json:"givenName"`
			Surname     string `json:"surname"`
			Mail        string `json:"mail"`
			MobilePhone string `json:"mobilePhone"`
		}
		if err := unmarshalExtensible(result.body, &document); err != nil {
			return VisitorContact{}, err
		}
		contact, err = visitorNames(document.ID, document.GivenName, document.Surname, document.Mail)
		if err != nil {
			return VisitorContact{}, err
		}
		if document.MobilePhone != "" {
			contact.Phone, err = googleidentity.ParsePhoneE164(document.MobilePhone)
			if err != nil {
				return VisitorContact{}, errors.Join(core.ErrTimeMatchConsent, err)
			}
		}
	}
	return contact, contact.Validate()
}

func visitorNames(subject, given, family, email string) (VisitorContact, error) {
	var contact VisitorContact
	var subjectErr, givenErr, familyErr, emailErr error
	contact.Subject, subjectErr = core.ParseOAuthSubject(subject)
	contact.GivenName, givenErr = core.ParseOAuthProfileName(given)
	contact.FamilyName, familyErr = core.ParseOAuthProfileName(family)
	contact.Email, emailErr = core.ParseOAuthEmail(email)
	if err := errors.Join(subjectErr, givenErr, familyErr, emailErr); err != nil {
		return VisitorContact{}, errors.Join(core.ErrTimeMatchConsent, err)
	}
	return contact, nil
}

func (VisitorContact) timematchProtocolFact() {}
