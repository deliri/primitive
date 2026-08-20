package chit

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

type signedQueryFixtureRequest struct {
	marker    byte
	offering  core.Offering
	selection Selection
	position  Position
	pageSize  uint16
}

type signedQueryFixture struct {
	payload  QueryPayload
	document QueryDocument
	trusted  attest.TrustedKeys
}

func TestSignedChitQueryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact all specific page and product boundaries authenticate", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name      string
			request   signedQueryFixtureRequest
			wantKind  core.CatalogSelectionKind
			wantLimit uint16
		}{
			{name: "all minimum page witness", request: signedQueryFixtureRequest{marker: 0x21, offering: core.OfferingWitness, pageSize: 1}, wantKind: core.CatalogSelectionAll, wantLimit: 1},
			{name: "all one above minimum page bug", request: signedQueryFixtureRequest{marker: 0x22, offering: core.OfferingBug, pageSize: 2}, wantKind: core.CatalogSelectionAll, wantLimit: 2},
			{name: "all midpoint page peachfuzz", request: signedQueryFixtureRequest{marker: 0x23, offering: core.OfferingPeachfuzz, pageSize: core.CatalogPageMaximumEntries / 2}, wantKind: core.CatalogSelectionAll, wantLimit: core.CatalogPageMaximumEntries / 2},
			{name: "all one below maximum page", request: signedQueryFixtureRequest{marker: 0x24, pageSize: core.CatalogPageMaximumEntries - 1}, wantKind: core.CatalogSelectionAll, wantLimit: core.CatalogPageMaximumEntries - 1},
			{name: "all exact maximum page", request: signedQueryFixtureRequest{marker: 0x25, pageSize: core.CatalogPageMaximumEntries}, wantKind: core.CatalogSelectionAll, wantLimit: core.CatalogPageMaximumEntries},
			{name: "all after opaque cursor minimum page", request: signedQueryFixtureRequest{marker: 0x26, position: signedQueryAfter(t, 0x26), pageSize: 1}, wantKind: core.CatalogSelectionAll, wantLimit: 1},
			{name: "all after opaque cursor maximum page", request: signedQueryFixtureRequest{marker: 0x27, position: signedQueryAfter(t, 0x27), pageSize: core.CatalogPageMaximumEntries}, wantKind: core.CatalogSelectionAll, wantLimit: core.CatalogPageMaximumEntries},
			{name: "specific minimum page", request: signedQueryFixtureRequest{marker: 0x28, selection: signedQuerySpecific(t, 0x28), pageSize: 1}, wantKind: core.CatalogSelectionSpecific, wantLimit: 1},
			{name: "specific midpoint page", request: signedQueryFixtureRequest{marker: 0x29, selection: signedQuerySpecific(t, 0x29), pageSize: core.CatalogPageMaximumEntries / 2}, wantKind: core.CatalogSelectionSpecific, wantLimit: core.CatalogPageMaximumEntries / 2},
			{name: "specific exact maximum page", request: signedQueryFixtureRequest{marker: 0x2a, selection: signedQuerySpecific(t, 0x2a), pageSize: core.CatalogPageMaximumEntries}, wantKind: core.CatalogSelectionSpecific, wantLimit: core.CatalogPageMaximumEntries},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				fixture := newSignedQueryFixture(t, tc.request)
				verified, err := VerifyQuery(QueryVerification{
					Document: fixture.document, TrustedKeys: fixture.trusted,
				})
				if err != nil {
					t.Fatalf("VerifyQuery() error = %v, want nil", err)
				}
				payload, err := verified.Payload()
				if err != nil || payload != fixture.payload ||
					payload.Query.Selection.Kind != tc.wantKind || payload.Query.Limit.Uint16() != tc.wantLimit {
					t.Fatalf("VerifiedQuery.Payload() = (%+v, %v), want exact payload kind %v limit %d",
						payload, err, tc.wantKind, tc.wantLimit)
				}
			})
		}
	})

	t.Run("negative missing conflicting future and foreign facts are refused", func(t *testing.T) {
		t.Parallel()

		base := newSignedQueryFixture(t, signedQueryFixtureRequest{marker: 0x41})
		other := newSignedQueryFixture(t, signedQueryFixtureRequest{
			marker: 0x61, offering: core.OfferingBug, pageSize: 2,
		})
		cases := []struct {
			want   error
			mutate func(*QueryVerification)
			name   string
		}{
			{name: "zero verification", mutate: func(value *QueryVerification) { *value = QueryVerification{} }, want: core.ErrChitContract},
			{name: "query document absent", mutate: func(value *QueryVerification) { value.Document = QueryDocument{} }, want: core.ErrChitContract},
			{name: "trusted keys absent", mutate: func(value *QueryVerification) { value.TrustedKeys = attest.TrustedKeys{} }, want: core.ErrChitContract},
			{name: "foreign signing key", mutate: func(value *QueryVerification) { value.TrustedKeys = other.trusted }, want: core.ErrChitVerification},
			{name: "scope account changed after signing", mutate: func(value *QueryVerification) {
				value.Document.Payload.Query.Scope.Account = other.payload.Query.Scope.Account
			}, want: core.ErrChitVerification},
			{name: "scope offering changed after signing", mutate: func(value *QueryVerification) {
				value.Document.Payload.Query.Scope.Offering = other.payload.Query.Scope.Offering
			}, want: core.ErrChitVerification},
			{name: "selection changed after signing", mutate: func(value *QueryVerification) { value.Document.Payload.Query.Selection = signedQuerySpecific(t, 0x62) }, want: core.ErrChitVerification},
			{name: "position changed after signing", mutate: func(value *QueryVerification) { value.Document.Payload.Query.Position = signedQueryAfter(t, 0x63) }, want: core.ErrChitVerification},
			{name: "page limit changed after signing", mutate: func(value *QueryVerification) { value.Document.Payload.Query.Limit = signedQueryLimit(t, 2) }, want: core.ErrChitVerification},
			{name: "build changed after signing", mutate: func(value *QueryVerification) { value.Document.Payload.Build = other.payload.Build }, want: core.ErrChitVerification},
			{name: "nonce changed after signing", mutate: func(value *QueryVerification) { value.Document.Payload.Nonce = other.payload.Nonce }, want: core.ErrChitVerification},
			{name: "revision changed to future value", mutate: func(value *QueryVerification) { value.Document.Payload.Revision = controlwire.Revision(255) }, want: core.ErrChitContract},
			{name: "attestation domain changed to chit document", mutate: func(value *QueryVerification) { value.Document.Attestation.Domain = SigningDomainChitV1 }, want: core.ErrChitVerification},
			{name: "attestation domain changed to catalog response", mutate: func(value *QueryVerification) { value.Document.Attestation.Domain = SigningDomainCatalogV1 }, want: core.ErrChitVerification},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				verification := QueryVerification{Document: base.document, TrustedKeys: base.trusted}
				tc.mutate(&verification)
				got, err := VerifyQuery(verification)
				if !errors.Is(err, tc.want) || got != (VerifiedQuery{}) {
					t.Fatalf("VerifyQuery(%s) = (%v, %v), want zero and errors.Is %v",
						tc.name, got, err, tc.want)
				}
			})
		}
	})

	t.Run("neutral zero issuance creates no signed query", func(t *testing.T) {
		t.Parallel()

		got, err := IssueQuery(QueryIssuance{})
		if !errors.Is(err, core.ErrChitContract) || got != (QueryDocument{}) {
			t.Fatalf("IssueQuery(zero) = (%+v, %v), want zero and errors.Is %v",
				got, err, core.ErrChitContract)
		}
	})
}

