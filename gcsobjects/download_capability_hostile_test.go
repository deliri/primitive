package gcsobjects

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/googleapi"
)

func TestGCSDownloadCapabilityRequestHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	ordinary := gcsDownloadCapabilityRequest(t)
	maximum := gcsCapabilityMaximumLifetime(t)
	minimum := gcsCapabilityLifetime(t, 1)
	maximumMinusOne := gcsCapabilityLifetime(t, maximum.Nanoseconds()-1)
	maximumPlusOne := gcsCapabilityLifetime(t, maximum.Nanoseconds()+1)

	type requestCase struct {
		mutate     func(*GCSDownloadCapabilityRequest)
		name       string
		wantAccept bool
	}
	cases := []requestCase{
		{name: "ordinary private object request is admitted", mutate: func(*GCSDownloadCapabilityRequest) {}, wantAccept: true},
		{name: "one nanosecond capability is admitted", mutate: setGCSDownloadLifetime(minimum), wantAccept: true},
		{name: "two nanosecond capability is admitted", mutate: setGCSDownloadLifetime(gcsCapabilityLifetime(t, 2)), wantAccept: true},
		{name: "one second capability is admitted", mutate: setGCSDownloadLifetime(gcsCapabilityLifetime(t, int64(temporal.NanosecondsPerSecond))), wantAccept: true},
		{name: "one minute capability is admitted", mutate: setGCSDownloadLifetime(gcsCapabilityDurationMinutes(t, 1)), wantAccept: true},
		{name: "one hour capability is admitted", mutate: setGCSDownloadLifetime(gcsCapabilityDurationHours(t, 1)), wantAccept: true},
		{name: "one day capability is admitted", mutate: setGCSDownloadLifetime(gcsCapabilityDurationDays(t, 1)), wantAccept: true},
		{name: "one nanosecond below provider maximum is admitted", mutate: setGCSDownloadLifetime(maximumMinusOne), wantAccept: true},
		{name: "exact provider maximum is admitted", mutate: setGCSDownloadLifetime(maximum), wantAccept: true},
		{name: "alternate validated object identity is admitted", mutate: func(value *GCSDownloadCapabilityRequest) {
			value.Bucket = parsedGCSBucket(t, "primitive-display-proof")
			value.Name = parsedGCSObjectName(t, "proof/alternate/image.webp")
		}, wantAccept: true},

		{name: "missing bucket is refused", mutate: clearGCSDownloadBucket},
		{name: "missing object name is refused", mutate: clearGCSDownloadName},
		{name: "missing service account is refused", mutate: clearGCSDownloadServiceAccount},
		{name: "missing lifetime is refused", mutate: clearGCSDownloadLifetime},
		{name: "missing bucket and object name are refused", mutate: combineGCSDownloadMutations(clearGCSDownloadBucket, clearGCSDownloadName)},
		{name: "missing bucket and service account are refused", mutate: combineGCSDownloadMutations(clearGCSDownloadBucket, clearGCSDownloadServiceAccount)},
		{name: "missing object name and service account are refused", mutate: combineGCSDownloadMutations(clearGCSDownloadName, clearGCSDownloadServiceAccount)},
		{name: "missing every provider identity is refused", mutate: combineGCSDownloadMutations(clearGCSDownloadBucket, clearGCSDownloadName, clearGCSDownloadServiceAccount)},
		{name: "zero request is refused", mutate: func(value *GCSDownloadCapabilityRequest) { *value = GCSDownloadCapabilityRequest{} }},
		{name: "one nanosecond beyond provider maximum is refused", mutate: setGCSDownloadLifetime(maximumPlusOne)},

		{name: "boundary with only bucket present is refused", mutate: retainGCSDownloadFields(true, false, false, false)},
		{name: "boundary with only object name present is refused", mutate: retainGCSDownloadFields(false, true, false, false)},
		{name: "boundary with only service account present is refused", mutate: retainGCSDownloadFields(false, false, true, false)},
		{name: "boundary with only lifetime present is refused", mutate: retainGCSDownloadFields(false, false, false, true)},
		{name: "boundary with bucket and object name present is refused", mutate: retainGCSDownloadFields(true, true, false, false)},
		{name: "boundary with bucket and service account present is refused", mutate: retainGCSDownloadFields(true, false, true, false)},
		{name: "boundary with bucket and lifetime present is refused", mutate: retainGCSDownloadFields(true, false, false, true)},
		{name: "boundary with object name and service account present is refused", mutate: retainGCSDownloadFields(false, true, true, false)},
		{name: "boundary with object name and lifetime present is refused", mutate: retainGCSDownloadFields(false, true, false, true)},
		{name: "boundary with service account and lifetime present is refused", mutate: retainGCSDownloadFields(false, false, true, true)},
		{name: "boundary missing only bucket is refused", mutate: retainGCSDownloadFields(false, true, true, true)},
		{name: "boundary missing only object name is refused", mutate: retainGCSDownloadFields(true, false, true, true)},
		{name: "boundary missing only service account is refused", mutate: retainGCSDownloadFields(true, true, false, true)},
		{name: "boundary missing only lifetime is refused", mutate: retainGCSDownloadFields(true, true, true, false)},
		{name: "boundary zero lifetime is refused", mutate: setGCSDownloadLifetime(temporal.Duration{})},
		{name: "boundary one nanosecond lifetime is admitted", mutate: setGCSDownloadLifetime(minimum), wantAccept: true},
		{name: "boundary two nanosecond lifetime is admitted", mutate: setGCSDownloadLifetime(gcsCapabilityLifetime(t, 2)), wantAccept: true},
		{name: "boundary maximum minus one nanosecond is admitted", mutate: setGCSDownloadLifetime(maximumMinusOne), wantAccept: true},
		{name: "boundary exact maximum lifetime is admitted", mutate: setGCSDownloadLifetime(maximum), wantAccept: true},
		{name: "boundary maximum plus one nanosecond is refused", mutate: setGCSDownloadLifetime(maximumPlusOne)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ordinary
			tc.mutate(&got)
			gotErr := got.Validate()
			if tc.wantAccept {
				if gotErr != nil {
					t.Fatalf("GCSDownloadCapabilityRequest.Validate() error = %v, want nil", gotErr)
				}
				return
			}
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("GCSDownloadCapabilityRequest.Validate() error = %v, want errors.Is(..., %v)", gotErr, core.ErrObjectStoreContract)
			}
		})
	}
}

