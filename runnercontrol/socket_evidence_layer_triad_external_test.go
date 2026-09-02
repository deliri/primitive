package runnercontrol_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestEvidenceSocketProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive experiment completion crosses authentication verification persistence and receipt", func(t *testing.T) {
		t.Parallel()

		payload := experimentCompletionPayloadFixture(t, true)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueExperimentCompletion(payload, key)
		wantRecord, recordErr := runnercontrol.NewExperimentCompletionRecord(document)
		if err := errors.Join(issueErr, recordErr); err != nil {
			t.Fatalf("experiment completion socket fixture error = %v, want nil", err)
		}
		repository := experimentCompletionRepositoryFunc(func(_ context.Context, got runnercontrol.ExperimentCompletionRecord) error {
			if got.Digest != wantRecord.Digest || got.Bytes != wantRecord.Bytes || !bytes.Equal(got.Canonical, wantRecord.Canonical) {
				return core.ErrPrimitiveContract
			}
			return nil
		})
		contract := experimentCompletionSocketContractFixture(t, "/runner/experiment-completion")
		boundary, err := runnercontrol.NewExperimentCompletionServer(contract, repository, trusted)
		if err != nil {
			t.Fatalf("runnercontrol.NewExperimentCompletionServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, payload.Fence.Machine.Machine, payload.Fence.Machine.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewExperimentCompletionClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewExperimentCompletionClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), document)
		if gotErr != nil || response.Body.Run != payload.Run || response.Body.Experiment != payload.Observation.Experiment || response.Body.Digest != wantRecord.Digest || response.Body.Bytes != wantRecord.Bytes {
			t.Fatalf("ExperimentCompletionClient.Submit() = (%+v, %v), want exact record %v/%v and nil", response.Body, gotErr, wantRecord.Digest, wantRecord.Bytes)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("ExperimentCompletionServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive runner completion crosses authentication verification persistence and receipt", func(t *testing.T) {
		t.Parallel()

		payload := directRunnerCompletionPayloadFixture(t)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueRunnerCompletion(payload, key)
		wantRecord, recordErr := runnercontrol.NewRunnerCompletionRecord(document)
		if err := errors.Join(issueErr, recordErr); err != nil {
			t.Fatalf("runner completion socket fixture error = %v, want nil", err)
		}
		repository := runnerCompletionRepositoryFunc(func(_ context.Context, got runnercontrol.RunnerCompletionRecord) error {
			if got.Digest != wantRecord.Digest || got.Bytes != wantRecord.Bytes || !bytes.Equal(got.Canonical, wantRecord.Canonical) {
				return core.ErrPrimitiveContract
			}
			return nil
		})
		contract := runnerCompletionSocketContractFixture(t, "/runner/completion")
		boundary, err := runnercontrol.NewRunnerCompletionServer(contract, repository, trusted)
		if err != nil {
			t.Fatalf("runnercontrol.NewRunnerCompletionServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, payload.Fence.Machine.Machine, payload.Fence.Machine.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewRunnerCompletionClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewRunnerCompletionClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), document)
		if gotErr != nil || response.Body.Run != payload.Run || response.Body.Digest != wantRecord.Digest || response.Body.Bytes != wantRecord.Bytes {
			t.Fatalf("RunnerCompletionClient.Submit() = (%+v, %v), want exact record %v/%v and nil", response.Body, gotErr, wantRecord.Digest, wantRecord.Bytes)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("RunnerCompletionServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive cleanup crosses authentication verification persistence and receipt", func(t *testing.T) {
		t.Parallel()

		payload := cleanupPayloadFixture(t)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueCleanup(payload, key)
		wantRecord, recordErr := runnercontrol.NewCleanupRecord(document)
		if err := errors.Join(issueErr, recordErr); err != nil {
			t.Fatalf("cleanup socket fixture error = %v, want nil", err)
		}
		repository := cleanupRepositoryFunc(func(_ context.Context, got runnercontrol.CleanupRecord) error {
			if got.Digest != wantRecord.Digest || got.Bytes != wantRecord.Bytes || !bytes.Equal(got.Canonical, wantRecord.Canonical) {
				return core.ErrPrimitiveContract
			}
			return nil
		})
		contract := cleanupSocketContractFixture(t, "/runner/cleanup")
		boundary, err := runnercontrol.NewCleanupServer(contract, repository, trusted)
		if err != nil {
			t.Fatalf("runnercontrol.NewCleanupServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, payload.Fence.Machine.Machine, payload.Fence.Machine.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewCleanupClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewCleanupClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), document)
		if gotErr != nil || response.Body.Fence != payload.Fence || response.Body.Digest != wantRecord.Digest || response.Body.Bytes != wantRecord.Bytes {
			t.Fatalf("CleanupClient.Submit() = (%+v, %v), want exact record %v/%v and nil", response.Body, gotErr, wantRecord.Digest, wantRecord.Bytes)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("CleanupServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive expansion crosses authentication verification policy and approval", func(t *testing.T) {
		t.Parallel()

		manifest := expansionManifestFixture(t, true)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueExpansion(manifest, key)
		wantRecord, recordErr := runnercontrol.NewExpansionRecord(document)
		approval := expansionApprovalSeed(t, manifest, key)
		if err := errors.Join(issueErr, recordErr, approval.Validate()); err != nil {
			t.Fatalf("expansion socket fixture error = %v, want nil", err)
		}
		repository := expansionRepositoryFunc(func(_ context.Context, got runnercontrol.ExpansionRecord) (runnercontrol.ExpansionApproval, error) {
			if got.Digest != wantRecord.Digest || got.Bytes != wantRecord.Bytes || !bytes.Equal(got.Canonical, wantRecord.Canonical) {
				return runnercontrol.ExpansionApproval{}, core.ErrPrimitiveContract
			}
			return approval, nil
		})
		contract := expansionSocketContractFixture(t, "/runner/expansion")
		boundary, err := runnercontrol.NewExpansionServer(contract, repository, trusted)
		if err != nil {
			t.Fatalf("runnercontrol.NewExpansionServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, manifest.Fence.Machine.Machine, manifest.Fence.Machine.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewExpansionClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewExpansionClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), document)
		if gotErr != nil || response.Body.Run != approval.Run || response.Body.ManifestDigest != approval.ManifestDigest || len(response.Body.Experiments) != len(approval.Experiments) {
			t.Fatalf("ExpansionClient.Submit() = (%+v, %v), want approval for run %v manifest %v and nil", response.Body, gotErr, approval.Run, approval.ManifestDigest)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("ExpansionServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive artifact manifest crosses replay binding and persists canonical bytes", func(t *testing.T) {
		t.Parallel()

		manifest, _ := artifactFixture(t, []byte("artifact-socket-evidence"))
		wantRecord, recordErr := runnercontrol.NewArtifactManifestRecord(manifest)
		if recordErr != nil {
			t.Fatalf("runnercontrol.NewArtifactManifestRecord() error = %v, want nil", recordErr)
		}
		repository := artifactManifestRepositoryFunc(func(_ context.Context, got runnercontrol.ArtifactManifestRecord) error {
			if got.Digest != wantRecord.Digest || got.Bytes != wantRecord.Bytes || !bytes.Equal(got.Canonical, wantRecord.Canonical) {
				return core.ErrPrimitiveContract
			}
			return nil
		})
		contract := artifactManifestSocketContractFixture(t, "/runner/artifact-manifest")
		boundary, err := runnercontrol.NewArtifactManifestServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewArtifactManifestServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, manifest.Fence.Machine.Machine, manifest.Fence.Machine.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewArtifactManifestClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewArtifactManifestClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), manifest)
		if gotErr != nil || response.Body.Run != manifest.Run || response.Body.Digest != wantRecord.Digest || response.Body.Bytes != wantRecord.Bytes {
			t.Fatalf("ArtifactManifestClient.Submit() = (%+v, %v), want exact record %v/%v and nil", response.Body, gotErr, wantRecord.Digest, wantRecord.Bytes)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("ArtifactManifestServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive artifact chunk crosses replay binding without changing byte identity", func(t *testing.T) {
		t.Parallel()

		_, chunk := artifactFixture(t, []byte("artifact-chunk-socket-evidence"))
		wantReceipt := runnercontrol.ArtifactChunkReceipt{
			SchemaVersion: runnercontrol.SchemaVersion,
			Run:           chunk.Run,
			Manifest:      chunk.ManifestDigest,
			Artifact:      chunk.Entry.Digest,
			Committed:     chunk.Entry.Bytes,
			Complete:      true,
		}
		if err := wantReceipt.Validate(); err != nil {
			t.Fatalf("ArtifactChunkReceipt.Validate() error = %v, want nil", err)
		}
		repository := artifactChunkRepositoryFunc(func(_ context.Context, got runnercontrol.ArtifactChunk) (runnercontrol.ArtifactChunkReceipt, error) {
			switch {
			case got.Run != chunk.Run:
				return runnercontrol.ArtifactChunkReceipt{}, errors.Join(core.ErrPrimitiveContract, errors.New("artifact run changed across the socket"))
			case got.ManifestDigest != chunk.ManifestDigest:
				return runnercontrol.ArtifactChunkReceipt{}, errors.Join(core.ErrPrimitiveContract, errors.New("artifact manifest changed across the socket"))
			case !artifactSocketEntryEqual(got.Entry, chunk.Entry):
				return runnercontrol.ArtifactChunkReceipt{}, errors.Join(core.ErrPrimitiveContract, errors.New("artifact entry changed across the socket"))
			case got.Offset != chunk.Offset:
				return runnercontrol.ArtifactChunkReceipt{}, errors.Join(core.ErrPrimitiveContract, errors.New("artifact offset changed across the socket"))
			case got.Final != chunk.Final:
				return runnercontrol.ArtifactChunkReceipt{}, errors.Join(core.ErrPrimitiveContract, errors.New("artifact final flag changed across the socket"))
			case !bytes.Equal(got.Data, chunk.Data):
				return runnercontrol.ArtifactChunkReceipt{}, errors.Join(core.ErrPrimitiveContract, errors.New("artifact bytes changed across the socket"))
			}
			return wantReceipt, nil
		})
		contract := artifactSocketContractFixture(t, "/runner/artifact-chunk")
		boundary, err := runnercontrol.NewArtifactServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewArtifactServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, chunk.Fence.Machine.Machine, chunk.Fence.Machine.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewArtifactClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewArtifactClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), chunk)
		serverErr := waitRunnerControlSocketServer(t, result)
		if gotErr != nil || response.Body != wantReceipt || serverErr != nil {
			t.Fatalf("ArtifactClient.Submit() = (%+v, %v, server %v), want %+v and nil", response.Body, gotErr, serverErr, wantReceipt)
		}
	})

	t.Run("negative missing repositories or trust construct no effect capability", func(t *testing.T) {
		t.Parallel()

		_, trusted := completionSignerFixture(t)
		zeroTrust := attest.TrustedKeys{}
		experimentContract := experimentCompletionSocketContractFixture(t, "/runner/experiment-refusal")
		runnerContract := runnerCompletionSocketContractFixture(t, "/runner/completion-refusal")
		cleanupContract := cleanupSocketContractFixture(t, "/runner/cleanup-refusal")
		expansionContract := expansionSocketContractFixture(t, "/runner/expansion-refusal")
		manifestContract := artifactManifestSocketContractFixture(t, "/runner/manifest-refusal")
		artifactContract := artifactSocketContractFixture(t, "/runner/artifact-refusal")
		_, experimentErr := runnercontrol.NewExperimentCompletionServer(experimentContract, nil, trusted)
		_, runnerErr := runnercontrol.NewRunnerCompletionServer(runnerContract, nil, trusted)
		_, cleanupErr := runnercontrol.NewCleanupServer(cleanupContract, nil, trusted)
		_, expansionErr := runnercontrol.NewExpansionServer(expansionContract, nil, trusted)
		_, manifestErr := runnercontrol.NewArtifactManifestServer(manifestContract, nil)
		_, artifactErr := runnercontrol.NewArtifactServer(artifactContract, nil)
		_, trustErr := runnercontrol.NewCleanupServer(cleanupContract, cleanupRepositoryFunc(func(context.Context, runnercontrol.CleanupRecord) error { return nil }), zeroTrust)
		if !errors.Is(experimentErr, core.ErrPrimitiveContract) || !errors.Is(runnerErr, core.ErrPrimitiveContract) ||
			!errors.Is(cleanupErr, core.ErrPrimitiveContract) || !errors.Is(expansionErr, core.ErrPrimitiveContract) ||
			!errors.Is(manifestErr, core.ErrPrimitiveContract) || !errors.Is(artifactErr, core.ErrPrimitiveContract) ||
			!errors.Is(trustErr, core.ErrPrimitiveContract) {
			t.Fatalf("invalid evidence server constructors errors = (%v, %v, %v, %v, %v, %v, %v), want typed refusals", experimentErr, runnerErr, cleanupErr, expansionErr, manifestErr, artifactErr, trustErr)
		}
	})
}

func artifactSocketEntryEqual(got, want runnercontrol.ArtifactManifestEntry) bool {
	if got.Path != want.Path || got.MediaType != want.MediaType || got.Bytes != want.Bytes ||
		got.Digest != want.Digest || got.Kind != want.Kind {
		return false
	}
	if got.Experiment == nil || want.Experiment == nil {
		return got.Experiment == nil && want.Experiment == nil
	}
	return *got.Experiment == *want.Experiment
}

type experimentCompletionRepositoryFunc func(context.Context, runnercontrol.ExperimentCompletionRecord) error

func (f experimentCompletionRepositoryFunc) StoreExperimentCompletion(ctx context.Context, record runnercontrol.ExperimentCompletionRecord) error {
	return f(ctx, record)
}

type runnerCompletionRepositoryFunc func(context.Context, runnercontrol.RunnerCompletionRecord) error

func (f runnerCompletionRepositoryFunc) StoreRunnerCompletion(ctx context.Context, record runnercontrol.RunnerCompletionRecord) error {
	return f(ctx, record)
}

type cleanupRepositoryFunc func(context.Context, runnercontrol.CleanupRecord) error

func (f cleanupRepositoryFunc) StoreCleanup(ctx context.Context, record runnercontrol.CleanupRecord) error {
	return f(ctx, record)
}

type expansionRepositoryFunc func(context.Context, runnercontrol.ExpansionRecord) (runnercontrol.ExpansionApproval, error)

func (f expansionRepositoryFunc) ApproveExpansion(ctx context.Context, record runnercontrol.ExpansionRecord) (runnercontrol.ExpansionApproval, error) {
	return f(ctx, record)
}

type artifactManifestRepositoryFunc func(context.Context, runnercontrol.ArtifactManifestRecord) error

func (f artifactManifestRepositoryFunc) StoreArtifactManifest(ctx context.Context, record runnercontrol.ArtifactManifestRecord) error {
	return f(ctx, record)
}

type artifactChunkRepositoryFunc func(context.Context, runnercontrol.ArtifactChunk) (runnercontrol.ArtifactChunkReceipt, error)

func (f artifactChunkRepositoryFunc) StoreArtifactChunk(ctx context.Context, chunk runnercontrol.ArtifactChunk) (runnercontrol.ArtifactChunkReceipt, error) {
	return f(ctx, chunk)
}

func experimentCompletionSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.ExperimentCompletionSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.ExperimentCompletionSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func runnerCompletionSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.RunnerCompletionSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.RunnerCompletionSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func cleanupSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.CleanupSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.CleanupSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func expansionSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.ExpansionSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.ExpansionSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func artifactManifestSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.ArtifactManifestSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.ArtifactManifestSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func artifactSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.ArtifactSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.ArtifactSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}
