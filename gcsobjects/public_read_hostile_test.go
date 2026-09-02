package gcsobjects

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/iam"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/googleapi"
	storageapi "google.golang.org/api/storage/v1"
)

func TestGCSBucketPublicReadRequestValidateRefusesUnsetBucket(t *testing.T) {
	t.Parallel()

	bucket := parsedGCSBucket(t, gcsProviderBucketText)
	cases := []struct {
		wantErr error
		name    string
		request GCSBucketPublicReadRequest
	}{
		{name: "positive request names one validated bucket", request: GCSBucketPublicReadRequest{Bucket: bucket}},
		{name: "negative zero request refuses an unset bucket", request: GCSBucketPublicReadRequest{}, wantErr: core.ErrObjectStoreContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.request.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("GCSBucketPublicReadRequest.Validate() error = %v, want errors.Is(..., %v)", gotErr, tc.wantErr)
			}
		})
	}
}

func TestGCSBucketPublicReadChangeExhaustsEveryUint8State(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		change := GCSBucketPublicReadChange(raw)
		wantValid := change == GCSBucketPublicReadUnchanged || change == GCSBucketPublicReadGranted
		gotErr := change.Validate()
		if gotValid := change.IsValid(); gotValid != wantValid {
			t.Fatalf("GCSBucketPublicReadChange(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		var wantErr error = core.ErrObjectStoreContract
		if wantValid {
			wantErr = nil
		}
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("GCSBucketPublicReadChange(%d).Validate() error = %v, want errors.Is(..., %v)", raw, gotErr, wantErr)
		}
		gotDiagnostic := change.String()
		if wantValid && gotDiagnostic == "" {
			t.Fatalf("GCSBucketPublicReadChange(%d).String() = %q, want a non-empty published diagnostic", raw, gotDiagnostic)
		}
		if !wantValid && gotDiagnostic != "" {
			t.Fatalf("GCSBucketPublicReadChange(%d).String() = %q, want empty for an unpublished state", raw, gotDiagnostic)
		}
	}
}

func TestGCSBucketPublicReadGrantContractLayerTriadRejectsUnsealedEvidence(t *testing.T) {
	t.Parallel()

	bucket := parsedGCSBucket(t, gcsProviderBucketText)
	cases := []struct {
		wantErr error
		name    string
		grant   GCSBucketPublicReadGrant
	}{
		{name: "neutral zero grant is unsealed and refused", wantErr: core.ErrObjectStoreContract},
		{name: "positive sealed unchanged grant is admitted", grant: GCSBucketPublicReadGrant{bucket: bucket, change: GCSBucketPublicReadUnchanged, set: true}},
		{name: "positive sealed granted grant is admitted", grant: GCSBucketPublicReadGrant{bucket: bucket, change: GCSBucketPublicReadGranted, set: true}},
		{name: "negative sealed grant refuses zero bucket", grant: GCSBucketPublicReadGrant{change: GCSBucketPublicReadGranted, set: true}, wantErr: core.ErrObjectStoreContract},
		{name: "negative sealed grant refuses unknown change", grant: GCSBucketPublicReadGrant{bucket: bucket, change: GCSBucketPublicReadChangeUnknown, set: true}, wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.grant.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("GCSBucketPublicReadGrant.Validate() error = %v, want errors.Is(..., %v)", gotErr, tc.wantErr)
			}
		})
	}
}

type gcsPublicReadProviderCase struct {
	wantErr          error
	name             string
	wantCauseErrors  []error
	initial          gcsPublicReadProviderResponse
	set              gcsPublicReadProviderResponse
	confirmed        gcsPublicReadProviderResponse
	wantProviderCode int
	wantPolicyGets   int64
	wantPolicySets   int64
	wantChange       GCSBucketPublicReadChange
	wantRuntimeCause bool
}

type gcsPublicReadProviderResponseKind uint8

