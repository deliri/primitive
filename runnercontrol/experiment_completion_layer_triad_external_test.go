package runnercontrol_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestExperimentCompletionProducerSchemaVerifierLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive started experiment signs exact process facts and verifies independently", func(t *testing.T) {
		t.Parallel()
		payload := experimentCompletionPayloadFixture(t, true)
		key, trusted := completionSignerFixture(t)

		got, gotErr := runnercontrol.IssueExperimentCompletion(payload, key)
		if gotErr != nil {
			t.Fatalf("IssueExperimentCompletion() error = %v, want nil", gotErr)
		}
		if gotErr := runnercontrol.VerifyExperimentCompletion(got, trusted); gotErr != nil {
			t.Fatalf("VerifyExperimentCompletion(issued) error = %v, want nil", gotErr)
		}
		record, recordErr := runnercontrol.NewExperimentCompletionRecord(got)
		if recordErr != nil {
			t.Fatalf("NewExperimentCompletionRecord() error = %v, want nil", recordErr)
		}
		if record.Bytes.Uint64() == 0 || record.Digest != core.SHA256Of(record.Canonical) || !bytes.Equal(record.Canonical, mustCompletionJSON(t, got)) {
			t.Fatalf("completion record = (bytes %d, digest %v, canonical %d bytes), want nonzero exact canonical digest and bytes", record.Bytes.Uint64(), record.Digest, len(record.Canonical))
		}
	})

	t.Run("negative one-fact mutation keeps structure valid but fails the independent signature oracle", func(t *testing.T) {
		t.Parallel()
		payload := experimentCompletionPayloadFixture(t, true)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueExperimentCompletion(payload, key)
		if issueErr != nil {
			t.Fatalf("IssueExperimentCompletion() setup error = %v, want nil", issueErr)
		}
		document.Payload.CompletedAt = temporal.InstantFromNanoseconds(3_000_001)
		if gotErr := document.Validate(); gotErr != nil {
			t.Fatalf("mutated ExperimentCompletionDocument.Validate() error = %v, want nil so signature verification owns rejection", gotErr)
		}

		gotErr := runnercontrol.VerifyExperimentCompletion(document, trusted)
		if !errors.Is(gotErr, core.ErrAttestVerification) {
			t.Fatalf("VerifyExperimentCompletion(mutated completed_at) error = %v, want errors.Is(..., %v)", gotErr, core.ErrAttestVerification)
		}
	})

	t.Run("neutral not-run experiment signs zero execution facts without inventing a process", func(t *testing.T) {
		t.Parallel()
		payload := experimentCompletionPayloadFixture(t, false)
		key, trusted := completionSignerFixture(t)

		got, gotErr := runnercontrol.IssueExperimentCompletion(payload, key)
		if gotErr != nil {
			t.Fatalf("IssueExperimentCompletion(not-run) error = %v, want nil", gotErr)
		}
		if got.Payload.StartedAt != nil || got.Payload.Process != nil || got.Payload.Observation.Measurements.DurationNs != 0 || len(got.Payload.Observation.Artifacts) != 0 {
			t.Fatalf("not-run completion execution facts = (started_at %v, process %v, duration %d, artifacts %d), want nil, nil, 0, 0", got.Payload.StartedAt, got.Payload.Process, got.Payload.Observation.Measurements.DurationNs, len(got.Payload.Observation.Artifacts))
		}
		if gotErr := runnercontrol.VerifyExperimentCompletion(got, trusted); gotErr != nil {
			t.Fatalf("VerifyExperimentCompletion(not-run) error = %v, want nil", gotErr)
		}
	})
}

