package taskmanager

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type receiveFuzzObservation struct {
	IdempotencyKey string
	Body           []byte
	BodyPresent    bool
}

func FuzzEveryTaskManagerHTTPReceiveIngressSemanticClosure(f *testing.F) {
	addTaskManagerReceiveSeeds(f)

	f.Fuzz(func(t *testing.T, selector uint8, document []byte, idempotencyKey string) {
		route := Route(selector%uint8(routeLimit-1)) + RouteListProjects
		got, gotErr := exerciseReceiveFuzzIngress(route, document, idempotencyKey)
		if gotErr != nil {
			want := receiveFuzzObservation{}
			if !errors.Is(gotErr, core.ErrTaskManagerContract) || got.BodyPresent || got.IdempotencyKey != want.IdempotencyKey || len(got.Body) != len(want.Body) {
				t.Fatalf("task-manager Receive(rejected) = (%+v, %v), want (%+v, %v)", got, gotErr, want, core.ErrTaskManagerContract)
			}
			return
		}

		if !got.BodyPresent {
			t.Fatalf("task-manager Receive(accepted) body present = %t, want true", got.BodyPresent)
		}
		second, secondErr := exerciseReceiveFuzzIngress(route, got.Body, got.IdempotencyKey)
		if secondErr != nil || !second.BodyPresent || second.IdempotencyKey != got.IdempotencyKey || !bytes.Equal(second.Body, got.Body) {
			t.Fatalf("task-manager Receive(canonical replay) = (%+v, %v), want (%+v, nil)", second, secondErr, got)
		}
	})
}

func addTaskManagerReceiveSeeds(f *testing.F) {
	project := createProjectRequestFixture(f)
	phase := createPhaseRequestFixture(f)
	task := createTaskRequestFixture(f)
	evidence := appendEvidenceRequestFixture(f)
	commit := appendGitCommitRequestFixture(f)

	addSingleReceiveSeed(f, RouteListProjects, ListProjectsRequest{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending, Limit: 7})
	addSingleReceiveSeed(f, RouteGetProject, GetProjectRequest{ProjectID: project.ID})
	addMutationReceiveSeed(f, RouteCreateProject, project)
	addSingleReceiveSeed(f, RouteListPhases, ListPhasesRequest{ProjectID: phase.ProjectID, Order: PageOrderAscending, Limit: 7})
	addMutationReceiveSeed(f, RouteCreatePhase, phase)
	addSingleReceiveSeed(f, RouteListTasks, ListTasksRequest{ProjectID: task.ProjectID, Collection: TaskCollectionActive, Order: PageOrderDescending, Limit: 7})
	addSingleReceiveSeed(f, RouteGetTask, GetTaskRequest{ProjectID: task.ProjectID, TaskID: task.ID})
	addMutationReceiveSeed(f, RouteCreateTask, task)
	addMutationReceiveSeed(f, RouteUpdateTask, updateTaskRequestFixture(f))
	addSingleReceiveSeed(f, RouteListEvidence, ListEvidenceRequest{ProjectID: evidence.ProjectID, TaskID: evidence.TaskID, Order: PageOrderDescending, Limit: 7})
	addMutationReceiveSeed(f, RouteAppendEvidence, evidence)
	addSingleReceiveSeed(f, RouteListGitCommits, ListGitCommitsRequest{ProjectID: commit.ProjectID, TaskID: commit.TaskID, Order: PageOrderDescending, Limit: 7})
	addMutationReceiveSeed(f, RouteAppendGitCommit, commit)
	addMutationReceiveSeed(f, RouteCompleteTask, completeTaskRequestFixture(f))
}

func addSingleReceiveSeed(f *testing.F, route Route, body core.ValidatedJSONMarshaler) {
	f.Helper()
	document, err := body.MarshalJSON()
	if err != nil {
		f.Fatalf("task-manager single receive seed MarshalJSON() error = %v, want nil", err)
	}
	f.Add(uint8(route-RouteListProjects), document, "")
}

func addMutationReceiveSeed[
	Body interface {
		core.ValidatedJSONMarshaler
		exchange.IdempotencyBound
	},
](f *testing.F, route Route, body Body) {
	f.Helper()
	document, err := body.MarshalJSON()
	if err != nil {
		f.Fatalf("task-manager mutation receive seed MarshalJSON() error = %v, want nil", err)
	}
	key, err := body.IdempotencyKey()
	if err != nil {
		f.Fatalf("task-manager mutation receive seed IdempotencyKey() error = %v, want nil", err)
	}
	f.Add(uint8(route-RouteListProjects), document, key.String())
}

func exerciseReceiveFuzzIngress(route Route, document []byte, idempotencyKey string) (receiveFuzzObservation, error) {
	request, err := receiveFuzzHTTPRequest(route, document, idempotencyKey)
	if err != nil {
		return receiveFuzzObservation{}, err
	}
	switch route {
	case RouteListProjects:
		return projectReceived(ReceiveListProjects(request))
	case RouteGetProject:
		return projectReceived(ReceiveGetProject(request))
	case RouteCreateProject:
		return projectReceived(ReceiveCreateProject(request))
	case RouteListPhases:
		return projectReceived(ReceiveListPhases(request))
	case RouteCreatePhase:
		return projectReceived(ReceiveCreatePhase(request))
	case RouteListTasks:
		return projectReceived(ReceiveListTasks(request))
	case RouteGetTask:
		return projectReceived(ReceiveGetTask(request))
	case RouteCreateTask:
		return projectReceived(ReceiveCreateTask(request))
	case RouteUpdateTask:
		return projectReceived(ReceiveUpdateTask(request))
	case RouteListEvidence:
		return projectReceived(ReceiveListEvidence(request))
	case RouteAppendEvidence:
		return projectReceived(ReceiveAppendEvidence(request))
	case RouteListGitCommits:
		return projectReceived(ReceiveListGitCommits(request))
	case RouteAppendGitCommit:
		return projectReceived(ReceiveAppendGitCommit(request))
	case RouteCompleteTask:
		return projectReceived(ReceiveCompleteTask(request))
	default:
		return receiveFuzzObservation{}, contractError()
	}
}

func receiveFuzzHTTPRequest(route Route, document []byte, idempotencyKey string) (*http.Request, error) {
	path, err := route.Path()
	if err != nil {
		return nil, err
	}
	semantics, err := route.Semantics()
	if err != nil {
		return nil, err
	}
	mediaType, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return nil, err
	}
	request := httptest.NewRequest(semantics.Method.String(), path, bytes.NewReader(document))
	request.Header.Set(core.HTTPHeaderContentType().String(), mediaType.String())
	if idempotencyKey != "" {
		request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), idempotencyKey)
	}
	return request, nil
}

func projectReceived[
	Body any,
	BodyPtr interface {
		*Body
		core.ValidatedJSONMarshaler
		comparable
	},
](received exchange.Received[BodyPtr], receivedErr error) (receiveFuzzObservation, error) {
	var zero BodyPtr
	if receivedErr != nil || received.Body == zero {
		return receiveFuzzObservation{
			BodyPresent:    received.Body != zero,
			IdempotencyKey: received.IdempotencyKey.String(),
		}, receivedErr
	}
	document, err := received.Body.MarshalJSON()
	if err != nil {
		return receiveFuzzObservation{}, err
	}
	return receiveFuzzObservation{Body: document, BodyPresent: true, IdempotencyKey: received.IdempotencyKey.String()}, nil
}