const (
	gcsPublicReadProviderResponseUnknown gcsPublicReadProviderResponseKind = iota
	gcsPublicReadProviderPolicy
	gcsPublicReadProviderWrittenPolicy
	gcsPublicReadProviderStatus
	gcsPublicReadProviderMalformed
	gcsPublicReadProviderNull
	gcsPublicReadProviderAccept
	gcsPublicReadProviderRaw
)

type gcsPublicReadProviderResponse struct {
	raw    string
	policy storageapi.Policy
	status int
	kind   gcsPublicReadProviderResponseKind
}

func gcsProviderPolicy(policy storageapi.Policy) gcsPublicReadProviderResponse {
	return gcsPublicReadProviderResponse{kind: gcsPublicReadProviderPolicy, policy: policy}
}

func gcsProviderWrittenPolicy() gcsPublicReadProviderResponse {
	return gcsPublicReadProviderResponse{kind: gcsPublicReadProviderWrittenPolicy}
}

func gcsProviderStatus(status int) gcsPublicReadProviderResponse {
	return gcsPublicReadProviderResponse{kind: gcsPublicReadProviderStatus, status: status}
}

func gcsProviderMalformed() gcsPublicReadProviderResponse {
	return gcsPublicReadProviderResponse{kind: gcsPublicReadProviderMalformed}
}

func gcsProviderNull() gcsPublicReadProviderResponse {
	return gcsPublicReadProviderResponse{kind: gcsPublicReadProviderNull}
}

func gcsPublicReadProviderAcceptResponse() gcsPublicReadProviderResponse {
	return gcsPublicReadProviderResponse{kind: gcsPublicReadProviderAccept}
}

func gcsProviderRaw(raw string) gcsPublicReadProviderResponse {
	return gcsPublicReadProviderResponse{kind: gcsPublicReadProviderRaw, raw: raw}
}

