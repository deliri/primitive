package gcsobjects_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gcsobjects"
)

type objectAddressParseCase struct {
	wantErr    error
	name       string
	value      string
	wantBucket string
	wantObject string
}

func TestParseObjectAddressConfinesCanonicalGCSIdentity(t *testing.T) {
	t.Parallel()

	positive := []objectAddressParseCase{
		{name: "flat evidence object", value: "https://storage.googleapis.com/evidence-bucket/proof.png", wantBucket: "evidence-bucket", wantObject: "proof.png"},
		{name: "nested evidence object", value: "https://storage.googleapis.com/evidence-bucket/taskmanager/task/digest/proof.png", wantBucket: "evidence-bucket", wantObject: "taskmanager/task/digest/proof.png"},
		{name: "minimum bucket and object", value: "https://storage.googleapis.com/abc/a", wantBucket: "abc", wantObject: "a"},
		{name: "dotted bucket", value: "https://storage.googleapis.com/a.b.c/object", wantBucket: "a.b.c", wantObject: "object"},
		{name: "underscored bucket", value: "https://storage.googleapis.com/a_b/object", wantBucket: "a_b", wantObject: "object"},
		{name: "space encoded by projection", value: "https://storage.googleapis.com/evidence-bucket/proof%20image.png", wantBucket: "evidence-bucket", wantObject: "proof image.png"},
		{name: "question mark held in object", value: "https://storage.googleapis.com/evidence-bucket/proof%3Ffinal.png", wantBucket: "evidence-bucket", wantObject: "proof?final.png"},
		{name: "fragment marker held in object", value: "https://storage.googleapis.com/evidence-bucket/proof%23final.png", wantBucket: "evidence-bucket", wantObject: "proof#final.png"},
		{name: "percent held in object", value: "https://storage.googleapis.com/evidence-bucket/proof%25final.png", wantBucket: "evidence-bucket", wantObject: "proof%final.png"},
		{name: "utf8 object", value: "https://storage.googleapis.com/evidence-bucket/qu%C3%A9bec.png", wantBucket: "evidence-bucket", wantObject: "québec.png"},
	}
	negative := []objectAddressParseCase{
		{name: "http transport", value: "http://storage.googleapis.com/evidence-bucket/proof.png", wantErr: core.ErrObjectStoreContract},
		{name: "foreign host", value: "https://example.com/evidence-bucket/proof.png", wantErr: core.ErrObjectStoreContract},
		{name: "lookalike host", value: "https://storage.googleapis.com.example.com/evidence-bucket/proof.png", wantErr: core.ErrObjectStoreContract},
		{name: "uppercase host", value: "https://STORAGE.GOOGLEAPIS.COM/evidence-bucket/proof.png", wantErr: core.ErrObjectStoreContract},
		{name: "explicit port", value: "https://storage.googleapis.com:443/evidence-bucket/proof.png", wantErr: core.ErrObjectStoreContract},
		{name: "query capability", value: "https://storage.googleapis.com/evidence-bucket/proof.png?token=secret", wantErr: core.ErrObjectStoreContract},
		{name: "missing bucket and object", value: "https://storage.googleapis.com/", wantErr: core.ErrObjectStoreContract},
		{name: "missing object", value: "https://storage.googleapis.com/evidence-bucket", wantErr: core.ErrObjectStoreContract},
		{name: "invalid bucket", value: "https://storage.googleapis.com/Google/proof.png", wantErr: core.ErrObjectStoreContract},
		{name: "virtual hosted address", value: "https://evidence-bucket.storage.googleapis.com/proof.png", wantErr: core.ErrObjectStoreContract},
	}
	boundary := []objectAddressParseCase{
		{name: "object one byte", value: "https://storage.googleapis.com/abc/a", wantBucket: "abc", wantObject: "a"},
		{name: "object nested one byte leaves", value: "https://storage.googleapis.com/abc/a/b", wantBucket: "abc", wantObject: "a/b"},
		{name: "object leading slash encoded", value: "https://storage.googleapis.com/abc/%2Fproof", wantErr: core.ErrObjectStoreContract},
		{name: "object trailing slash", value: "https://storage.googleapis.com/abc/proof/", wantBucket: "abc", wantObject: "proof/"},
		{name: "object dot within name", value: "https://storage.googleapis.com/abc/a.b", wantBucket: "abc", wantObject: "a.b"},
		{name: "object repeated separator", value: "https://storage.googleapis.com/abc/a//b", wantBucket: "abc", wantObject: "a//b"},
		{name: "object escaped newline", value: "https://storage.googleapis.com/abc/a%0Ab", wantErr: core.ErrObjectStoreContract},
		{name: "object escaped carriage return", value: "https://storage.googleapis.com/abc/a%0Db", wantErr: core.ErrObjectStoreContract},
		{name: "bucket too short", value: "https://storage.googleapis.com/ab/proof", wantErr: core.ErrObjectStoreContract},
		{name: "bucket starts dash", value: "https://storage.googleapis.com/-abc/proof", wantErr: core.ErrObjectStoreContract},
		{name: "bucket ends dash", value: "https://storage.googleapis.com/abc-/proof", wantErr: core.ErrObjectStoreContract},
		{name: "bucket adjacent dots", value: "https://storage.googleapis.com/ab..cd/proof", wantErr: core.ErrObjectStoreContract},
		{name: "bucket ip address", value: "https://storage.googleapis.com/127.0.0.1/proof", wantErr: core.ErrObjectStoreContract},
		{name: "bucket reserved google prefix", value: "https://storage.googleapis.com/google-proof/proof", wantErr: core.ErrObjectStoreContract},
		{name: "encoded bucket separator", value: "https://storage.googleapis.com/abc%2Fdef/proof", wantErr: core.ErrObjectStoreContract},
		{name: "path double leading slash", value: "https://storage.googleapis.com//abc/proof", wantErr: core.ErrObjectStoreContract},
		{name: "empty query marker", value: "https://storage.googleapis.com/abc/proof?", wantErr: core.ErrObjectStoreContract},
		{name: "lowercase path escape", value: "https://storage.googleapis.com/abc/proof%2fimage", wantErr: core.ErrObjectStoreContract},
		{name: "encoded unreserved byte", value: "https://storage.googleapis.com/abc/%70roof", wantErr: core.ErrObjectStoreContract},
		{name: "zero endpoint", wantErr: core.ErrObjectStoreContract},
	}

	runObjectAddressParseCases(t, "positive", positive)
	runObjectAddressParseCases(t, "negative", negative)
	runObjectAddressParseCases(t, "boundary", boundary)
}

