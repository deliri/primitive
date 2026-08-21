package upgrade

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestStageAuthorityClosesAuthenticatedFactsAgainstDurableSelection(t *testing.T) {
	t.Parallel()

	t.Run("positive exact installed authority admits only the newer artifact", func(t *testing.T) {
		t.Parallel()

		root, _ := stageRootForTest(t)
		installed := artifactForTest(t, []byte("installed"), 1)
		candidate := artifactForTest(t, []byte("candidate"), 2)
		prior := selectionDocument{
			Revision: selectionRevisionCurrent,
			Slot:     SlotA,
			Artifact: installed,
		}
		if err := writeSelection(t.Context(), root, prior, filestore.InstallCreate); err != nil {
			t.Fatalf("writeSelection() error = %v, want nil", err)
		}

		gotCandidate, gotPrior, gotErr := authorizeStage(
			t.Context(), root,
			stageAuthorityFacts{candidate: candidate, installed: installed},
		)
		if gotErr != nil || gotCandidate != candidate || gotPrior != prior {
			t.Fatalf("authorizeStage(exact) = (%v, %v, %v), want exact candidate/prior/nil",
				gotCandidate, gotPrior, gotErr)
		}
	})

	t.Run("negative changed durable primary seals both returned facts", func(t *testing.T) {
		t.Parallel()

		root, _ := stageRootForTest(t)
		installed := artifactForTest(t, []byte("installed"), 1)
		changed := artifactForTest(t, []byte("changed primary"), 1)
		candidate := artifactForTest(t, []byte("candidate"), 2)
		if err := writeSelection(t.Context(), root, selectionDocument{
			Revision: selectionRevisionCurrent,
			Slot:     SlotA,
			Artifact: changed,
		}, filestore.InstallCreate); err != nil {
			t.Fatalf("writeSelection(changed) error = %v, want nil", err)
		}

		gotCandidate, gotPrior, gotErr := authorizeStage(
			t.Context(), root,
			stageAuthorityFacts{candidate: candidate, installed: installed},
		)
		if !errors.Is(gotErr, core.ErrUpgradeConflict) ||
			!errors.Is(gotErr, diagnosticCurrentSelection) {
			t.Fatalf("authorizeStage(changed) error = %v, want %v and typed current-selection diagnostic",
				gotErr, core.ErrUpgradeConflict)
		}
		if gotCandidate != (release.Artifact{}) || gotPrior != (selectionDocument{}) {
			t.Fatalf("authorizeStage(changed) = (%v, %v), want sealed zero facts",
				gotCandidate, gotPrior)
		}
	})

	t.Run("negative malformed and non-upgrade pairs refuse before persistence", func(t *testing.T) {
		t.Parallel()

		installed := artifactForTest(t, []byte("installed"), 2)
		cases := []struct {
			name  string
			facts stageAuthorityFacts
		}{
			{name: "both facts unset"},
			{name: "candidate unset", facts: stageAuthorityFacts{installed: installed}},
			{name: "installed unset", facts: stageAuthorityFacts{
				candidate: artifactForTest(t, []byte("candidate"), 3),
			}},
			{name: "equal version", facts: stageAuthorityFacts{
				installed: installed,
				candidate: artifactForTest(t, []byte("equal"), 2),
			}},
			{name: "downgrade", facts: stageAuthorityFacts{
				installed: installed,
				candidate: artifactForTest(t, []byte("older"), 1),
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gotCandidate, gotPrior, gotErr := authorizeStage(
					t.Context(), nil, tc.facts,
				)
				if !errors.Is(gotErr, core.ErrUpgradeContract) {
					t.Fatalf("authorizeStage(%s) error = %v, want %v",
						tc.name, gotErr, core.ErrUpgradeContract)
				}
				if gotCandidate != (release.Artifact{}) || gotPrior != (selectionDocument{}) {
					t.Fatalf("authorizeStage(%s) = (%v, %v), want sealed zero facts",
						tc.name, gotCandidate, gotPrior)
				}
			})
		}
	})

	t.Run("neutral cancellation preserves the selector and returns no authority", func(t *testing.T) {
		t.Parallel()

		root, _ := stageRootForTest(t)
		installed := artifactForTest(t, []byte("installed"), 1)
		candidate := artifactForTest(t, []byte("candidate"), 2)
		prior := selectionDocument{
			Revision: selectionRevisionCurrent,
			Slot:     SlotA,
			Artifact: installed,
		}
		if err := writeSelection(t.Context(), root, prior, filestore.InstallCreate); err != nil {
			t.Fatalf("writeSelection() error = %v, want nil", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		gotCandidate, gotPrior, gotErr := authorizeStage(
			ctx, root,
			stageAuthorityFacts{candidate: candidate, installed: installed},
		)
		if !errors.Is(gotErr, context.Canceled) ||
			!errors.Is(gotErr, core.ErrUpgradePersistence) {
			t.Fatalf("authorizeStage(canceled) error = %v, want %v and %v",
				gotErr, context.Canceled, core.ErrUpgradePersistence)
		}
		if gotCandidate != (release.Artifact{}) || gotPrior != (selectionDocument{}) {
			t.Fatalf("authorizeStage(canceled) = (%v, %v), want sealed zero facts",
				gotCandidate, gotPrior)
		}
		gotSelection, readErr := readSelection(t.Context(), root)
		if readErr != nil || gotSelection != prior {
			t.Fatalf("readSelection(after cancellation) = (%v, %v), want unchanged prior",
				gotSelection, readErr)
		}
	})
}

func TestDownloadSourceAndStagePolicyCloseEveryExecutionDependency(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte{0xa5}, (32<<10)+1)
	transport := &stageDownloadTransport{payload: payload}
	source := stageDownloadSourceForTest(t, stageDownloadSourceFixture{
		objectName: "bucket/candidate", transport: transport,
	})
	if err := source.Validate(); err != nil {
		t.Fatalf("DownloadSource.Validate(valid) error = %v, want nil", err)
	}

	other := stageDownloadSourceForTest(t, stageDownloadSourceFixture{
		objectName: "bucket/other-candidate", transport: transport,
	})
	cases := []struct {
		mutate func(*DownloadSource)
		name   string
	}{
		{name: "client unset", mutate: func(value *DownloadSource) {
			value.Client = objectstore.Client{}
		}},
		{name: "capability unset", mutate: func(value *DownloadSource) {
			value.Capability = objectstore.DownloadCapability{}
		}},
		{name: "commitment unset", mutate: func(value *DownloadSource) {
			value.Commitment = objectstore.DownloadCapabilityCommitment{}
		}},
		{name: "commitment belongs to another bearer", mutate: func(value *DownloadSource) {
			value.Commitment = other.Commitment
		}},
		{name: "operation timeout unset", mutate: func(value *DownloadSource) {
			value.Policy.OperationTimeout = temporal.Duration{}
		}},
		{name: "attempt timeout unset", mutate: func(value *DownloadSource) {
			value.Policy.AttemptTimeout = temporal.Duration{}
		}},
		{name: "attempt exceeds operation", mutate: func(value *DownloadSource) {
			value.Policy.OperationTimeout, value.Policy.AttemptTimeout =
				value.Policy.AttemptTimeout, value.Policy.OperationTimeout
		}},
		{name: "error body bound unset", mutate: func(value *DownloadSource) {
			value.Policy.ErrorBodyLimit = core.ByteCount{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidate := source
			tc.mutate(&candidate)
			gotErr := candidate.Validate()
			if !errors.Is(gotErr, core.ErrUpgradeContract) {
				t.Fatalf("DownloadSource.Validate(%s) error = %v, want %v",
					tc.name, gotErr, core.ErrUpgradeContract)
			}
		})
	}

	zero := StagePolicy{}
	if err := zero.Validate(); err != nil {
		t.Fatalf("StagePolicy.Validate(zero reserve) error = %v, want explicit disabled reserve", err)
	}
	maximum, err := core.NewByteLength(math.MaxInt64)
	if err != nil {
		t.Fatalf("core.NewByteLength(max int64) error = %v, want nil", err)
	}
	if err := (StagePolicy{FreeSpaceReserve: maximum}).Validate(); err != nil {
		t.Fatalf("StagePolicy.Validate(maximum reserve) error = %v, want nil", err)
	}
}

func TestDownloadAndVerifyCandidateLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive streams exact bytes and reports exact progress", func(t *testing.T) {
		t.Parallel()

		payload := bytes.Repeat([]byte{0x5a}, (64<<10)+1)
		root, directory := stageRootForTest(t)
		installed := artifactForTest(t, []byte("installed"), 1)
		candidate := artifactForTest(t, payload, 2)
		target := stageTargetForTest(t, stageTargetFixture{
			directory: directory, installed: installed, candidate: candidate,
		})
		if err := prepareCandidateSlot(t.Context(), root, target); err != nil {
			t.Fatalf("prepareCandidateSlot() error = %v, want nil", err)
		}
		var observed objectstore.TransferProgress
		source := stageDownloadSourceForTest(t, stageDownloadSourceFixture{
			objectName: "bucket/exact-candidate",
			transport:  &stageDownloadTransport{payload: payload},
		})
		source.Observer = func(progress objectstore.TransferProgress) error {
			observed = progress
			return nil
		}

		gotErr := downloadAndVerifyCandidate(t.Context(), StageRequest{
			Root: root, Directory: directory, Source: source,
		}, target)
		if gotErr != nil {
			t.Fatalf("downloadAndVerifyCandidate(exact) error = %v, want nil", gotErr)
		}
		got, readErr := root.ReadFile(target.Path().String())
		if readErr != nil || !bytes.Equal(got, payload) {
			t.Fatalf("candidate bytes = (%d, %v), want exact %d-byte payload",
				len(got), readErr, len(payload))
		}
		wantExtent, extentErr := candidate.Integrity().Extent().Uint64()
		if extentErr != nil {
			t.Fatalf("candidate extent error = %v, want nil", extentErr)
		}
		if observed.Validate() != nil ||
			observed.Direction() != objectstore.DirectionDownload ||
			observed.Completed().Uint64() != wantExtent ||
			observed.Total().Uint64() != wantExtent {
			t.Fatalf("final progress = (%v, %d/%d, %v), want valid download %d/%d",
				observed.Direction(), observed.Completed().Uint64(), observed.Total().Uint64(),
				observed.Validate(), wantExtent, wantExtent)
		}
	})

	t.Run("negative transport failure removes only the owned partial candidate", func(t *testing.T) {
		t.Parallel()

		root, directory := stageRootForTest(t)
		installedBytes := []byte("installed")
		installed := artifactForTest(t, installedBytes, 1)
		candidate := artifactForTest(t, []byte("candidate"), 2)
		installArtifactForTest(t, root, SlotA, installed, installedBytes)
		target := stageTargetForTest(t, stageTargetFixture{
			directory: directory, installed: installed, candidate: candidate,
		})
		if err := prepareCandidateSlot(t.Context(), root, target); err != nil {
			t.Fatalf("prepareCandidateSlot() error = %v, want nil", err)
		}
		transportCause := errors.New("closed test transport")
		transport := &stageDownloadTransport{cause: transportCause}
		source := stageDownloadSourceForTest(t, stageDownloadSourceFixture{
			objectName: "bucket/failed-candidate", transport: transport,
		})

		gotErr := downloadAndVerifyCandidate(t.Context(), StageRequest{
			Root: root, Directory: directory, Source: source,
		}, target)
		requireAttemptFailure(t, gotErr, attemptFailureExpectation{
			phase: FailurePhaseDownload, identity: core.ErrUpgradeDownload,
			cause: transportCause, candidate: candidate.Build(),
		})
		if _, statErr := root.Stat(target.Path().String()); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("failed candidate stat error = %v, want %v", statErr, os.ErrNotExist)
		}
		primaryPath, pathErr := binaryPath(SlotA, installed.Build())
		if pathErr != nil {
			t.Fatalf("binaryPath(primary) error = %v, want nil", pathErr)
		}
		gotPrimary, readErr := root.ReadFile(primaryPath.String())
		if readErr != nil || !bytes.Equal(gotPrimary, installedBytes) {
			t.Fatalf("primary after failed download = (%q, %v), want exact preserved bytes",
				gotPrimary, readErr)
		}
	})

	t.Run("negative post-transfer tampering is caught by independent artifact verification", func(t *testing.T) {
		t.Parallel()

		payload := bytes.Repeat([]byte{0x3c}, (32<<10)+1)
		root, directory := stageRootForTest(t)
		installed := artifactForTest(t, []byte("installed"), 1)
		candidate := artifactForTest(t, payload, 2)
		target := stageTargetForTest(t, stageTargetFixture{
			directory: directory, installed: installed, candidate: candidate,
		})
		if err := prepareCandidateSlot(t.Context(), root, target); err != nil {
			t.Fatalf("prepareCandidateSlot() error = %v, want nil", err)
		}
		source := stageDownloadSourceForTest(t, stageDownloadSourceFixture{
			objectName: "bucket/tampered-candidate",
			transport:  &stageDownloadTransport{payload: payload},
		})
		source.Observer = func(progress objectstore.TransferProgress) error {
			if progress.Completed().Uint64() != progress.Total().Uint64() {
				return nil
			}
			return root.WriteFile(target.Path().String(), []byte("tampered"), executableMode)
		}

		gotErr := downloadAndVerifyCandidate(t.Context(), StageRequest{
			Root: root, Directory: directory, Source: source,
		}, target)
		requireAttemptFailure(t, gotErr, attemptFailureExpectation{
			phase: FailurePhaseVerification, identity: core.ErrUpgradeVerification,
			candidate: candidate.Build(),
		})
		if _, statErr := root.Stat(target.Path().String()); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("tampered candidate stat error = %v, want %v", statErr, os.ErrNotExist)
		}
	})

	t.Run("neutral cancellation performs no transport and leaves no candidate", func(t *testing.T) {
		t.Parallel()

		payload := []byte("candidate")
		root, directory := stageRootForTest(t)
		installed := artifactForTest(t, []byte("installed"), 1)
		candidate := artifactForTest(t, payload, 2)
		target := stageTargetForTest(t, stageTargetFixture{
			directory: directory, installed: installed, candidate: candidate,
		})
		if err := prepareCandidateSlot(t.Context(), root, target); err != nil {
			t.Fatalf("prepareCandidateSlot() error = %v, want nil", err)
		}
		transport := &stageDownloadTransport{payload: payload}
		source := stageDownloadSourceForTest(t, stageDownloadSourceFixture{
			objectName: "bucket/canceled-candidate", transport: transport,
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		gotErr := downloadAndVerifyCandidate(ctx, StageRequest{
			Root: root, Directory: directory, Source: source,
		}, target)
		requireAttemptFailure(t, gotErr, attemptFailureExpectation{
			phase: FailurePhaseDownload, identity: core.ErrUpgradeDownload,
			cause: context.Canceled, candidate: candidate.Build(),
		})
		if transport.calls.Load() != 0 {
			t.Fatalf("transport calls = %d, want zero after pre-effect cancellation",
				transport.calls.Load())
		}
		if _, statErr := root.Stat(target.Path().String()); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("canceled candidate stat error = %v, want %v", statErr, os.ErrNotExist)
		}
	})
}