// TestGCSBucketPublicReadProviderLayerTriadPreservesPolicyAndProvesAfterState
// drives the official Cloud Storage SDK over real HTTP framing. Positive rows
// add the exact get-only public member and prove it after the write, negative
// rows refuse provider and confirmation failures with no sealed result, and
// neutral rows observe an existing grant without writing policy.
func TestGCSBucketPublicReadProviderLayerTriadPreservesPolicyAndProvesAfterState(t *testing.T) {
	t.Parallel()

	public := func() *storageapi.PolicyBindings { return gcsPublicReadBinding(iam.AllUsers) }
	publicWithServiceAccount := func() *storageapi.PolicyBindings {
		return gcsPublicReadBinding("serviceAccount:reader@example.test", iam.AllUsers)
	}
	publicWithAuthenticated := func() *storageapi.PolicyBindings {
		return gcsPublicReadBinding(iam.AllAuthenticatedUsers, iam.AllUsers)
	}
	targetServiceAccount := func() *storageapi.PolicyBindings {
		return gcsPublicReadBinding("serviceAccount:reader@example.test")
	}
	targetAuthenticated := func() *storageapi.PolicyBindings {
		return gcsPublicReadBinding(iam.AllAuthenticatedUsers)
	}
	owner := func() *storageapi.PolicyBindings {
		return gcsPolicyBinding(iam.Owner, "serviceAccount:owner@example.test")
	}
	viewerPublic := func() *storageapi.PolicyBindings { return gcsPolicyBinding(iam.Viewer, iam.AllUsers) }
	emptyTarget := func() *storageapi.PolicyBindings { return gcsPublicReadBinding() }
	conditionalPublic := func() *storageapi.PolicyBindings {
		binding := gcsPublicReadBinding(iam.AllUsers)
		binding.Condition = &storageapi.Expr{Expression: "request.time < timestamp('2030-01-01T00:00:00Z')"}
		return binding
	}

	cases := []gcsPublicReadProviderCase{
		{name: "neutral exact public grant avoids a write", initial: gcsProviderPolicy(gcsPolicy("etag-a", public())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant after owner binding avoids a write", initial: gcsProviderPolicy(gcsPolicy("etag-b", owner(), public())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant before owner binding avoids a write", initial: gcsProviderPolicy(gcsPolicy("etag-c", public(), owner())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant preserves another target member", initial: gcsProviderPolicy(gcsPolicy("etag-d", publicWithServiceAccount())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant preserves authenticated users", initial: gcsProviderPolicy(gcsPolicy("etag-e", publicWithAuthenticated())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant accepts empty provider etag", initial: gcsProviderPolicy(gcsPolicy("", public())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant accepts one-byte provider etag", initial: gcsProviderPolicy(gcsPolicy("e", public())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant in a repeated later binding avoids a duplicate write", initial: gcsProviderPolicy(gcsPolicy(strings.Repeat("e", 128), targetServiceAccount(), public())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant ignores an empty unrelated binding", initial: gcsProviderPolicy(gcsPolicy("etag-f", gcsPolicyBinding(iam.Viewer), public())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "neutral public grant ignores public membership on another role", initial: gcsProviderPolicy(gcsPolicy("etag-g", viewerPublic(), public())), wantChange: GCSBucketPublicReadUnchanged, wantPolicyGets: 1},
		{name: "positive empty policy receives exact public grant", initial: gcsProviderPolicy(gcsPolicy("etag-h")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive nil bindings receive exact public grant", initial: gcsProviderPolicy(storageapi.Policy{Etag: "etag-i"}), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive owner policy preserves owner and adds public grant", initial: gcsProviderPolicy(gcsPolicy("etag-j", owner())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive public viewer does not replace exact get-only grant", initial: gcsProviderPolicy(gcsPolicy("etag-k", viewerPublic())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive target service account gains public member", initial: gcsProviderPolicy(gcsPolicy("etag-l", targetServiceAccount())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive target authenticated users gain public member", initial: gcsProviderPolicy(gcsPolicy("etag-m", targetAuthenticated())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive empty target binding gains public member", initial: gcsProviderPolicy(gcsPolicy("etag-n", emptyTarget())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive multiple unrelated bindings remain intact", initial: gcsProviderPolicy(gcsPolicy("etag-o", owner(), viewerPublic())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive empty etag policy still receives public grant", initial: gcsProviderPolicy(gcsPolicy("")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive one-byte etag policy receives public grant", initial: gcsProviderPolicy(gcsPolicy("e", owner())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "positive conditional public grant gains separate unconditional grant", initial: gcsProviderPolicy(gcsPolicy("etag-condition", conditionalPublic())), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderWrittenPolicy(), wantChange: GCSBucketPublicReadGranted, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative initial bad request preserves provider cause", initial: gcsProviderStatus(http.StatusBadRequest), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusBadRequest, wantPolicyGets: 1},
		{name: "negative initial unauthenticated preserves provider cause", initial: gcsProviderStatus(http.StatusUnauthorized), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusUnauthorized, wantPolicyGets: 1},
		{name: "negative initial forbidden preserves provider cause", initial: gcsProviderStatus(http.StatusForbidden), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusForbidden, wantPolicyGets: 1},
		{name: "negative initial absent bucket preserves provider cause", initial: gcsProviderStatus(http.StatusNotFound), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusNotFound, wantPolicyGets: 1},
		{name: "negative initial conflict preserves provider cause", initial: gcsProviderStatus(http.StatusConflict), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusConflict, wantPolicyGets: 1},
		{name: "negative initial precondition preserves provider cause", initial: gcsProviderStatus(http.StatusPreconditionFailed), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusPreconditionFailed, wantPolicyGets: 1},
		{name: "negative malformed initial policy preserves Exchange JSON refusal", initial: gcsProviderMalformed(), wantErr: core.ErrObjectStoreDestination, wantCauseErrors: []error{core.ErrExchangeResponse, core.ErrJSONContract}, wantPolicyGets: 1},
		{name: "negative invalid UTF-8 sibling binding preserves Exchange JSON refusal", initial: gcsProviderRaw("{\"bindings\":[{\"members\":[\"serviceAc\xff\xff\x7f\xfft:reader@example.test\"],\"role\":\"roles/storage.legacyObjectReader\"},{\"members\":[\"allUsers\"],\"role\":\"roles/storage.legacyObjectReader\"}],\"etag\":\"etag-repeated\"}"), wantErr: core.ErrObjectStoreDestination, wantCauseErrors: []error{core.ErrExchangeResponse, core.ErrJSONContract}, wantPolicyGets: 1},
		{name: "negative null initial policy preserves the SDK runtime cause", initial: gcsProviderNull(), wantErr: core.ErrObjectStoreDestination, wantRuntimeCause: true, wantPolicyGets: 1},
		{name: "negative set bad request preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-p")), set: gcsProviderStatus(http.StatusBadRequest), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusBadRequest, wantPolicyGets: 1, wantPolicySets: 1},
		{name: "negative set unauthenticated preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-q")), set: gcsProviderStatus(http.StatusUnauthorized), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusUnauthorized, wantPolicyGets: 1, wantPolicySets: 1},
		{name: "negative set forbidden preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-r")), set: gcsProviderStatus(http.StatusForbidden), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusForbidden, wantPolicyGets: 1, wantPolicySets: 1},
		{name: "negative set absent bucket preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-s")), set: gcsProviderStatus(http.StatusNotFound), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusNotFound, wantPolicyGets: 1, wantPolicySets: 1},
		{name: "negative set conflict preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-t")), set: gcsProviderStatus(http.StatusConflict), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusConflict, wantPolicyGets: 1, wantPolicySets: 1},
		{name: "negative set precondition preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-u")), set: gcsProviderStatus(http.StatusPreconditionFailed), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusPreconditionFailed, wantPolicyGets: 1, wantPolicySets: 1},
		{name: "negative confirmation bad request preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-v")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderStatus(http.StatusBadRequest), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusBadRequest, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative confirmation unauthenticated preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-w")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderStatus(http.StatusUnauthorized), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusUnauthorized, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative confirmation forbidden preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-x")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderStatus(http.StatusForbidden), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusForbidden, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative confirmation absent bucket preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-y")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderStatus(http.StatusNotFound), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusNotFound, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative confirmation conflict preserves provider cause", initial: gcsProviderPolicy(gcsPolicy("etag-z")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderStatus(http.StatusConflict), wantErr: core.ErrObjectStoreDestination, wantProviderCode: http.StatusConflict, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative malformed confirmation preserves Exchange JSON refusal", initial: gcsProviderPolicy(gcsPolicy("etag-aa")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderMalformed(), wantErr: core.ErrObjectStoreDestination, wantCauseErrors: []error{core.ErrExchangeResponse, core.ErrJSONContract}, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative missing confirmed member returns conflict and no grant", initial: gcsProviderPolicy(gcsPolicy("etag-ab")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderPolicy(gcsPolicy("etag-after", owner())), wantErr: core.ErrObjectStoreConflict, wantPolicyGets: 2, wantPolicySets: 1},
		{name: "negative conditional-only confirmation returns conflict and no grant", initial: gcsProviderPolicy(gcsPolicy("etag-ac")), set: gcsPublicReadProviderAcceptResponse(), confirmed: gcsProviderPolicy(gcsPolicy("etag-after-condition", conditionalPublic())), wantErr: core.ErrObjectStoreConflict, wantPolicyGets: 2, wantPolicySets: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider := &gcsPublicReadProvider{t: t, testCase: tc}
			client := bucketTestClient(t, provider)
			bucket := parsedGCSBucket(t, gcsProviderBucketText)
			got, gotErr := GrantGCSBucketPublicRead(context.Background(), client, GCSBucketPublicReadRequest{Bucket: bucket})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSBucketPublicReadGrant{}) {
					t.Fatalf("GrantGCSBucketPublicRead() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				for _, wantCauseErr := range tc.wantCauseErrors {
					if !errors.Is(gotErr, wantCauseErr) {
						t.Fatalf("GrantGCSBucketPublicRead() error = %v, want preserved errors.Is(..., %v)", gotErr, wantCauseErr)
					}
				}
				if tc.wantProviderCode != 0 {
					providerCause, found := errors.AsType[*googleapi.Error](gotErr)
					if !found || providerCause.Code != tc.wantProviderCode {
						t.Fatalf("GrantGCSBucketPublicRead() provider cause = (%v, %t), want code %d", providerCause, found, tc.wantProviderCode)
					}
				}
				wantAbsent := tc.wantProviderCode == http.StatusNotFound
				gotAbsent := errors.Is(gotErr, core.ErrObjectStoreAbsent)
				if gotAbsent != wantAbsent {
					t.Fatalf("GrantGCSBucketPublicRead() absence identity = %t, want %t", gotAbsent, wantAbsent)
				}
				wantConflict := tc.wantProviderCode == http.StatusConflict ||
					tc.wantProviderCode == http.StatusPreconditionFailed ||
					errors.Is(tc.wantErr, core.ErrObjectStoreConflict)
				gotConflict := errors.Is(gotErr, core.ErrObjectStoreConflict)
				if gotConflict != wantConflict {
					t.Fatalf("GrantGCSBucketPublicRead() conflict identity = %t, want %t", gotConflict, wantConflict)
				}
				if tc.wantRuntimeCause {
					_, found := errors.AsType[runtime.Error](gotErr)
					if !found {
						t.Fatalf("GrantGCSBucketPublicRead() error = %v, want preserved runtime.Error cause", gotErr)
					}
				}
			} else if gotErr != nil || got.Validate() != nil || got.Bucket() != bucket || got.Change() != tc.wantChange {
				t.Fatalf("GrantGCSBucketPublicRead() = (%v, %v), want bucket %q change %v and nil",
					got, gotErr, bucket.String(), tc.wantChange)
			}
			if gotGets := provider.gets.Load(); gotGets != tc.wantPolicyGets {
				t.Fatalf("provider policy GET calls = %d, want %d", gotGets, tc.wantPolicyGets)
			}
			if gotSets := provider.sets.Load(); gotSets != tc.wantPolicySets {
				t.Fatalf("provider policy SET calls = %d, want %d", gotSets, tc.wantPolicySets)
			}
			if tc.wantPolicySets == 1 {
				written := provider.writtenPolicy()
				if !gcsStoragePolicyHasUnconditionalRole(written, iam.AllUsers, gcsPublicReadRole) ||
					written.Etag != tc.initial.policy.Etag {
					t.Fatalf("provider written policy = %+v, want unconditional public get role and preserved etag %q",
						written, tc.initial.policy.Etag)
				}
				if !gcsStoragePolicyContains(tc.initial.policy, written) {
					t.Fatalf("provider written policy = %+v, want every initial binding from %+v preserved",
						written, tc.initial.policy)
				}
			}
		})
	}
}

