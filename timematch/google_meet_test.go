package timematch

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func meetIntent(t testing.TB) EventIntent {
	t.Helper()
	owned := googleOwnershipIntent(t)
	owned.Venue = core.TimeMatchAppointmentVenue{Kind: core.TimeMatchVenueAutomaticVideo}
	if err := owned.Validate(); err != nil {
		t.Fatal(err)
	}
	return owned
}

// Native controlled TLS peer through the production Exchange path, not a live
// Google account. Its response never invents a conference the request omitted.
func TestGoogleMeetCreationRequestsAndReturnsOwnedConference(t *testing.T) {
	t.Parallel()
	owned := meetIntent(t)
	client, fixture := newNative(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		var request struct {
			ConferenceData struct {
				CreateRequest struct {
					RequestID string `json:"requestId"`
					Solution  struct {
						Type string `json:"type"`
					} `json:"conferenceSolutionKey"`
				} `json:"createRequest"`
			} `json:"conferenceData"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
			writeJSON(w, 400, `{}`)
			return
		}
		if r.URL.Query().Get("conferenceDataVersion") != "1" || request.ConferenceData.CreateRequest.RequestID != owned.ID.String() || request.ConferenceData.CreateRequest.Solution.Type != "hangoutsMeet" {
			t.Errorf("native conference request=(%q,%+v), want version1 and exact owned Meet request", r.URL.RawQuery, request)
			writeJSON(w, 200, string(body))
			return
		}
		var response struct {
			ID                 string                    `json:"id"`
			Summary            string                    `json:"summary"`
			Start              googleEventTime           `json:"start"`
			End                googleEventTime           `json:"end"`
			ExtendedProperties *googleExtendedProperties `json:"extendedProperties"`
			ConferenceData     struct {
				CreateRequest struct {
					RequestID string `json:"requestId"`
					Solution  struct {
						Type string `json:"type"`
					} `json:"conferenceSolutionKey"`
					Status struct {
						StatusCode string `json:"statusCode"`
					} `json:"status"`
				} `json:"createRequest"`
				Solution struct {
					Key struct {
						Type string `json:"type"`
					} `json:"key"`
				} `json:"conferenceSolution"`
				EntryPoints []struct {
					Type string `json:"entryPointType"`
					URI  string `json:"uri"`
				} `json:"entryPoints"`
			} `json:"conferenceData"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Error(err)
			writeJSON(w, 400, `{}`)
			return
		}
		response.ConferenceData.CreateRequest.Status.StatusCode = "success"
		response.ConferenceData.Solution.Key.Type = "hangoutsMeet"
		response.ConferenceData.EntryPoints = append(response.ConferenceData.EntryPoints, struct {
			Type string `json:"entryPointType"`
			URI  string `json:"uri"`
		}{Type: "video", URI: "https://meet.google.com/abc-defg-hij"})
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Error(err)
			writeJSON(w, 500, `{}`)
			return
		}
		writeJSON(w, 200, string(encoded))
	})
	got, err := client.PutEvent(t.Context(), eventWrite(t, oauthConnection(t, ProviderGoogle), owned))
	if err != nil || got.Ref.ID != owned.ID || got.VenueStatus != EventVenueReady || got.Venue.JoinURL != "https://meet.google.com/abc-defg-hij" || len(fixture.record()) != 1 {
		t.Fatalf("owned Meet create=(%+v,%v), calls%d, want exact Ready receipt and1create", got, err, len(fixture.record()))
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticVideoUnsupportedProviderHasNoEffects(t *testing.T) {
	t.Parallel()
	for _, provider := range []Provider{ProviderMicrosoft, ProviderApple} {
		t.Run(provider.String(), func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, 503, `{}`) })
			connection := oauthConnection(t, provider)
			if provider == ProviderApple {
				connection = appleConnection(t)
			}
			owned := meetIntent(t)
			got, err := client.PutEvent(t.Context(), eventWrite(t, connection, owned))
			if !errors.Is(err, core.ErrTimeMatchContract) || got != (EventReceipt{}) || len(fixture.record()) != 0 {
				t.Fatalf("unsupported video=(%+v,%v),calls%d,want contract refusal/noeffects", got, err, len(fixture.record()))
			}
		})
	}
}
