package runnercontrol_test

import (
	"context"
	"errors"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestSourceAcquisitionSocketProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive runner receives the exact source grant archive and bounded bearer", func(t *testing.T) {
		t.Parallel()

		request, projection := sourceAcquisitionSocketFixture(t)
		repository := sourceAcquisitionRepositoryFunc(func(_ context.Context, got runnercontrol.SourceAcquisitionRequest) (runnercontrol.SourceAcquisitionProjection, error) {
			if got.Fence != request.Fence || got.Source != request.Source || got.Grant != request.Grant || got.RequestedAt != request.RequestedAt {
				return runnercontrol.SourceAcquisitionProjection{}, core.ErrPrimitiveContract
			}
			gotMembers, gotErr := got.Members.Digest()
			wantMembers, wantErr := request.Members.Digest()
			if gotErr != nil || wantErr != nil || gotMembers != wantMembers {
				return runnercontrol.SourceAcquisitionProjection{}, errors.Join(core.ErrPrimitiveContract, gotErr, wantErr)
			}
			return projection, nil
		})
		contract := sourceAcquisitionSocketContractFixture(t, "/runner/source")
		boundary, err := runnercontrol.NewSourceAcquisitionServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewSourceAcquisitionServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, request.Fence.Machine.Machine, request.Fence.Machine.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewSourceAcquisitionClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewSourceAcquisitionClient() error = %v, want nil", err)
		}
		response, gotErr := client.Acquire(t.Context(), request)
		gotTarget, targetErr := response.Body.Capability.Target()
		wantTarget := sourceProjectionTargetFixture(t, projection)
		serverErr := waitRunnerControlSocketServer(t, result)
		if gotErr != nil || targetErr != nil || response.Body.Fence != request.Fence || response.Body.Grant != projection.Grant ||
			response.Body.Document.Manifest != projection.Document.Manifest || response.Body.Integrity != projection.Integrity || response.Body.ContentType != projection.ContentType || gotTarget.ExpiresAt != wantTarget.ExpiresAt || serverErr != nil {
			t.Fatalf("SourceAcquisitionClient.Acquire() = (body %+v, target expiry %v, errors %v/%v, server %v), want exact source projection and nil", response.Body, gotTarget.ExpiresAt, gotErr, targetErr, serverErr)
		}
	})

	t.Run("negative missing repository constructs no source effect capability", func(t *testing.T) {
		t.Parallel()

		contract := sourceAcquisitionSocketContractFixture(t, "/runner/source-refusal")
		_, gotErr := runnercontrol.NewSourceAcquisitionServer(contract, nil)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("NewSourceAcquisitionServer(no repository) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral zero request reaches no network or source repository", func(t *testing.T) {
		t.Parallel()

		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
		t.Cleanup(server.Close)
		contract := sourceAcquisitionSocketContractFixture(t, "/runner/source-zero")
		client, err := runnercontrol.NewSourceAcquisitionClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewSourceAcquisitionClient() error = %v, want nil", err)
		}
		response, gotErr := client.Acquire(t.Context(), runnercontrol.SourceAcquisitionRequest{})
		if gotErr == nil || response.Body.SchemaVersion != 0 || !response.Body.Capability.IsZero() || calls != 0 {
			t.Fatalf("SourceAcquisitionClient.Acquire(zero) = (%+v, %v, network calls %d), want zero, typed refusal, zero", response.Body, gotErr, calls)
		}
	})
}

func TestMachineObservationSocketProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive runner observation persists before an exact clean-state receipt returns", func(t *testing.T) {
		t.Parallel()

		submission := machineObservationSubmissionFixture(t)
		calls := 0
		repository := machineObservationRepositoryFunc(func(_ context.Context, got runnercontrol.MachineObservationSubmission) error {
			calls++
			if got.Observation.ID != submission.Observation.ID || got.Observation.GenerationID != submission.Observation.GenerationID || got.Clean != submission.Clean {
				return core.ErrPrimitiveContract
			}
			return nil
		})
		contract := machineObservationSocketContractFixture(t, "/runner/machine-observation")
		boundary, err := runnercontrol.NewMachineObservationServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewMachineObservationServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, submission.Observation.Configuration.Identity.ID, submission.Observation.GenerationID)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewMachineObservationClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewMachineObservationClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), submission)
		cleanBytes, cleanErr := core.MarshalCanonicalJSONDocument(submission.Clean)
		wantDigest := core.SHA256Of(cleanBytes)
		if gotErr != nil || cleanErr != nil || response.Body.ObservationID != submission.Observation.ID || response.Body.CleanDigest != wantDigest || calls != 1 {
			t.Fatalf("MachineObservationClient.Submit() = (%+v, %v, clean error %v, repository calls %d), want observation %v digest %v and one call", response.Body, gotErr, cleanErr, calls, submission.Observation.ID, wantDigest)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("MachineObservationServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("negative missing repository constructs no observation effect capability", func(t *testing.T) {
		t.Parallel()

		contract := machineObservationSocketContractFixture(t, "/runner/machine-observation-refusal")
		_, gotErr := runnercontrol.NewMachineObservationServer(contract, nil)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("NewMachineObservationServer(no repository) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral zero observation reaches no network or repository", func(t *testing.T) {
		t.Parallel()

		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
		t.Cleanup(server.Close)
		contract := machineObservationSocketContractFixture(t, "/runner/machine-observation-zero")
		client, err := runnercontrol.NewMachineObservationClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewMachineObservationClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), runnercontrol.MachineObservationSubmission{})
		if gotErr == nil || response.Body != (runnercontrol.MachineObservationReceipt{}) || calls != 0 {
			t.Fatalf("MachineObservationClient.Submit(zero) = (%+v, %v, network calls %d), want zero, typed refusal, zero", response.Body, gotErr, calls)
		}
	})
}

type sourceAcquisitionRepositoryFunc func(context.Context, runnercontrol.SourceAcquisitionRequest) (runnercontrol.SourceAcquisitionProjection, error)

func (f sourceAcquisitionRepositoryFunc) AcquireSource(ctx context.Context, request runnercontrol.SourceAcquisitionRequest) (runnercontrol.SourceAcquisitionProjection, error) {
	return f(ctx, request)
}

type machineObservationRepositoryFunc func(context.Context, runnercontrol.MachineObservationSubmission) error

func (f machineObservationRepositoryFunc) RecordMachineObservation(ctx context.Context, submission runnercontrol.MachineObservationSubmission) error {
	return f(ctx, submission)
}

