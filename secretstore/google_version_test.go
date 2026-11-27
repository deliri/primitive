package secretstore

import (
	"bytes"
	"context"
	"errors"
	"hash/crc32"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const syntheticVersionData = "synthetic replacement grant"

func localAddVersionRequest(t testing.TB) AddVersionRequest {
	t.Helper()
	project, projectErr := NewGoogleProjectNumber(1122334455)
	secret, secretErr := ParseGoogleSecretID("fixture_refresh_secret")
	value, valueErr := NewValue([]byte(syntheticVersionData))
	if err := errors.Join(projectErr, secretErr, valueErr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := value.Destroy(); err != nil {
			t.Error(err)
		}
	})
	return AddVersionRequest{Project: project, Secret: secret, Value: value}
}

func enabledVersionFixture() *secretmanagerpb.SecretVersion {
	return &secretmanagerpb.SecretVersion{
		Name:                           "projects/1122334455/secrets/fixture_refresh_secret/versions/7",
		State:                          secretmanagerpb.SecretVersion_ENABLED,
		ClientSpecifiedPayloadChecksum: true,
	}
}

func TestGoogleVersionWriterNativeAddOnce(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		changeResult func(*secretmanagerpb.SecretVersion)
		transport    codes.Code
		want         error
	}{
		{name: "exact enabled version"},
		{name: "wrong secret", changeResult: func(r *secretmanagerpb.SecretVersion) {
			r.Name = "projects/1122334455/secrets/other/versions/7"
		}, want: core.ErrSecretStorePayload},
		{name: "wrong project", changeResult: func(r *secretmanagerpb.SecretVersion) {
			r.Name = "projects/998877/secrets/fixture_refresh_secret/versions/7"
		}, want: core.ErrSecretStorePayload},
		{name: "not numeric", changeResult: func(r *secretmanagerpb.SecretVersion) {
			r.Name = "projects/1122334455/secrets/fixture_refresh_secret/versions/latest"
		}, want: core.ErrSecretStorePayload},
		{name: "checksum unacknowledged", changeResult: func(r *secretmanagerpb.SecretVersion) {
			r.ClientSpecifiedPayloadChecksum = false
		}, want: core.ErrSecretStorePayload},
		{name: "bad version state", changeResult: func(r *secretmanagerpb.SecretVersion) {
			r.State = secretmanagerpb.SecretVersion_DISABLED
		}, want: core.ErrSecretStorePayload},
		{name: "transport lost reply", transport: codes.Unavailable, want: core.ErrSecretStoreAccess},
		{name: "scoped permission refused", transport: codes.PermissionDenied, want: core.ErrSecretStoreAccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := localAddVersionRequest(t)
			result := enabledVersionFixture()
			if tc.changeResult != nil {
				tc.changeResult(result)
			}
			var calls atomic.Int32
			conn := localSecretConnection(t, &localSecretServer{add: func(_ context.Context, wire *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
				calls.Add(1)
				if wire.GetParent() != "projects/1122334455/secrets/fixture_refresh_secret" {
					t.Error("secret scope changed at the provider boundary")
				}
				payload := wire.GetPayload()
				defer clear(payload.Data)
				wantCRC := int64(crc32.Checksum([]byte(syntheticVersionData), crc32.MakeTable(crc32.Castagnoli)))
				if !bytes.Equal(payload.GetData(), []byte(syntheticVersionData)) || payload.GetDataCrc32C() != wantCRC {
					t.Error("provider received a different grant or checksum")
				}
				if tc.transport != codes.OK {
					return nil, status.Error(tc.transport, "private-provider-marker")
				}
				return result, nil
			}})
			writer, err := newGoogleVersionWriter(t.Context(), option.WithGRPCConn(conn))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := writer.Close(); err != nil {
					t.Error(err)
				}
			})
			refused := request
			refused.Project = 0
			reject, err := writer.Execute(t.Context(), refused)
			if !errors.Is(err, core.ErrSecretStoreContract) || reject != (ResolvedReference{}) || calls.Load() != 0 {
				t.Fatal("validation failure performed provider work")
			}
			got, err := writer.Execute(t.Context(), request)
			if !errors.Is(err, tc.want) || calls.Load() != 1 {
				t.Fatalf("add outcome = %v, %v, %d calls, want %v after exactly 1 call", got, err, calls.Load(), tc.want)
			}
			if tc.want != nil {
				if got != (ResolvedReference{}) || strings.Contains(err.Error(), "private-provider-marker") {
					t.Fatal("failed secret write disclosed native provider data")
				}
				return
			}
			if got.ProjectNumber != request.Project || got.Secret != request.Secret || got.Version.Uint64() != 7 || got.Validate() != nil {
				t.Fatal("version receipt did not retain exact authoritative resource identity")
			}
		})
	}
}

func FuzzSecretVersionAddResponse(f *testing.F) {
	resource := localAddVersionRequest(f)
	fixture := enabledVersionFixture()
	for _, state := range []secretmanagerpb.SecretVersion_State{secretmanagerpb.SecretVersion_ENABLED, secretmanagerpb.SecretVersion_DISABLED, secretmanagerpb.SecretVersion_DESTROYED} {
		fixture.State = state
		raw, err := proto.Marshal(fixture)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw)
	}
	f.Add([]byte{0x0a, 0xff})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > googleVersionAddResponseMaximumBytes {
			return
		}
		var wire secretmanagerpb.SecretVersion
		if err := proto.Unmarshal(raw, &wire); err != nil {
			return
		}
		got, err := projectAddedVersion(resource.Project, resource.Secret, &wire)
		if err != nil {
			if got != (ResolvedReference{}) || !errors.Is(err, core.ErrSecretStorePayload) {
				t.Fatal("invalid provider reply escaped as a partial receipt")
			}
			return
		}
		const prefix = "projects/1122334455/secrets/fixture_refresh_secret/versions/"
		tail, ok := strings.CutPrefix(wire.GetName(), prefix)
		number, parseErr := strconv.ParseUint(tail, 10, 64)
		if !ok || parseErr != nil || number == 0 || strconv.FormatUint(number, 10) != tail ||
			got.Validate() != nil || got.ProjectNumber != resource.Project || got.Secret != resource.Secret ||
			got.Version.Uint64() != number || wire.GetState() != secretmanagerpb.SecretVersion_ENABLED ||
			!wire.GetClientSpecifiedPayloadChecksum() {
			t.Fatal("version receipt admitted without authoritative metadata and checksum")
		}
	})
}
