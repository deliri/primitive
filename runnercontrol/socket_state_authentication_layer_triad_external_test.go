package runnercontrol_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestRunStateAndCancellationSocketProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive origin reads the exact current run state", func(t *testing.T) {
		t.Parallel()

		cancellation := cancellationRequestFixture(t)
		request := runnercontrol.RunStateRequest{SchemaVersion: runnercontrol.SchemaVersion, Run: cancellation.Coordinate.Run, RequestedAt: temporal.InstantFromNanoseconds(2)}
		want := runnercontrol.RunStateResponse{SchemaVersion: runnercontrol.SchemaVersion, Run: request.Run, State: runnercontrol.RunControlExecuting, UpdatedAt: temporal.InstantFromNanoseconds(3)}
		peer := originPeerFixture(t, cancellation.Coordinate.Origin)
		repository := runStateRepositoryFunc(func(_ context.Context, got runnercontrol.AuthenticatedRunStateRequest) (runnercontrol.RunStateResponse, error) {
			if got.Peer != peer || got.Request != request {
				return runnercontrol.RunStateResponse{}, core.ErrPrimitiveContract
			}
			return want, nil
		})
		contract := runStateSocketContractFixture(t, "/runner/state")
		boundary, err := runnercontrol.NewRunStateServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewRunStateServer() error = %v, want nil", err)
		}
		server, result := newRunnerControlSocketServer(t, nil, func(call exchange.SocketServerCall) error {
			return boundary.ServeAuthenticated(call, peer)
		})
		client, err := runnercontrol.NewRunStateClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewRunStateClient() error = %v, want nil", err)
		}
		response, gotErr := client.Fetch(t.Context(), request)
		if gotErr != nil || response.Body != want {
			t.Fatalf("RunStateClient.Fetch() = (%+v, %v), want (%+v, nil)", response.Body, gotErr, want)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("RunStateServer.ServeAuthenticated() error = %v, want nil", serverErr)
		}
	})

	t.Run("positive origin cancellation preserves identity and exposes the later state", func(t *testing.T) {
		t.Parallel()

		request := cancellationRequestFixture(t)
		want := runnercontrol.CancellationResponse{
			SchemaVersion: runnercontrol.SchemaVersion,
			Identity:      request.Identity,
			Run:           request.Coordinate.Run,
			State:         runnercontrol.RunControlCancellationRequested,
			RecordedAt:    temporal.InstantFromNanoseconds(2),
		}
		peer := originPeerFixture(t, request.Coordinate.Origin)
		repository := cancellationRepositoryFunc(func(_ context.Context, got runnercontrol.AuthenticatedCancellationRequest) (runnercontrol.CancellationResponse, error) {
			if got.Peer != peer || got.Request != request {
				return runnercontrol.CancellationResponse{}, core.ErrPrimitiveContract
			}
			return want, nil
		})
		contract := cancellationSocketContractFixture(t, "/runner/cancellation")
		boundary, err := runnercontrol.NewCancellationServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewCancellationServer() error = %v, want nil", err)
		}
		server, result := newRunnerControlSocketServer(t, nil, func(call exchange.SocketServerCall) error {
			return boundary.ServeAuthenticated(call, peer)
		})
		client, err := runnercontrol.NewCancellationClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewCancellationClient() error = %v, want nil", err)
		}
		response, gotErr := client.Cancel(t.Context(), request)
		if gotErr != nil || response.Body != want {
			t.Fatalf("CancellationClient.Cancel() = (%+v, %v), want (%+v, nil)", response.Body, gotErr, want)
		}
		if serverErr := waitRunnerControlSocketServer(t, result); serverErr != nil {
			t.Fatalf("CancellationServer.ServeAuthenticated() error = %v, want nil", serverErr)
		}
	})

	t.Run("negative missing repositories construct no state-changing capability", func(t *testing.T) {
		t.Parallel()

		stateContract := runStateSocketContractFixture(t, "/runner/state-refusal")
		cancellationContract := cancellationSocketContractFixture(t, "/runner/cancellation-refusal")
		_, stateErr := runnercontrol.NewRunStateServer(stateContract, nil)
		_, cancellationErr := runnercontrol.NewCancellationServer(cancellationContract, nil)
		if !errors.Is(stateErr, core.ErrPrimitiveContract) || !errors.Is(cancellationErr, core.ErrPrimitiveContract) {
			t.Fatalf("zero run-control repositories errors = (%v, %v), want two errors.Is(..., %v)", stateErr, cancellationErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral foreign origin cannot create a cancellation effect", func(t *testing.T) {
		t.Parallel()

		request := cancellationRequestFixture(t)
		foreign := runprotocol.OriginIdentity{Offering: core.Offering{Token: "foreign-origin"}}
		peer := originPeerFixture(t, foreign)
		calls := 0
		repository := cancellationRepositoryFunc(func(context.Context, runnercontrol.AuthenticatedCancellationRequest) (runnercontrol.CancellationResponse, error) {
			calls++
			return runnercontrol.CancellationResponse{}, core.ErrPrimitiveContract
		})
		contract := cancellationSocketContractFixture(t, "/runner/cancellation-foreign-origin")
		boundary, err := runnercontrol.NewCancellationServer(contract, repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewCancellationServer() error = %v, want nil", err)
		}
		server, result := newRunnerControlSocketServer(t, nil, func(call exchange.SocketServerCall) error {
			return boundary.ServeAuthenticated(call, peer)
		})
		client, err := runnercontrol.NewCancellationClient(runnerControlClientConfiguration(t, server, contract))
		if err != nil {
			t.Fatalf("runnercontrol.NewCancellationClient() error = %v, want nil", err)
		}
		_, _ = client.Cancel(t.Context(), request)
		serverErr := waitRunnerControlSocketServer(t, result)
		if !errors.Is(serverErr, core.ErrPrimitiveContract) || calls != 0 {
			t.Fatalf("foreign-origin cancellation = (server error %v, repository calls %d), want typed refusal and zero effects", serverErr, calls)
		}
	})
}

func TestMutualTLSAuthenticationProductionBoundaryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive verified leaf resolves one exact runner peer", func(t *testing.T) {
		t.Parallel()

		certificateBytes := []byte("runner-leaf-certificate")
		machine := externalStructureSeeds(t).claimRequest.Machine
		generation := externalStructureSeeds(t).claimRequest.Generation
		want := runnerPeerFixture(t, machine, generation)
		want.Credential, _ = runnercontrol.NewPeerCredential(runnercontrol.PeerCredentialMutualTLS, core.SHA256Of(certificateBytes))
		repository := peerIdentityRepositoryFunc(func(_ context.Context, credential runnercontrol.PeerCredential, role runnercontrol.PeerRole) (runnercontrol.AuthenticatedPeer, error) {
			if credential != want.Credential || role != runnercontrol.PeerRoleRunner {
				return runnercontrol.AuthenticatedPeer{}, core.ErrPrimitiveContract
			}
			return want, nil
		})
		authenticator, err := runnercontrol.NewMutualTLSAuthenticator(repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewMutualTLSAuthenticator() error = %v, want nil", err)
		}
		call := mutualTLSCallFixture(t, certificateBytes)
		got, gotErr := authenticator.Authenticate(call, runnercontrol.PeerRoleRunner)
		bound, bindErr := runnercontrol.BindAuthenticatedPeer(call, got)
		ctx, contextErr := bound.Context()
		runnerErr := runnercontrol.RequireRunnerPeer(ctx, machine, generation)
		fromContext, fromContextErr := runnercontrol.AuthenticatedPeerFromContext(ctx)
		if gotErr != nil || bindErr != nil || contextErr != nil || runnerErr != nil || fromContextErr != nil || got != want || fromContext != want {
			t.Fatalf("mutual TLS runner authentication = (got %+v, from context %+v, errors %v/%v/%v/%v/%v), want exact peer %+v and nil", got, fromContext, gotErr, bindErr, contextErr, runnerErr, fromContextErr, want)
		}
	})

	t.Run("positive verified leaf resolves one exact control peer", func(t *testing.T) {
		t.Parallel()

		certificateBytes := []byte("control-leaf-certificate")
		credential, credentialErr := runnercontrol.NewPeerCredential(runnercontrol.PeerCredentialMutualTLS, core.SHA256Of(certificateBytes))
		want := runnercontrol.AuthenticatedPeer{Role: runnercontrol.PeerRoleControl, Credential: credential}
		repository := peerIdentityRepositoryFunc(func(context.Context, runnercontrol.PeerCredential, runnercontrol.PeerRole) (runnercontrol.AuthenticatedPeer, error) {
			return want, nil
		})
		authenticator, authenticatorErr := runnercontrol.NewMutualTLSAuthenticator(repository)
		call := mutualTLSCallFixture(t, certificateBytes)
		got, gotErr := authenticator.Authenticate(call, runnercontrol.PeerRoleControl)
		bound, bindErr := runnercontrol.BindAuthenticatedPeer(call, got)
		ctx, contextErr := bound.Context()
		controlErr := runnercontrol.RequireControlPeer(ctx)
		if credentialErr != nil || authenticatorErr != nil || gotErr != nil || bindErr != nil || contextErr != nil || controlErr != nil || got != want {
			t.Fatalf("mutual TLS control authentication = (got %+v, errors %v/%v/%v/%v/%v/%v), want exact peer %+v and nil", got, credentialErr, authenticatorErr, gotErr, bindErr, contextErr, controlErr, want)
		}
	})

	t.Run("negative resolver cannot substitute a foreign credential", func(t *testing.T) {
		t.Parallel()

		certificateBytes := []byte("presented-leaf-certificate")
		machine := externalStructureSeeds(t).claimRequest.Machine
		generation := externalStructureSeeds(t).claimRequest.Generation
		foreign := runnerPeerFixture(t, machine, generation)
		repository := peerIdentityRepositoryFunc(func(context.Context, runnercontrol.PeerCredential, runnercontrol.PeerRole) (runnercontrol.AuthenticatedPeer, error) {
			return foreign, nil
		})
		authenticator, err := runnercontrol.NewMutualTLSAuthenticator(repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewMutualTLSAuthenticator() error = %v, want nil", err)
		}
		got, gotErr := authenticator.Authenticate(mutualTLSCallFixture(t, certificateBytes), runnercontrol.PeerRoleRunner)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != (runnercontrol.AuthenticatedPeer{}) {
			t.Fatalf("MutualTLSAuthenticator.Authenticate(foreign credential) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral absent verified chain resolves no peer", func(t *testing.T) {
		t.Parallel()

		repositoryCalls := 0
		repository := peerIdentityRepositoryFunc(func(context.Context, runnercontrol.PeerCredential, runnercontrol.PeerRole) (runnercontrol.AuthenticatedPeer, error) {
			repositoryCalls++
			return runnercontrol.AuthenticatedPeer{}, core.ErrPrimitiveContract
		})
		authenticator, err := runnercontrol.NewMutualTLSAuthenticator(repository)
		if err != nil {
			t.Fatalf("runnercontrol.NewMutualTLSAuthenticator() error = %v, want nil", err)
		}
		request := httptest.NewRequest(http.MethodPost, "https://runner.example.invalid/auth", nil)
		call, callErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
		got, gotErr := authenticator.Authenticate(call, runnercontrol.PeerRoleRunner)
		if callErr != nil || !errors.Is(gotErr, core.ErrPrimitiveContract) || got != (runnercontrol.AuthenticatedPeer{}) || repositoryCalls != 0 {
			t.Fatalf("MutualTLSAuthenticator.Authenticate(no chain) = (%+v, %v, call %v, repository calls %d), want zero, typed refusal, nil, zero", got, gotErr, callErr, repositoryCalls)
		}
	})
}

type runStateRepositoryFunc func(context.Context, runnercontrol.AuthenticatedRunStateRequest) (runnercontrol.RunStateResponse, error)

func (f runStateRepositoryFunc) RunState(ctx context.Context, request runnercontrol.AuthenticatedRunStateRequest) (runnercontrol.RunStateResponse, error) {
	return f(ctx, request)
}

type cancellationRepositoryFunc func(context.Context, runnercontrol.AuthenticatedCancellationRequest) (runnercontrol.CancellationResponse, error)

func (f cancellationRepositoryFunc) Cancel(ctx context.Context, request runnercontrol.AuthenticatedCancellationRequest) (runnercontrol.CancellationResponse, error) {
	return f(ctx, request)
}

type peerIdentityRepositoryFunc func(context.Context, runnercontrol.PeerCredential, runnercontrol.PeerRole) (runnercontrol.AuthenticatedPeer, error)

func (f peerIdentityRepositoryFunc) ResolvePeer(ctx context.Context, credential runnercontrol.PeerCredential, role runnercontrol.PeerRole) (runnercontrol.AuthenticatedPeer, error) {
	return f(ctx, credential, role)
}

func originPeerFixture(t testing.TB, origin runprotocol.OriginIdentity) runnercontrol.AuthenticatedPeer {
	t.Helper()
	credential, credentialErr := runnercontrol.NewPeerCredential(runnercontrol.PeerCredentialMutualTLS, core.SHA256Of([]byte("origin-certificate")))
	peer := runnercontrol.AuthenticatedPeer{Role: runnercontrol.PeerRoleOrigin, Credential: credential, Origin: &origin}
	if err := errors.Join(credentialErr, peer.Validate()); err != nil {
		t.Fatalf("origin peer fixture error = %v, want nil", err)
	}
	return peer
}

func runStateSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.RunStateSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.RunStateSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func cancellationSocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.CancellationSocketContract(runnerControlSocketRouteFixture(t, value))
	if err != nil {
		t.Fatalf("runnercontrol.CancellationSocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}

func mutualTLSCallFixture(t testing.TB, certificateBytes []byte) exchange.SocketServerCall {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "https://runner.example.invalid/auth", nil)
	request.TLS = &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{Raw: append([]byte(nil), certificateBytes...)}}}}
	call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatalf("exchange.NewSocketServerCall(mutual TLS) error = %v, want nil", err)
	}
	return call
}
