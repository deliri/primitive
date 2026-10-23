package googleidentity

import (
	json "encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/people/v1"
)

func TestPhoneE164SyntacticBoundaries(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name, text string
		wantErr    error
	}{
		{name: "minimum one international digit", text: "+1"},
		{name: "two decimal digits", text: "+12"},
		{name: "ordinary North America number", text: "+14165550123"},
		{name: "ordinary UK number", text: "+442071234567"},
		{name: "fourteen digits below ceiling", text: "+12345678901234"},
		{name: "fifteen digits at ceiling", text: "+123456789012345"},
		{name: "sixteen digits above ceiling", text: "+1234567890123456", wantErr: core.ErrGoogleIdentityContract},
		{name: "empty is not an observation", wantErr: core.ErrGoogleIdentityContract},
		{name: "plus without country digit", text: "+", wantErr: core.ErrGoogleIdentityContract},
		{name: "missing plus", text: "14165550123", wantErr: core.ErrGoogleIdentityContract},
		{name: "country digit cannot be zero", text: "+012345", wantErr: core.ErrGoogleIdentityContract},
		{name: "interior zero remains decimal", text: "+100000"},
		{name: "highest decimal country digit", text: "+9"},
		{name: "alphabetic national part refuses", text: "+1abc", wantErr: core.ErrGoogleIdentityContract},
		{name: "embedded plus refuses", text: "+1+2", wantErr: core.ErrGoogleIdentityContract},
		{name: "formatting space refuses", text: "+1 2", wantErr: core.ErrGoogleIdentityContract},
		{name: "trailing whitespace refuses", text: "+12 ", wantErr: core.ErrGoogleIdentityContract},
		{name: "leading whitespace refuses", text: " +12", wantErr: core.ErrGoogleIdentityContract},
		{name: "hyphen refuses", text: "+1-2", wantErr: core.ErrGoogleIdentityContract},
		{name: "Unicode digits refuse", text: "+1١٢", wantErr: core.ErrGoogleIdentityContract},
		{name: "NUL refuses", text: "+1\x002", wantErr: core.ErrGoogleIdentityContract},
		{name: "invalid UTF8 refuses", text: "+1\xff", wantErr: core.ErrGoogleIdentityContract},
		{name: "native range lower punctuation refuses", text: "+1/", wantErr: core.ErrGoogleIdentityContract},
		{name: "native range upper punctuation refuses", text: "+1:", wantErr: core.ErrGoogleIdentityContract},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePhoneE164(tc.text)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (PhoneE164{}) {
					t.Fatalf("phone=(%v,%v), want zero/%v", got, err, tc.wantErr)
				}
			} else if err != nil || got.Validate() != nil || got.String() != tc.text {
				t.Fatalf("phone=(%q,%v), want exact %q", got.String(), err, tc.text)
			}
		})
	}
}

func FuzzPhoneE164SemanticClosure(f *testing.F) {
	phone, err := ParsePhoneE164("+14165550123")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(phone.String())
	f.Add("+1")
	f.Add("+123456789012345")
	f.Add("+1234567890123456")
	f.Add("")
	f.Add("+01")
	f.Add("+1\x00")
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			return
		}
		got, err := ParsePhoneE164(input)
		want := len(input) >= 2 && len(input) <= 16 && strings.HasPrefix(input, "+") && input[1] != '0'
		if want {
			_, digits, ok := strings.Cut(input, "+")
			want = ok && strings.Trim(digits, "0123456789") == ""
		}
		if (err == nil) != want {
			t.Fatalf("phone=(%v,%v), syntactic international decimal admission=%t", got, err, want)
		}
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (PhoneE164{}) {
				t.Fatalf("phone refusal=(%v,%v), want zero/contract", got, err)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("admitted phone=%v, want exact validated input", got)
		}
		again, err := ParsePhoneE164(got.String())
		if err != nil || again != got {
			t.Fatalf("phone canonical=(%v,%v), want %v", again, err, got)
		}
	})
}

