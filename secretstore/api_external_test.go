package secretstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/secretstore"
)

func TestGoogleReaderPublicRefusalContract(t *testing.T) {
	t.Parallel()

	project, err := secretstore.ParseGoogleProjectID("project1")
	if err != nil {
		t.Fatalf("secretstore.ParseGoogleProjectID() error = %v, want nil", err)
	}
	secret, err := secretstore.ParseGoogleSecretID("runtime-secret")
	if err != nil {
		t.Fatalf("secretstore.ParseGoogleSecretID() error = %v, want nil", err)
	}
	request := secretstore.AccessRequest{
		Project: project,
		Secret:  secret,
		Version: secretstore.GoogleVersionSelectorLatest,
	}
	var nilReader *secretstore.GoogleReader
	tests := []struct {
		wantErr error
		run     func(*testing.T) error
		name    string
	}{
		{
			name: "constructor refuses nil context before provider access",
			run: func(t *testing.T) error {
				//lint:ignore SA1012 the nil context is exactly the rejected public ingress under test
				got, gotErr := secretstore.NewGoogleReader(nil)
				if got != nil {
					t.Fatalf("secretstore.NewGoogleReader(nil) reader = %v, want nil", got)
				}
				return gotErr
			},
			wantErr: core.ErrSecretStoreContract,
		},
		{name: "validation refuses nil reader", run: func(*testing.T) error { return nilReader.Validate() }, wantErr: core.ErrSecretStoreContract},
		{
			name: "access refuses nil reader with zero value",
			run: func(t *testing.T) error {
				got, gotErr := nilReader.Access(context.Background(), request)
				if got != (secretstore.AccessResult{}) {
					t.Fatalf("nil GoogleReader.Access() result = %v, want zero", got)
				}
				return gotErr
			},
			wantErr: core.ErrSecretStoreContract,
		},
		{name: "close refuses nil reader", run: func(*testing.T) error { return nilReader.Close() }, wantErr: core.ErrSecretStoreContract},
		{
			name: "off-wire version witness remains reachable",
			run: func(*testing.T) error {
				secretstore.GoogleVersionSelectorLatest.OffWireEnum()
				return secretstore.GoogleVersionSelectorLatest.Validate()
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotErr := testCase.run(t)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("public refusal error = %v, want errors.Is(..., %v)", gotErr, testCase.wantErr)
			}
		})
	}
}
