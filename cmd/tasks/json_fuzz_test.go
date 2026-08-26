package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/taskmanager"
)

func FuzzTaskCommandJobJSONSemanticClosure(f *testing.F) {
	for _, seed := range taskCommandJobFuzzSeeds(f) {
		canonical, err := seed.MarshalJSON()
		if err != nil {
			f.Fatalf("job seed %s MarshalJSON() error = %v, want nil", seed.Operation, err)
		}
		f.Add(canonical)
	}
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte(`{"revision":1,"operation":"unknown"}`))
	limits, err := documentLimits(commandDocumentMaxBytes)
	if err != nil {
		f.Fatalf("documentLimits() error = %v, want nil", err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, gotErr := core.DecodeStrictJSON[jobDocument](bytes.NewReader(data), limits)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) && !errors.Is(gotErr, core.ErrTaskManagerContract) {
				t.Fatalf("DecodeStrictJSON(rejected job) error = %v, want typed JSON or task-manager identity", gotErr)
			}
			if got.Operation != operationUnknown || got.payloadCount() != 0 {
				t.Fatalf("DecodeStrictJSON(rejected job) = %+v, want zero", got)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted job Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > commandDocumentMaxBytes {
			t.Fatalf("accepted job MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		roundTrip, err := core.DecodeStrictJSON[jobDocument](bytes.NewReader(encoded), limits)
		if err != nil {
			t.Fatalf("canonical job DecodeStrictJSON() error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second canonical job = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func taskCommandJobFuzzSeeds(t testing.TB) []jobDocument {
	t.Helper()
	projectID := commandUUIDFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6")
	phaseID := commandUUIDFixture(t, "019ff548-346e-77cc-be1e-be78ab803327")
	taskID := commandUUIDFixture(t, "019ff548-346e-77cc-be1e-be78ab803328")
	revision, err := taskmanager.NewRevision(7)
	if err != nil {
		t.Fatalf("taskmanager.NewRevision(seed) error = %v, want nil", err)
	}
	contentType, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType(seed) error = %v, want nil", err)
	}
	parent, err := core.ParseBuildCommit(strings.Repeat("a", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit(parent seed) error = %v, want nil", err)
	}
	result, err := core.ParseBuildCommit(strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit(result seed) error = %v, want nil", err)
	}
	state := taskmanager.TaskStateInProgress
	return []jobDocument{
		{Revision: commandDocumentRevisionV2, Operation: operationListProjects, ListProjects: &listProjectsInput{Lifecycle: taskmanager.ProjectLifecycleActive, Order: taskmanager.PageOrderDescending, Limit: 7}},
		{Revision: commandDocumentRevisionV2, Operation: operationGetProject},
		{Revision: commandDocumentRevisionV2, Operation: operationCreateProject, CreateProject: &createProjectInput{Name: "Project", Description: "Bounded working memory.", Lifecycle: taskmanager.ProjectLifecycleActive}},
		{Revision: commandDocumentRevisionV2, Operation: operationListPhases, ListPhases: &listPhasesInput{Order: taskmanager.PageOrderAscending, Limit: 7}},
		{Revision: commandDocumentRevisionV2, Operation: operationCreatePhase, CreatePhase: &createPhaseInput{Name: "Phase", Description: "Bounded phase.", Position: 1}},
		{Revision: commandDocumentRevisionV2, Operation: operationListTasks, ListTasks: &listTasksInput{PhaseID: &phaseID, Collection: taskmanager.TaskCollectionActive, Order: taskmanager.PageOrderDescending, Limit: 7}},
		{Revision: commandDocumentRevisionV2, Operation: operationGetTask, GetTask: &getTaskInput{TaskID: taskID}},
		{Revision: commandDocumentRevisionV2, Operation: operationCreateTask, CreateTask: &createTaskInput{PhaseID: phaseID, Title: "Task", Description: "Bounded task.", Kind: taskmanager.TaskKindFeature, State: taskmanager.TaskStateBacklog}},
		{Revision: commandDocumentRevisionV2, Operation: operationUpdateTask, UpdateTask: &updateTaskInput{TaskID: taskID, ExpectedRevision: revision, Change: taskChangeInput{State: &state}}},
		{Revision: commandDocumentRevisionV2, Operation: operationCompleteTask, CompleteTask: &completeTaskInput{TaskID: taskID, ExpectedRevision: revision}},
		{Revision: commandDocumentRevisionV2, Operation: operationListEvidence, ListEvidence: &listEvidenceInput{TaskID: taskID, Order: taskmanager.PageOrderDescending, Limit: 7}},
		{Revision: commandDocumentRevisionV2, Operation: operationAppendEvidence, AppendEvidence: &appendEvidenceInput{TaskID: taskID, Kind: taskmanager.EvidenceKindTest, Summary: "Hostile tests pass.", Source: "taskops/evidence/proof.json", ContentType: contentType, ExpectedRevision: revision}},
		{Revision: commandDocumentRevisionV2, Operation: operationListGitCommits, ListGitCommits: &listGitCommitsInput{TaskID: taskID, Order: taskmanager.PageOrderDescending, Limit: 7}},
		{Revision: commandDocumentRevisionV2, Operation: operationAppendGitCommit, AppendGitCommit: &appendGitCommitInput{TaskID: taskID, Repository: "github.com/example/project", Parent: parent, Result: result, Summary: "Bounded change.", ExpectedRevision: revision}},
		{Revision: commandDocumentRevisionV2, Operation: operationListPhases, ListPhases: &listPhasesInput{After: &taskmanager.PhaseCursor{ProjectID: projectID, Position: 1, ID: phaseID}, Order: taskmanager.PageOrderAscending, Limit: 7}},
	}
}

func FuzzTaskConfigurationJSONSemanticClosure(f *testing.F) {
	authority, err := core.ParseHTTPEndpoint("https://admin.example.com")
	if err != nil {
		f.Fatalf("ParseHTTPEndpoint(seed) error = %v, want nil", err)
	}
	projectID, err := id.ParseUUIDv7("019ff548-29cb-7451-869e-aa644c0947e6")
	if err != nil {
		f.Fatalf("ParseUUIDv7(seed) error = %v, want nil", err)
	}
	identity, err := exchange.ParseBasicAuthorizationIdentity("agent-operator")
	if err != nil {
		f.Fatalf("ParseBasicAuthorizationIdentity(seed) error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(64 << 20)
	if err != nil {
		f.Fatalf("core.NewByteCount(seed) error = %v, want nil", err)
	}
	seed := configurationDocument{
		Revision: commandDocumentRevisionV2, Authority: authority, Username: identity,
		PasswordSecret: googleSecretReference{Project: "example-task-project", Secret: "task-manager-admin-password"},
		ProjectID:      &projectID,
		EvidenceStorage: &evidenceStorageReference{
			Bucket: "example-task-proof", Prefix: "evidence/", MaximumBytes: maximum,
		},
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("configuration seed MarshalJSON() error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte(`{"revision":1}`))
	limits, err := documentLimits(configurationMaxBytes)
	if err != nil {
		f.Fatalf("documentLimits() error = %v, want nil", err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, gotErr := core.DecodeStrictJSON[configurationDocument](bytes.NewReader(data), limits)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) && !errors.Is(gotErr, core.ErrTaskManagerContract) {
				t.Fatalf("DecodeStrictJSON(rejected configuration) error = %v, want typed JSON or task-manager identity", gotErr)
			}
			if got.Authority.String() != "" || got.Username.String() != "" || got.ProjectID != nil || got.EvidenceStorage != nil {
				t.Fatalf("DecodeStrictJSON(rejected configuration) = %+v, want zero", got)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted configuration Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > configurationMaxBytes {
			t.Fatalf("accepted configuration MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		roundTrip, err := core.DecodeStrictJSON[configurationDocument](bytes.NewReader(encoded), limits)
		if err != nil {
			t.Fatalf("canonical configuration DecodeStrictJSON() error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second canonical configuration = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}