func TestGCSBucketPublicReadProviderResponseExtentRefusesOneByteAboveTheOwnedMaximum(t *testing.T) {
	t.Parallel()

	policy := gcsPolicy("etag-extent", gcsPublicReadBinding(iam.AllUsers))
	canonical, gotMarshalErr := json.Marshal(policy)
	if gotMarshalErr != nil {
		t.Fatalf("json.Marshal(provider policy) error = %v, want nil", gotMarshalErr)
	}
	cases := []struct {
		wantErr    error
		name       string
		response   []byte
		wantChange GCSBucketPublicReadChange
	}{
		{name: "one byte below response maximum remains admissible", response: paddedGCSPolicyResponse(t, canonical, GCSProviderResponseMaximumBytes-1), wantChange: GCSBucketPublicReadUnchanged},
		{name: "exact response maximum remains admissible", response: paddedGCSPolicyResponse(t, canonical, GCSProviderResponseMaximumBytes), wantChange: GCSBucketPublicReadUnchanged},
		{name: "one byte above response maximum is refused", response: paddedGCSPolicyResponse(t, canonical, GCSProviderResponseMaximumBytes+1), wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotCalls atomic.Int64
			client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				gotCalls.Add(1)
				writer.Header().Set("Content-Type", "application/json")
				writer.Header().Set("Content-Length", strconv.Itoa(len(tc.response)))
				if len(tc.response) > GCSProviderResponseMaximumBytes {
					return
				}
				if _, gotWriteErr := writer.Write(tc.response); gotWriteErr != nil {
					t.Errorf("provider response write error = %v, want nil", gotWriteErr)
				}
			}))
			bucket := parsedGCSBucket(t, gcsProviderBucketText)
			got, gotErr := GrantGCSBucketPublicRead(context.Background(), client, GCSBucketPublicReadRequest{Bucket: bucket})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrObjectStoreSize) ||
					!errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrExchangeBodyLimit) ||
					got != (GCSBucketPublicReadGrant{}) {
					t.Fatalf("GrantGCSBucketPublicRead(%d-byte response) = (%v, %v), want zero with object-store and Exchange body-limit identities",
						len(tc.response), got, gotErr)
				}
			} else if gotErr != nil || got.Validate() != nil || got.Change() != tc.wantChange {
				t.Fatalf("GrantGCSBucketPublicRead(%d-byte response) = (%v, %v), want change %v and nil",
					len(tc.response), got, gotErr, tc.wantChange)
			}
			if got := gotCalls.Load(); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
		})
	}
}

