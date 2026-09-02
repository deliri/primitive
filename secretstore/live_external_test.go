//go:build integration

package secretstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/secretstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	liveProjectEnvironment = "PRIMITIVE_SECRETSTORE_LIVE_PROJECT"
	liveSecretEnvironment  = "PRIMITIVE_SECRETSTORE_LIVE_SECRET"
)

func TestGoogleReaderUsesRealSecretManager(t *testing.T) {
	t.Parallel()

	projectText := liveEnvironmentValue(t, liveProjectEnvironment)
	project, err := secretstore.ParseGoogleProjectID(projectText)
	if err != nil {
		t.Fatalf("secretstore.ParseGoogleProjectID(%s) error = %v, want nil", liveProjectEnvironment, err)
	}
	secretText := liveEnvironmentValue(t, liveSecretEnvironment)
	secret, err := secretstore.ParseGoogleSecretID(secretText)
	if err != nil {
		t.Fatalf("secretstore.ParseGoogleSecretID(%s) error = %v, want nil", liveSecretEnvironment, err)
	}
	reader, err := secretstore.NewGoogleReader(context.Background())
	if err != nil {
		t.Fatalf("secretstore.NewGoogleReader() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("GoogleReader.Close() error = %v, want nil", closeErr)
		}
	})
	request := secretstore.AccessRequest{
		Project: project, Secret: secret, Version: secretstore.GoogleVersionSelectorLatest,
	}
	result, err := reader.Access(context.Background(), request)
	if err != nil || result.Validate() != nil || result.Request != request ||
		result.Reference.ProjectNumber.Uint64() == 0 || result.Reference.Version.Uint64() == 0 ||
		result.Reference.Secret != secret {
		t.Fatalf("GoogleReader.Access(real existing version) = (%v, %v), want exact validated result", result, err)
	}
	if err := result.Value.Destroy(); err != nil {
		t.Fatalf("Value.Destroy() error = %v, want nil", err)
	}

	absent, err := secretstore.ParseGoogleSecretID("primitive-integration-intentionally-absent")
	if err != nil {
		t.Fatalf("secretstore.ParseGoogleSecretID(absent) error = %v, want nil", err)
	}
	missing, missingErr := reader.Access(context.Background(), secretstore.AccessRequest{
		Project: project, Secret: absent, Version: secretstore.GoogleVersionSelectorLatest,
	})
	if missing != (secretstore.AccessResult{}) || !errors.Is(missingErr, core.ErrSecretStoreAccess) || status.Code(missingErr) != codes.NotFound {
		t.Fatalf("GoogleReader.Access(real absent version) = (%v, %v), want zero, access identity, and provider NotFound", missing, missingErr)
	}
}

func liveEnvironmentValue(t *testing.T, value string) string {
	t.Helper()

	name, err := process.NewEnvironmentName(value)
	if err != nil {
		t.Fatalf("process.NewEnvironmentName(%q) error = %v, want nil", value, err)
	}
	lookup, err := hostfacts.LookupAmbientEnvironment(name)
	if err != nil {
		t.Fatalf("hostfacts.LookupAmbientEnvironment(%q) error = %v, want nil", value, err)
	}
	if lookup.Presence != process.EnvironmentPresencePresent {
		t.Fatalf("hostfacts.LookupAmbientEnvironment(%q).Presence = %v, want %v", value, lookup.Presence, process.EnvironmentPresencePresent)
	}
	result, err := lookup.Value.Value()
	if err != nil {
		t.Fatalf("EnvironmentValue.Value(%q) error = %v, want nil", value, err)
	}
	return result
}
