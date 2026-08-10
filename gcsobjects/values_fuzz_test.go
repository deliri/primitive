package gcsobjects

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzParseGCSProjectIDSemanticClosure(f *testing.F) {
	seed, err := ParseGCSProjectID("primitive-project")
	if err != nil {
		f.Fatalf("ParseGCSProjectID(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("Project")

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseGCSProjectID(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSProjectID{}) {
				t.Fatalf("ParseGCSProjectID(%q) = (%v, %v), want zero typed rejection", input, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseGCSProjectID(%q) = %q with Validate %v, want exact valid input",
				input, got.String(), got.Validate())
		}
		roundTrip, err := ParseGCSProjectID(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("GCSProjectID canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzParseGCSLocationSemanticClosure(f *testing.F) {
	seed, err := ParseGCSLocation("northamerica-northeast2")
	if err != nil {
		f.Fatalf("ParseGCSLocation(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("region/path")

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseGCSLocation(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSLocation{}) {
				t.Fatalf("ParseGCSLocation(%q) = (%v, %v), want zero typed rejection", input, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseGCSLocation(%q) = %q with Validate %v, want exact valid input",
				input, got.String(), got.Validate())
		}
		roundTrip, err := ParseGCSLocation(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("GCSLocation canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzParseGCSObjectSegmentSemanticClosure(f *testing.F) {
	seed, err := ParseGCSObjectSegment("results.json")
	if err != nil {
		f.Fatalf("ParseGCSObjectSegment(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("parent/leaf")

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseGCSObjectSegment(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSObjectSegment{}) {
				t.Fatalf("ParseGCSObjectSegment(%q) = (%v, %v), want zero typed rejection", input, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseGCSObjectSegment(%q) = %q with Validate %v, want exact valid input",
				input, got.String(), got.Validate())
		}
		roundTrip, err := ParseGCSObjectSegment(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("GCSObjectSegment canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzParseGCSBucketSemanticClosure(f *testing.F) {
	seed, err := ParseGCSBucket("primitive-custody")
	if err != nil {
		f.Fatalf("ParseGCSBucket(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("Invalid_Bucket")

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseGCSBucket(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSBucket{}) {
				t.Fatalf("ParseGCSBucket(%q) = (%v, %v), want zero typed rejection", input, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseGCSBucket(%q) = %q with Validate %v, want exact valid input",
				input, got.String(), got.Validate())
		}
		roundTrip, err := ParseGCSBucket(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("GCSBucket canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzParseGCSObjectNameSemanticClosure(f *testing.F) {
	seed, err := ParseGCSObjectName("accounts/chits/results.json")
	if err != nil {
		f.Fatalf("ParseGCSObjectName(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add(".well-known/acme-challenge/token")

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseGCSObjectName(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSObjectName{}) {
				t.Fatalf("ParseGCSObjectName(%q) = (%v, %v), want zero typed rejection", input, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseGCSObjectName(%q) = %q with Validate %v, want exact valid input",
				input, got.String(), got.Validate())
		}
		roundTrip, err := ParseGCSObjectName(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("GCSObjectName canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzParseGCSObjectPrefixSemanticClosure(f *testing.F) {
	seed, err := ParseGCSObjectPrefix("accounts/chits/")
	if err != nil {
		f.Fatalf("ParseGCSObjectPrefix(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("accounts//chits/")

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseGCSObjectPrefix(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSObjectPrefix{}) {
				t.Fatalf("ParseGCSObjectPrefix(%q) = (%v, %v), want zero typed rejection", input, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseGCSObjectPrefix(%q) = %q with Validate %v, want exact valid input",
				input, got.String(), got.Validate())
		}
		roundTrip, err := ParseGCSObjectPrefix(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("GCSObjectPrefix canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzParseGCSCacheControlSemanticClosure(f *testing.F) {
	seed, err := ParseGCSCacheControl("public, max-age=3600")
	if err != nil {
		f.Fatalf("ParseGCSCacheControl(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("public\r\nInjected: true")

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := ParseGCSCacheControl(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSCacheControl{}) {
				t.Fatalf("ParseGCSCacheControl(%q) = (%v, %v), want zero typed rejection", input, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != input {
			t.Fatalf("ParseGCSCacheControl(%q) = %q with Validate %v, want exact valid input",
				input, got.String(), got.Validate())
		}
		roundTrip, err := ParseGCSCacheControl(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("GCSCacheControl canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}