func sourceAcquisitionSocketFixture(t testing.TB) (runnercontrol.SourceAcquisitionRequest, runnercontrol.SourceAcquisitionProjection) {
	t.Helper()
	authenticated, admitted := admissionFixture(t)
	payload := experimentCompletionPayloadFixture(t, true)
	archive := []byte("bounded-source-archive")
	archiveBytes, bytesErr := core.NewByteLength(uint64(len(archive)))
	fileMaximum, fileErr := core.NewByteCount(1 << 20)
	manifest := runnercontrol.SourceArchiveManifest{
		SchemaVersion: runnercontrol.SchemaVersion, Repository: authenticated.Requested.Probe.Source.Repository,
		Commit: authenticated.Requested.Probe.Source.Commit, Tree: authenticated.Requested.Probe.Source.Tree,
		ArchiveDigest: core.SHA256Of(archive), ArchiveBytes: archiveBytes, EntryMaximum: 128, DepthMaximum: 32,
		FileMaximumBytes: fileMaximum, IssuedAt: temporal.InstantFromNanoseconds(1), ExpiresAt: temporal.InstantFromNanoseconds(90),
	}
	key, _ := completionSignerFixture(t)
	document, documentErr := runnercontrol.IssueSourceArchive(manifest, key)
	signedURL, urlErr := objectstore.ParseSignedURL("https://storage.googleapis.com/runner/source.tar?X-Goog-Signature=sig&X-Goog-SignedHeaders=host")
	headers, headersErr := objectstore.NewSignedHeaders(nil)
	target := objectstore.DownloadTarget{URL: signedURL, Headers: headers, ExpiresAt: temporal.InstantFromNanoseconds(50)}
	capability, capabilityErr := objectstore.NewDownloadCapabilityProjection(objectstore.ProviderGoogleCloudStorage, target)
	errorLimit, limitErr := core.NewByteCount(4096)
	operationTimeout, operationErr := temporal.DurationFromSeconds(10)
	attemptTimeout, attemptErr := temporal.DurationFromSeconds(5)
	integrity := objectstore.Integrity{SHA256: manifest.ArchiveDigest, Length: manifest.ArchiveBytes, CRC32C: core.NewCRC32C(crc32.Checksum(archive, crc32.MakeTable(crc32.Castagnoli)))}
	policy := objectstore.Policy{OperationTimeout: operationTimeout, AttemptTimeout: attemptTimeout, ErrorBodyLimit: errorLimit}
	grant := authenticated.Requested.Probe.Source
	request := runnercontrol.SourceAcquisitionRequest{
		SchemaVersion: runnercontrol.SchemaVersion, Fence: payload.Fence, Members: payload.Members,
		Source: grant, Grant: admitted.Source.Identity, RequestedAt: temporal.InstantFromNanoseconds(2),
	}
	projection := runnercontrol.SourceAcquisitionProjection{
		SchemaVersion: runnercontrol.SchemaVersion, Fence: request.Fence, Members: request.Members,
		Grant: admitted.Source, Document: document, Capability: capability,
		Integrity: integrity, ContentType: core.HTTPMediaTypeOctetStream(), Policy: policy,
	}
	if err := errors.Join(bytesErr, fileErr, documentErr, urlErr, headersErr, capabilityErr, limitErr, operationErr, attemptErr, request.Validate(), projection.Validate()); err != nil {
		t.Fatalf("source acquisition socket fixture error = %v, want nil", err)
	}
	return request, projection
}

func sourceProjectionTargetFixture(t testing.TB, projection runnercontrol.SourceAcquisitionProjection) objectstore.DownloadTarget {
	t.Helper()
	encoded, err := projection.Capability.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var capability objectstore.DownloadCapability
	if err := capability.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON() error = %v, want nil", err)
	}
	target, err := capability.Target()
	if err != nil {
		t.Fatalf("DownloadCapability.Target() error = %v, want nil", err)
	}
	return target
}

