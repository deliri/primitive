package timematch

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"net/url"
)

// EventVenueStatus is the observed outcome of automatic video-call creation.
// Pending proves the owned event exists, not that a usable call is ready.
// Failed is an explicit provider refusal. The caller may read the same event
// again; this package never polls, chooses another identity or retries a failure.
type EventVenueStatus uint8

const (
	EventVenueNone EventVenueStatus = iota
	EventVenuePending
	EventVenueReady
	EventVenueFailed
)

func (s EventVenueStatus) Validate() error {
	switch s {
	case EventVenueNone, EventVenuePending, EventVenueReady, EventVenueFailed:
		return nil
	default:
		return core.ErrTimeMatchContract
	}
}

func (EventVenueStatus) OffWireEnum() {}

var (
	_ core.Validatable = EventVenueNone
	_ core.OffWireEnum = EventVenueNone
)

// These narrow Google wire records contain only the owned conference request
// and its video entry point. Phone credentials, notes and provider secrets are
// neither retained nor projected to the application.
type googleConferenceKey struct {
	Type string `json:"type"`
}
type googleConferenceStatus struct {
	Code string `json:"statusCode"`
}
type googleConferenceRequest struct {
	RequestID string                  `json:"requestId"`
	Solution  googleConferenceKey     `json:"conferenceSolutionKey"`
	Status    *googleConferenceStatus `json:"status,omitzero"`
}
type googleConferenceSolution struct {
	Key googleConferenceKey `json:"key"`
}
type googleConferenceEntryPoint struct {
	Type string `json:"entryPointType"`
	URI  string `json:"uri"`
}
type googleConferenceData struct {
	CreateRequest *googleConferenceRequest     `json:"createRequest,omitzero"`
	Solution      *googleConferenceSolution    `json:"conferenceSolution,omitzero"`
	EntryPoints   []googleConferenceEntryPoint `json:"entryPoints,omitzero"`
}

const (
	googleMeetSolution          = "hangoutsMeet"
	googleMeetHost              = "meet.google.com"
	googleConferencePending     = "pending"
	googleConferenceSuccess     = "success"
	googleConferenceFailure     = "failure"
	googleConferenceVideo       = "video"
	googleConferencePhone       = "phone"
	googleConferenceSIP         = "sip"
	googleConferenceMore        = "more"
	googleOwnedConferenceFields = ",conferenceData(createRequest(requestId,conferenceSolutionKey/type,status/statusCode),conferenceSolution/key/type,entryPoints(entryPointType,uri))"
)

func googleMeetCreateData(intent EventIntent) *googleConferenceData {
	if intent.Venue.Kind != core.TimeMatchVenueAutomaticVideo {
		return nil
	}
	// EventID's validated base32hex domain is a subset of requestId's documented
	// string domain. Distinct event identities have distinct request identities;
	// retries and reschedules keep the same request, never a second conference.
	return &googleConferenceData{CreateRequest: &googleConferenceRequest{RequestID: intent.ID.value, Solution: googleConferenceKey{Type: googleMeetSolution}}}
}

func (r EventReceipt) validateVenue() error {
	if err := r.VenueStatus.Validate(); err != nil {
		return err
	}
	if r.VenueStatus == EventVenueNone {
		if r.Venue != (core.TimeMatchAppointmentVenue{}) {
			return core.ErrTimeMatchContract
		}
		return nil
	}
	if r.Ref.Provider != ProviderGoogle {
		return core.ErrTimeMatchContract
	}
	switch r.VenueStatus {
	case EventVenuePending, EventVenueFailed:
		if r.Venue != (core.TimeMatchAppointmentVenue{}) {
			return core.ErrTimeMatchContract
		}
		return nil
	case EventVenueReady:
		if r.Venue.Kind != core.TimeMatchVenueSupplied || r.Venue.Address != "" {
			return core.ErrTimeMatchContract
		}
		return validateGoogleMeetURL(r.Venue.JoinURL)
	default:
		return core.ErrTimeMatchContract
	}
}

