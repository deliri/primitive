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
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

const machineAboutTestRoute = "/v1/about/machine"

type fixtureMachineRepository struct {
	machine CurrentMachine
	calls   atomic.Uint64
}

func (r *fixtureMachineRepository) CurrentMachine(context.Context, MachineID) (CurrentMachine, error) {
	r.calls.Add(1)
	return r.machine, nil
}

func TestMachineServiceAndSocketLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive real paired socket returns the exact observed machine and transport metadata", func(t *testing.T) {
		t.Parallel()
		repository := &fixtureMachineRepository{machine: fixtureCurrentMachine(t)}
		server, path := fixtureMachineHTTPServer(t, repository)
		defer server.Close()
		client := fixtureMachineClient(t, server.Client(), server.URL, path)
		query := fixtureMachineQuery(repository.machine)

		got, gotErr := client.Fetch(t.Context(), query)
		if gotErr != nil {
			t.Fatalf("MachineClient.Fetch() error = %v, want nil", gotErr)
		}
		if got.Response.Machine.Generation.ID != repository.machine.Generation.ID || got.Response.Machine.Observation.ID != repository.machine.Observation.ID {
			t.Fatalf("MachineClient.Fetch().Machine identities = (%v, %v), want (%v, %v)", got.Response.Machine.Generation.ID, got.Response.Machine.Observation.ID, repository.machine.Generation.ID, repository.machine.Observation.ID)
		}
		if got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() == 0 || repository.calls.Load() != 1 {
			t.Fatalf("MachineClient.Fetch() execution = (attempts %d, bytes %d, repository calls %d), want (1, nonzero, 1)", got.Metadata.Attempts, got.Metadata.Bytes.Uint64(), repository.calls.Load())
		}
	})

	t.Run("negative repository cannot substitute another machine identity", func(t *testing.T) {
		t.Parallel()
		requested := fixtureCurrentMachine(t)
		repository := &fixtureMachineRepository{machine: fixtureCurrentMachine(t)}
		repository.machine.Generation.Configuration.Identity.ID = fixtureForeignMachineID(t)
		repository.machine.Observation.Configuration.Identity.ID = repository.machine.Generation.Configuration.Identity.ID
		server, path := fixtureMachineHTTPServer(t, repository)
		defer server.Close()
		client := fixtureMachineClient(t, server.Client(), server.URL, path)

		got, gotErr := client.Fetch(t.Context(), fixtureMachineQuery(requested))
		if !errors.Is(gotErr, core.ErrAboutTransport) || got.Metadata.Attempts != 0 || got.Response.SchemaVersion != 0 {
			t.Fatalf("MachineClient.Fetch(conflicting identity) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrAboutTransport)
		}
		if repository.calls.Load() != 1 {
			t.Fatalf("machine repository calls = %d, want 1", repository.calls.Load())
		}
	})

	t.Run("neutral cancelled query performs no repository effect", func(t *testing.T) {
		t.Parallel()
		repository := &fixtureMachineRepository{machine: fixtureCurrentMachine(t)}
		service, setupErr := NewMachineService(repository)
		if setupErr != nil {
			t.Fatalf("NewMachineService() setup error = %v, want nil", setupErr)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got, gotErr := service.Resolve(ctx, fixtureMachineQuery(repository.machine))
		if !errors.Is(gotErr, context.Canceled) || got.SchemaVersion != 0 {
			t.Fatalf("MachineService.Resolve(cancelled) = (%+v, %v), want zero and errors.Is(..., context.Canceled)", got, gotErr)
		}
		if repository.calls.Load() != 0 {
			t.Fatalf("machine repository calls after cancellation = %d, want 0", repository.calls.Load())
		}
	})
}

func fixtureMachineQuery(machine CurrentMachine) MachineQuery {
	return MachineQuery{SchemaVersion: SchemaVersion, Machine: machine.Generation.Configuration.Identity.ID}
}

func fixtureForeignMachineID(t testing.TB) MachineID {
	t.Helper()
	uuid, err := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000002")
	if err != nil {
		t.Fatalf("id.ParseUUIDv7(foreign machine) setup error = %v, want nil", err)
	}
	got, err := NewMachineID(uuid)
	if err != nil {
		t.Fatalf("NewMachineID(foreign machine) setup error = %v, want nil", err)
	}
	return got
}

func fixtureMachineHTTPServer(t testing.TB, repository MachineRepository) (*httptest.Server, exchange.SocketRoutePath) {
	t.Helper()
	path, pathErr := exchange.ParseSocketRoutePath(machineAboutTestRoute)
	if pathErr != nil {
		t.Fatalf("exchange.ParseSocketRoutePath(%q) setup error = %v, want nil", machineAboutTestRoute, pathErr)
	}
	server, serverErr := NewMachineServer(path, repository)
	if serverErr != nil {
		t.Fatalf("NewMachineServer() setup error = %v, want nil", serverErr)
	}
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if gotErr := server.Serve(writer, request); gotErr != nil {
			http.Error(writer, "about machine request refused", http.StatusInternalServerError)
		}
	}))
	return httpServer, path
}

func fixtureMachineClient(t testing.TB, httpClient *http.Client, baseURL string, path exchange.SocketRoutePath) MachineClient {
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
	got, gotErr := NewMachineClient(exchange.ClientSocketConfiguration{
		Client: client, Target: target,
		Operation: exchange.OperationPolicy{
			OperationTimeout: operationTimeout,
			AttemptTimeout:   attemptTimeout,
			Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
			Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
		},
		Contract: contract,
	})
	if gotErr != nil {
		t.Fatalf("NewMachineClient() setup error = %v, want nil", gotErr)
	}
	return got
}
