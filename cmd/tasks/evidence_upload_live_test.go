package main

import (
	"context"
	"errors"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/gcsobjects"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/taskmanager"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	taskEvidenceLiveBucketEnvironment = "PRIMITIVE_TASKS_GCS_LIVE_BUCKET"
	taskEvidenceLivePrefixEnvironment = "PRIMITIVE_TASKS_GCS_LIVE_PREFIX"
)

func TestTaskEvidenceUploadUsesRealProviderAndTaskSocketLive(t *testing.T) {
	t.Parallel()

	bucketText, bucketSet := os.LookupEnv(taskEvidenceLiveBucketEnvironment)
	prefixText, prefixSet := os.LookupEnv(taskEvidenceLivePrefixEnvironment)
	if !bucketSet || !prefixSet {
		t.Skipf("real task evidence proof requires %s and %s", taskEvidenceLiveBucketEnvironment, taskEvidenceLivePrefixEnvironment)
	}
	bucket, err := gcsobjects.ParseGCSBucket(bucketText)
	if err != nil {
		t.Fatalf("gcsobjects.ParseGCSBucket(%q) error = %v, want nil", bucketText, err)
	}
	if _, err := gcsobjects.ParseGCSObjectPrefix(prefixText); err != nil {
		t.Fatalf("gcsobjects.ParseGCSObjectPrefix(%q) error = %v, want nil", prefixText, err)
	}

	root, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(tempdir) error = %v, want nil", err)
	}
	payload := []byte("primitive task evidence live provider proof\n")
	input := taskEvidenceInputFixture(t)
	input.Source = "live-proof.png"
	if err := os.WriteFile(filepath.Join(root.String(), input.Source), payload, 0o600); err != nil {
		t.Fatalf("os.WriteFile(live source) error = %v, want nil", err)
	}
	configuration := taskEvidenceConfigurationFixture(t, uint64(len(payload)))
	configuration.EvidenceStorage.Bucket = bucketText
	configuration.EvidenceStorage.Prefix = prefixText
	instant, err := temporal.NewInstant(time.Unix(1_786_183_200, 0))
	if err != nil {
		t.Fatalf("temporal.NewInstant() error = %v, want nil", err)
	}
	plan, err := prepareTaskEvidenceUpload(t.Context(), taskEvidencePreparationRequest{
		WorkingDirectory: root, Configuration: configuration, Input: input, Instant: instant,
	})
	if err != nil {
		t.Fatalf("prepareTaskEvidenceUpload(live) error = %v, want nil", err)
	}
	provider, err := gcsobjects.NewGCSClient(t.Context(), gcsobjects.GCSClientConfig{
		Authentication: gcsobjects.GCSAuthenticationApplicationDefault,
	})
	if err != nil {
		t.Fatalf("gcsobjects.NewGCSClient() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if closeErr := provider.Close(); closeErr != nil {
			t.Errorf("GCSClient.Close() error = %v, want nil", closeErr)
		}
	})
	metadata, err := executeTaskEvidenceUpload(t.Context(), provider, plan)
	if err != nil {
		t.Fatalf("executeTaskEvidenceUpload(create or verified replay) error = %v, want nil", err)
	}
	location, err := metadata.Address()
	if err != nil {
		t.Fatalf("uploaded metadata Address() error = %v, want nil", err)
	}
	uploaded := taskEvidenceUploadReceipt{Location: location, Digest: plan.Integrity.SHA256}

	var received atomic.Uint32
	server := liveTaskEvidenceServer(t, &received)
	defer server.Close()
	httpClient, err := exchange.NewClient(server.Client())
	if err != nil {
		t.Fatalf("exchange.NewClient(task server) error = %v, want nil", err)
	}
	authority, err := core.ParseHTTPEndpoint(server.URL)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint(task server) error = %v, want nil", err)
	}
	configuration.Authority = authority
	client, err := taskmanager.NewClient(taskmanager.ClientConfiguration{HTTP: httpClient, Authority: authority})
	if err != nil {
		t.Fatalf("taskmanager.NewClient() error = %v, want nil", err)
	}
	job := taskEvidenceAppendJobFixture(t)
	job.AppendEvidence.Source = input.Source
	evidenceID := commandUUIDFixture(t, "019ff548-346e-77cc-be1e-be78ab803329")
	mutationID := commandUUIDFixture(t, "019ff548-346e-77cc-be1e-be78ab803330")
	result, err := appendUploadedTaskEvidence(t.Context(), taskEvidenceAppendRequest{
		Client: client, Configuration: configuration, Job: job, Uploaded: uploaded,
		EvidenceID: evidenceID, MutationID: mutationID,
	})
	if err != nil {
		t.Fatalf("appendUploadedTaskEvidence(real task socket) error = %v, want nil", err)
	}
	if result.Evidence == nil || result.Evidence.Location != location || result.Evidence.Digest != plan.Integrity.SHA256 || received.Load() != 1 {
		t.Fatalf("appended live evidence = (%+v, requests=%d), want exact provider receipt and one request", result.Evidence, received.Load())
	}

	replayed, err := executeTaskEvidenceUpload(t.Context(), provider, plan)
	if err != nil || replayed.Name() != metadata.Name() || replayed.Generation() != metadata.Generation() {
		t.Fatalf("executeTaskEvidenceUpload(existing exact object) = (%+v, %v), want original immutable generation and nil", replayed, err)
	}

	provePoisonedTaskEvidenceRefusesBeforeSocket(t, poisonedTaskEvidenceProofRequest{
		Root: root, Configuration: configuration, Client: client, Provider: provider,
		Bucket: bucket, Received: &received, Instant: instant,
	})
}

