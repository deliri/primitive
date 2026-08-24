package main

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/taskmanager"
)

func BenchmarkDecodeCreateTaskJob(b *testing.B) {
	job := jobDocument{
		Revision:  commandDocumentRevisionV1,
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

func benchmarkUUID(b *testing.B) id.UUIDv7 {
	b.Helper()
	value, err := id.ParseUUIDv7("019ff548-29cb-7451-869e-aa644c0947e6")
	if err != nil {
		b.Fatalf("id.ParseUUIDv7() error = %v, want nil", err)
	}
	return value
}
