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

func TestSchedulingCapabilitySignatureChainLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every capability layer carries an independently verified control signature", func(t *testing.T) {
		t.Parallel()
		claim, trusted := schedulingClaimDocumentFixture(t)
		if err := runnercontrol.VerifySchedulingClaim(claim, trusted); err != nil {
			t.Fatalf("VerifySchedulingClaim(genuine chain) error = %v, want nil", err)
		}
		record, err := runnercontrol.NewSchedulingClaimRecord(claim)
		if err != nil || record.Bytes.Uint64() == 0 || record.Digest != core.SHA256Of(record.Canonical) {
			t.Fatalf("NewSchedulingClaimRecord(genuine chain) = (%+v, %v), want non-vacuous exact bytes and nil", record, err)
		}
	})

	t.Run("negative one valid member mutation cannot retain the original control signature", func(t *testing.T) {
		t.Parallel()
		claim, trusted := schedulingClaimDocumentFixture(t)
		mutated := claim.Members[0]
		mutated.Payload.Nonce = core.SHA256Of([]byte("foreign member nonce"))
		if err := mutated.Validate(); err != nil {
			t.Fatalf("MemberCapabilityDocument.Validate(structurally valid mutation) error = %v, want nil before authentication", err)
		}
		if err := runnercontrol.VerifyMemberCapability(mutated, trusted); !errors.Is(err, core.ErrAttestVerification) {
			t.Fatalf("VerifyMemberCapability(mutated signed body) error = %v, want %v", err, core.ErrAttestVerification)
		}
	})

	t.Run("neutral canonical replay verifies identically without adding or dropping capability layers", func(t *testing.T) {
		t.Parallel()
		claim, trusted := schedulingClaimDocumentFixture(t)
		first, firstErr := claim.MarshalJSON()
		var replay runnercontrol.SchedulingClaim
		decodeErr := replay.UnmarshalJSON(first)
		second, secondErr := replay.MarshalJSON()
		verifyErr := runnercontrol.VerifySchedulingClaim(replay, trusted)
		if err := errors.Join(firstErr, decodeErr, secondErr, verifyErr); err != nil || !bytes.Equal(first, second) || len(replay.Members) != len(claim.Members) || len(replay.Direct) != len(claim.Direct) {
			t.Fatalf("scheduling claim canonical replay = (equal %t, members %d, direct %d, error %v), want (true, %d, %d, nil)", bytes.Equal(first, second), len(replay.Members), len(replay.Direct), err, len(claim.Members), len(claim.Direct))
		}
	})
}

func TestExpansionApprovalCapabilitySignatureLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive approved expansion verifies every issued experiment capability", func(t *testing.T) {
		t.Parallel()
		signer, trusted := completionSignerFixture(t)
		approval := expansionApprovalSeed(t, expansionManifestFixture(t, true), signer)
		if err := runnercontrol.VerifyExpansionApproval(approval, trusted); err != nil {
			t.Fatalf("VerifyExpansionApproval(genuine approval) error = %v, want nil", err)
		}
	})

	t.Run("negative structurally valid experiment mutation cannot retain approval authority", func(t *testing.T) {
		t.Parallel()
		signer, trusted := completionSignerFixture(t)
		approval := expansionApprovalSeed(t, expansionManifestFixture(t, true), signer)
		approval.Experiments[0].Payload.BuildContextDigest = core.SHA256Of([]byte("foreign build context"))
		if err := approval.Validate(); err != nil {
			t.Fatalf("ExpansionApproval.Validate(structural mutation) error = %v, want nil before authentication", err)
		}
		if err := runnercontrol.VerifyExpansionApproval(approval, trusted); !errors.Is(err, core.ErrAttestVerification) {
			t.Fatalf("VerifyExpansionApproval(mutated experiment) error = %v, want %v", err, core.ErrAttestVerification)
		}
	})

	t.Run("neutral typed refusal carries no experiment authority", func(t *testing.T) {
		t.Parallel()
		_, trusted := completionSignerFixture(t)
		manifest := expansionManifestFixture(t, true)
		manifestDigest, digestErr := manifest.Digest()
		if digestErr != nil {
			t.Fatalf("ExpansionManifest.Digest(refusal) error = %v, want nil", digestErr)
		}
		refusal := projectstandards.RefusalBudget
		approval := runnercontrol.ExpansionApproval{
			SchemaVersion:  runnercontrol.SchemaVersion,
			Run:            manifest.Run,
			ManifestDigest: manifestDigest,
			Refusal:        &refusal,
			Experiments:    []runnercontrol.ExperimentCapabilityDocument{},
		}
		if err := runnercontrol.VerifyExpansionApproval(approval, trusted); err != nil || len(approval.Experiments) != 0 {
			t.Fatalf("VerifyExpansionApproval(refusal) = (experiments %d, error %v), want (0, nil)", len(approval.Experiments), err)
		}
	})
}