func TestGCSBucketPublicReadProviderResponseWithoutDeclaredLengthRemainsBounded(t *testing.T) {
	t.Parallel()

	policy := gcsPolicy("etag-chunked", gcsPublicReadBinding(iam.AllUsers))
	canonical, gotMarshalErr := json.Marshal(policy)
	if gotMarshalErr != nil {
		t.Fatalf("json.Marshal(provider policy) error = %v, want nil", gotMarshalErr)
	}
	response := paddedGCSPolicyResponse(t, canonical, GCSProviderResponseMaximumBytes+1)
	var gotCalls atomic.Int64
	client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		gotCalls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Errorf("provider writer implements http.Flusher = false, want true")
			return
		}
		flusher.Flush()
		if _, gotWriteErr := writer.Write(response); gotWriteErr != nil {
			t.Errorf("provider chunked response write error = %v, want nil", gotWriteErr)
		}
	}))
	bucket := parsedGCSBucket(t, gcsProviderBucketText)
	got, gotErr := GrantGCSBucketPublicRead(context.Background(), client, GCSBucketPublicReadRequest{Bucket: bucket})
	if !errors.Is(gotErr, core.ErrObjectStoreContract) || !errors.Is(gotErr, core.ErrObjectStoreSize) ||
		!errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrExchangeBodyLimit) ||
		got != (GCSBucketPublicReadGrant{}) {
		t.Fatalf("GrantGCSBucketPublicRead(chunked %d-byte response) = (%v, %v), want zero with object-store and Exchange body-limit identities",
			len(response), got, gotErr)
	}
	if got := gotCalls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func paddedGCSPolicyResponse(t testing.TB, canonical []byte, wantBytes int) []byte {
	t.Helper()

	if len(canonical) > wantBytes {
		t.Fatalf("canonical provider policy bytes = %d, want at most %d", len(canonical), wantBytes)
	}
	return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, wantBytes-len(canonical))...)
}