func TestQueryCommitmentBindsEveryVariableRequestFact(t *testing.T) {
	t.Parallel()

	base := newSignedQueryFixture(t, signedQueryFixtureRequest{marker: 0x51, pageSize: 1})
	want, err := CommitQuery(base.payload)
	if err != nil {
		t.Fatalf("CommitQuery(base) error = %v, want nil", err)
	}
	repeated, err := CommitQuery(base.payload)
	if err != nil || repeated != want {
		t.Fatalf("CommitQuery(repeated) = (%v, %v), want deterministic %v and nil", repeated, err, want)
	}
	other := newSignedQueryFixture(t, signedQueryFixtureRequest{
		marker: 0x71, offering: core.OfferingBug, pageSize: 2,
	})
	cases := []struct {
		mutate func(*QueryPayload)
		name   string
	}{
		{name: "scope", mutate: func(value *QueryPayload) { value.Query.Scope = other.payload.Query.Scope }},
		{name: "selection", mutate: func(value *QueryPayload) { value.Query.Selection = signedQuerySpecific(t, 0x72) }},
		{name: "position", mutate: func(value *QueryPayload) { value.Query.Position = signedQueryAfter(t, 0x73) }},
		{name: "page limit", mutate: func(value *QueryPayload) { value.Query.Limit = signedQueryLimit(t, 2) }},
		{name: "build", mutate: func(value *QueryPayload) { value.Build = other.payload.Build }},
		{name: "nonce", mutate: func(value *QueryPayload) { value.Nonce = other.payload.Nonce }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidate := base.payload
			tc.mutate(&candidate)
			got, gotErr := CommitQuery(candidate)
			if gotErr != nil || got == want {
				t.Fatalf("CommitQuery(%s mutation) = (%v, %v), want distinct from %v and nil", tc.name, got, gotErr, want)
			}
		})
	}
	if got, gotErr := CommitQuery(QueryPayload{}); got != (QueryCommitment{}) ||
		!errors.Is(gotErr, core.ErrChitContract) {
		t.Fatalf("CommitQuery(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitContract)
	}
}