func TestEverySchedulingCapabilityLayerRejectsAuthenticLookingSubstitution(t *testing.T) {
	t.Parallel()
	claim, trusted := schedulingClaimDocumentFixture(t)
	foreignTrusted := foreignSchedulingTrustedKeys(t)

	cases := []struct {
		name   string
		verify func() error
	}{
		{name: "scheduling expiry changed under original signature", verify: func() error {
			got := claim.Capability
			got.Payload.ExpiresAt = temporal.InstantFromNanoseconds(3_499_999)
			return runnercontrol.VerifySchedulingCapability(got, trusted)
		}},
		{name: "scheduling deadline changed under original signature", verify: func() error {
			got := claim.Capability
			got.Payload.AbsoluteDeadline = temporal.InstantFromNanoseconds(3_999_999)
			return runnercontrol.VerifySchedulingCapability(got, trusted)
		}},
		{name: "scheduling budget changed under original signature", verify: func() error {
			got := claim.Capability
			got.Payload.AggregateBudget = mustCapabilityDuration(t, 999)
			return runnercontrol.VerifySchedulingCapability(got, trusted)
		}},
		{name: "member nonce changed under original signature", verify: func() error {
			got := claim.Members[0]
			got.Payload.Nonce = core.SHA256Of([]byte("changed nonce"))
			return runnercontrol.VerifyMemberCapability(got, trusted)
		}},
		{name: "member admitted digest changed under original signature", verify: func() error {
			got := claim.Members[0]
			got.Payload.AdmittedRunDigest = core.SHA256Of([]byte("foreign admission"))
			return runnercontrol.VerifyMemberCapability(got, trusted)
		}},
		{name: "member request changed under original signature", verify: func() error {
			got := claim.Members[0]
			got.Payload.Request = capabilityRequestIdentity(t, 77)
			return runnercontrol.VerifyMemberCapability(got, trusted)
		}},
		{name: "experiment expiry changed under original signature", verify: func() error {
			got := claim.Direct[0]
			got.Payload.ExpiresAt = temporal.InstantFromNanoseconds(2_999_999)
			return runnercontrol.VerifyExperimentCapability(got, trusted)
		}},
		{name: "experiment build context changed under original signature", verify: func() error {
			got := claim.Direct[0]
			got.Payload.BuildContextDigest = core.SHA256Of([]byte("foreign build context"))
			return runnercontrol.VerifyExperimentCapability(got, trusted)
		}},
		{name: "experiment resource ceiling changed under original signature", verify: func() error {
			got := claim.Direct[0]
			got.Payload.Resources.CPUCount++
			return runnercontrol.VerifyExperimentCapability(got, trusted)
		}},
		{name: "genuine chain presented to a foreign trust set", verify: func() error { return runnercontrol.VerifySchedulingClaim(claim, foreignTrusted) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.verify(); !errors.Is(err, core.ErrAttestVerification) {
				t.Fatalf("capability verification error = %v, want %v", err, core.ErrAttestVerification)
			}
		})
	}
}

