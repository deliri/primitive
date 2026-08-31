package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestAdmissionSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive authenticated origin admits one exact requested probe and grant chain", func(t *testing.T) {
		t.Parallel()
		authenticated, admitted := admissionFixture(t)
		if gotErr := authenticated.Validate(); gotErr != nil {
			t.Fatalf("AuthenticatedAdmissionRequest.Validate() error = %v, want nil", gotErr)
		}
		response := runnercontrol.AdmissionResponse{SchemaVersion: runnercontrol.SchemaVersion, Request: authenticated.Requested.Request, Admitted: &admitted}
		encoded, encodeErr := response.MarshalJSON()
		var roundTrip runnercontrol.AdmissionResponse
		decodeErr := roundTrip.UnmarshalJSON(encoded)
		if encodeErr != nil || decodeErr != nil || roundTrip.Admitted == nil || roundTrip.Admitted.Run != admitted.Run || roundTrip.Admitted.Source.Identity != admitted.Source.Identity {
			t.Fatalf("admission canonical round trip = (bytes %d, admitted %+v, encode %v, decode %v), want exact run/source and nil errors", len(encoded), roundTrip.Admitted, encodeErr, decodeErr)
		}
	})

	t.Run("negative authenticated origin cannot claim another origin request", func(t *testing.T) {
		t.Parallel()
		authenticated, _ := admissionFixture(t)
		foreign := projectstandards.OriginIdentity{Offering: core.Offering{Token: "forge"}}
		authenticated.Peer.Origin = &foreign
		gotErr := authenticated.Validate()
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("AuthenticatedAdmissionRequest.Validate(foreign origin) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral typed refusal carries no admitted run", func(t *testing.T) {
		t.Parallel()
		authenticated, _ := admissionFixture(t)
		refusal := projectstandards.RefusalUnauthorized
		response := runnercontrol.AdmissionResponse{SchemaVersion: runnercontrol.SchemaVersion, Request: authenticated.Requested.Request, Refusal: &refusal}
		if gotErr := response.Validate(); gotErr != nil || response.Admitted != nil {
			t.Fatalf("AdmissionResponse.Validate(refusal) = (%v, admitted %v), want nil and no admitted run", gotErr, response.Admitted)
		}
	})
}

func TestArtifactManifestAndChunkLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive manifest identity and final chunk bind exact bytes", func(t *testing.T) {
		t.Parallel()
		manifest, chunk := artifactFixture(t, []byte("artifact-evidence"))
		digest, digestErr := manifest.Digest()
		if digestErr != nil || digest != chunk.ManifestDigest || !runnercontrol.ArtifactChunkMatchesEntry(chunk, manifest.Entries[0]) {
			t.Fatalf("artifact closure = (manifest %v, chunk %v, matches %t, error %v), want one exact digest and true", digest, chunk.ManifestDigest, runnercontrol.ArtifactChunkMatchesEntry(chunk, manifest.Entries[0]), digestErr)
		}
		record, recordErr := runnercontrol.NewArtifactManifestRecord(manifest)
		if recordErr != nil || record.Digest != digest || record.Bytes.Uint64() == 0 || !bytes.Equal(record.Canonical, mustArtifactManifestJSON(t, manifest)) {
			t.Fatalf("NewArtifactManifestRecord() = (%+v, %v), want exact non-vacuous canonical manifest", record, recordErr)
		}
	})

	t.Run("negative one-byte mutation cannot satisfy the sealed entry digest", func(t *testing.T) {
		t.Parallel()
		_, chunk := artifactFixture(t, []byte("artifact-evidence"))
		chunk.Data[0] ^= 1
		if got := runnercontrol.ArtifactChunkMatchesEntry(chunk, chunk.Entry); got {
			t.Fatalf("ArtifactChunkMatchesEntry(mutated data) = %t, want false", got)
		}
	})

	t.Run("neutral empty manifest records no artifact and zero bytes", func(t *testing.T) {
		t.Parallel()
		payload := experimentCompletionPayloadFixture(t, false)
		zero, zeroErr := core.NewByteLength(0)
		manifest := runnercontrol.ArtifactManifest{SchemaVersion: runnercontrol.SchemaVersion, Run: payload.Run, Fence: payload.Fence, Members: payload.Members, Entries: []runnercontrol.ArtifactManifestEntry{}, TotalBytes: zero}
		if gotErr := errors.Join(zeroErr, manifest.Validate()); gotErr != nil || len(manifest.Entries) != 0 || manifest.TotalBytes.Uint64() != 0 {
			t.Fatalf("ArtifactManifest.Validate(empty) = (%v, entries %d, bytes %d), want nil, 0, 0", gotErr, len(manifest.Entries), manifest.TotalBytes.Uint64())
		}
	})
}

func TestCleanupProofLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive signed cleanup proves one populated before state became exact zero", func(t *testing.T) {
		t.Parallel()
		payload := cleanupPayloadFixture(t)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueCleanup(payload, key)
		verifyErr := runnercontrol.VerifyCleanup(document, trusted)
		record, recordErr := runnercontrol.NewCleanupRecord(document)
		if issueErr != nil || verifyErr != nil || recordErr != nil || record.Bytes.Uint64() == 0 || record.Digest != core.SHA256Of(record.Canonical) {
			t.Fatalf("cleanup proof = (issue %v, verify %v, record %v, bytes %d), want nil errors and exact nonzero record", issueErr, verifyErr, recordErr, record.Bytes.Uint64())
		}
	})

	t.Run("negative signed after-state mutation fails independent authentication", func(t *testing.T) {
		t.Parallel()
		payload := cleanupPayloadFixture(t)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueCleanup(payload, key)
		if issueErr != nil {
			t.Fatalf("IssueCleanup() setup error = %v, want nil", issueErr)
		}
		document.Payload.CompletedAt = temporal.InstantFromNanoseconds(5)
		if gotErr := document.Validate(); gotErr != nil {
			t.Fatalf("CleanupDocument.Validate(mutated completion) error = %v, want nil so authentication owns rejection", gotErr)
		}
		gotErr := runnercontrol.VerifyCleanup(document, trusted)
		if !errors.Is(gotErr, core.ErrAttestVerification) {
			t.Fatalf("VerifyCleanup(mutated completion) error = %v, want errors.Is(..., %v)", gotErr, core.ErrAttestVerification)
		}
	})

	t.Run("neutral already-clean before state remains a truthful zero-to-zero transition", func(t *testing.T) {
		t.Parallel()
		payload := cleanupPayloadFixture(t)
		payload.Before.Entries = 0
		if gotErr := payload.Validate(); gotErr != nil || !payload.Before.IsClean() || !payload.After.Observation.IsClean() {
			t.Fatalf("CleanupPayload.Validate(already clean) = (%v, before clean %t, after clean %t), want nil, true, true", gotErr, payload.Before.IsClean(), payload.After.Observation.IsClean())
		}
	})
}

func admissionFixture(t testing.TB) (runnercontrol.AuthenticatedAdmissionRequest, runnercontrol.AdmittedRun) {
	t.Helper()
	completion := experimentCompletionPayloadFixture(t, true)
	request, requestErr := projectstandards.NewRequestIdentity(completionUUIDFixture(t))
	output, outputErr := core.NewByteCount(1 << 20)
	artifact, artifactErr := core.NewByteCount(4 << 20)
	duration, durationErr := temporal.DurationFromSeconds(300)
	sourceAuthority, sourceErr := core.ParseHTTPEndpoint("https://source.example.invalid/archive")
	credentialIssuer, credentialErr := core.ParseHTTPEndpoint("https://source.example.invalid/credentials")
	deliveryEndpoint, deliveryErr := core.ParseHTTPEndpoint("https://origin.example.invalid/v1/anvil/observations")
	audience, audienceErr := projectstandards.NewIdentifier("origin-anvil")
	application, applicationErr := projectstandards.NewIdentifier("anvil")
	credential, custodyErr := projectstandards.NewIdentifier("source-read-once")
	if err := errors.Join(requestErr, outputErr, artifactErr, durationErr, sourceErr, credentialErr, deliveryErr, audienceErr, applicationErr, custodyErr); err != nil {
		t.Fatalf("admission fixture construction error = %v, want nil", err)
	}
	requestedProbe := projectstandards.RequestedProbe{
		Origin: completion.Probe.Origin, Subject: completion.Probe.Subject, Source: completion.Probe.Source,
		Target: completion.Probe.Target, Kinds: []projectstandards.ProbeKind{completion.Probe.Kind}, Profile: completion.Probe.Profile,
		Constraints: projectstandards.EnvironmentRequirement{MachineClass: completion.Probe.Environment.MachineClass, Fingerprint: completion.Probe.Environment.RequirementFingerprint},
	}
	limits := runnercontrol.RunLimits{Duration: duration, OutputBytes: output, ArtifactBytes: artifact, ArtifactCount: 8, WorkerMaximum: 4, ProcessMaximum: 64, FileMaximum: 4096, QueueDepth: 16}
	requested := runnercontrol.RequestedRun{SchemaVersion: runnercontrol.SchemaVersion, Request: request, Probe: requestedProbe, Limits: limits, EvidencePlan: core.SHA256Of([]byte("evidence-plan")), RequestedAt: temporal.InstantFromNanoseconds(1)}
	expires := temporal.InstantFromNanoseconds(100)
	repository, repositoryErr := runnercontrol.NewRepositoryGrant(runnercontrol.RepositoryGrant{Origin: completion.Probe.Origin, Subject: completion.Probe.Subject, Repository: completion.Probe.Source.Repository, SourceAuthority: sourceAuthority, CredentialIssuer: credentialIssuer, Enabled: true, ExpiresAt: expires})
	delivery, grantErr := runnercontrol.NewOriginDeliveryGrant(runnercontrol.OriginDeliveryGrant{Origin: completion.Probe.Origin, Endpoint: deliveryEndpoint, TLSIdentity: core.SHA256Of([]byte("origin-tls")), Audience: audience, Application: application, Enabled: true, ExpiresAt: expires})
	source, sourceGrantErr := runnercontrol.NewSourceGrant(runnercontrol.SourceGrant{RepositoryGrant: repository.Identity, Source: completion.Probe.Source, Authority: sourceAuthority, Credential: credential, ExpiresAt: expires})
	if err := errors.Join(repositoryErr, grantErr, sourceGrantErr); err != nil {
		t.Fatalf("admission grant fixture error = %v, want nil", err)
	}
	origin := completion.Probe.Origin
	peer := runnercontrol.AuthenticatedPeer{Role: runnercontrol.PeerRoleOrigin, Certificate: core.SHA256Of([]byte("origin-certificate")), Origin: &origin}
	authenticated := runnercontrol.AuthenticatedAdmissionRequest{Peer: peer, Requested: requested}
	admitted := runnercontrol.AdmittedRun{SchemaVersion: runnercontrol.SchemaVersion, Request: request, Run: completion.Run, Requested: requestedProbe, Probe: completion.Probe, Limits: limits, EvidencePlan: requested.EvidencePlan, Repository: repository, Delivery: delivery, Source: source, AdmittedAt: temporal.InstantFromNanoseconds(2)}
	if err := errors.Join(authenticated.Validate(), admitted.Validate()); err != nil {
		t.Fatalf("admission fixture validation error = %v, want nil", err)
	}
	return authenticated, admitted
}

