package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/taskmanager"
	"github.com/deliri/primitive/v2026/temporal"
)

func BenchmarkDecodeCreateTaskJob(b *testing.B) {
	job := jobDocument{
		Revision:  commandDocumentRevisionV2,
		Operation: operationCreateTask,
		CreateTask: &createTaskInput{
			PhaseID:     benchmarkUUID(b),
			Title:       "Implement the bounded operation",
			Description: "Build and prove the compiler-owned task contract.",
			Kind:        taskmanager.TaskKindFeature,
			State:       taskmanager.TaskStateBacklog,
		},
	}
	encoded, err := job.MarshalJSON()
	if err != nil {
		b.Fatalf("job MarshalJSON() error = %v, want nil", err)
	}
	limits, err := documentLimits(commandDocumentMaxBytes)
	if err != nil {
		b.Fatalf("documentLimits() error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := core.DecodeStrictJSON[jobDocument](bytes.NewReader(encoded), limits); err != nil {
			b.Fatalf("DecodeStrictJSON(job) error = %v, want nil", err)
		}
	}
}

func BenchmarkPrepareTaskEvidenceUpload(b *testing.B) {
	root, err := core.ParseAbsolutePath(b.TempDir())
	if err != nil {
		b.Fatalf("core.ParseAbsolutePath(tempdir) error = %v, want nil", err)
	}
	payload := bytes.Repeat([]byte("evidence"), 1024)
	input := taskEvidenceInputFixture(b)
	input.Source = "proof.bin"
	if err := os.WriteFile(filepath.Join(root.String(), input.Source), payload, 0o600); err != nil {
		b.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	configuration := taskEvidenceConfigurationFixture(b, uint64(len(payload)))
	instant, err := temporal.NewInstant(time.Unix(1_786_183_200, 0))
	if err != nil {
		b.Fatalf("temporal.NewInstant() error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := prepareTaskEvidenceUpload(b.Context(), taskEvidencePreparationRequest{
			WorkingDirectory: root, Configuration: configuration, Input: input, Instant: instant,
		}); err != nil {
			b.Fatalf("prepareTaskEvidenceUpload() error = %v, want nil", err)
		}
	}
}

func benchmarkUUID(b *testing.B) id.UUIDv7 {
	b.Helper()
	value, err := id.ParseUUIDv7("019ff548-29cb-7451-869e-aa644c0947e6")
	if err != nil {
		b.Fatalf("id.ParseUUIDv7() error = %v, want nil", err)
	}
	return value
}