func schedulingClaimDocumentFixture(t testing.TB) (runnercontrol.SchedulingClaim, attest.TrustedKeys) {
	t.Helper()
	completion := experimentCompletionPayloadFixture(t, true)
	signer, trusted := completionSignerFixture(t)
	observation, observationErr := projectstandards.NewMachineObservationID(capabilityUUID(t, 30))
	request := capabilityRequestIdentity(t, 31)
	if observationErr != nil {
		t.Fatalf("projectstandards.NewMachineObservationID(capability fixture) error = %v, want nil", observationErr)
	}
	digest := core.SHA256Of([]byte("scheduling capability contract"))
	schedulingPayload := runnercontrol.SchedulingCapability{
		SchemaVersion: runnercontrol.SchemaVersion, Observation: observation, Fence: completion.Fence,
		Members: completion.Members, Source: completion.Probe.Source, SourceGrant: runnercontrol.SourceGrantIdentity{Digest: digest},
		RepositoryGrant: digest, DeliveryGrant: digest, IsolationPolicy: capabilityIsolationIdentity(),
		AggregateBudget: mustCapabilityDuration(t, 1_000), AbsoluteDeadline: temporal.InstantFromNanoseconds(4_000_000),
		ExpiresAt: temporal.InstantFromNanoseconds(3_500_000),
	}
	scheduling, schedulingErr := runnercontrol.IssueSchedulingCapability(schedulingPayload, signer)
	if schedulingErr != nil {
		t.Fatalf("IssueSchedulingCapability(fixture) error = %v, want nil", schedulingErr)
	}
	schedulingDigest, digestErr := scheduling.Digest()
	if digestErr != nil {
		t.Fatalf("SchedulingCapabilityDocument.Digest(fixture) error = %v, want nil", digestErr)
	}
	memberPayload := runnercontrol.MemberCapability{
		SchemaVersion: runnercontrol.SchemaVersion, SchedulingDigest: schedulingDigest, Fence: completion.Fence,
		Request: request, Run: completion.Run, AdmittedRunDigest: core.SHA256Of([]byte("admitted run")), Probe: completion.Probe,
		Limits: capabilityRunLimits(t), Nonce: core.SHA256Of([]byte("member nonce")), ExpiresAt: temporal.InstantFromNanoseconds(3_250_000),
	}
	member, memberErr := runnercontrol.IssueMemberCapability(memberPayload, signer)
	if memberErr != nil {
		t.Fatalf("IssueMemberCapability(fixture) error = %v, want nil", memberErr)
	}
	memberDigest, memberDigestErr := member.Digest()
	if memberDigestErr != nil {
		t.Fatalf("MemberCapabilityDocument.Digest(fixture) error = %v, want nil", memberDigestErr)
	}
	experimentPayload := runnercontrol.ExperimentCapability{
		SchemaVersion: runnercontrol.SchemaVersion, MemberCapabilityDigest: memberDigest, Fence: completion.Fence,
		Run: completion.Run, Experiment: completion.Observation.Experiment, Probe: completion.Probe, Source: completion.Probe.Source,
		Execution: capabilityExecution(t), Resources: capabilityResources(t), BuildContextDigest: digest,
		ExpiresAt: temporal.InstantFromNanoseconds(3_000_000),
	}
	experiment, experimentErr := runnercontrol.IssueExperimentCapability(experimentPayload, signer)
	if experimentErr != nil {
		t.Fatalf("IssueExperimentCapability(fixture) error = %v, want nil", experimentErr)
	}
	claim := runnercontrol.SchedulingClaim{Capability: scheduling, Members: []runnercontrol.MemberCapabilityDocument{member}, Direct: []runnercontrol.ExperimentCapabilityDocument{experiment}}
	if err := errors.Join(claim.Validate(), runnercontrol.VerifySchedulingClaim(claim, trusted)); err != nil {
		t.Fatalf("SchedulingClaim signed fixture error = %v, want nil", err)
	}
	return claim, trusted
}

func capabilityExecution(t testing.TB) runnercontrol.ExperimentExecution {
	t.Helper()
	workspace := runnercontrol.WritableWorkspace{
		Root: capabilityPath(t, "/workspace"), Home: capabilityPath(t, "/workspace/home"), Output: capabilityPath(t, "/workspace/output"),
		Cache: capabilityPath(t, "/workspace/cache"), Temporary: capabilityPath(t, "/workspace/tmp"),
	}
	arguments, argumentsErr := process.ParseArguments([]string{"-c", "true"})
	environment, environmentErr := process.ParseExactEnvironment([]string{
		core.EnvironmentHomeName + "=" + workspace.Home.String(), core.EnvironmentTemporaryName + "=" + workspace.Temporary.String(), core.EnvironmentCacheName + "=" + workspace.Cache.String(),
	})
	user, userErr := projectstandards.NewIdentifier("isolated-subject")
	if err := errors.Join(argumentsErr, environmentErr, userErr); err != nil {
		t.Fatalf("capability execution fixture setup error = %v, want nil", err)
	}
	egress := runnercontrol.EgressPolicy{Mode: runnercontrol.EgressDenied, DNSPolicy: core.SHA256Of([]byte("deny all"))}
	egressDigest, egressErr := egress.Digest()
	budget, budgetErr := runnercontrol.NewExecutionBudget(mustCapabilityDuration(t, 1_000), 1, 1)
	if err := errors.Join(egressErr, budgetErr); err != nil {
		t.Fatalf("capability execution policy fixture error = %v, want nil", err)
	}
	plan := process.Plan{
		SchemaVersion: process.ExecutionPlanSchemaVersion, Command: capabilityPath(t, "/bin/sh"), WorkingDirectory: capabilityPath(t, "/source"),
		Arguments: arguments, Environment: environment, OutputLimit: capabilityByteCount(t, 1_024), WaitDelay: mustCapabilityDuration(t, 100),
		Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
	}
	subject := runnercontrol.SubjectExecution{
		Engine: runnercontrol.SubjectIsolationSystemd, Supervisor: capabilityPath(t, "/usr/bin/systemd-run"), Controller: capabilityPath(t, "/usr/bin/systemctl"),
		PolicyIdentity: capabilityIsolationIdentity(), ProcessUser: user, SourceRoot: capabilityPath(t, "/source"), EgressPolicyIdentity: egressDigest,
		ControlSocket: capabilityPath(t, "/run/primitive-control.sock"), HostCredentials: capabilityPath(t, "/var/lib/primitive/credentials"),
		SigningState: capabilityPath(t, "/var/lib/primitive/signing"), ExecutableState: capabilityPath(t, "/var/lib/primitive/runtime"),
	}
	got := runnercontrol.ExperimentExecution{Process: plan, Workspace: workspace, Subject: subject, Artifacts: []runnercontrol.ArtifactExpectation{}, Observation: runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationOpaque}, Budget: budget}
	if err := got.Validate(); err != nil {
		t.Fatalf("ExperimentExecution.Validate(capability fixture) error = %v, want nil", err)
	}
	return got
}