func TestGCSDownloadCapabilityIssuerLayerTriadUsesOfficialSDKSigningLeaf(t *testing.T) {
	t.Parallel()

	type issuerCase struct {
		wantErr         error
		name            string
		wantCalls       uint64
		outcome         gcsCapabilityProviderOutcome
		cancelIngress   bool
		wantCapability  bool
		wantProviderErr bool
	}
	cases := []issuerCase{
		{
			name:    "positive official IAM response releases one exact receive-only capability",
			outcome: gcsCapabilityProviderOutcomeSigned, wantCalls: 1,
			wantCapability: true,
		},
		{
			name:    "negative provider refusal preserves typed destination identity and releases no capability",
			outcome: gcsCapabilityProviderOutcomeRefused, wantCalls: 1,
			wantErr: core.ErrObjectStoreDestination, wantProviderErr: true,
		},
		{
			name:    "negative empty provider signature preserves typed destination identity and releases no capability",
			outcome: gcsCapabilityProviderOutcomeEmptySignature, wantCalls: 1,
			wantErr: core.ErrObjectStoreDestination,
		},
		{
			name:    "neutral canceled ingress performs no provider request and releases no capability",
			outcome: gcsCapabilityProviderOutcomeSigned, cancelIngress: true,
			wantErr: context.Canceled,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issuer, calls := gcsCapabilityIssuer(t, tc.outcome)
			ctx := context.Background()
			if tc.cancelIngress {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			got, gotErr := IssueGCSDownloadCapability(ctx, issuer, gcsDownloadCapabilityRequest(t))
			if gotCalls := calls.Load(); gotCalls != tc.wantCalls {
				t.Fatalf("official IAM SignBlob calls = %d, want %d", gotCalls, tc.wantCalls)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || !got.IsZero() {
					t.Fatalf("IssueGCSDownloadCapability() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				if tc.wantProviderErr {
					providerErr, gotProviderIdentity := errors.AsType[*googleapi.Error](gotErr)
					if !gotProviderIdentity || providerErr.Code != 500 {
						t.Fatalf("IssueGCSDownloadCapability() provider error = (%v, %t), want Google API status 500", providerErr, gotProviderIdentity)
					}
				}
				return
			}
			if gotErr != nil || !tc.wantCapability || got.IsZero() || got.Validate() != nil {
				t.Fatalf("IssueGCSDownloadCapability() = (%v, %v), want validated nonzero projection and nil", got, gotErr)
			}
			encoded, gotMarshalErr := got.MarshalJSON()
			if gotMarshalErr != nil || len(encoded) > objectstore.CapabilityJSONMaximumBytes {
				t.Fatalf("DownloadCapabilityProjection.MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), gotMarshalErr)
			}
			var received objectstore.DownloadCapability
			if gotDecodeErr := json.Unmarshal(encoded, &received); gotDecodeErr != nil || received.Validate() != nil {
				t.Fatalf("DownloadCapability.UnmarshalJSON() = (%v, %v), want validated receive-only capability and nil", received, gotDecodeErr)
			}
			gotProvider, gotProviderErr := received.Provider()
			if gotProvider != objectstore.ProviderGoogleCloudStorage || gotProviderErr != nil {
				t.Fatalf("DownloadCapability.Provider() = (%v, %v), want (%v, nil)", gotProvider, gotProviderErr, objectstore.ProviderGoogleCloudStorage)
			}
			gotTarget, gotTargetErr := received.Target()
			if gotTargetErr != nil || gotTarget.ValidateFor(objectstore.ProviderGoogleCloudStorage) != nil {
				t.Fatalf("DownloadCapability.Target() = (%v, %v), want GCS-validated target and nil", gotTarget, gotTargetErr)
			}
		})
	}
}

func BenchmarkGCSDownloadCapabilityRequestValidation(b *testing.B) {
	request := gcsDownloadCapabilityRequest(b)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := request.Validate(); err != nil {
			b.Fatalf("GCSDownloadCapabilityRequest.Validate() error = %v, want nil", err)
		}
	}
}

func gcsDownloadCapabilityRequest(t testing.TB) GCSDownloadCapabilityRequest {
	t.Helper()

	upload := gcsCapabilityRequest(t)
	return GCSDownloadCapabilityRequest{
		Bucket: upload.Bucket, Name: upload.Name,
		ServiceAccount: upload.ServiceAccount, Lifetime: upload.Lifetime,
	}
}

func gcsCapabilityMaximumLifetime(t testing.TB) temporal.Duration {
	t.Helper()

	duration, err := temporal.DurationFromDays(GCSCapabilityMaximumDays)
	if err != nil {
		t.Fatalf("temporal.DurationFromDays(%d) error = %v, want nil", GCSCapabilityMaximumDays, err)
	}
	return duration
}

func gcsCapabilityLifetime(t testing.TB, nanoseconds int64) temporal.Duration {
	t.Helper()

	duration, err := temporal.DurationFromNanoseconds(nanoseconds)
	if err != nil {
		t.Fatalf("temporal.DurationFromNanoseconds(%d) error = %v, want nil", nanoseconds, err)
	}
	return duration
}

func gcsCapabilityDurationMinutes(t testing.TB, minutes uint64) temporal.Duration {
	t.Helper()

	duration, err := temporal.DurationFromMinutes(minutes)
	if err != nil {
		t.Fatalf("temporal.DurationFromMinutes(%d) error = %v, want nil", minutes, err)
	}
	return duration
}

func gcsCapabilityDurationHours(t testing.TB, hours uint64) temporal.Duration {
	t.Helper()

	duration, err := temporal.DurationFromHours(hours)
	if err != nil {
		t.Fatalf("temporal.DurationFromHours(%d) error = %v, want nil", hours, err)
	}
	return duration
}

func gcsCapabilityDurationDays(t testing.TB, days uint64) temporal.Duration {
	t.Helper()

	duration, err := temporal.DurationFromDays(days)
	if err != nil {
		t.Fatalf("temporal.DurationFromDays(%d) error = %v, want nil", days, err)
	}
	return duration
}

func setGCSDownloadLifetime(lifetime temporal.Duration) func(*GCSDownloadCapabilityRequest) {
	return func(value *GCSDownloadCapabilityRequest) { value.Lifetime = lifetime }
}

func clearGCSDownloadBucket(value *GCSDownloadCapabilityRequest) { value.Bucket = GCSBucket{} }
func clearGCSDownloadName(value *GCSDownloadCapabilityRequest)   { value.Name = GCSObjectName{} }
func clearGCSDownloadServiceAccount(value *GCSDownloadCapabilityRequest) {
	value.ServiceAccount = GCSServiceAccount{}
}
func clearGCSDownloadLifetime(value *GCSDownloadCapabilityRequest) {
	value.Lifetime = temporal.Duration{}
}

func combineGCSDownloadMutations(mutations ...func(*GCSDownloadCapabilityRequest)) func(*GCSDownloadCapabilityRequest) {
	return func(value *GCSDownloadCapabilityRequest) {
		for _, mutate := range mutations {
			mutate(value)
		}
	}
}

func retainGCSDownloadFields(bucket, name, account, lifetime bool) func(*GCSDownloadCapabilityRequest) {
	return func(value *GCSDownloadCapabilityRequest) {
		if !bucket {
			clearGCSDownloadBucket(value)
		}
		if !name {
			clearGCSDownloadName(value)
		}
		if !account {
			clearGCSDownloadServiceAccount(value)
		}
		if !lifetime {
			clearGCSDownloadLifetime(value)
		}
	}
}