func runObjectAddressParseCases(t *testing.T, group string, cases []objectAddressParseCase) {
	t.Helper()
	for _, testCase := range cases {
		t.Run(group+"/"+testCase.name, func(t *testing.T) {
			t.Parallel()
			var endpoint core.HTTPEndpoint
			if testCase.value != "" {
				parsed, err := core.ParseHTTPEndpoint(testCase.value)
				if err != nil {
					t.Fatalf("core.ParseHTTPEndpoint(%q) error = %v, want nil general HTTP syntax", testCase.value, err)
				}
				endpoint = parsed
			}
			got, gotErr := gcsobjects.ParseObjectAddress(endpoint)
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, testCase.wantErr) || got != (gcsobjects.GCSObjectAddress{}) {
					t.Fatalf("ParseObjectAddress() = (%+v, %v), want (zero, errors.Is(..., %v))", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ParseObjectAddress() error = %v, want nil", gotErr)
			}
			if got.Bucket().String() != testCase.wantBucket || got.Name().String() != testCase.wantObject {
				t.Fatalf("ParseObjectAddress() = (%q, %q), want (%q, %q)", got.Bucket().String(), got.Name().String(), testCase.wantBucket, testCase.wantObject)
			}
			roundTrip, err := gcsobjects.ObjectAddress(got.Bucket(), got.Name())
			if err != nil || roundTrip != endpoint {
				t.Fatalf("ObjectAddress(parsed) = (%q, %v), want (%q, nil)", roundTrip, err, endpoint)
			}
		})
	}
}

func FuzzParseObjectAddressPreservesExactCanonicalIdentity(f *testing.F) {
	validBucket, err := gcsobjects.ParseGCSBucket("evidence-bucket")
	if err != nil {
		f.Fatalf("ParseGCSBucket(seed) error = %v, want nil", err)
	}
	validObject, err := gcsobjects.ParseGCSObjectName("taskmanager/task/digest/proof image.png")
	if err != nil {
		f.Fatalf("ParseGCSObjectName(seed) error = %v, want nil", err)
	}
	canonical, err := gcsobjects.ObjectAddress(validBucket, validObject)
	if err != nil {
		f.Fatalf("ObjectAddress(seed) error = %v, want nil", err)
	}
	f.Add(canonical.String())
	f.Add("http://storage.googleapis.com/evidence-bucket/proof.png")
	f.Add("https://storage.googleapis.com.example.com/evidence-bucket/proof.png")
	f.Add("https://storage.googleapis.com/evidence-bucket/proof.png?secret=value")
	f.Fuzz(func(t *testing.T, value string) {
		endpoint, endpointErr := core.ParseHTTPEndpoint(value)
		if endpointErr != nil {
			return
		}
		got, gotErr := gcsobjects.ParseObjectAddress(endpoint)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (gcsobjects.GCSObjectAddress{}) {
				t.Fatalf("ParseObjectAddress(%q) = (%+v, %v), want zero and object-store identity", value, got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted ParseObjectAddress(%q).Validate() error = %v, want nil", value, err)
		}
		roundTrip, err := gcsobjects.ObjectAddress(got.Bucket(), got.Name())
		if err != nil || roundTrip != endpoint {
			t.Fatalf("accepted ParseObjectAddress(%q) round trip = (%q, %v), want (%q, nil)", value, roundTrip, err, endpoint)
		}
	})
}