func TestStageCapacityAndCleanupKeepNumericAndNativeIdentity(t *testing.T) {
	t.Parallel()

	root, directory := stageRootForTest(t)
	candidate := artifactForTest(t, []byte("candidate"), 2)
	if err := admitStageCapacity(t.Context(), StageRequest{
		Root: root, Directory: directory,
	}, candidate); err != nil {
		t.Fatalf("admitStageCapacity(small candidate) error = %v, want nil", err)
	}

	maximum, err := core.NewByteLength(math.MaxInt64)
	if err != nil {
		t.Fatalf("core.NewByteLength(max int64) error = %v, want nil", err)
	}
	gotErr := admitStageCapacity(t.Context(), StageRequest{
		Root: root, Directory: directory,
		Policy: StagePolicy{FreeSpaceReserve: maximum},
	}, candidate)
	if !errors.Is(gotErr, core.ErrNumericOverflow) {
		t.Fatalf("admitStageCapacity(overflowing floor) error = %v, want %v",
			gotErr, core.ErrNumericOverflow)
	}

	native := errors.New("test cleanup refusal")
	if got := classifyAttemptCleanup(nil); got != nil {
		t.Fatalf("classifyAttemptCleanup(nil) = %v, want nil", got)
	}
	classified := classifyAttemptCleanup(native)
	if !errors.Is(classified, core.ErrUpgradeCleanup) || !errors.Is(classified, native) {
		t.Fatalf("classifyAttemptCleanup(native) = %v, want %v and preserved native cause",
			classified, core.ErrUpgradeCleanup)
	}
}