func capabilityResources(t testing.TB) runnercontrol.ResourceRequirement {
	t.Helper()
	got := runnercontrol.ResourceRequirement{CPUCount: 1, MemoryBytes: capabilityByteCount(t, 1_024), ProcessMaximum: 4, FileMaximum: 128, Egress: runnercontrol.EgressPolicy{Mode: runnercontrol.EgressDenied, DNSPolicy: core.SHA256Of([]byte("deny all"))}}
	if err := got.Validate(); err != nil {
		t.Fatalf("ResourceRequirement.Validate(capability fixture) error = %v, want nil", err)
	}
	return got
}

func capabilityRunLimits(t testing.TB) runnercontrol.RunLimits {
	t.Helper()
	got := runnercontrol.RunLimits{Duration: mustCapabilityDuration(t, 2_000), OutputBytes: capabilityByteCount(t, 1_024), ArtifactBytes: capabilityByteCount(t, 2_048), ArtifactCount: 2, WorkerMaximum: 2, ProcessMaximum: 8, FileMaximum: 256, QueueDepth: 4}
	if err := got.Validate(); err != nil {
		t.Fatalf("RunLimits.Validate(capability fixture) error = %v, want nil", err)
	}
	return got
}

func foreignSchedulingTrustedKeys(t testing.TB) attest.TrustedKeys {
	t.Helper()
	seed := sha256.Sum256([]byte("foreign scheduling capability signer"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey, publicErr := core.NewEd25519PublicKey(privateKey.Public().(ed25519.PublicKey))
	trusted, trustedErr := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{publicKey}})
	if err := errors.Join(publicErr, trustedErr); err != nil {
		t.Fatalf("foreign scheduling trust fixture error = %v, want nil", err)
	}
	return trusted
}

func capabilityRequestIdentity(t testing.TB, value int) projectstandards.RequestIdentity {
	t.Helper()
	got, err := projectstandards.NewRequestIdentity(capabilityUUID(t, value))
	if err != nil {
		t.Fatalf("projectstandards.NewRequestIdentity(%d) error = %v, want nil", value, err)
	}
	return got
}

func capabilityUUID(t testing.TB, value int) primitiveid.UUIDv7 {
	t.Helper()
	text := []byte("01890f2e-7b00-7000-8000-000000000000")
	hex := "0123456789abcdef"
	text[len(text)-2] = hex[(value>>4)&15]
	text[len(text)-1] = hex[value&15]
	got, err := primitiveid.ParseUUIDv7(string(text))
	if err != nil {
		t.Fatalf("id.ParseUUIDv7(capability %d) error = %v, want nil", value, err)
	}
	return got
}

func mustCapabilityDuration(t testing.TB, nanoseconds int64) temporal.Duration {
	t.Helper()
	got, err := temporal.DurationFromNanoseconds(nanoseconds)
	if err != nil {
		t.Fatalf("temporal.DurationFromNanoseconds(%d) error = %v, want nil", nanoseconds, err)
	}
	return got
}

func capabilityByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return got
}

func capabilityPath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	got, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return got
}

func capabilityIsolationIdentity() core.SHA256Digest {
	return core.SHA256Of([]byte("capability isolation policy"))
}
