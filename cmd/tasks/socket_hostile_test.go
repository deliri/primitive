package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/taskmanager"
	"github.com/deliri/primitive/v2026/temporal"
)

type authenticatedSocketCase struct {
	name          string
	clientSecret  byte
	collection    taskmanager.TaskCollection
	responseItems uint8
	wantErr       bool
}

func TestCommandAuthenticatedSocketLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []authenticatedSocketCase{
		{
			name:         "positive exact config and provider secret return one bound completed task",
			clientSecret: 'a', collection: taskmanager.TaskCollectionCompleted, responseItems: 1,
		},
		{
			name:         "negative wrong provider secret is refused with zero typed result",
			clientSecret: 'b', collection: taskmanager.TaskCollectionCompleted, wantErr: true,
		},
		{
			name:         "neutral valid active query returns a bound empty partition",
			clientSecret: 'a', collection: taskmanager.TaskCollectionActive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runAuthenticatedSocketCase(t, tc)
		})
	}
}

func runAuthenticatedSocketCase(t testing.TB, testCase authenticatedSocketCase) {
	t.Helper()
	identity := commandIdentityFixture(t)
	projectID := commandUUIDFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6")
	serverSecret := bytes.Repeat([]byte{'a'}, 16)
	clientSecret := bytes.Repeat([]byte{testCase.clientSecret}, 16)
	defer clear(serverSecret)
	defer clear(clientSecret)
	server := authenticatedTaskServer(t, authenticatedTaskServerRequest{
		identity: identity, secret: serverSecret, projectID: projectID,
		collection: testCase.collection, responseItems: testCase.responseItems,
	})
	defer server.Close()
	configuration := commandConfigurationFixture(t, server, identity, projectID)
	httpClient, err := exchange.NewClient(server.Client())
	if err != nil {
		t.Fatalf("exchange.NewClient(server) error = %v, want nil", err)
	}
	client, err := taskManagerClient(configuration, clientSecret, httpClient)
	if err != nil {
		t.Fatalf("taskManagerClient() error = %v, want nil", err)
	}
	job := jobDocument{
		Revision: commandDocumentRevisionV2, Operation: operationListTasks,
		ListTasks: &listTasksInput{Collection: testCase.collection, Order: taskmanager.PageOrderDescending, Limit: 13},
	}
	workingDirectory, err := process.WorkingDirectory()
	if err != nil {
		t.Fatalf("process.WorkingDirectory() error = %v, want nil", err)
	}
	result, gotErr := executeJob(executionRequest{
		ctx: context.Background(), workingDirectory: workingDirectory,
		client: client, configuration: configuration, job: job,
	})
	if testCase.wantErr {
		if !errors.Is(gotErr, core.ErrTaskManagerContract) || result.payloadCount() != 0 {
			t.Fatalf("executeJob(refused auth) = (%+v, %v), want zero and %v", result, gotErr, core.ErrTaskManagerContract)
		}
		return
	}
	if gotErr != nil {
		t.Fatalf("executeJob() error = %v, want nil", gotErr)
	}
	if result.Tasks == nil || result.Tasks.ProjectID != projectID ||
		result.Tasks.Collection != testCase.collection || len(result.Tasks.Items) != int(testCase.responseItems) {
		t.Fatalf("executeJob() result = %+v, want %d bound items", result, testCase.responseItems)
	}
	proveCommandResultCanonical(t, result)
}

type authenticatedTaskServerRequest struct {
	identity      exchange.BasicAuthorizationIdentity
	secret        []byte
	projectID     id.UUIDv7
	collection    taskmanager.TaskCollection
	responseItems uint8
}