func TestUpgradePathProjectionsRemainBoundToTheirAuthenticatedArtifacts(t *testing.T) {
	t.Parallel()

	_, directory := stageRootForTest(t)
	installed := artifactForTest(t, []byte("installed"), 1)
	candidate := artifactForTest(t, []byte("candidate"), 2)
	target := stageTargetForTest(t, stageTargetFixture{
		directory: directory, installed: installed, candidate: candidate,
	})
	if target.Candidate() != candidate || target.Directory() != directory {
		t.Fatalf("TrialTarget projections = (%v, %v), want exact candidate and directory",
			target.Candidate(), target.Directory())
	}
	primary, err := newPrimary(directory, selectionDocument{
		Revision: selectionRevisionCurrent,
		Slot:     SlotA,
		Artifact: installed,
	})
	if err != nil {
		t.Fatalf("newPrimary() error = %v, want nil", err)
	}
	if primary.Directory() != directory || primary.Artifact() != installed {
		t.Fatalf("Primary projections = (%v, %v), want exact directory and artifact",
			primary.Directory(), primary.Artifact())
	}
}

type stageDownloadSourceFixture struct {
	transport  http.RoundTripper
	objectName string
}

func stageDownloadSourceForTest(
	t testing.TB,
	fixture stageDownloadSourceFixture,
) DownloadSource {
	t.Helper()

	exchangeClient, err := exchange.NewClient(&http.Client{Transport: fixture.transport})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	objectClient, err := objectstore.NewClient(exchangeClient)
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("objectstore.NewSignedHeaders(nil) error = %v, want nil", err)
	}
	expiresAt := temporal.InstantFromNanoseconds(math.MaxInt64)
	signed, err := objectstore.ParseSignedURL(
		core.SchemeHTTPS + "://" + core.GoogleCloudStorageHost + "/" + fixture.objectName +
			"?X-Goog-Signature=fixture&X-Goog-SignedHeaders=host",
	)
	if err != nil {
		t.Fatalf("objectstore.ParseSignedURL(fixture) error = %v, want nil", err)
	}
	projection, err := objectstore.NewDownloadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage,
		objectstore.DownloadTarget{URL: signed, Headers: headers, ExpiresAt: expiresAt},
	)
	if err != nil {
		t.Fatalf("objectstore.NewDownloadCapabilityProjection() error = %v, want nil", err)
	}
	document, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var capability objectstore.DownloadCapability
	if err := json.Unmarshal(document, &capability); err != nil {
		t.Fatalf("json.Unmarshal(DownloadCapability) error = %v, want nil", err)
	}
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("DownloadCapability.Commitment() error = %v, want nil", err)
	}
	operation, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(operation) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(2)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(attempt) error = %v, want nil", err)
	}
	errorLimit, err := core.NewByteCount(4 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount(error limit) error = %v, want nil", err)
	}
	return DownloadSource{
		Client: objectClient, Capability: capability, Commitment: commitment,
		Policy: objectstore.Policy{
			OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: errorLimit,
		},
	}
}

