package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzRequestedRunSemanticClosure(f *testing.F) {
	authenticated, _ := admissionFixture(f)
	seed := mustRequestedRunJSON(f, authenticated.Requested)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{"schema_version":1}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := authenticated.Requested
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustRequestedRunJSON(t, got), seed) {
				t.Fatalf("RequestedRun.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustRequestedRunJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustRequestedRunJSON(t, got)
		var roundTrip runnercontrol.RequestedRun
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustRequestedRunJSON(t, roundTrip), encoded) {
			t.Fatalf("RequestedRun canonical closure = (second %q, error %v), want %q and nil", mustRequestedRunJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzAdmissionResponseSemanticClosure(f *testing.F) {
	_, admitted := admissionFixture(f)
	seedValue := runnercontrol.AdmissionResponse{SchemaVersion: runnercontrol.SchemaVersion, Request: admitted.Request, Admitted: &admitted}
	seed := mustAdmissionResponseJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustAdmissionResponseJSON(t, got), seed) {
				t.Fatalf("AdmissionResponse.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustAdmissionResponseJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustAdmissionResponseJSON(t, got)
		var roundTrip runnercontrol.AdmissionResponse
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustAdmissionResponseJSON(t, roundTrip), encoded) {
			t.Fatalf("AdmissionResponse canonical closure = (second %q, error %v), want %q and nil", mustAdmissionResponseJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzArtifactManifestSemanticClosure(f *testing.F) {
	seedValue, _ := artifactFixture(f, []byte("artifact-evidence"))
	seed := mustArtifactManifestJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{"schema_version":1,"entries":[]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustArtifactManifestJSON(t, got), seed) {
				t.Fatalf("ArtifactManifest.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustArtifactManifestJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustArtifactManifestJSON(t, got)
		var roundTrip runnercontrol.ArtifactManifest
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustArtifactManifestJSON(t, roundTrip), encoded) {
			t.Fatalf("ArtifactManifest canonical closure = (second %q, error %v), want %q and nil", mustArtifactManifestJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzArtifactChunkSemanticClosure(f *testing.F) {
	_, seedValue := artifactFixture(f, []byte("artifact-evidence"))
	seed := mustArtifactChunkJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustArtifactChunkJSON(t, got), seed) {
				t.Fatalf("ArtifactChunk.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustArtifactChunkJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustArtifactChunkJSON(t, got)
		var roundTrip runnercontrol.ArtifactChunk
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustArtifactChunkJSON(t, roundTrip), encoded) {
			t.Fatalf("ArtifactChunk canonical closure = (second %q, error %v), want %q and nil", mustArtifactChunkJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzCleanupDocumentSemanticClosure(f *testing.F) {
	payload := cleanupPayloadFixture(f)
	key, _ := completionSignerFixture(f)
	seedValue, issueErr := runnercontrol.IssueCleanup(payload, key)
	if issueErr != nil {
		f.Fatalf("IssueCleanup(seed) error = %v, want nil", issueErr)
	}
	seed := mustCleanupDocumentJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustCleanupDocumentJSON(t, got), seed) {
				t.Fatalf("CleanupDocument.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustCleanupDocumentJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustCleanupDocumentJSON(t, got)
		var roundTrip runnercontrol.CleanupDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustCleanupDocumentJSON(t, roundTrip), encoded) {
			t.Fatalf("CleanupDocument canonical closure = (second %q, error %v), want %q and nil", mustCleanupDocumentJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzObservationEnvelopeSemanticClosure(f *testing.F) {
	seedValue, _, _, _, _ := completedObservationDeliveryFixture(f)
	seed := mustObservationEnvelopeJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustObservationEnvelopeJSON(t, got), seed) {
				t.Fatalf("ObservationEnvelope.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustObservationEnvelopeJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustObservationEnvelopeJSON(t, got)
		var roundTrip runnercontrol.ObservationEnvelope
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustObservationEnvelopeJSON(t, roundTrip), encoded) {
			t.Fatalf("ObservationEnvelope canonical closure = (second %q, error %v), want %q and nil", mustObservationEnvelopeJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzObservationDeliveryStageSemanticClosure(f *testing.F) {
	seedValue, _, _ := deliveryProtocolFixture(f)
	seed := mustObservationDeliveryStageJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustObservationDeliveryStageJSON(t, got), seed) {
				t.Fatalf("ObservationDeliveryStage.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustObservationDeliveryStageJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustObservationDeliveryStageJSON(t, got)
		var roundTrip runnercontrol.ObservationDeliveryStage
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustObservationDeliveryStageJSON(t, roundTrip), encoded) {
			t.Fatalf("ObservationDeliveryStage canonical closure = (second %q, error %v), want %q and nil", mustObservationDeliveryStageJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzObservationDeliveryPageUploadSemanticClosure(f *testing.F) {
	stage, pages, _ := deliveryProtocolFixture(f)
	identity, identityErr := stage.Identity()
	if identityErr != nil {
		f.Fatalf("ObservationDeliveryStage.Identity(seed) error = %v, want nil", identityErr)
	}
	seedValue := runnercontrol.ObservationDeliveryPageUpload{SchemaVersion: runnercontrol.SchemaVersion, Identity: identity, Page: pages[0]}
	seed := mustObservationDeliveryPageUploadJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustObservationDeliveryPageUploadJSON(t, got), seed) {
				t.Fatalf("ObservationDeliveryPageUpload.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustObservationDeliveryPageUploadJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustObservationDeliveryPageUploadJSON(t, got)
		var roundTrip runnercontrol.ObservationDeliveryPageUpload
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustObservationDeliveryPageUploadJSON(t, roundTrip), encoded) {
			t.Fatalf("ObservationDeliveryPageUpload canonical closure = (second %q, error %v), want %q and nil", mustObservationDeliveryPageUploadJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzObservationDeliveryCommitSemanticClosure(f *testing.F) {
	stage, _, _ := deliveryProtocolFixture(f)
	identity, identityErr := stage.Identity()
	if identityErr != nil {
		f.Fatalf("ObservationDeliveryStage.Identity(seed) error = %v, want nil", identityErr)
	}
	seedValue := runnercontrol.ObservationDeliveryCommit{SchemaVersion: runnercontrol.SchemaVersion, Identity: identity, Run: stage.Envelope.Payload.Run, PageCount: stage.Manifest.PageCount}
	seed := mustObservationDeliveryCommitJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustObservationDeliveryCommitJSON(t, got), seed) {
				t.Fatalf("ObservationDeliveryCommit.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustObservationDeliveryCommitJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustObservationDeliveryCommitJSON(t, got)
		var roundTrip runnercontrol.ObservationDeliveryCommit
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustObservationDeliveryCommitJSON(t, roundTrip), encoded) {
			t.Fatalf("ObservationDeliveryCommit canonical closure = (second %q, error %v), want %q and nil", mustObservationDeliveryCommitJSON(t, roundTrip), err, encoded)
		}
	})
}

func FuzzSourceArchiveDocumentSemanticClosure(f *testing.F) {
	completion := experimentCompletionPayloadFixture(f, true)
	archiveBytes, bytesErr := core.NewByteLength(1024)
	fileMaximum, maximumErr := core.NewByteCount(1 << 20)
	manifest := runnercontrol.SourceArchiveManifest{
		SchemaVersion: runnercontrol.SchemaVersion, Repository: completion.Probe.Source.Repository, Commit: completion.Probe.Source.Commit,
		Tree: completion.Probe.Source.Tree, ArchiveDigest: core.SHA256Of([]byte("archive")), ArchiveBytes: archiveBytes,
		EntryMaximum: 128, DepthMaximum: 32, FileMaximumBytes: fileMaximum, IssuedAt: temporal.InstantFromNanoseconds(1), ExpiresAt: temporal.InstantFromNanoseconds(100),
	}
	key, _ := completionSignerFixture(f)
	seedValue, issueErr := runnercontrol.IssueSourceArchive(manifest, key)
	if err := errors.Join(bytesErr, maximumErr, issueErr); err != nil {
		f.Fatalf("source archive document seed error = %v, want nil", err)
	}
	seed := mustSourceArchiveDocumentJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustSourceArchiveDocumentJSON(t, got), seed) {
				t.Fatalf("SourceArchiveDocument.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustSourceArchiveDocumentJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustSourceArchiveDocumentJSON(t, got)
		var roundTrip runnercontrol.SourceArchiveDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustSourceArchiveDocumentJSON(t, roundTrip), encoded) {
			t.Fatalf("SourceArchiveDocument canonical closure = (second %q, error %v), want %q and nil", mustSourceArchiveDocumentJSON(t, roundTrip), err, encoded)
		}
	})
}

func TestVerifySourceArchivePinsTheSignedObservationInterval(t *testing.T) {
	t.Parallel()

	completion := experimentCompletionPayloadFixture(t, true)
	archiveBytes, err := core.NewByteLength(1024)
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	fileMaximum, err := core.NewByteCount(1 << 20)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	manifest := runnercontrol.SourceArchiveManifest{
		SchemaVersion: runnercontrol.SchemaVersion, Repository: completion.Probe.Source.Repository,
		Commit: completion.Probe.Source.Commit, Tree: completion.Probe.Source.Tree,
		ArchiveDigest: core.SHA256Of([]byte("archive")), ArchiveBytes: archiveBytes,
		EntryMaximum: 128, DepthMaximum: 32, FileMaximumBytes: fileMaximum,
		IssuedAt: temporal.InstantFromNanoseconds(10), ExpiresAt: temporal.InstantFromNanoseconds(20),
	}
	key, trusted := completionSignerFixture(t)
	document, err := runnercontrol.IssueSourceArchive(manifest, key)
	if err != nil {
		t.Fatalf("runnercontrol.IssueSourceArchive() error = %v, want nil", err)
	}
	cases := []struct {
		name      string
		observed  int64
		wantValid bool
	}{
		{name: "one before issuance is refused", observed: 9},
		{name: "exact issuance instant is admitted", observed: 10, wantValid: true},
		{name: "one before expiry is admitted", observed: 19, wantValid: true},
		{name: "exact expiry instant is refused", observed: 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := runnercontrol.VerifySourceArchive(runnercontrol.SourceArchiveVerification{
				Document: document, TrustedKeys: trusted, ObservedAt: temporal.InstantFromNanoseconds(tc.observed),
			})
			if tc.wantValid && gotErr != nil {
				t.Fatalf("VerifySourceArchive(%d) error = %v, want nil", tc.observed, gotErr)
			}
			if !tc.wantValid && !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("VerifySourceArchive(%d) error = %v, want %v", tc.observed, gotErr, core.ErrPrimitiveContract)
			}
		})
	}
}

func mustRequestedRunJSON(t testing.TB, value runnercontrol.RequestedRun) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("RequestedRun.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustAdmissionResponseJSON(t testing.TB, value runnercontrol.AdmissionResponse) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("AdmissionResponse.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustArtifactChunkJSON(t testing.TB, value runnercontrol.ArtifactChunk) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("ArtifactChunk.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustCleanupDocumentJSON(t testing.TB, value runnercontrol.CleanupDocument) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("CleanupDocument.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustObservationEnvelopeJSON(t testing.TB, value runnercontrol.ObservationEnvelope) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("ObservationEnvelope.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustObservationDeliveryStageJSON(t testing.TB, value runnercontrol.ObservationDeliveryStage) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("ObservationDeliveryStage.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustObservationDeliveryPageUploadJSON(t testing.TB, value runnercontrol.ObservationDeliveryPageUpload) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("ObservationDeliveryPageUpload.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustObservationDeliveryCommitJSON(t testing.TB, value runnercontrol.ObservationDeliveryCommit) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("ObservationDeliveryCommit.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}

func mustSourceArchiveDocumentJSON(t testing.TB, value runnercontrol.SourceArchiveDocument) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("SourceArchiveDocument.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}
