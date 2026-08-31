package about

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const aboutTestRoute = "/v1/about"

type fixtureAboutRepository struct {
	catalog      Catalog
	projectCalls atomic.Uint64
	packageCalls atomic.Uint64
}

func (r *fixtureAboutRepository) Project(context.Context, SubjectIdentity, core.BuildCommit) (Project, error) {
	r.projectCalls.Add(1)
	return r.catalog.Project, nil
}

func (r *fixtureAboutRepository) Package(context.Context, SubjectIdentity, core.BuildCommit, SourcePath) (PackageSnapshot, error) {
	r.packageCalls.Add(1)
	return r.catalog.Packages[0], nil
}

func TestAboutServiceAndSocketLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive real paired socket returns the exact project projection and execution metadata", func(t *testing.T) {
		t.Parallel()
		repository := &fixtureAboutRepository{catalog: fixtureCatalog(t)}
		server, path := fixtureAboutHTTPServer(t, repository)
		defer server.Close()
		client := fixtureAboutClient(t, server.Client(), server.URL, path)
		query := fixtureProjectQuery(repository.catalog)

		got, gotErr := client.Fetch(context.Background(), query)
		if gotErr != nil {
			t.Fatalf("Client.Fetch() error = %v, want nil", gotErr)
		}
		if got.Response.Project == nil || got.Response.Project.Revision != repository.catalog.Project.Revision {
			t.Fatalf("Client.Fetch().Response.Project = %+v, want revision %s", got.Response.Project, repository.catalog.Project.Revision)
		}
		if got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() == 0 || repository.projectCalls.Load() != 1 {
			t.Fatalf("Client.Fetch() execution = (attempts %d, bytes %d, repository calls %d), want (1, nonzero, 1)", got.Metadata.Attempts, got.Metadata.Bytes.Uint64(), repository.projectCalls.Load())
		}
	})

	t.Run("negative repository projection for another revision cannot cross the socket", func(t *testing.T) {
		t.Parallel()
		repository := &fixtureAboutRepository{catalog: fixtureCatalog(t)}
		repository.catalog.Project.Revision = fixtureCommit(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
		server, path := fixtureAboutHTTPServer(t, repository)
		defer server.Close()
		client := fixtureAboutClient(t, server.Client(), server.URL, path)
		query := fixtureProjectQuery(fixtureCatalog(t))

		got, gotErr := client.Fetch(context.Background(), query)
		gotIsZero := got.Response.Project == nil && got.Response.Package == nil && got.Metadata.Attempts == 0
		if !errors.Is(gotErr, core.ErrAboutTransport) || !gotIsZero {
			t.Fatalf("Client.Fetch(conflicting projection) = (%+v, %v), want (zero, errors.Is(..., %v))", got, gotErr, core.ErrAboutTransport)
		}
		if repository.projectCalls.Load() != 1 {
			t.Fatalf("repository project calls = %d, want 1", repository.projectCalls.Load())
		}
	})

	t.Run("neutral cancelled query preserves cancellation and performs no repository effect", func(t *testing.T) {
		t.Parallel()
		repository := &fixtureAboutRepository{catalog: fixtureCatalog(t)}
		service, setupErr := NewService(repository)
		if setupErr != nil {
			t.Fatalf("NewService() setup error = %v, want nil", setupErr)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got, gotErr := service.Resolve(ctx, fixtureProjectQuery(repository.catalog))
		if !errors.Is(gotErr, context.Canceled) || got != (Response{}) {
			t.Fatalf("Service.Resolve(cancelled) = (%+v, %v), want (zero, errors.Is(..., context.Canceled))", got, gotErr)
		}
		if repository.projectCalls.Load() != 0 || repository.packageCalls.Load() != 0 {
			t.Fatalf("repository calls after cancellation = (%d project, %d package), want (0, 0)", repository.projectCalls.Load(), repository.packageCalls.Load())
		}
	})
}

func fixtureProjectQuery(catalog Catalog) Query {
	return Query{
		SchemaVersion: SchemaVersion,
		Subject:       catalog.Project.Subject,
		Revision:      catalog.Project.Revision,
		Kind:          QueryProject,
	}
}

func fixtureAboutHTTPServer(t testing.TB, repository Repository) (*httptest.Server, exchange.SocketRoutePath) {
	t.Helper()
	path, pathErr := exchange.ParseSocketRoutePath(aboutTestRoute)
	if pathErr != nil {
		t.Fatalf("exchange.ParseSocketRoutePath(%q) setup error = %v, want nil", aboutTestRoute, pathErr)
	}
	server, serverErr := NewServer(path, repository)
	if serverErr != nil {
		t.Fatalf("NewServer() setup error = %v, want nil", serverErr)
	}
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if gotErr := server.Serve(writer, request); gotErr != nil {
			http.Error(writer, "about request refused", http.StatusInternalServerError)
		}
	}))
	return httpServer, path
}

func fixtureAboutClient(t testing.TB, httpClient *http.Client, baseURL string, path exchange.SocketRoutePath) Client {
	t.Helper()
	client, clientErr := exchange.NewClient(httpClient)
	if clientErr != nil {
		t.Fatalf("exchange.NewClient() setup error = %v, want nil", clientErr)
	}
	target, targetErr := core.ParseHTTPEndpoint(baseURL + path.String())
	if targetErr != nil {
		t.Fatalf("core.ParseHTTPEndpoint() setup error = %v, want nil", targetErr)
	}
	contract, contractErr := SocketContract(path)
	if contractErr != nil {
		t.Fatalf("SocketContract() setup error = %v, want nil", contractErr)
	}
	operationTimeout, operationErr := temporal.DurationFromMilliseconds(60_000)
	if operationErr != nil {
		t.Fatalf("temporal.DurationFromMilliseconds(operation) setup error = %v, want nil", operationErr)
	}
	attemptTimeout, attemptErr := temporal.DurationFromMilliseconds(30_000)
	if attemptErr != nil {
		t.Fatalf("temporal.DurationFromMilliseconds(attempt) setup error = %v, want nil", attemptErr)
	}
	configuration := exchange.ClientSocketConfiguration{
		Client: client, Target: target,
		Operation: exchange.OperationPolicy{
			OperationTimeout: operationTimeout,
			AttemptTimeout:   attemptTimeout,
			Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
			Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
		},
		Contract: contract,
	}
	got, gotErr := NewClient(configuration)
	if gotErr != nil {
		t.Fatalf("NewClient() setup error = %v, want nil", gotErr)
	}
	return got
}