func authenticatedTaskServer(t testing.TB, request authenticatedTaskServerRequest) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, httpRequest *http.Request) {
		username, password, okay := httpRequest.BasicAuth()
		if !okay || username != request.identity.String() || password != string(request.secret) {
			http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusUnauthorized)
			return
		}
		received, err := taskmanager.ReceiveListTasks(httpRequest)
		if err != nil || received.Body.ProjectID != request.projectID ||
			received.Body.Collection != request.collection || received.Body.Limit != 13 {
			http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusBadRequest)
			return
		}
		page := taskmanager.TaskPage{ProjectID: request.projectID, Collection: request.collection, Order: taskmanager.PageOrderDescending}
		if request.responseItems == 1 {
			page.Items = []taskmanager.TaskSummary{commandTaskSummaryFixture(t, request.projectID, request.collection)}
		}
		if err := taskmanager.WriteTaskPage(writer, page); err != nil {
			http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusInternalServerError)
		}
	}))
}

func commandConfigurationFixture(
	t testing.TB,
	server *httptest.Server,
	identity exchange.BasicAuthorizationIdentity,
	projectID id.UUIDv7,
) configurationDocument {
	t.Helper()
	authority, err := core.ParseHTTPEndpoint(server.URL)
	if err != nil {
		t.Fatalf("ParseHTTPEndpoint(server) error = %v, want nil", err)
	}
	return configurationDocument{
		Revision: commandDocumentRevisionV2, Authority: authority, Username: identity,
		PasswordSecret: googleSecretReference{Project: "example-task-project", Secret: "task-manager-admin-password"},
		ProjectID:      &projectID,
	}
}

func commandTaskSummaryFixture(
	t testing.TB,
	projectID id.UUIDv7,
	collection taskmanager.TaskCollection,
) taskmanager.TaskSummary {
	t.Helper()
	state := taskmanager.TaskStateCompleted
	if collection == taskmanager.TaskCollectionActive {
		state = taskmanager.TaskStateBacklog
	}
	title, err := taskmanager.ParseTitle("Bound task result")
	if err != nil {
		t.Fatalf("taskmanager.ParseTitle() error = %v, want nil", err)
	}
	revision, err := taskmanager.NewRevision(3)
	if err != nil {
		t.Fatalf("taskmanager.NewRevision() error = %v, want nil", err)
	}
	instant, err := temporal.NewInstant(time.Unix(1_776_000_000, 0))
	if err != nil {
		t.Fatalf("temporal.NewInstant() error = %v, want nil", err)
	}
	numeric, err := temporal.NewNumericInstant(instant)
	if err != nil {
		t.Fatalf("temporal.NewNumericInstant() error = %v, want nil", err)
	}
	return taskmanager.TaskSummary{
		ID:        commandUUIDFixture(t, "019ff548-346e-77cc-be1e-be78ab803328"),
		ProjectID: projectID,
		PhaseID:   commandUUIDFixture(t, "019ff548-346e-77cc-be1e-be78ab803327"),
		Title:     title,
		Kind:      taskmanager.TaskKindFeature,
		State:     state,
		Revision:  revision,
		UpdatedAt: numeric,
	}
}

func commandIdentityFixture(t testing.TB) exchange.BasicAuthorizationIdentity {
	t.Helper()
	identity, err := exchange.ParseBasicAuthorizationIdentity("configured-agent")
	if err != nil {
		t.Fatalf("ParseBasicAuthorizationIdentity() error = %v, want nil", err)
	}
	return identity
}

func commandUUIDFixture(t testing.TB, value string) id.UUIDv7 {
	t.Helper()
	parsed, err := id.ParseUUIDv7(value)
	if err != nil {
		t.Fatalf("ParseUUIDv7() error = %v, want nil", err)
	}
	return parsed
}

func proveCommandResultCanonical(t testing.TB, result commandResult) {
	t.Helper()
	encoded, err := result.MarshalJSON()
	if err != nil {
		t.Fatalf("command result MarshalJSON() error = %v, want nil", err)
	}
	limits, err := documentLimits(commandDocumentMaxBytes)
	if err != nil {
		t.Fatalf("documentLimits() error = %v, want nil", err)
	}
	roundTrip, err := core.DecodeStrictJSON[commandResult](bytes.NewReader(encoded), limits)
	if err != nil || roundTrip.Operation != operationListTasks {
		t.Fatalf("command result round trip = (%+v, %v), want list_tasks and nil", roundTrip, err)
	}
}