func TestGCSBucketPublicReadRefusesInvalidIngressBeforeProviderCalls(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr      error
		name         string
		nilClient    bool
		nilContext   bool
		canceled     bool
		closedClient bool
		zeroRequest  bool
	}{
		{name: "zero request", zeroRequest: true, wantErr: core.ErrObjectStoreContract},
		{name: "nil client", nilClient: true, wantErr: core.ErrObjectStoreContract},
		{name: "closed client", closedClient: true, wantErr: core.ErrObjectStoreContract},
		{name: "nil context", nilContext: true, wantErr: core.ErrObjectStoreContract},
		{name: "canceled context", canceled: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider := &gcsPublicReadProvider{t: t}
			var client *GCSClient
			if !tc.nilClient {
				client = bucketTestClient(t, provider)
				if tc.closedClient {
					if gotCloseErr := client.Close(); gotCloseErr != nil {
						t.Fatalf("GCSClient.Close() error = %v, want nil", gotCloseErr)
					}
				}
			}
			ctx := context.Background()
			if tc.nilContext {
				ctx = nil
			}
			if tc.canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}
			request := GCSBucketPublicReadRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText)}
			if tc.zeroRequest {
				request = GCSBucketPublicReadRequest{}
			}
			got, gotErr := GrantGCSBucketPublicRead(ctx, client, request)
			if !errors.Is(gotErr, tc.wantErr) || got != (GCSBucketPublicReadGrant{}) {
				t.Fatalf("GrantGCSBucketPublicRead(%s) = (%v, %v), want zero and errors.Is(..., %v)", tc.name, got, gotErr, tc.wantErr)
			}
			if gotGets, gotSets := provider.gets.Load(), provider.sets.Load(); gotGets != 0 || gotSets != 0 {
				t.Fatalf("invalid ingress provider calls = (%d GET, %d SET), want (0, 0)", gotGets, gotSets)
			}
		})
	}
}

