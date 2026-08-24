package taskmanager

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestRouteDomainExhaustsEveryBackingByteAndPinsSocketFacts(t *testing.T) {
	t.Parallel()

	validRoutes := []Route{
		RouteListProjects, RouteGetProject, RouteCreateProject, RouteListPhases, RouteCreatePhase,
		RouteListTasks, RouteGetTask, RouteCreateTask, RouteUpdateTask, RouteListEvidence,
		RouteAppendEvidence, RouteListGitCommits, RouteAppendGitCommit, RouteCompleteTask,
	}
	for raw := range 256 {
		got := Route(raw)
		wantValid := false
		for _, route := range validRoutes {
			wantValid = wantValid || got == route
		}
		gotErr := got.Validate()
		if wantValid && gotErr != nil {
			t.Fatalf("Route(%d).Validate() error = %v, want nil", raw, gotErr)
		}
		if !wantValid && !errors.Is(gotErr, core.ErrTaskManagerContract) {
			t.Fatalf("Route(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrTaskManagerContract)
		}
		path, pathErr := got.Path()
		if wantValid && (pathErr != nil || path == "") {
			t.Fatalf("Route(%d).Path() = (%q, %v), want nonempty and nil", raw, path, pathErr)
		}
		if !wantValid && !errors.Is(pathErr, core.ErrTaskManagerContract) {
			t.Fatalf("Route(%d).Path() error = %v, want %v", raw, pathErr, core.ErrTaskManagerContract)
		}
	}

	listSemantics, err := RouteListProjects.Semantics()
	if err != nil || listSemantics.Method != exchange.MethodPost || listSemantics.Replay != exchange.ReplaySingleAttempt {
		t.Fatalf("RouteListProjects.Semantics() = (%+v, %v), want POST/single-attempt", listSemantics, err)
	}
	updateSemantics, err := RouteUpdateTask.Semantics()
	if err != nil || updateSemantics.Method != exchange.MethodPatch || updateSemantics.Replay != exchange.ReplayIdempotencyKey {
		t.Fatalf("RouteUpdateTask.Semantics() = (%+v, %v), want PATCH/idempotency-key", updateSemantics, err)
	}
}

func TestTaskManagerSocketLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive list request crosses real TLS socket as typed facts", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			received, err := ReceiveListProjects(request)
			if err != nil || received.Body.Lifecycle != ProjectLifecycleActive || received.Body.Limit != 17 {
				http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusBadRequest)
				return
			}
			page := ProjectPage{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending, Items: []ProjectSummary{projectSummaryFixture(t)}}
			if err := WriteProjectPage(writer, page); err != nil {
				http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusInternalServerError)
			}
		}))
		defer server.Close()

		client := socketClientFixture(t, server)
		got, err := client.ListProjects(context.Background(), ListProjectsRequest{
			Lifecycle: ProjectLifecycleActive,
			Order:     PageOrderDescending,
			Limit:     17,
		})
		if err != nil || len(got.Items) != 1 || got.Items[0].ID != projectSummaryFixture(t).ID {
			t.Fatalf("Client.ListProjects() = (%+v, %v), want one typed project and nil", got, err)
		}
	})

	t.Run("negative wrong mounted path is refused before body admission", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodPost, "https://tasks.example.invalid/v1/task-manager/projects/wrong", nil)
		got, err := ReceiveListProjects(request)
		if !errors.Is(err, core.ErrTaskManagerContract) || got != (exchange.Received[*ListProjectsRequest]{}) {
			t.Fatalf("ReceiveListProjects(wrong path) = (%+v, %v), want zero and %v", got, err, core.ErrTaskManagerContract)
		}
	})

	t.Run("neutral empty active page crosses real TLS socket without invented projects", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if _, err := ReceiveListProjects(request); err != nil {
				http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusBadRequest)
				return
			}
			if err := WriteProjectPage(writer, ProjectPage{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending}); err != nil {
				http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusInternalServerError)
			}
		}))
		defer server.Close()

		client := socketClientFixture(t, server)
		got, err := client.ListProjects(context.Background(), ListProjectsRequest{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending, Limit: 1})
		if err != nil || len(got.Items) != 0 || got.Next != nil {
			t.Fatalf("Client.ListProjects(empty) = (%+v, %v), want empty terminal page and nil", got, err)
		}
	})
}

func TestUpdateSocketBindsRealHeaderBodyIdentityAndReturnsTypedTask(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received, err := ReceiveUpdateTask(request)
		if err != nil || received.Body.Change.Title == nil {
			http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusBadRequest)
			return
		}
		response := TaskDetail{
			Summary: taskSummaryFixture(t), Description: mustDescription(t, "updated description"),
			CreatedAt: instantFixture(t, 1_775_000_000_000_000_000),
		}
		response.Summary.Title = *received.Body.Change.Title
		response.Summary.Kind = *received.Body.Change.Kind
		response.Summary.State = *received.Body.Change.State
		response.Summary.Revision = mustRevision(t, 8)
		if received.Body.Change.Description != nil {
			response.Description = *received.Body.Change.Description
		}
		if err := WriteTaskDetail(writer, response); err != nil {
			http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := socketClientFixture(t, server)
	request := updateTaskRequestFixture(t)
	got, err := client.UpdateTask(context.Background(), request)
	if err != nil || got.Summary.Title != *request.Change.Title || got.Description != *request.Change.Description {
		t.Fatalf("Client.UpdateTask() = (%+v, %v), want title %q and nil", got, err, request.Change.Title.String())
	}
	gotRevision, revisionErr := got.Summary.Revision.Uint64()
	if revisionErr != nil || gotRevision != 8 {
		t.Fatalf("Client.UpdateTask().Revision = (%d, %v), want (8, nil)", gotRevision, revisionErr)
	}
}

func socketClientFixture(t testing.TB, server *httptest.Server) Client {
	t.Helper()
	httpClient, err := exchange.NewClient(server.Client())
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	authority, err := core.ParseHTTPEndpoint(server.URL)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	client, err := NewClient(ClientConfiguration{HTTP: httpClient, Authority: authority})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	return client
}

func projectSummaryFixture(t testing.TB) ProjectSummary {
	t.Helper()
	return ProjectSummary{
		ID:              uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6"),
		Name:            mustTitle(t, "Primitive"),
		Lifecycle:       ProjectLifecycleActive,
		Revision:        mustRevision(t, 3),
		CreatedAt:       instantFixture(t, 1_776_000_000_000_000_000),
		UpdatedAt:       instantFixture(t, 1_777_000_000_000_000_000),
		PhaseCount:      2,
		OpenTaskCount:   7,
		ClosedTaskCount: 11,
	}
}
