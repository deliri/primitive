package taskmanager

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

func TestTaskCollectionPinsEveryExactStateMembership(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		collection TaskCollection
		state      TaskState
		want       bool
	}{
		{name: "active admits backlog", collection: TaskCollectionActive, state: TaskStateBacklog, want: true},
		{name: "active admits in progress", collection: TaskCollectionActive, state: TaskStateInProgress, want: true},
		{name: "active admits blocked", collection: TaskCollectionActive, state: TaskStateBlocked, want: true},
		{name: "active rejects completed", collection: TaskCollectionActive, state: TaskStateCompleted},
		{name: "active rejects cancelled", collection: TaskCollectionActive, state: TaskStateCancelled},
		{name: "completed rejects backlog", collection: TaskCollectionCompleted, state: TaskStateBacklog},
		{name: "completed rejects in progress", collection: TaskCollectionCompleted, state: TaskStateInProgress},
		{name: "completed rejects blocked", collection: TaskCollectionCompleted, state: TaskStateBlocked},
		{name: "completed admits completed", collection: TaskCollectionCompleted, state: TaskStateCompleted, want: true},
		{name: "completed admits cancelled", collection: TaskCollectionCompleted, state: TaskStateCancelled, want: true},
		{name: "unknown collection rejects exact state", collection: TaskCollectionUnknown, state: TaskStateBacklog},
		{name: "active rejects unknown state", collection: TaskCollectionActive, state: TaskStateUnknown},
		{name: "future collection rejects exact state", collection: taskCollectionLimit, state: TaskStateBacklog},
		{name: "active rejects future state", collection: TaskCollectionActive, state: taskStateLimit},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.collection.Contains(tc.state)
			if got != tc.want {
				t.Fatalf("TaskCollection.Contains() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestOperationRequestsRejectEveryMissingIdentityAndDivergentProof(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr  error
		validate func() error
		name     string
	}{
		{name: "project creation admits complete direct identity", validate: func() error { return createProjectRequestFixture(t).Validate() }},
		{name: "phase creation admits complete direct identities", validate: func() error { return createPhaseRequestFixture(t).Validate() }},
		{name: "task creation admits complete direct identities", validate: func() error { return createTaskRequestFixture(t).Validate() }},
		{name: "active task listing admits project-local page", validate: func() error { return listTasksRequestFixture(t).Validate() }},
		{name: "completion admits exact task revision", validate: func() error { return completeTaskRequestFixture(t).Validate() }},
		{name: "evidence append admits immutable digest reference", validate: func() error { return appendEvidenceRequestFixture(t).Validate() }},
		{name: "git append admits distinct parent and result", validate: func() error { return appendGitCommitRequestFixture(t).Validate() }},
		{name: "project creation rejects unset project identity", validate: func() error { value := createProjectRequestFixture(t); value.ID = id.UUIDv7{}; return value.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "project creation rejects unset mutation identity", validate: func() error {
			value := createProjectRequestFixture(t)
			value.MutationID = id.UUIDv7{}
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "phase creation rejects unset project identity", validate: func() error {
			value := createPhaseRequestFixture(t)
			value.ProjectID = id.UUIDv7{}
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "phase creation rejects unset phase identity", validate: func() error { value := createPhaseRequestFixture(t); value.ID = id.UUIDv7{}; return value.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "task creation rejects unset phase identity", validate: func() error {
			value := createTaskRequestFixture(t)
			value.PhaseID = id.UUIDv7{}
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "task creation rejects unknown kind", validate: func() error {
			value := createTaskRequestFixture(t)
			value.Kind = TaskKindUnknown
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "task listing rejects unset project identity", validate: func() error {
			value := listTasksRequestFixture(t)
			value.ProjectID = id.UUIDv7{}
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "task listing rejects unknown collection", validate: func() error {
			value := listTasksRequestFixture(t)
			value.Collection = TaskCollectionUnknown
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "task listing rejects foreign cursor", validate: func() error {
			value := listTasksRequestFixture(t)
			value.After = taskCursorFixture(t)
			value.After.ProjectID = uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e7")
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "completion rejects zero expected revision", validate: func() error {
			value := completeTaskRequestFixture(t)
			value.ExpectedRevision = Revision{}
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "evidence append rejects unset digest", validate: func() error {
			value := appendEvidenceRequestFixture(t)
			value.Digest = core.SHA256Digest{}
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "evidence append rejects unknown kind", validate: func() error {
			value := appendEvidenceRequestFixture(t)
			value.Kind = EvidenceKindUnknown
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "evidence append rejects overlong summary", validate: func() error {
			value := appendEvidenceRequestFixture(t)
			value.Summary = EvidenceSummary(strings.Repeat("e", EvidenceSummaryMaximumRunes+1))
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "git append rejects identical parent and result", validate: func() error {
			value := appendGitCommitRequestFixture(t)
			value.Result = value.Parent
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "git append rejects unset repository", validate: func() error {
			value := appendGitCommitRequestFixture(t)
			value.Repository = ""
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "git append rejects zero task revision", validate: func() error {
			value := appendGitCommitRequestFixture(t)
			value.ExpectedRevision = Revision{}
			return value.Validate()
		}, wantErr: core.ErrTaskManagerContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("operation Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func createProjectRequestFixture(t testing.TB) CreateProjectRequest {
	t.Helper()
	return CreateProjectRequest{
		ID:         uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6"),
		MutationID: uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803329"),
		Name:       mustTitle(t, "Primitive"), Description: mustDescription(t, "Typed product-neutral effects."),
		Lifecycle: ProjectLifecycleActive,
	}
}

func createPhaseRequestFixture(t testing.TB) CreatePhaseRequest {
	t.Helper()
	return CreatePhaseRequest{
		ID:         uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803327"),
		ProjectID:  createProjectRequestFixture(t).ID,
		MutationID: uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803329"),
		Name:       mustTitle(t, "Task manager"), Description: mustDescription(t, "Ship typed sockets."), Position: 4,
	}
}

func createTaskRequestFixture(t testing.TB) CreateTaskRequest {
	t.Helper()
	return CreateTaskRequest{
		ID:        uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803328"),
		ProjectID: createProjectRequestFixture(t).ID, PhaseID: createPhaseRequestFixture(t).ID,
		MutationID: uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803329"),
		Title:      mustTitle(t, "Add task sockets"), Description: mustDescription(t, "Keep the package blind."), Kind: TaskKindFeature,
		State: TaskStateBacklog,
	}
}

func listTasksRequestFixture(t testing.TB) ListTasksRequest {
	t.Helper()
	return ListTasksRequest{ProjectID: createProjectRequestFixture(t).ID, Collection: TaskCollectionActive, Order: PageOrderDescending, Limit: 25}
}

func completeTaskRequestFixture(t testing.TB) CompleteTaskRequest {
	t.Helper()
	return CompleteTaskRequest{
		ProjectID: createProjectRequestFixture(t).ID, TaskID: createTaskRequestFixture(t).ID,
		MutationID: uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803329"), ExpectedRevision: mustRevision(t, 9),
	}
}

func appendEvidenceRequestFixture(t testing.TB) AppendEvidenceRequest {
	t.Helper()
	location, err := core.ParseHTTPEndpoint("https://evidence.example.invalid/tasks/proof.json")
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	var digestBytes [core.SHA256DigestBytes]byte
	for index := range digestBytes {
		digestBytes[index] = byte(index + 1)
	}
	digest := core.NewSHA256Digest(digestBytes)
	return AppendEvidenceRequest{
		ID:        uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803330"),
		ProjectID: createProjectRequestFixture(t).ID, TaskID: createTaskRequestFixture(t).ID,
		MutationID: uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803329"),
		Kind:       EvidenceKindTest, Summary: mustEvidenceSummary(t, "Focused hostile tests pass."),
		Location: location, Digest: digest, ExpectedRevision: mustRevision(t, 9),
	}
}

func appendGitCommitRequestFixture(t testing.TB) AppendGitCommitRequest {
	t.Helper()
	parent, err := core.ParseBuildCommit(strings.Repeat("a", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit(parent) error = %v, want nil", err)
	}
	result, err := core.ParseBuildCommit(strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit(result) error = %v, want nil", err)
	}
	return AppendGitCommitRequest{
		ID:        uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803331"),
		ProjectID: createProjectRequestFixture(t).ID, TaskID: createTaskRequestFixture(t).ID,
		MutationID: uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803329"),
		Repository: mustRepositoryIdentity(t, "github.com/deliri/primitive"), Parent: parent, Result: result,
		Summary: mustCommitSummary(t, "Add blind task-manager sockets"), ExpectedRevision: mustRevision(t, 10),
	}
}

func mustDescription(t testing.TB, value string) Description {
	t.Helper()
	got, err := ParseDescription(value)
	if err != nil {
		t.Fatalf("ParseDescription(%q) error = %v, want nil", value, err)
	}
	return got
}

func mustEvidenceSummary(t testing.TB, value string) EvidenceSummary {
	t.Helper()
	got, err := ParseEvidenceSummary(value)
	if err != nil {
		t.Fatalf("ParseEvidenceSummary(%q) error = %v, want nil", value, err)
	}
	return got
}

func mustRepositoryIdentity(t testing.TB, value string) RepositoryIdentity {
	t.Helper()
	got, err := ParseRepositoryIdentity(value)
	if err != nil {
		t.Fatalf("ParseRepositoryIdentity(%q) error = %v, want nil", value, err)
	}
	return got
}

func mustCommitSummary(t testing.TB, value string) CommitSummary {
	t.Helper()
	got, err := ParseCommitSummary(value)
	if err != nil {
		t.Fatalf("ParseCommitSummary(%q) error = %v, want nil", value, err)
	}
	return got
}
