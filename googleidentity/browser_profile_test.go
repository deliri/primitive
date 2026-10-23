package googleidentity

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/people/v1"
)

func peopleProfileSeed() *people.Person {
	return &people.Person{ResourceName: "people/stable-google-subject", Names: []*people.Name{{DisplayName: "Ada Lovelace", GivenName: "Ada", FamilyName: "Lovelace", Metadata: &people.FieldMetadata{Primary: true, Source: &people.Source{Type: "PROFILE"}}}}, PhoneNumbers: []*people.PhoneNumber{{CanonicalForm: "+14165550123", Metadata: &people.FieldMetadata{Primary: true, Verified: true, Source: &people.Source{Type: "PROFILE"}}}}, Birthdays: []*people.Birthday{{Date: &people.Date{Year: 2000, Month: 2, Day: 29}, Metadata: &people.FieldMetadata{Primary: true, Source: &people.Source{Type: "PROFILE"}}}}}
}

// The only HTTP seam is the local TLS provider. Official People SDK request,
// framing/decoding, Exchange transport and the production projector all run.
func peopleProfileLocalClient(t testing.TB, handler http.Handler) exchange.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("TLS transport=%T, want native HTTP", server.Client().Transport)
	}
	owned := transport.Clone()
	owned.Proxy = nil
	owned.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	owned.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(owned.CloseIdleConnections)
	client, err := exchange.NewClient(&http.Client{Transport: owned})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func peopleCredential(t testing.TB, scope *string) browserProfileCredential {
	t.Helper()
	subject, err := core.ParseOAuthSubject("stable-google-subject")
	if err != nil {
		t.Fatal(err)
	}
	token := &oauth2.Token{AccessToken: "public-test-people-bearer", TokenType: "Bearer"}
	if scope != nil {
		token = token.WithExtra(map[string]any{"scope": *scope})
	}
	return browserProfileCredential{Token: token, Subject: subject}
}