type gcsPublicReadProvider struct {
	t        testing.TB
	written  storageapi.Policy
	testCase gcsPublicReadProviderCase
	gets     atomic.Int64
	sets     atomic.Int64
	mu       sync.Mutex
}

func (p *gcsPublicReadProvider) ServeHTTP(writer http.ResponseWriter, incoming *http.Request) {
	if incoming.URL.Path != "/storage/v1/b/"+gcsProviderBucketText+"/iam" {
		p.t.Errorf("provider path = %q, want exact bucket IAM path", incoming.URL.Path)
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	switch incoming.Method {
	case http.MethodGet:
		p.serveGet(writer)
	case http.MethodPut:
		p.serveSet(writer, incoming)
	default:
		p.t.Errorf("provider method = %q, want GET or PUT", incoming.Method)
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (p *gcsPublicReadProvider) serveGet(writer http.ResponseWriter) {
	call := p.gets.Add(1)
	if call == 1 {
		p.writeResponse(writer, p.testCase.initial)
		return
	}
	p.writeResponse(writer, p.testCase.confirmed)
}

func (p *gcsPublicReadProvider) serveSet(writer http.ResponseWriter, incoming *http.Request) {
	p.sets.Add(1)
	var policy storageapi.Policy
	if err := json.UnmarshalRead(incoming.Body, &policy); err != nil {
		p.t.Errorf("provider policy decode error = %v, want nil", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	p.written = policy
	p.mu.Unlock()
	switch p.testCase.set.kind {
	case gcsPublicReadProviderAccept:
		writeGCSPolicy(p.t, writer, policy)
	case gcsPublicReadProviderStatus:
		writeGoogleAPIError(writer, p.testCase.set.status)
	default:
		p.t.Errorf("provider SET response kind = %d, want accept or status", p.testCase.set.kind)
		writer.WriteHeader(http.StatusInternalServerError)
	}
}

func (p *gcsPublicReadProvider) writeResponse(
	writer http.ResponseWriter,
	response gcsPublicReadProviderResponse,
) {
	switch response.kind {
	case gcsPublicReadProviderPolicy:
		writeGCSPolicy(p.t, writer, response.policy)
	case gcsPublicReadProviderWrittenPolicy:
		writeGCSPolicy(p.t, writer, p.writtenPolicy())
	case gcsPublicReadProviderStatus:
		writeGoogleAPIError(writer, response.status)
	case gcsPublicReadProviderMalformed:
		writeMalformedGCSPolicy(p.t, writer)
	case gcsPublicReadProviderNull:
		writeNullGCSPolicy(p.t, writer)
	case gcsPublicReadProviderRaw:
		writer.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(writer, response.raw); err != nil {
			p.t.Errorf("provider raw policy response error = %v, want nil", err)
		}
	default:
		p.t.Errorf("provider GET response kind = %d, want a declared response", response.kind)
		writer.WriteHeader(http.StatusInternalServerError)
	}
}

func (p *gcsPublicReadProvider) writtenPolicy() storageapi.Policy {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.written
}

func gcsPolicy(etag string, bindings ...*storageapi.PolicyBindings) storageapi.Policy {
	return storageapi.Policy{Etag: etag, Bindings: bindings}
}

func gcsPolicyBinding(role iam.RoleName, members ...string) *storageapi.PolicyBindings {
	return &storageapi.PolicyBindings{Role: string(role), Members: members}
}

func gcsPublicReadBinding(members ...string) *storageapi.PolicyBindings {
	return gcsPolicyBinding(gcsPublicReadRole, members...)
}

func gcsStoragePolicyHasUnconditionalRole(policy storageapi.Policy, member string, role iam.RoleName) bool {
	for _, binding := range policy.Bindings {
		if binding == nil || binding.Condition != nil || binding.Role != string(role) {
			continue
		}
		if slices.Contains(binding.Members, member) {
			return true
		}
	}
	return false
}

func gcsStoragePolicyContains(wantSubset storageapi.Policy, got storageapi.Policy) bool {
	used := make([]bool, len(got.Bindings))
	for _, wantBinding := range wantSubset.Bindings {
		if wantBinding == nil {
			continue
		}
		matched := false
		for index, gotBinding := range got.Bindings {
			if used[index] || !gcsPolicyBindingContains(wantBinding, gotBinding) {
				continue
			}
			used[index] = true
			matched = true
			break
		}
		if !matched {
			return false
		}
	}
	return true
}

func gcsPolicyBindingContains(want *storageapi.PolicyBindings, got *storageapi.PolicyBindings) bool {
	if got == nil || want.Role != got.Role || !gcsPolicyConditionsEqual(want.Condition, got.Condition) {
		return false
	}
	for _, wantMember := range want.Members {
		if !gcsPolicyMemberPresent(got.Members, wantMember) {
			return false
		}
	}
	return true
}

func gcsPolicyConditionsEqual(want *storageapi.Expr, got *storageapi.Expr) bool {
	if want == nil || got == nil {
		return want == nil && got == nil
	}
	return want.Expression == got.Expression && want.Title == got.Title &&
		want.Description == got.Description && want.Location == got.Location
}

func gcsPolicyMemberPresent(members []string, want string) bool {
	return slices.Contains(members, want)
}

func writeGCSPolicy(t testing.TB, writer http.ResponseWriter, policy storageapi.Policy) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(writer, policy); err != nil {
		t.Errorf("provider policy response error = %v, want nil", err)
	}
}

func writeMalformedGCSPolicy(t testing.TB, writer http.ResponseWriter) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if _, err := writer.Write([]byte("{")); err != nil {
		t.Errorf("provider malformed policy response error = %v, want nil", err)
	}
}

func writeNullGCSPolicy(t testing.TB, writer http.ResponseWriter) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if _, err := writer.Write([]byte("null")); err != nil {
		t.Errorf("provider null policy response error = %v, want nil", err)
	}
}