func experimentCompletionPayloadFixture(t testing.TB, started bool) runnercontrol.ExperimentCompletionPayload {
	t.Helper()
	uuid := completionUUIDFixture(t)
	run, runErr := projectstandards.NewRunID(uuid)
	experiment, experimentErr := projectstandards.NewExperimentID(uuid)
	machine, machineErr := projectstandards.NewMachineID(uuid)
	generation, generationErr := projectstandards.NewMachineGenerationID(uuid)
	repository, repositoryErr := projectstandards.NewRepositoryIdentity("github.com/example/project")
	tool, toolErr := projectstandards.NewIdentifier("go-test")
	machineClass, machineClassErr := projectstandards.NewIdentifier("runner-standard")
	profileName, profileNameErr := projectstandards.NewIdentifier("acceptance")
	profile, profileErr := projectstandards.NewProfileIdentity(profileName, 1)
	commit, commitErr := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err := errors.Join(runErr, experimentErr, machineErr, generationErr, repositoryErr, toolErr, machineClassErr, profileNameErr, profileErr, commitErr); err != nil {
		t.Fatalf("experiment completion identity fixture error = %v, want nil", err)
	}
	digest := core.SHA256Of([]byte("runner-experiment-completion"))
	members := runnercontrol.MemberSet{Entries: []projectstandards.RunID{run}}
	memberDigest, memberErr := members.Digest()
	if memberErr != nil {
		t.Fatalf("MemberSet.Digest() completion fixture error = %v, want nil", memberErr)
	}
	probe := projectstandards.ProbeIdentity{
		Origin:  projectstandards.OriginIdentity{Offering: core.Offering{Token: "cinkin"}},
		Subject: projectstandards.SubjectIdentity{Project: core.Offering{Token: "runner"}, Repository: repository},
		Source:  projectstandards.SourceCoordinate{Repository: repository, Commit: commit, Tree: digest},
		Role:    projectstandards.ProbeRoleExperiment, Kind: projectstandards.ProbeKindTool,
		Target:  projectstandards.ProbeTarget{Kind: projectstandards.ProbeTargetTool, Tool: &projectstandards.ToolTarget{Identity: tool}},
		Profile: profile,
		Environment: projectstandards.AdmittedEnvironment{
			MachineClass: machineClass, RequirementFingerprint: digest,
			EnvironmentFingerprint: digest, MachineGeneration: generation, MachineSheetDigest: digest,
		},
	}
	outcome := projectstandards.OutcomeNotRun
	measurements := projectstandards.ExperimentMeasurements{Benchmarks: []projectstandards.BenchmarkMeasurement{}, Complexity: []projectstandards.ComplexityCapture{}}
	var startedAt *temporal.Instant
	var result *process.ResultObservation
	if started {
		outcome = projectstandards.OutcomePassed
		measurements.DurationNs = 1
		start := temporal.InstantFromNanoseconds(2_000_000)
		startedAt = &start
		processResult := process.ResultObservation{
			ExitCode: 0, CPUTime: mustCompletionDuration(t, 1),
			StdinBytes: mustCompletionByteLength(t, 0), StdoutBytes: mustCompletionByteLength(t, 11), StderrBytes: mustCompletionByteLength(t, 0),
		}
		result = &processResult
	}
	payload := runnercontrol.ExperimentCompletionPayload{
		SchemaVersion: runnercontrol.SchemaVersion, Run: run, Probe: probe,
		Members: members,
		Fence: runnercontrol.SchedulingFence{
			Machine:         runnercontrol.MachineFence{Machine: machine, Generation: generation, Epoch: 1, ExpiresAt: temporal.InstantFromNanoseconds(4_000_000)},
			Unit:            runnercontrol.SchedulingUnitIdentity{Kind: runnercontrol.SchedulingUnitRunPlan, Identity: uuid},
			MemberSetDigest: memberDigest,
		},
		Observation: projectstandards.ExperimentObservation{
			Experiment: experiment, Started: started, Outcome: outcome,
			EnvironmentFingerprint: digest, ExecutionFingerprint: core.SHA256Of([]byte("execution")), MachineSheetDigest: digest,
			Measurements: measurements, Artifacts: []projectstandards.ArtifactReference{},
		},
		StartedAt: startedAt, CompletedAt: temporal.InstantFromNanoseconds(3_000_000), Process: result,
	}
	if err := payload.Validate(); err != nil {
		t.Fatalf("ExperimentCompletionPayload.Validate() fixture error = %v, want nil", err)
	}
	return payload
}

func mustCompletionDuration(t testing.TB, nanoseconds int64) temporal.Duration {
	t.Helper()
	got, err := temporal.DurationFromNanoseconds(nanoseconds)
	if err != nil {
		t.Fatalf("temporal.DurationFromNanoseconds(%d) completion fixture error = %v, want nil", nanoseconds, err)
	}
	return got
}

func completionSignerFixture(t testing.TB) (ed25519.PrivateKey, attest.TrustedKeys) {
	t.Helper()
	seed := sha256.Sum256([]byte("primitive-runnercontrol-experiment-completion-test"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey, publicErr := core.NewEd25519PublicKey(privateKey.Public().(ed25519.PublicKey))
	trusted, trustedErr := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{publicKey}})
	if err := errors.Join(publicErr, trustedErr); err != nil {
		t.Fatalf("completion signer fixture error = %v, want nil", err)
	}
	return privateKey, trusted
}

func completionUUIDFixture(t testing.TB) primitiveid.UUIDv7 {
	t.Helper()
	got, err := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	if err != nil {
		t.Fatalf("id.ParseUUIDv7() completion fixture error = %v, want nil", err)
	}
	return got
}

func mustCompletionByteLength(t testing.TB, value uint64) core.ByteLength {
	t.Helper()
	got, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) completion fixture error = %v, want nil", value, err)
	}
	return got
}

func mustCompletionJSON(t testing.TB, document runnercontrol.ExperimentCompletionDocument) []byte {
	t.Helper()
	got, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("ExperimentCompletionDocument.MarshalJSON() setup error = %v, want nil", err)
	}
	return got
}