func validateAutomaticVideoProvider(venue core.TimeMatchAppointmentVenue, provider Provider) error {
	if venue.Kind == core.TimeMatchVenueAutomaticVideo && provider != ProviderGoogle {
		return core.ErrTimeMatchContract
	}
	return nil
}

func validateGoogleMeetURL(value string) error {
	if value == "" {
		return core.ErrTimeMatchResponse
	}
	if err := (core.TimeMatchAppointmentVenue{JoinURL: value}).Validate(); err != nil {
		return errors.Join(core.ErrTimeMatchResponse, err)
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != core.SchemeHTTPS || u.Host != googleMeetHost || u.User != nil || u.Opaque != "" {
		return core.ErrTimeMatchResponse
	}
	return validateGoogleMeetURLDestination(u)
}

func validateGoogleMeetURLDestination(u *url.URL) error {
	if u.Path == "" || u.Path == "/" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" {
		return core.ErrTimeMatchResponse
	}
	return nil
}

func googleEventReceipt(observed googleEventObservation, intent EventIntent, ref EventRef) (EventReceipt, error) {
	r := EventReceipt{Ref: ref}
	if intent.Venue.Kind != core.TimeMatchVenueAutomaticVideo {
		return r, nil
	}
	conference, err := googleMeetObservation(observed, intent)
	if err != nil {
		return EventReceipt{}, err
	}
	request := conference.CreateRequest
	switch request.Status.Code {
	case googleConferencePending, googleConferenceFailure:
		if len(conference.EntryPoints) != 0 {
			return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
		}
		r.VenueStatus = EventVenuePending
		if request.Status.Code == googleConferenceFailure {
			r.VenueStatus = EventVenueFailed
		}
	case googleConferenceSuccess:
		return googleReadyMeetReceipt(*conference, ref)
	default:
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
	}
	return r, nil
}

func googleReadyMeetReceipt(conference googleConferenceData, ref EventRef) (EventReceipt, error) {
	if conference.Solution == nil || conference.Solution.Key.Type != googleMeetSolution {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
	}
	join, err := googleMeetVideoEntry(conference.EntryPoints)
	if err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	return EventReceipt{Ref: ref, VenueStatus: EventVenueReady, Venue: core.TimeMatchAppointmentVenue{JoinURL: join}}, nil
}

func googleMeetVideoEntry(entries []googleConferenceEntryPoint) (string, error) {
	join := ""
	for _, entry := range entries {
		switch entry.Type {
		case googleConferenceVideo:
			if join != "" {
				return "", core.ErrTimeMatchResponse
			}
			if err := validateGoogleMeetURL(entry.URI); err != nil {
				return "", err
			}
			join = entry.URI
		case googleConferencePhone, googleConferenceSIP, googleConferenceMore:
		default:
			return "", core.ErrTimeMatchResponse
		}
	}
	if join == "" {
		return "", core.ErrTimeMatchResponse
	}
	return join, nil
}

func (googleConferenceKey) timematchInternalFlow()        {}
func (googleConferenceStatus) timematchInternalFlow()     {}
func (googleConferenceRequest) timematchInternalFlow()    {}
func (googleConferenceSolution) timematchInternalFlow()   {}
func (googleConferenceEntryPoint) timematchInternalFlow() {}
func (googleConferenceData) timematchInternalFlow()       {}

func googleMeetObservation(observed googleEventObservation, intent EventIntent) (*googleConferenceData, error) {
	conference := observed.conference
	if conference == nil || conference.CreateRequest == nil || conference.CreateRequest.Status == nil {
		return nil, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
	}
	request := conference.CreateRequest
	if request.RequestID != intent.ID.value || request.Solution.Type != googleMeetSolution {
		return nil, core.ErrTimeMatchConflict
	}
	return conference, nil
}