func TestBrowserPeopleProfileNativeLayerTriad(t *testing.T) {
	t.Parallel()
	all := BrowserProfileRequest{Name: true, PhoneNumber: true, Birthday: true}
	available, missing, partial, invalid, denied, unavailable := core.OAuthProfileAvailable, core.OAuthProfileMissing, core.OAuthProfilePartial, core.OAuthProfileInvalid, core.OAuthProfileDenied, core.OAuthProfileUnavailable
	rows := []struct {
		name                                 string
		mutate                               func(*people.Person)
		request                              BrowserProfileRequest
		scope                                *string
		status                               int
		raw                                  string
		wantName, wantPhone, wantBirthday    core.OAuthProfileOutcome
		wantMask, full, given, family, phone string
		year                                 int32
		month, day                           uint8
		verified, refusal                    bool
	}{
		{name: "positive all selected verified observations", request: all, wantMask: "names,phoneNumbers,birthdays", wantName: available, wantPhone: available, wantBirthday: available, full: "Ada Lovelace", given: "Ada", family: "Lovelace", phone: "+14165550123", year: 2000, month: 2, day: 29, verified: true},
		{name: "neutral no selection performs no HTTP", mutate: nil},
		{name: "positive phone selection excludes names and birthday", request: BrowserProfileRequest{PhoneNumber: true}, wantMask: "phoneNumbers", wantPhone: available, phone: "+14165550123", verified: true},
		{name: "positive birthday selection excludes names and phone", request: BrowserProfileRequest{Birthday: true}, wantMask: "birthdays", wantBirthday: available, year: 2000, month: 2, day: 29},
		{name: "positive name selection excludes private fields", request: BrowserProfileRequest{Name: true}, wantMask: "names", wantName: available, full: "Ada Lovelace", given: "Ada", family: "Lovelace"},
		{name: "positive missing field source metadata follows authenticated filter", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers[0].Metadata.Source = nil }, wantMask: "phoneNumbers", wantPhone: available, phone: "+14165550123", verified: true},
		{name: "positive empty field source type follows authenticated filter", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers[0].Metadata.Source.Type = "" }, wantMask: "phoneNumbers", wantPhone: available, phone: "+14165550123", verified: true},
		{name: "positive unverified phone remains observation only", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers[0].Metadata.Verified = false }, wantMask: "phoneNumbers", wantPhone: available, phone: "+14165550123"},
		{name: "neutral requested fields absent remain missing", request: all, mutate: func(p *people.Person) { p.Names = nil; p.PhoneNumbers = nil; p.Birthdays = nil }, wantMask: "names,phoneNumbers,birthdays", wantName: missing, wantPhone: missing, wantBirthday: missing},
		{name: "refusal foreign subject cannot supply observations", request: all, mutate: func(p *people.Person) { p.ResourceName = "people/foreign" }, wantMask: "names,phoneNumbers,birthdays", wantName: invalid, wantPhone: invalid, wantBirthday: invalid, refusal: true},
		{name: "refusal missing resource cannot supply observations", request: all, mutate: func(p *people.Person) { p.ResourceName = "" }, wantMask: "names,phoneNumbers,birthdays", wantName: invalid, wantPhone: invalid, wantBirthday: invalid, refusal: true},
		{name: "refusal CONTACT primary observations never escape", request: all, mutate: func(p *people.Person) {
			p.Names[0].Metadata.Source.Type = "CONTACT"
			p.PhoneNumbers[0].Metadata.Source.Type = "CONTACT"
			p.Birthdays[0].Metadata.Source.Type = "CONTACT"
		}, wantMask: "names,phoneNumbers,birthdays", wantName: invalid, wantPhone: invalid, wantBirthday: invalid},
		{name: "refusal unknown primary source never escapes", request: all, mutate: func(p *people.Person) {
			p.Names[0].Metadata.Source.Type = "FUTURE"
			p.PhoneNumbers[0].Metadata.Source.Type = "FUTURE"
			p.Birthdays[0].Metadata.Source.Type = "FUTURE"
		}, wantMask: "names,phoneNumbers,birthdays", wantName: invalid, wantPhone: invalid, wantBirthday: invalid},
		{name: "refusal denied scopes perform no HTTP", request: BrowserProfileRequest{PhoneNumber: true, Birthday: true}, scope: browserString("openid email profile"), wantPhone: denied, wantBirthday: denied},
		{name: "positive only granted phone enters mask", request: BrowserProfileRequest{PhoneNumber: true, Birthday: true}, scope: browserString(BrowserPhoneScope), wantMask: "phoneNumbers", wantPhone: available, phone: "+14165550123", verified: true, wantBirthday: denied},
		{name: "positive only granted birthday enters mask", request: BrowserProfileRequest{PhoneNumber: true, Birthday: true}, scope: browserString(BrowserBirthdayScope), wantMask: "birthdays", wantPhone: denied, wantBirthday: available, year: 2000, month: 2, day: 29},
		{name: "neutral birthday without year never invents one", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Year = 0 }, wantMask: "birthdays", wantBirthday: partial},
		{name: "neutral birthday without month remains partial", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Month = 0 }, wantMask: "birthdays", wantBirthday: partial},
		{name: "neutral birthday without day remains partial", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Day = 0 }, wantMask: "birthdays", wantBirthday: partial},
		{name: "neutral birthday nil date remains partial", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date = nil }, wantMask: "birthdays", wantBirthday: partial},
		{name: "refusal invalid nonleap date", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Year = 1900 }, wantMask: "birthdays", wantBirthday: invalid},
		{name: "boundary earliest full civil date", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date = &people.Date{Year: 1, Month: 1, Day: 1} }, wantMask: "birthdays", wantBirthday: available, year: 1, month: 1, day: 1},
		{name: "boundary latest full civil date", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date = &people.Date{Year: 9999, Month: 12, Day: 31} }, wantMask: "birthdays", wantBirthday: available, year: 9999, month: 12, day: 31},
		{name: "boundary year above domain refuses", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Year = 10000 }, wantMask: "birthdays", wantBirthday: invalid},
		{name: "boundary negative year refuses", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Year = -1 }, wantMask: "birthdays", wantBirthday: invalid},
		{name: "boundary month above domain refuses", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Month = 13 }, wantMask: "birthdays", wantBirthday: invalid},
		{name: "boundary day above domain refuses", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Date.Day = 32 }, wantMask: "birthdays", wantBirthday: invalid},
		{name: "refusal duplicate primary birthdays", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays = append(p.Birthdays, p.Birthdays[0]) }, wantMask: "birthdays", wantBirthday: invalid},
		{name: "neutral only secondary birthday remains partial", request: BrowserProfileRequest{Birthday: true}, mutate: func(p *people.Person) { p.Birthdays[0].Metadata.Primary = false }, wantMask: "birthdays", wantBirthday: partial},
		{name: "refusal duplicate primary phones", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers = append(p.PhoneNumbers, p.PhoneNumbers[0]) }, wantMask: "phoneNumbers", wantPhone: invalid},
		{name: "refusal malformed canonical phone", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers[0].CanonicalForm = "416-555-0123" }, wantMask: "phoneNumbers", wantPhone: invalid},
		{name: "neutral display phone cannot invent canonical phone", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers[0].CanonicalForm = ""; p.PhoneNumbers[0].Value = "416 555 0123" }, wantMask: "phoneNumbers", wantPhone: partial},
		{name: "neutral only secondary phone remains partial", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers[0].Metadata.Primary = false }, wantMask: "phoneNumbers", wantPhone: partial},
		{name: "neutral phone without metadata remains partial", request: BrowserProfileRequest{PhoneNumber: true}, mutate: func(p *people.Person) { p.PhoneNumbers[0].Metadata = nil }, wantMask: "phoneNumbers", wantPhone: partial},
		{name: "refusal duplicate primary names", request: BrowserProfileRequest{Name: true}, mutate: func(p *people.Person) { p.Names = append(p.Names, p.Names[0]) }, wantMask: "names", wantName: invalid},
		{name: "neutral structured name without display remains partial", request: BrowserProfileRequest{Name: true}, mutate: func(p *people.Person) { p.Names[0].DisplayName = "" }, wantMask: "names", wantName: partial, given: "Ada", family: "Lovelace"},
		{name: "refusal malformed name preserves safe sibling", request: BrowserProfileRequest{Name: true}, mutate: func(p *people.Person) { p.Names[0].GivenName = "bad\x00name" }, wantMask: "names", wantName: invalid, full: "Ada Lovelace", family: "Lovelace"},
		{name: "neutral nonprimary names remain partial", request: BrowserProfileRequest{Name: true}, mutate: func(p *people.Person) { p.Names[0].Metadata.Primary = false }, wantMask: "names", wantName: partial},
		{name: "refusal native403 denies optional fields", request: all, status: 403, raw: "private-provider-diagnostic public-test-people-bearer", wantMask: "names,phoneNumbers,birthdays", wantName: denied, wantPhone: denied, wantBirthday: denied, refusal: true},
		{name: "refusal native503 leaves fields unavailable", request: all, status: 503, raw: "private-provider-diagnostic public-test-people-bearer", wantMask: "names,phoneNumbers,birthdays", wantName: unavailable, wantPhone: unavailable, wantBirthday: unavailable, refusal: true},
		{name: "refusal malformed SDK body never becomes missing", request: all, raw: "{", wantMask: "names,phoneNumbers,birthdays", wantName: unavailable, wantPhone: unavailable, wantBirthday: unavailable, refusal: true},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			person := peopleProfileSeed()
			if tc.mutate != nil {
				tc.mutate(person)
			}
			body, err := core.MarshalCanonicalJSONDocument(person)
			if err != nil {
				t.Fatal(err)
			}
			if tc.status != 0 {
				body, err = core.MarshalCanonicalJSONDocument(struct {
					Error *googleapi.Error `json:"error"`
				}{Error: &googleapi.Error{Code: tc.status, Message: tc.raw}})
				if err != nil {
					t.Fatal(err)
				}
			} else if tc.raw != "" {
				body = []byte(tc.raw)
			}
			var calls atomic.Uint32
			client := peopleProfileLocalClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "GET" || r.Host != "people.googleapis.com" || r.URL.Path != "/v1/people/me" || r.URL.Query().Get("personFields") != tc.wantMask || r.URL.Query().Get("sources") != "READ_SOURCE_TYPE_PROFILE" || r.Header.Get("Authorization") != "Bearer public-test-people-bearer" {
					t.Errorf("People request=%s %s%s mask=%q source=%q, want exact authenticated PROFILE-only selected request", r.Method, r.Host, r.URL.Path, r.URL.Query().Get("personFields"), r.URL.Query().Get("sources"))
				}
				w.Header().Set("Content-Type", "application/json")
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				if _, err := w.Write(body); err != nil {
					t.Errorf("provider write=%v, want nil", err)
				}
			}))
			got, err := browserPeopleProfile(t.Context(), BrowserAuthorizationCompleteRequest{Client: client, Profile: tc.request}, peopleCredential(t, tc.scope))
			if err != nil || got.Validate() != nil {
				t.Fatalf("profile=(%+v,%v), want valid optional observation", got, err)
			}
			if got.NameOutcome != tc.wantName || got.PhoneOutcome != tc.wantPhone || got.BirthdayOutcome != tc.wantBirthday || got.FullName.String() != tc.full || got.GivenName.String() != tc.given || got.FamilyName.String() != tc.family || got.PhoneNumber.String() != tc.phone || got.PhoneVerified != tc.verified {
				t.Fatalf("profile=%+v, want outcomes(%v,%v,%v) names(%q,%q,%q) phone(%q,%t)", got, tc.wantName, tc.wantPhone, tc.wantBirthday, tc.full, tc.given, tc.family, tc.phone, tc.verified)
			}
			if tc.year != 0 {
				date, err := got.Birthday.Coordinates()
				if err != nil || date.Year != tc.year || uint8(date.Month) != tc.month || date.Day != tc.day {
					t.Fatalf("birthday=(%+v,%v), want %d/%d/%d", date, err, tc.year, tc.month, tc.day)
				}
			}
			if (got.ObservationError() != nil) != tc.refusal {
				t.Fatalf("optional refusal=%v, want present=%t", got.ObservationError(), tc.refusal)
			}
			if tc.refusal && !errors.Is(got.ObservationError(), core.ErrGoogleIdentityContract) {
				t.Fatalf("optional refusal=%v, want contract identity", got.ObservationError())
			}
			if tc.status != 0 {
				native, ok := errors.AsType[*googleapi.Error](got.ObservationError())
				if !ok || native.Code != tc.status || native.Body != "" || len(native.Header) != 0 || strings.Contains(native.Message, "private-provider-diagnostic") || strings.Contains(native.Message, "public-test-people-bearer") {
					t.Fatalf("native optional refusal=%+v, want sanitized status%d", native, tc.status)
				}
			}
			wantCalls := uint32(1)
			if tc.wantMask == "" {
				wantCalls = 0
			}
			if calls.Load() != wantCalls {
				t.Fatalf("People calls=%d, want %d", calls.Load(), wantCalls)
			}
		})
	}
}