func machineObservationSubmissionFixture(t testing.TB) runnercontrol.MachineObservationSubmission {
	t.Helper()
	uuid := completionUUIDFixture(t)
	machineID, machineErr := runprotocol.NewMachineID(uuid)
	observationID, observationErr := runprotocol.NewMachineObservationID(uuid)
	generationID, generationErr := runprotocol.NewMachineGenerationID(uuid)
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{1})
	configuration := runprotocol.MachineConfiguration{
		Identity: runprotocol.MachineIdentity{
			ID: machineID, Provider: core.Offering{Token: "fixture-provider"}, Project: machineIdentifierFixture(t, "project"),
			Instance: machineIdentifierFixture(t, "machine"), Zone: machineIdentifierFixture(t, "zone"), MachineType: machineIdentifierFixture(t, "class"),
		},
		Compute: runprotocol.MachineCompute{
			CPUPlatform: mustProfileName(t, "cpu"), Processor: mustProfileName(t, "processor"), Architecture: mustProfileName(t, "amd64"), Virtualization: mustProfileName(t, "virtual"),
			VCPU: 2, Sockets: 1, CoresPerSocket: 1, ThreadsPerCore: 2, NUMANodes: 1,
			MemoryConfiguredBytes: mustProfileByteCount(t, 1024), MemoryGuestBytes: mustProfileByteCount(t, 1024),
		},
		System: runprotocol.MachineSystem{
			OperatingSystem: mustProfileName(t, "linux"), OperatingSystemVersion: mustProfileName(t, "1"),
			OperatingSystemImage: mustProfileName(t, "image"), Kernel: mustProfileName(t, "kernel"),
		},
		Storage: runprotocol.MachineStorage{
			BootDiskType: mustProfileName(t, "disk"), Interface: mustProfileName(t, "interface"), Filesystem: mustProfileName(t, "filesystem"),
			PhysicalBlockBytes: mustProfileByteCount(t, 4096), CapacityBytes: mustProfileByteCount(t, 4096),
			BaselineIOPS: 1, BaselineReadBytes: 1, InstanceCeilingIOPS: 1, InstanceCeilingReadBytes: 1,
			SwapBytes: mustCompletionByteLength(t, 0),
		},
		Network: runprotocol.MachineNetwork{
			Interface: mustProfileName(t, "network"), NetworkTier: mustProfileName(t, "tier"), Addressing: mustProfileName(t, "addressing"), VPC: machineIdentifierFixture(t, "vpc"),
			MTU: 1500, ReceiveQueues: 1, TransmitQueues: 1, EgressFloorBits: 1, EgressCeilingBits: 1,
		},
		Lifecycle:  runprotocol.MachineLifecycleSecurity{ProvisioningModel: runprotocol.MachineProvisioningStandard, HostMaintenance: runprotocol.MachineMaintenanceMigrate},
		Toolchains: []runprotocol.MachineToolchain{{Tool: runprotocol.MachineToolchainGo, Version: mustProfileName(t, "go1.27"), Platform: mustProfileName(t, "linux/amd64"), InstallMode: runprotocol.MachineInstallModeInstalled, ExecutableSHA256: digest}},
	}
	fingerprint, fingerprintErr := configuration.Fingerprint()
	observation := runprotocol.MachineObservation{
		SchemaVersion: runprotocol.MachineProbeSchemaVersion, ID: observationID, GenerationID: generationID,
		ObservedAt: temporal.InstantFromNanoseconds(10), Collector: runprotocol.EvidenceAuthority{Offering: core.Offering{Token: "runner"}},
		Configuration: configuration, Fingerprint: fingerprint,
		Runtime: runprotocol.MachineRuntime{
			BootID: machineIdentifierFixture(t, "boot"), Address: mustProfileName(t, "127.0.0.1"), Uptime: mustProfileDuration(t, 1),
			MemoryAvailableBytes: mustCompletionByteLength(t, 512), DiskAvailableBytes: mustCompletionByteLength(t, 2048),
		},
		Execution: runprotocol.MachineProbeExecution{
			Bash: mustProfileAbsolutePath(t, "/bin/sh"), Script: mustProfileAbsolutePath(t, "/runner/probe.sh"),
			ScriptBytes: mustCompletionByteLength(t, 1), OutputLimit: mustProfileByteCount(t, 1024), CPUTime: mustProfileDuration(t, 1),
			StdoutBytes: mustCompletionByteLength(t, 1), StderrBytes: mustCompletionByteLength(t, 0),
			ScriptDigest: digest, StdoutDigest: digest, StderrDigest: core.SHA256Of(nil),
		},
	}
	clean := cleanupPayloadFixture(t).After
	submission := runnercontrol.MachineObservationSubmission{SchemaVersion: runnercontrol.SchemaVersion, Observation: observation, Clean: clean}
	if err := errors.Join(machineErr, observationErr, generationErr, fingerprintErr, submission.Validate()); err != nil {
		t.Fatalf("machine observation submission fixture error = %v, want nil", err)
	}
	return submission
}

func machineIdentifierFixture(t testing.TB, value string) runprotocol.Identifier {
	t.Helper()
	identifier, err := runprotocol.NewIdentifier(value)
	if err != nil {
		t.Fatalf("runprotocol.NewIdentifier(%q) error = %v, want nil", value, err)
	}
	return identifier
}

func sourceAcquisitionSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.SourceAcquisitionSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.SourceAcquisitionSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func machineObservationSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.MachineObservationSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.MachineObservationSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}