type stageDownloadTransport struct {
	cause   error
	payload []byte
	calls   atomic.Uint32
}

func (t *stageDownloadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.calls.Add(1)
	if t.cause != nil {
		return nil, t.cause
	}
	headers := make(http.Header)
	headers.Set(
		core.HTTPHeaderContentType().String(),
		core.HTTPMediaTypeOctetStream().String(),
	)
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        headers,
		Body:          io.NopCloser(bytes.NewReader(t.payload)),
		ContentLength: int64(len(t.payload)),
		Request:       request,
	}, nil
}

func stageRootForTest(t testing.TB) (*os.Root, core.AbsolutePath) {
	t.Helper()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, absolutePathForTest(t, directory)
}

type stageTargetFixture struct {
	directory core.AbsolutePath
	installed release.Artifact
	candidate release.Artifact
}

func stageTargetForTest(t testing.TB, fixture stageTargetFixture) TrialTarget {
	t.Helper()

	target, err := newTrialTarget(fixture.directory, selectionDocument{
		Revision: selectionRevisionCurrent,
		Slot:     SlotA,
		Artifact: fixture.installed,
	}, fixture.candidate)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	return target
}

type attemptFailureExpectation struct {
	identity  error
	cause     error
	candidate core.BuildIdentity
	phase     FailurePhase
}

func requireAttemptFailure(
	t testing.TB,
	got error,
	want attemptFailureExpectation,
) {
	t.Helper()

	if !errors.Is(got, want.identity) {
		t.Fatalf("attempt error = %v, want identity %v", got, want.identity)
	}
	if want.cause != nil && !errors.Is(got, want.cause) {
		t.Fatalf("attempt error = %v, want preserved cause %v", got, want.cause)
	}
	var attempt AttemptError
	if !errors.As(got, &attempt) || attempt.Validate() != nil ||
		attempt.Phase() != want.phase || attempt.Candidate() != want.candidate {
		t.Fatalf("attempt = (%v, %v, %v, %v), want valid phase %v candidate %v",
			attempt, attempt.Phase(), attempt.Candidate(), attempt.Validate(),
			want.phase, want.candidate)
	}
}
