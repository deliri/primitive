package runnercontrol_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestObservationDeliverySocketProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive stage page and commit publish the independently verified observation", func(t *testing.T) {
		t.Parallel()

		stage, pages, verifier := deliveryProtocolFixture(t)
		identity, identityErr := stage.Identity()
		if identityErr != nil || len(pages) != 1 {
			t.Fatalf("delivery socket fixture = (identity %v, pages %d), want nil and one page", identityErr, len(pages))
		}
		store := &observationDeliveryStoreFixture{identity: identity}
		stageContract := observationDeliverySocketContractFixture(t, "/runner/delivery/stage")
		pageContract := observationDeliverySocketContractFixture(t, "/runner/delivery/page")
		commitContract := observationDeliverySocketContractFixture(t, "/runner/delivery/commit")
		boundary, err := runnercontrol.NewObservationDeliveryServer(runnercontrol.ObservationDeliveryServerConfiguration{
			Stage: stageContract, Page: pageContract, Commit: commitContract, Store: store, Verifier: verifier,
		})
		if err != nil {
			t.Fatalf("runnercontrol.NewObservationDeliveryServer() error = %v, want nil", err)
		}
		peer := controlPeerFixture(t)
		stageServer, stageResult := newRunnerControlSocketServer(t, &peer, boundary.ServeStage)
		pageServer, pageResult := newRunnerControlSocketServer(t, &peer, boundary.ServePage)
		commitServer, commitResult := newRunnerControlSocketServer(t, &peer, boundary.ServeCommit)
		client, err := runnercontrol.NewObservationDeliveryClient(runnercontrol.ObservationDeliveryClientConfiguration{
			Stage:  runnerControlClientConfiguration(t, stageServer, stageContract),
			Page:   runnerControlClientConfiguration(t, pageServer, pageContract),
			Commit: runnerControlClientConfiguration(t, commitServer, commitContract),
		})
		if err != nil {
			t.Fatalf("runnercontrol.NewObservationDeliveryClient() error = %v, want nil", err)
		}
		stageResponse, stageErr := client.Stage(t.Context(), stage)
		if stageErr != nil || stageResponse.Body.Identity != identity || stageResponse.Body.Run != stage.Envelope.Payload.Run || stageResponse.Body.PagesStored != 0 || stageResponse.Body.Published {
			t.Fatalf("ObservationDeliveryClient.Stage() = (%+v, %v), want exact unpublished zero-page receipt", stageResponse.Body, stageErr)
		}
		if serverErr := waitRunnerControlSocketServer(t, stageResult); serverErr != nil {
			t.Fatalf("ObservationDeliveryServer.ServeStage() error = %v, want nil", serverErr)
		}
		upload := runnercontrol.ObservationDeliveryPageUpload{SchemaVersion: runnercontrol.SchemaVersion, Identity: identity, Page: pages[0]}
		pageResponse, pageErr := client.StorePage(t.Context(), upload)
		if pageErr != nil || pageResponse.Body.Identity != identity || pageResponse.Body.PagesStored != 1 || pageResponse.Body.Published {
			t.Fatalf("ObservationDeliveryClient.StorePage() = (%+v, %v), want exact unpublished one-page receipt", pageResponse.Body, pageErr)
		}
		if serverErr := waitRunnerControlSocketServer(t, pageResult); serverErr != nil {
			t.Fatalf("ObservationDeliveryServer.ServePage() error = %v, want nil", serverErr)
		}
		commit := runnercontrol.ObservationDeliveryCommit{SchemaVersion: runnercontrol.SchemaVersion, Identity: identity, Run: stage.Envelope.Payload.Run, PageCount: 1}
		commitResponse, commitErr := client.Commit(t.Context(), commit)
		if commitErr != nil || commitResponse.Body.Identity != identity || commitResponse.Body.Run != commit.Run || commitResponse.Body.PagesStored != 1 || !commitResponse.Body.Published {
			t.Fatalf("ObservationDeliveryClient.Commit() = (%+v, %v), want exact published one-page receipt", commitResponse.Body, commitErr)
		}
		if serverErr := waitRunnerControlSocketServer(t, commitResult); serverErr != nil {
			t.Fatalf("ObservationDeliveryServer.ServeCommit() error = %v, want nil", serverErr)
		}
		if store.stage.Envelope.Payload.Run != stage.Envelope.Payload.Run || len(store.pages) != 1 || store.publishCount != 1 {
			t.Fatalf("delivery store state = (run %v, pages %d, publishes %d), want (%v, 1, 1)", store.stage.Envelope.Payload.Run, len(store.pages), store.publishCount, stage.Envelope.Payload.Run)
		}
	})

	t.Run("negative missing store constructs no delivery boundary", func(t *testing.T) {
		t.Parallel()

		_, _, verifier := deliveryProtocolFixture(t)
		contract := observationDeliverySocketContractFixture(t, "/runner/delivery/refusal")
		_, gotErr := runnercontrol.NewObservationDeliveryServer(runnercontrol.ObservationDeliveryServerConfiguration{
			Stage: contract, Page: contract, Commit: contract, Verifier: verifier,
		})
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("NewObservationDeliveryServer(no store) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral zero-page observation publishes without inventing page evidence", func(t *testing.T) {
		t.Parallel()

		envelope, manifest, controlKeys, runnerKeys := preRunnerObservationDeliveryFixture(t)
		stage := runnercontrol.ObservationDeliveryStage{SchemaVersion: runnercontrol.SchemaVersion, Envelope: envelope, Manifest: manifest}
		identity, identityErr := stage.Identity()
		verifier := runnercontrol.ObservationDeliveryVerifier{
			Origin: envelope.Payload.Origin, Destination: envelope.Payload.Destination, Audience: envelope.Payload.Audience,
			Grant: envelope.Payload.DeliveryGrant, ControlKeys: controlKeys, RunnerKeys: runnerKeys,
		}
		if identityErr != nil {
			t.Fatalf("ObservationDeliveryStage.Identity(zero-page) error = %v, want nil", identityErr)
		}
		store := &observationDeliveryStoreFixture{identity: identity}
		stageContract := observationDeliverySocketContractFixture(t, "/runner/delivery/neutral-stage")
		pageContract := observationDeliverySocketContractFixture(t, "/runner/delivery/neutral-page")
		commitContract := observationDeliverySocketContractFixture(t, "/runner/delivery/neutral-commit")
		boundary, err := runnercontrol.NewObservationDeliveryServer(runnercontrol.ObservationDeliveryServerConfiguration{
			Stage: stageContract, Page: pageContract, Commit: commitContract, Store: store, Verifier: verifier,
		})
		if err != nil {
			t.Fatalf("runnercontrol.NewObservationDeliveryServer(zero-page) error = %v, want nil", err)
		}
		peer := controlPeerFixture(t)
		stageServer, stageResult := newRunnerControlSocketServer(t, &peer, boundary.ServeStage)
		pageServer, _ := newRunnerControlSocketServer(t, &peer, boundary.ServePage)
		commitServer, commitResult := newRunnerControlSocketServer(t, &peer, boundary.ServeCommit)
		client, err := runnercontrol.NewObservationDeliveryClient(runnercontrol.ObservationDeliveryClientConfiguration{
			Stage:  runnerControlClientConfiguration(t, stageServer, stageContract),
			Page:   runnerControlClientConfiguration(t, pageServer, pageContract),
			Commit: runnerControlClientConfiguration(t, commitServer, commitContract),
		})
		if err != nil {
			t.Fatalf("runnercontrol.NewObservationDeliveryClient(zero-page) error = %v, want nil", err)
		}
		stageResponse, stageErr := client.Stage(t.Context(), stage)
		if stageErr != nil || stageResponse.Body.PagesStored != 0 || stageResponse.Body.Published {
			t.Fatalf("ObservationDeliveryClient.Stage(zero-page) = (%+v, %v), want zero unpublished receipt", stageResponse.Body, stageErr)
		}
		if serverErr := waitRunnerControlSocketServer(t, stageResult); serverErr != nil {
			t.Fatalf("ObservationDeliveryServer.ServeStage(zero-page) error = %v, want nil", serverErr)
		}
		commit := runnercontrol.ObservationDeliveryCommit{SchemaVersion: runnercontrol.SchemaVersion, Identity: identity, Run: envelope.Payload.Run, PageCount: 0}
		commitResponse, commitErr := client.Commit(t.Context(), commit)
		if commitErr != nil || commitResponse.Body.PagesStored != 0 || !commitResponse.Body.Published || len(store.pages) != 0 {
			t.Fatalf("ObservationDeliveryClient.Commit(zero-page) = (%+v, %v, stored pages %d), want published zero-page receipt and no page evidence", commitResponse.Body, commitErr, len(store.pages))
		}
		if serverErr := waitRunnerControlSocketServer(t, commitResult); serverErr != nil {
			t.Fatalf("ObservationDeliveryServer.ServeCommit(zero-page) error = %v, want nil", serverErr)
		}
	})
}

type observationDeliveryStoreFixture struct {
	pages        []runnercontrol.ExperimentDeliveryPage
	stage        runnercontrol.ObservationDeliveryStage
	publishCount uint64
	identity     runnercontrol.ObservationDeliveryIdentity
}

func (s *observationDeliveryStoreFixture) StageObservation(_ context.Context, stage runnercontrol.ObservationDeliveryStage) (runnercontrol.ObservationDeliveryReceipt, error) {
	s.stage = stage
	return runnercontrol.ObservationDeliveryReceipt{SchemaVersion: runnercontrol.SchemaVersion, Identity: s.identity, Run: stage.Envelope.Payload.Run}, nil
}

func (s *observationDeliveryStoreFixture) StageExperimentPage(_ context.Context, upload runnercontrol.ObservationDeliveryPageUpload) (runnercontrol.ObservationDeliveryReceipt, error) {
	if upload.Identity != s.identity {
		return runnercontrol.ObservationDeliveryReceipt{}, core.ErrPrimitiveContract
	}
	s.pages = append(s.pages, upload.Page)
	return runnercontrol.ObservationDeliveryReceipt{SchemaVersion: runnercontrol.SchemaVersion, Identity: s.identity, Run: upload.Page.Run, PagesStored: uint16(len(s.pages))}, nil
}

func (s *observationDeliveryStoreFixture) LoadStagedObservation(_ context.Context, commit runnercontrol.ObservationDeliveryCommit) (runnercontrol.ObservationDeliveryStage, []runnercontrol.ExperimentDeliveryPage, error) {
	if commit.Identity != s.identity {
		return runnercontrol.ObservationDeliveryStage{}, nil, core.ErrPrimitiveContract
	}
	return s.stage, append([]runnercontrol.ExperimentDeliveryPage(nil), s.pages...), nil
}

func (s *observationDeliveryStoreFixture) PublishObservation(_ context.Context, commit runnercontrol.ObservationDeliveryCommit) (runnercontrol.ObservationDeliveryReceipt, error) {
	if commit.Identity != s.identity {
		return runnercontrol.ObservationDeliveryReceipt{}, core.ErrPrimitiveContract
	}
	s.publishCount++
	return runnercontrol.ObservationDeliveryReceipt{
		SchemaVersion: runnercontrol.SchemaVersion, Identity: s.identity, Run: commit.Run,
		PagesStored: uint16(len(s.pages)), Published: true,
	}, nil
}

func controlPeerFixture(t testing.TB) runnercontrol.AuthenticatedPeer {
	t.Helper()
	credential, credentialErr := runnercontrol.NewPeerCredential(runnercontrol.PeerCredentialMutualTLS, core.SHA256Of([]byte("control-certificate")))
	peer := runnercontrol.AuthenticatedPeer{Role: runnercontrol.PeerRoleControl, Credential: credential}
	if err := errors.Join(credentialErr, peer.Validate()); err != nil {
		t.Fatalf("control peer fixture error = %v, want nil", err)
	}
	return peer
}

func observationDeliverySocketContractFixture(t testing.TB, value string) exchange.JSONSocketContract {
	t.Helper()
	contract, err := runnercontrol.ObservationDeliverySocketContract(runnerControlSocketRouteFixture(t, value), core.JSONDocumentMaximumBytes)
	if err != nil {
		t.Fatalf("runnercontrol.ObservationDeliverySocketContract(%q) error = %v, want nil", value, err)
	}
	return contract
}