func artifactFixture(t testing.TB, data []byte) (runnercontrol.ArtifactManifest, runnercontrol.ArtifactChunk) {
	t.Helper()
	payload := experimentCompletionPayloadFixture(t, true)
	path, pathErr := projectstandards.ParseSourcePath("experiments/stdout.jsonl")
	mediaType, mediaErr := core.ParseHTTPMediaType("application/jsonl")
	extent, extentErr := core.NewByteLength(uint64(len(data)))
	offset, offsetErr := core.NewByteLength(0)
	if err := errors.Join(pathErr, mediaErr, extentErr, offsetErr); err != nil {
		t.Fatalf("artifact fixture construction error = %v, want nil", err)
	}
	experiment := payload.Observation.Experiment
	entry := runnercontrol.ArtifactManifestEntry{Kind: runnercontrol.ArtifactStdout, Path: path, MediaType: mediaType, Digest: core.SHA256Of(data), Bytes: extent, Experiment: &experiment}
	manifest := runnercontrol.ArtifactManifest{SchemaVersion: runnercontrol.SchemaVersion, Run: payload.Run, Fence: payload.Fence, Members: payload.Members, Entries: []runnercontrol.ArtifactManifestEntry{entry}, TotalBytes: extent}
	digest, digestErr := manifest.Digest()
	if digestErr != nil {
		t.Fatalf("ArtifactManifest.Digest() fixture error = %v, want nil", digestErr)
	}
	chunk := runnercontrol.ArtifactChunk{SchemaVersion: runnercontrol.SchemaVersion, Run: payload.Run, Fence: payload.Fence, Members: payload.Members, ManifestDigest: digest, Entry: entry, Offset: offset, Data: append([]byte(nil), data...), Final: true}
	if err := chunk.Validate(); err != nil {
		t.Fatalf("ArtifactChunk.Validate() fixture error = %v, want nil", err)
	}
	return manifest, chunk
}

func cleanupPayloadFixture(t testing.TB) runnercontrol.CleanupPayload {
	t.Helper()
	completion := experimentCompletionPayloadFixture(t, true)
	root := core.SHA256Of([]byte("workspace-root"))
	before := runnercontrol.MachineStateObservation{RootIdentity: root, Entries: 8, ObservedAt: temporal.InstantFromNanoseconds(2)}
	afterObservation := runnercontrol.MachineStateObservation{RootIdentity: root, ObservedAt: temporal.InstantFromNanoseconds(3)}
	payload := runnercontrol.CleanupPayload{SchemaVersion: runnercontrol.SchemaVersion, Fence: completion.Fence, Members: completion.Members, WorkspaceRoot: root, Before: before, After: runnercontrol.CleanMachineState{Observation: afterObservation}, CompletedAt: temporal.InstantFromNanoseconds(4)}
	if err := payload.Validate(); err != nil {
		t.Fatalf("CleanupPayload.Validate() fixture error = %v, want nil", err)
	}
	return payload
}

func mustArtifactManifestJSON(t testing.TB, manifest runnercontrol.ArtifactManifest) []byte {
	t.Helper()
	got, err := manifest.MarshalJSON()
	if err != nil {
		t.Fatalf("ArtifactManifest.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}