func TestSignedChitQueryJSONPressuresMalformedAndExactByteBoundaries(t *testing.T) {
	t.Parallel()

	fixture := newSignedQueryFixture(t, signedQueryFixtureRequest{
		marker: 0x31, selection: signedQuerySpecific(t, 0x31), pageSize: core.CatalogPageMaximumEntries,
	})
	encoded, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatalf("QueryDocument.MarshalJSON() error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
		Payload     QueryPayload                   `json:"payload"`
	}{Attestation: fixture.document.Attestation, Payload: fixture.payload})
	if err != nil {
		t.Fatalf("json.Marshal(reordered query document) error = %v, want nil", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, encoded, "", "  "); err != nil {
		t.Fatalf("json.Indent(query document) error = %v, want nil", err)
	}
	valid := []struct {
		name string
		data []byte
	}{
		{name: "canonical document", data: encoded},
		{name: "one leading space", data: append([]byte(" "), encoded...)},
		{name: "one trailing space", data: append(bytes.Clone(encoded), ' ')},
		{name: "mixed outer whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "members reordered", data: reordered},
		{name: "indented document", data: indented.Bytes()},
		{name: "one below document ceiling", data: signedQueryPadJSON(encoded, QueryDocumentJSONMaximumBytes-1)},
		{name: "exact document ceiling", data: signedQueryPadJSON(encoded, QueryDocumentJSONMaximumBytes)},
		{name: "canonical clone", data: bytes.Clone(encoded)},
		{name: "second independent canonical decode", data: append([]byte(nil), encoded...)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var receiver QueryDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil || receiver != fixture.document {
				t.Fatalf("QueryDocument.UnmarshalJSON(%s) = (%+v, %v), want exact document and nil",
					tc.name, receiver, err)
			}
		})
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicatePayload := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"payload":null}`)...)
	duplicateAttestation := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"attestation":null}`)...)
	invalid := []struct {
		name string
		data []byte
	}{
		{name: "empty input"},
		{name: "whitespace without value", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "string root", data: []byte(`"query"`)},
		{name: "number root", data: []byte(`1`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown top-level member", data: unknown},
		{name: "duplicate payload", data: duplicatePayload},
		{name: "duplicate attestation", data: duplicateAttestation},
		{name: "missing payload", data: []byte(`{"attestation":null}`)},
		{name: "missing attestation", data: []byte(`{"payload":null}`)},
		{name: "payload scalar type", data: []byte(`{"payload":1,"attestation":null}`)},
		{name: "attestation scalar type", data: []byte(`{"payload":null,"attestation":1}`)},
		{name: "truncated opening brace", data: []byte(`{`)},
		{name: "truncated after payload name", data: []byte(`{"payload":`)},
		{name: "truncated canonical document", data: encoded[:len(encoded)-1]},
		{name: "second document trails value", data: append(bytes.Clone(encoded), encoded...)},
		{name: "one above document ceiling", data: signedQueryPadJSON(encoded, QueryDocumentJSONMaximumBytes+1)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := fixture.document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("QueryDocument.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != fixture.document {
				t.Fatalf("QueryDocument.UnmarshalJSON(%s) mutated receiver = %+v, want preserved %+v",
					tc.name, receiver, fixture.document)
			}
		})
	}
}

