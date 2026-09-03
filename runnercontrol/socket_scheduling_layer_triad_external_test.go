package runnercontrol_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

const runnerControlSocketTimeout = 10 * time.Second

func TestSchedulingSocketProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive origin admission crosses the shared authenticated socket", func(t *testing.T) {
		t.Parallel()

		authenticated, admitted := admissionFixture(t)
		repository := admissionRepositoryFunc(func(_ context.Context, got runnercontrol.AuthenticatedAdmissionRequest) (runnercontrol.AdmissionResponse, error) {
			if got.Peer != authenticated.Peer || got.Requested.Request != authenticated.Requested.Request {
				return runnercontrol.AdmissionResponse{}, core.ErrPrimitiveContract
			}
			return runnercontrol.AdmissionResponse{SchemaVersion: runnercontrol.SchemaVersion, Request: got.Requested.Request, Admitted: &admitted}, nil
		})
		contract := admissionSocketContractFixture(t, "/runner/admission")
		boundary, err := runnercontrol.NewAdmissionServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewAdmissionServer() error = %v, want nil", err)
		}
		server, result := newRunnerControlSocketServer(t, nil, func(call exchange.SocketServerCall) error {
			return boundary.ServeAuthenticated(call, authenticated.Peer)
		})
		client, err := runnercontrol.NewAdmissionClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewAdmissionClient() error = %v, want nil", err)
		}
		response, gotErr := client.Submit(t.Context(), authenticated.Requested)
		if gotErr != nil || response.Body.Admitted == nil || response.Body.Admitted.Run != admitted.Run || response.Body.Request != authenticated.Requested.Request {
			t.Fatalf("AdmissionClient.Submit() = (%+v, %v), want exact admitted run %v and nil", response.Body, gotErr, admitted.Run)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("AdmissionServer.ServeAuthenticated() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive runner claim preserves the exact machine fence", func(t *testing.T) {
		t.Parallel()

		seeds := externalStructureSeeds(t)
		repository := claimRepositoryFunc(func(_ context.Context, got runnercontrol.ClaimRequest) (runnercontrol.ClaimResponse, error) {
			if got != seeds.claimRequest {
				return runnercontrol.ClaimResponse{}, core.ErrPrimitiveContract
			}
			return seeds.claimResponse, nil
		})
		contract := claimSocketContractFixture(t, "/runner/claim")
		boundary, err := runnercontrol.NewClaimServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewClaimServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, seeds.claimRequest.Machine, seeds.claimRequest.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewClaimClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewClaimClient() error = %v, want nil", err)
		}
		response, gotErr := client.Claim(t.Context(), seeds.claimRequest)
		if gotErr != nil || response.Body.Kind != seeds.claimResponse.Kind || response.Body.Fence != seeds.claimResponse.Fence {
			t.Fatalf("ClaimClient.Claim() = (%+v, %v), want kind %v fence %v and nil", response.Body, gotErr, seeds.claimResponse.Kind, seeds.claimResponse.Fence)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("ClaimServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive runner heartbeat preserves the exact directive fence", func(t *testing.T) {
		t.Parallel()

		seeds := externalStructureSeeds(t)
		repository := heartbeatRepositoryFunc(func(_ context.Context, got runnercontrol.HeartbeatRequest) (runnercontrol.HeartbeatResponse, error) {
			if got.Fence != seeds.heartbeatRequest.Fence || got.Observation != seeds.heartbeatRequest.Observation || got.State != seeds.heartbeatRequest.State {
				return runnercontrol.HeartbeatResponse{}, core.ErrPrimitiveContract
			}
			return seeds.heartbeatResponse, nil
		})
		contract := heartbeatSocketContractFixture(t, "/runner/heartbeat")
		boundary, err := runnercontrol.NewHeartbeatServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewHeartbeatServer() error = %v, want nil", err)
		}
		peer := runnerPeerFixture(t, seeds.heartbeatRequest.Fence.Machine, seeds.heartbeatRequest.Fence.Generation)
		server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
		client, err := runnercontrol.NewHeartbeatClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewHeartbeatClient() error = %v, want nil", err)
		}
		response, gotErr := client.Heartbeat(t.Context(), seeds.heartbeatRequest)
		if gotErr != nil || response.Body.Fence != seeds.heartbeatResponse.Fence || response.Body.Directive.Kind != seeds.heartbeatResponse.Directive.Kind {
			t.Fatalf("HeartbeatClient.Heartbeat() = (%+v, %v), want fence %v directive %v and nil", response.Body, gotErr, seeds.heartbeatResponse.Fence, seeds.heartbeatResponse.Directive.Kind)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("HeartbeatServer.Serve() error = %v, want nil", serverErr)
		}
	})

	t.Run("negative zero repositories construct no server capability", func(t *testing.T) {
		t.Parallel()

		admissionContract := admissionSocketContractFixture(t, "/runner/admission-refusal")
		claimContract := claimSocketContractFixture(t, "/runner/claim-refusal")
		heartbeatContract := heartbeatSocketContractFixture(t, "/runner/heartbeat-refusal")
		admission, admissionErr := runnercontrol.NewAdmissionServer(admissionContract, nil)
		claim, claimErr := runnercontrol.NewClaimServer(claimContract, nil)
		heartbeat, heartbeatErr := runnercontrol.NewHeartbeatServer(heartbeatContract, nil)
		if !errors.Is(admissionErr, core.ErrPrimitiveContract) || admission != (runnercontrol.AdmissionServer{}) ||
			!errors.Is(claimErr, core.ErrPrimitiveContract) || claim != (runnercontrol.ClaimServer{}) ||
			!errors.Is(heartbeatErr, core.ErrPrimitiveContract) || heartbeat != (runnercontrol.HeartbeatServer{}) {
			t.Fatalf("zero scheduling repositories = (%v/%v, %v/%v, %v/%v), want zero servers and typed refusals", admission, admissionErr, claim, claimErr, heartbeat, heartbeatErr)
		}
	})

	t.Run("neutral unauthenticated admission cannot consume a request", func(t *testing.T) {
		t.Parallel()

		contract := admissionSocketContractFixture(t, "/runner/admission-unauthenticated")
		boundary, err := runnercontrol.NewAdmissionServer(contract, admissionRepositoryFunc(func(context.Context, runnercontrol.AuthenticatedAdmissionRequest) (runnercontrol.AdmissionResponse, error) {
			return runnercontrol.AdmissionResponse{}, core.ErrPrimitiveContract
		}))
		if err != nil {
			t.Fatalf("runnercontrol.NewAdmissionServer() error = %v, want nil", err)
		}
		gotErr := boundary.Serve(exchange.SocketServerCall{})
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("AdmissionServer.Serve(unauthenticated) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})
}

type admissionRepositoryFunc func(context.Context, runnercontrol.AuthenticatedAdmissionRequest) (runnercontrol.AdmissionResponse, error)

func (f admissionRepositoryFunc) Admit(ctx context.Context, request runnercontrol.AuthenticatedAdmissionRequest) (runnercontrol.AdmissionResponse, error) {
	return f(ctx, request)
}

type claimRepositoryFunc func(context.Context, runnercontrol.ClaimRequest) (runnercontrol.ClaimResponse, error)

func (f claimRepositoryFunc) Claim(ctx context.Context, request runnercontrol.ClaimRequest) (runnercontrol.ClaimResponse, error) {
	return f(ctx, request)
}

type heartbeatRepositoryFunc func(context.Context, runnercontrol.HeartbeatRequest) (runnercontrol.HeartbeatResponse, error)

func (f heartbeatRepositoryFunc) Heartbeat(ctx context.Context, request runnercontrol.HeartbeatRequest) (runnercontrol.HeartbeatResponse, error) {
	return f(ctx, request)
}

func admissionSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	path := runnerControlSocketRouteFixture(t, value)
	contract, err := runnercontrol.AdmissionSocketContract(path)
	if err != nil {
		t.Fatalf("runnercontrol.AdmissionSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func claimSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	path := runnerControlSocketRouteFixture(t, value)
	contract, err := runnercontrol.ClaimSocketContract(path)
	if err != nil {
		t.Fatalf("runnercontrol.ClaimSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func heartbeatSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	path := runnerControlSocketRouteFixture(t, value)
	contract, err := runnercontrol.HeartbeatSocketContract(path)
	if err != nil {
		t.Fatalf("runnercontrol.HeartbeatSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func runnerControlSocketRouteFixture(t testing.TB, value string) exchange.SocketRoutePath {
	t.Helper()
	path, err := exchange.ParseSocketRoutePath(value)
	if err != nil {
		t.Fatalf("exchange.ParseSocketRoutePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func runnerPeerFixture(t testing.TB, machine runprotocol.MachineID, generation runprotocol.MachineGenerationID) runnercontrol.AuthenticatedPeer {
	t.Helper()

	credential, credentialErr := runnercontrol.NewPeerCredential(runnercontrol.PeerCredentialMutualTLS, core.SHA256Of([]byte("runner-certificate")))
	peer := runnercontrol.AuthenticatedPeer{Role: runnercontrol.PeerRoleRunner, Credential: credential, Machine: &machine, Generation: &generation}
	if err := errors.Join(credentialErr, peer.Validate()); err != nil {
		t.Fatalf("runner peer fixture error = %v, want nil", err)
	}
	return peer
}

func newRunnerControlSocketServer(t testing.TB, peer *runnercontrol.AuthenticatedPeer, serve func(exchange.SocketServerCall) error) (*httptest.Server, <-chan error) {
	t.Helper()

	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call, err := exchange.NewSocketServerCall(writer, request)
		if err == nil && peer != nil {
			call, err = runnercontrol.BindAuthenticatedPeer(call, *peer)
		}
		if err == nil {
			err = serve(call)
		}
		result <- err
	}))
	t.Cleanup(server.Close)
	return server, result
}

func waitRunnerControlSocketServer(t testing.TB, result <-chan error) error {
	t.Helper()

	timer := time.NewTimer(runnerControlSocketTimeout)
	defer timer.Stop()
	select {
	case err := <-result:
		return err
	case <-timer.C:
		t.Fatalf("runner-control socket server did not complete within %v", runnerControlSocketTimeout)
		return core.ErrPrimitiveContract
	}
}

func runnerControlClientConfiguration(t testing.TB, server *httptest.Server, contract exchange.JSONSocketContract) exchange.ClientSocketConfiguration {
	t.Helper()

	target, targetErr := core.ParseHTTPEndpoint(server.URL + contract.Path.String())
	client, clientErr := exchange.NewClient(server.Client())
	operationTimeout, operationErr := temporal.DurationFromSeconds(10)
	attemptTimeout, attemptErr := temporal.DurationFromSeconds(5)
	if err := errors.Join(targetErr, clientErr, operationErr, attemptErr); err != nil {
		t.Fatalf("runner-control client configuration error = %v, want nil", err)
	}
	return exchange.ClientSocketConfiguration{
		Target:   target,
		Client:   client,
		Contract: contract,
		Operation: exchange.OperationPolicy{
			OperationTimeout: operationTimeout,
			AttemptTimeout:   attemptTimeout,
			Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
			Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
		},
	}
}