func TestBrowserPeopleProfilePublicCompleteFallback(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		claims := browserClaimsSeed()
		token := signBrowserFixture(t, claims, false)
		key := verifierTestKey(t, "testdata/verifier_rsa.pem")
		keys, err := core.MarshalCanonicalJSONDocument(verifierTestKeySet{Keys: []verifierTestJWK{{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID, KeyType: "RSA", Use: "sig", Exponent: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()), Modulus: base64.RawURLEncoding.EncodeToString(key.N.Bytes())}}})
		if err != nil {
			t.Fatal(err)
		}
		var count atomic.Uint32
		client := peopleProfileLocalClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			w.Header().Set("Content-Type", "application/json")
			var body []byte
			var encodeErr error
			switch r.Host {
			case "oauth2.googleapis.com":
				body, encodeErr = core.MarshalCanonicalJSONDocument(browserTokenFixture{AccessToken: "public-test-people-bearer", TokenType: "Bearer", IDToken: token, Expires: 300})
			case "people.googleapis.com":
				if r.URL.Query().Get("personFields") != "names" || r.URL.Query().Get("sources") != "READ_SOURCE_TYPE_PROFILE" || r.Header.Get("Authorization") != "Bearer public-test-people-bearer" {
					t.Errorf("public Complete People binding=%s, want names-only PROFILE bearer", r.URL.String())
				}
				body, encodeErr = core.MarshalCanonicalJSONDocument(peopleProfileSeed())
			default:
				body = keys
				w.Header().Set("Cache-Control", "public,max-age=3600")
			}
			if encodeErr != nil {
				t.Error(encodeErr)
				return
			}
			if _, err := w.Write(body); err != nil {
				t.Error(err)
			}
		}))
		request := browserRequestFixture(t, client)
		request.Profile.Name = true
		got, err := CompleteBrowserAuthorization(t.Context(), request)
		if err != nil || got.Subject.String() != claims.Subject || got.Profile.FullName.String() != "Ada Lovelace" || got.Profile.GivenName.String() != "Ada" || got.Profile.FamilyName.String() != "Lovelace" || got.Profile.NameOutcome != core.OAuthProfileAvailable || got.Profile.PhoneOutcome != core.OAuthProfileNotRequested || count.Load() != 3 {
			t.Fatalf("public Complete=(%+v,%v), calls=%d, want verified same subject and People name fallback only", got, err, count.Load())
		}
	})
}