func TestChitSigningDomainExhaustsPublishedAndFutureByteValues(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value := 0; value <= 255; value++ {
		domain := SigningDomain(value)
		if !domain.IsValid() {
			if err := domain.Validate(); !errors.Is(err, core.ErrChitContract) {
				t.Fatalf("SigningDomain(%d).Validate() error = %v, want errors.Is %v",
					value, err, core.ErrChitContract)
			}
			continue
		}
		admitted++
		encoded, err := domain.MarshalJSON()
		if err != nil {
			t.Fatalf("SigningDomain(%d).MarshalJSON() error = %v, want nil", value, err)
		}
		var decoded SigningDomain
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != domain {
			t.Fatalf("SigningDomain(%d) round trip = (%v, %v), want (%v, nil)",
				value, decoded, err, domain)
		}
	}
	if admitted != 3 || SigningDomainQueryV1.String() != SigningDomainQueryV1Token {
		t.Fatalf("admitted signing domains/query token = (%d, %q), want (3, %q)",
			admitted, SigningDomainQueryV1.String(), SigningDomainQueryV1Token)
	}
}

func newSignedQueryFixture(t testing.TB, request signedQueryFixtureRequest) signedQueryFixture {
	t.Helper()

	if request.marker == 0 {
		request.marker = 0x21
	}
	if request.offering == core.OfferingUnknown {
		request.offering = core.OfferingWitness
	}
	if request.selection == (Selection{}) {
		request.selection = All()
	}
	if request.position == (Position{}) {
		request.position = Start()
	}
	if request.pageSize == 0 {
		request.pageSize = 1
	}
	query, err := NewQuery(QueryRequest{
		Scope: chitScopeFixture(t, request.marker), Selection: request.selection,
		Position: request.position, PageSize: request.pageSize,
	})
	if err != nil {
		t.Fatalf("NewQuery() error = %v, want nil", err)
	}
	payload := QueryPayload{
		Query: query, Build: signedQueryBuild(t, request.offering),
		Nonce: signedQueryNonce(t, request.marker), Revision: controlwire.Revision2026V1,
	}
	private, trusted := chitSigningFixture(t, request.marker+0x40)
	document, err := IssueQuery(QueryIssuance{Signer: private, Payload: payload})
	if err != nil {
		t.Fatalf("IssueQuery() error = %v, want nil", err)
	}
	return signedQueryFixture{payload: payload, document: document, trusted: trusted}
}

func signedQueryBuild(t testing.TB, offering core.Offering) core.BuildIdentity {
	t.Helper()

	commit, err := core.ParseBuildCommit("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: offering, Version: core.NewReleaseVersion(2026, 0, 56), Commit: commit,
		Platform: core.Platform{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64},
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	return build
}

func signedQueryNonce(t testing.TB, marker byte) controlwire.RequestNonce {
	t.Helper()

	raw := [core.SHA256DigestBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	nonce, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return nonce
}

func signedQuerySpecific(t testing.TB, marker byte) Selection {
	t.Helper()

	selection, err := Specific(mustChitID(t, marker, int64(marker)+10))
	if err != nil {
		t.Fatalf("Specific() error = %v, want nil", err)
	}
	return selection
}

func signedQueryAfter(t testing.TB, marker byte) Position {
	t.Helper()

	cursor, err := CursorFor(mustChitID(t, marker, int64(marker)+20))
	if err != nil {
		t.Fatalf("CursorFor() error = %v, want nil", err)
	}
	position, err := After(cursor)
	if err != nil {
		t.Fatalf("After() error = %v, want nil", err)
	}
	return position
}

func signedQueryLimit(t testing.TB, value uint16) core.CatalogPageLimit {
	t.Helper()

	limit, err := core.NewCatalogPageLimit(value)
	if err != nil {
		t.Fatalf("core.NewCatalogPageLimit() error = %v, want nil", err)
	}
	return limit
}

func signedQueryPadJSON(encoded []byte, length int) []byte {
	if length < len(encoded) {
		return nil
	}
	padded := make([]byte, length)
	for index := 0; index < length-len(encoded); index++ {
		padded[index] = ' '
	}
	copy(padded[length-len(encoded):], encoded)
	return padded
}