func liveTaskEvidenceServer(t testing.TB, received *atomic.Uint32) *httptest.Server {
	t.Helper()
	created, err := temporal.NewInstant(time.Unix(1_786_183_201, 0))
	if err != nil {
		t.Fatalf("temporal.NewInstant(server record) error = %v, want nil", err)
	}
	createdAt, err := temporal.NewNumericInstant(created)
	if err != nil {
		t.Fatalf("temporal.NewNumericInstant(server record) error = %v, want nil", err)
	}
	taskRevision, err := taskmanager.NewRevision(3)
	if err != nil {
		t.Fatalf("taskmanager.NewRevision(server record) error = %v, want nil", err)
	}
	return httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		incoming, receiveErr := taskmanager.ReceiveAppendEvidence(request)
		if receiveErr != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		received.Add(1)
		body := incoming.Body
		record := taskmanager.EvidenceRecord{
			ID: body.ID, ProjectID: body.ProjectID, TaskID: body.TaskID, Kind: body.Kind,
			Summary: body.Summary, Location: body.Location, Digest: body.Digest,
			TaskRevision: taskRevision, CreatedAt: createdAt,
		}
		if writeErr := taskmanager.WriteEvidenceRecord(writer, record); writeErr != nil {
			writer.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

type poisonedTaskEvidenceProofRequest struct {
	Root          core.AbsolutePath
	Configuration configurationDocument
	Client        taskmanager.Client
	Provider      *gcsobjects.GCSClient
	Bucket        gcsobjects.GCSBucket
	Received      *atomic.Uint32
	Instant       temporal.Instant
}

func provePoisonedTaskEvidenceRefusesBeforeSocket(t testing.TB, request poisonedTaskEvidenceProofRequest) {
	t.Helper()
	expected := []byte("expected second task evidence\n")
	input := taskEvidenceInputFixture(t)
	input.Source = "poisoned-proof.png"
	if err := os.WriteFile(filepath.Join(request.Root.String(), input.Source), expected, 0o600); err != nil {
		t.Fatalf("os.WriteFile(poison target) error = %v, want nil", err)
	}
	request.Configuration.EvidenceStorage.MaximumBytes = taskEvidenceByteCount(t, uint64(len(expected)))
	plan, err := prepareTaskEvidenceUpload(context.Background(), taskEvidencePreparationRequest{
		WorkingDirectory: request.Root, Configuration: request.Configuration,
		Input: input, Instant: request.Instant,
	})
	if err != nil {
		t.Fatalf("prepareTaskEvidenceUpload(poison target) error = %v, want nil", err)
	}
	poison := []byte("different provider bytes\n")
	poisonLength, err := core.NewByteLength(uint64(len(poison)))
	if err != nil {
		t.Fatalf("core.NewByteLength(poison) error = %v, want nil", err)
	}
	poisonRequest := gcsobjects.GCSMediaUpload{
		Source: strings.NewReader(string(poison)), Bucket: request.Bucket, Name: plan.Name,
		ContentType: input.ContentType, CustomTime: request.Instant,
		Integrity: objectstore.Integrity{
			SHA256: core.SHA256Of(poison), Length: poisonLength,
			CRC32C: core.NewCRC32C(crc32.Checksum(poison, crc32.MakeTable(crc32.Castagnoli))),
		},
	}
	if _, uploadErr := gcsobjects.UploadMedia(context.Background(), request.Provider, poisonRequest); uploadErr != nil &&
		!errors.Is(uploadErr, core.ErrObjectStoreConflict) {
		t.Fatalf("gcsobjects.UploadMedia(poison precondition) error = %v, want nil or immutable conflict", uploadErr)
	}
	beforeRequests := request.Received.Load()
	job := taskEvidenceAppendJobFixture(t)
	job.AppendEvidence.Source = input.Source
	got, gotErr := executeAppendEvidence(executionRequest{
		ctx: context.Background(), workingDirectory: request.Root,
		client: request.Client, configuration: request.Configuration, job: job,
	})
	if !errors.Is(gotErr, core.ErrObjectStoreIntegrity) || got != (commandResult{}) {
		t.Fatalf("executeAppendEvidence(poisoned immutable object) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrObjectStoreIntegrity)
	}
	if gotRequests := request.Received.Load(); gotRequests != beforeRequests {
		t.Fatalf("task socket requests after poisoned object = %d, want unchanged %d", gotRequests, beforeRequests)
	}
}

func taskEvidenceByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return count
}