func TestBrowserPeopleProfileCancellationAndCredentialRefusal(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("owned People cancellation")
	cancel(cause)
	got, err := browserPeopleProfile(ctx, BrowserAuthorizationCompleteRequest{Profile: BrowserProfileRequest{Name: true}}, peopleCredential(t, nil))
	if !errors.Is(err, context.Canceled) || got != (BrowserProfile{}) {
		t.Fatalf("canceled People=(%+v,%v), want zero/native cancellation", got, err)
	}
	got, err = browserPeopleProfile(t.Context(), BrowserAuthorizationCompleteRequest{}, browserProfileCredential{})
	if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (BrowserProfile{}) {
		t.Fatalf("unset credential=(%+v,%v), want zero/contract", got, err)
	}
}

func TestBrowserPeopleProfileNameOutcomePresenceExhaustive(t *testing.T) {
	t.Parallel()
	value, err := core.ParseOAuthProfileName("Ada")
	if err != nil {
		t.Fatal(err)
	}
	for outcome := core.OAuthProfileNotRequested; outcome <= core.OAuthProfileUnavailable; outcome++ {
		for presence := uint8(0); presence < 8; presence++ {
			t.Run(fmt.Sprintf("%s/names_%03b", outcome.String(), presence), func(t *testing.T) {
				t.Parallel()
				profile := BrowserProfile{NameOutcome: outcome}
				if presence&1 != 0 {
					profile.FullName = value
				}
				if presence&2 != 0 {
					profile.GivenName = value
				}
				if presence&4 != 0 {
					profile.FamilyName = value
				}
				wantAccepted := presence == 0
				switch outcome {
				case core.OAuthProfileAvailable:
					wantAccepted = presence&1 != 0
				case core.OAuthProfilePartial, core.OAuthProfileInvalid:
					wantAccepted = true
				}
				err := profile.Validate()
				if wantAccepted && err != nil || !wantAccepted && !errors.Is(err, core.ErrGoogleIdentityContract) {
					t.Fatalf("name outcome=%v presence=%03b Validate=%v, want admitted=%t", outcome, presence, err, wantAccepted)
				}
			})
		}
	}
}

func TestBrowserPeopleProfileNativeCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered, serverExited := make(chan struct{}), make(chan struct{})
	client := peopleProfileLocalClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(serverExited)
	}))
	type result struct {
		profile BrowserProfile
		err     error
	}
	returned := make(chan result, 1)
	joined := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		select {
		case <-joined:
		case <-time.After(10 * time.Second):
			t.Error("People cancellation worker join=timed out, want owned exit")
		}
	})
	go func() {
		defer close(joined)
		profile, err := browserPeopleProfile(ctx, BrowserAuthorizationCompleteRequest{Client: client, Profile: BrowserProfileRequest{Name: true}}, peopleCredential(t, nil))
		returned <- result{profile: profile, err: err}
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("People SDK server admission=timed out, want observed native request")
	}
	cancel()
	select {
	case got := <-returned:
		if !errors.Is(got.err, context.Canceled) || got.profile != (BrowserProfile{}) {
			t.Fatalf("canceled native People=(%+v,%v), want zero/native cancellation", got.profile, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("People SDK cancellation result=timed out, want native cancellation")
	}
	select {
	case <-serverExited:
	case <-time.After(10 * time.Second):
		t.Fatal("native server request context=active after cancellation, want terminal context")
	}
}
