package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestRunnerCompletionProducerSchemaVerifierLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive direct experiment seals the exact acknowledged child record", func(t *testing.T) {
		t.Parallel()
		payload := directRunnerCompletionPayloadFixture(t)
		key, trusted := completionSignerFixture(t)

		got, gotErr := runnercontrol.IssueRunnerCompletion(payload, key)
		if gotErr != nil {
			t.Fatalf("IssueRunnerCompletion() error = %v, want nil", gotErr)
		}
		if gotErr := runnercontrol.VerifyRunnerCompletion(got, trusted); gotErr != nil {
			t.Fatalf("VerifyRunnerCompletion(issued) error = %v, want nil", gotErr)
		}
		record, recordErr := runnercontrol.NewRunnerCompletionRecord(got)
		if recordErr != nil {
			t.Fatalf("NewRunnerCompletionRecord() error = %v, want nil", recordErr)
		}
		if record.Bytes.Uint64() == 0 || record.Digest != core.SHA256Of(record.Canonical) || !bytes.Equal(record.Canonical, mustRunnerCompletionJSON(t, got)) {
			t.Fatalf("runner completion record = (bytes %d, digest %v, canonical %d bytes), want nonzero exact canonical digest and bytes", record.Bytes.Uint64(), record.Digest, len(record.Canonical))
		}
	})

	t.Run("negative two completion variants are refused before signing", func(t *testing.T) {
		t.Parallel()
		payload := directRunnerCompletionPayloadFixture(t)
		payload.PreSource = &runnercontrol.PreSourceRunnerCompletion{
			Attempted: runnercontrol.AttemptedSource{Repository: payload.DirectExperiment.Source.Repository, Commit: payload.DirectExperiment.Source.Commit},
			Failure:   core.ErrFilestoreSource,
		}
		key, _ := completionSignerFixture(t)

		got, gotErr := runnercontrol.IssueRunnerCompletion(payload, key)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Payload.SchemaVersion != 0 || got.Attestation.Domain != runnercontrol.CompletionSigningDomainUnknown {
			t.Fatalf("IssueRunnerCompletion(two variants) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral pre-source failure retains attempted commit and forbids subject evidence", func(t *testing.T) {
		t.Parallel()
		payload := preSourceRunnerCompletionPayloadFixture(t)
		key, trusted := completionSignerFixture(t)

		got, gotErr := runnercontrol.IssueRunnerCompletion(payload, key)
		if gotErr != nil {
			t.Fatalf("IssueRunnerCompletion(pre-source) error = %v, want nil", gotErr)
		}
		if got.Payload.Selection != nil || got.Payload.DirectExperiment != nil || got.Payload.PreSource == nil || got.Payload.Terminal == runprotocol.TerminalCompleted {
			t.Fatalf("pre-source runner completion = (selection %v, direct %v, pre-source %v, terminal %v), want nil, nil, present, non-completed", got.Payload.Selection, got.Payload.DirectExperiment, got.Payload.PreSource, got.Payload.Terminal)
		}
		if gotErr := runnercontrol.VerifyRunnerCompletion(got, trusted); gotErr != nil {
			t.Fatalf("VerifyRunnerCompletion(pre-source) error = %v, want nil", gotErr)
		}
	})

	t.Run("negative signed terminal mutation fails authentication without becoming structurally invalid", func(t *testing.T) {
		t.Parallel()
		payload := directRunnerCompletionPayloadFixture(t)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueRunnerCompletion(payload, key)
		if issueErr != nil {
			t.Fatalf("IssueRunnerCompletion() setup error = %v, want nil", issueErr)
		}
		document.Payload.Terminal = runprotocol.TerminalFailed
		if gotErr := document.Validate(); gotErr != nil {
			t.Fatalf("mutated RunnerCompletionDocument.Validate() error = %v, want nil so signature verification owns rejection", gotErr)
		}

		gotErr := runnercontrol.VerifyRunnerCompletion(document, trusted)
		if !errors.Is(gotErr, core.ErrAttestVerification) {
			t.Fatalf("VerifyRunnerCompletion(mutated terminal) error = %v, want errors.Is(..., %v)", gotErr, core.ErrAttestVerification)
		}
	})
}

func directRunnerCompletionPayloadFixture(t testing.TB) runnercontrol.RunnerCompletionPayload {
	t.Helper()
	experimentPayload := experimentCompletionPayloadFixture(t, true)
	key, _ := completionSignerFixture(t)
	experimentDocument, issueErr := runnercontrol.IssueExperimentCompletion(experimentPayload, key)
	experimentRecord, recordErr := runnercontrol.NewExperimentCompletionRecord(experimentDocument)
	if err := errors.Join(issueErr, recordErr); err != nil {
		t.Fatalf("direct runner completion child fixture error = %v, want nil", err)
	}
	entry := runnercontrol.ExperimentManifestEntry{
		Experiment: experimentPayload.Observation.Experiment, Probe: experimentPayload.Probe,
		CompletionDigest: experimentRecord.Digest, CompletionBytes: experimentRecord.Bytes,
		ArtifactManifestDigest: core.SHA256Of(nil),
	}
	payload := runnercontrol.RunnerCompletionPayload{
		SchemaVersion: runnercontrol.SchemaVersion, Run: experimentPayload.Run,
		AdmittedIntentDigest: core.SHA256Of([]byte("admitted-intent")),
		SourceGrant:          runnercontrol.SourceGrantIdentity{Digest: core.SHA256Of([]byte("source-grant"))},
		Fence:                experimentPayload.Fence, Members: experimentPayload.Members, Terminal: runprotocol.TerminalCompleted,
		BeganAt: temporal.InstantFromNanoseconds(1_000_000), CompletedAt: temporal.InstantFromNanoseconds(4_000_000),
		ArtifactManifestDigest: core.SHA256Of(nil),
		DirectExperiment: &runnercontrol.DirectExperimentRunnerCompletion{
			Source: experimentPayload.Probe.Source, Probe: experimentPayload.Probe, Experiment: entry,
		},
	}
	if err := payload.Validate(); err != nil {
		t.Fatalf("RunnerCompletionPayload.Validate() direct fixture error = %v, want nil", err)
	}
	return payload
}

func preSourceRunnerCompletionPayloadFixture(t testing.TB) runnercontrol.RunnerCompletionPayload {
	t.Helper()
	direct := directRunnerCompletionPayloadFixture(t)
	payload := runnercontrol.RunnerCompletionPayload{
		SchemaVersion: runnercontrol.SchemaVersion, Run: direct.Run,
		AdmittedIntentDigest: direct.AdmittedIntentDigest, SourceGrant: direct.SourceGrant,
		Fence: direct.Fence, Members: direct.Members, Terminal: runprotocol.TerminalUnavailable,
		BeganAt: direct.BeganAt, CompletedAt: direct.CompletedAt,
		ArtifactManifestDigest: core.SHA256Of(nil),
		PreSource: &runnercontrol.PreSourceRunnerCompletion{
			Attempted: runnercontrol.AttemptedSource{Repository: direct.DirectExperiment.Source.Repository, Commit: direct.DirectExperiment.Source.Commit},
			Failure:   core.ErrFilestoreSource,
		},
	}
	if err := payload.Validate(); err != nil {
		t.Fatalf("RunnerCompletionPayload.Validate() pre-source fixture error = %v, want nil", err)
	}
	return payload
}

func mustRunnerCompletionJSON(t testing.TB, document runnercontrol.RunnerCompletionDocument) []byte {
	t.Helper()
	got, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("RunnerCompletionDocument.MarshalJSON() setup error = %v, want nil", err)
	}
	return got
}