func FuzzBrowserPeopleProfileNativeReply(f *testing.F) {
	canonical, err := core.MarshalCanonicalJSONDocument(peopleProfileSeed())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	foreign := peopleProfileSeed()
	foreign.ResourceName = "people/foreign"
	encoded, err := core.MarshalCanonicalJSONDocument(foreign)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	partial := peopleProfileSeed()
	partial.Birthdays[0].Date.Year = 0
	encoded, err = core.MarshalCanonicalJSONDocument(partial)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	f.Add([]byte("{"))
	f.Add([]byte("null"))
	f.Add([]byte(`{"phoneNumbers":"wrong-type"}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 8192 {
			return
		}
		client := peopleProfileLocalClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(body); err != nil {
				t.Errorf("People fuzz provider write=%v, want nil", err)
			}
		}))
		got, err := browserPeopleProfile(t.Context(), BrowserAuthorizationCompleteRequest{Client: client, Profile: BrowserProfileRequest{Name: true, PhoneNumber: true, Birthday: true}}, peopleCredential(t, nil))
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (BrowserProfile{}) {
				t.Fatalf("People refusal=(%+v,%v), want zero/contract", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("admitted optional profile invalid: %+v %v", got, err)
		}
		var person people.Person
		nativeErr := json.Unmarshal(body, &person)
		if nativeErr != nil || person.ResourceName != "people/stable-google-subject" {
			if got.FullName != (core.OAuthProfileName{}) || got.GivenName != (core.OAuthProfileName{}) || got.FamilyName != (core.OAuthProfileName{}) || got.PhoneNumber != (PhoneE164{}) || got.BirthdayOutcome == core.OAuthProfileAvailable || got.ObservationError() == nil {
				t.Fatalf("foreign/malformed People reply escaped observations: %+v native=%v", got, nativeErr)
			}
			return
		}
		if got.PhoneOutcome == core.OAuthProfileAvailable {
			matches := 0
			for _, phone := range person.PhoneNumbers {
				if phone != nil && phone.Metadata != nil && phone.Metadata.Primary && (phone.Metadata.Source == nil || phone.Metadata.Source.Type == "" || phone.Metadata.Source.Type == "PROFILE") && phone.CanonicalForm == got.PhoneNumber.String() && phone.Metadata.Verified == got.PhoneVerified {
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("phone observation=%q verified=%t has %d matching primary PROFILE facts", got.PhoneNumber.String(), got.PhoneVerified, matches)
			}
		}
		if got.BirthdayOutcome == core.OAuthProfileAvailable {
			date, err := got.Birthday.Coordinates()
			if err != nil {
				t.Fatal(err)
			}
			matches := 0
			for _, birthday := range person.Birthdays {
				if birthday != nil && birthday.Metadata != nil && birthday.Metadata.Primary && (birthday.Metadata.Source == nil || birthday.Metadata.Source.Type == "" || birthday.Metadata.Source.Type == "PROFILE") && birthday.Date != nil && birthday.Date.Year != 0 && birthday.Date.Year == int64(date.Year) && birthday.Date.Month == int64(date.Month) && birthday.Date.Day == int64(date.Day) {
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("birthday observation=%+v has %d matching complete primary PROFILE facts", date, matches)
			}
		}
		for _, part := range []struct {
			value core.OAuthProfileName
			index int
		}{{got.FullName, 0}, {got.GivenName, 1}, {got.FamilyName, 2}} {
			if part.value == (core.OAuthProfileName{}) {
				continue
			}
			matches := 0
			for _, name := range person.Names {
				if name == nil || name.Metadata == nil || !name.Metadata.Primary || name.Metadata.Source != nil && name.Metadata.Source.Type != "" && name.Metadata.Source.Type != "PROFILE" {
					continue
				}
				text := []string{name.DisplayName, name.GivenName, name.FamilyName}[part.index]
				if text == part.value.String() {
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("name part%d=%q has %d matching primary PROFILE facts", part.index, part.value.String(), matches)
			}
		}
	})
}
