package secretstore

import (
	"context"
	"errors"
	"hash/crc32"
	"log/slog"
	"strconv"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

const googleVersionAddResponseMaximumBytes = 16 << 10

// AddVersionRequest names one existing secret by its configured project number
// and secret ID. The caller retains ownership of the confidential Value.
type AddVersionRequest struct {
	Project GoogleProjectNumber
	Secret  GoogleSecretID
	Value   Value
}

func (r AddVersionRequest) Validate() error {
	return errors.Join(r.Project.Validate(), r.Secret.Validate(), r.Value.Validate())
}

// GoogleVersionWriter is a typed AddSecretVersion capability. It never reads a
// secret version, changes a secret's lifecycle, or returns payload material.
type GoogleVersionWriter struct{ transport *GoogleReader }

func NewGoogleVersionWriter(ctx context.Context) (*GoogleVersionWriter, error) {
	return newGoogleVersionWriter(ctx)
}

func newGoogleVersionWriter(ctx context.Context, options ...option.ClientOption) (*GoogleVersionWriter, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, errors.Join(core.ErrSecretStoreContract, err)
	}
	options = append(options,
		option.WithLogger(slog.New(slog.DiscardHandler)),
		option.WithGRPCDialOption(grpc.WithDisableRetry()),
		option.WithGRPCDialOption(grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(googleVersionAddResponseMaximumBytes))),
	)
	transport, err := newGoogleReader(ctx, options...)
	if err != nil {
		return nil, core.ErrSecretStoreAccess
	}
	return &GoogleVersionWriter{transport: transport}, nil
}

func (w *GoogleVersionWriter) Validate() error {
	if w == nil {
		return core.ErrSecretStoreContract
	}
	return w.transport.Validate()
}

func (w *GoogleVersionWriter) Close() error {
	if w == nil {
		return core.ErrSecretStoreContract
	}
	return w.transport.Close()
}

// Execute creates one immutable, checksummed version. The underlying SDK
// AddSecretVersion call has a deadline and no default operation retry.
// https://cloud.google.com/secret-manager/docs/reference/rest/v1/projects.secrets/addVersion
// https://cloud.google.com/secret-manager/docs/data-integrity
func (w *GoogleVersionWriter) Execute(ctx context.Context, r AddVersionRequest) (ResolvedReference, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate(), w.Validate()); err != nil {
		return ResolvedReference{}, errors.Join(core.ErrSecretStoreContract, err)
	}
	data, err := r.Value.CopyBytes()
	if err != nil {
		return ResolvedReference{}, err
	}
	defer clear(data)
	parent := googleProjectResourcePrefix + strconv.FormatUint(r.Project.Uint64(), 10) + googleSecretResourceSegment + r.Secret.String()
	checksum := int64(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
	transport := w.transport
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	if transport.client == nil {
		return ResolvedReference{}, core.ErrSecretStoreContract
	}
	result, err := transport.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent, Payload: &secretmanagerpb.SecretPayload{Data: data, DataCrc32C: &checksum},
	})
	if err != nil {
		return ResolvedReference{}, errors.Join(core.ErrSecretStoreAccess, ctx.Err())
	}
	return projectAddedVersion(r.Project, r.Secret, result)
}

func projectAddedVersion(project GoogleProjectNumber, secret GoogleSecretID, version *secretmanagerpb.SecretVersion) (ResolvedReference, error) {
	if version == nil || version.GetState() != secretmanagerpb.SecretVersion_ENABLED || !version.GetClientSpecifiedPayloadChecksum() {
		return ResolvedReference{}, core.ErrSecretStorePayload
	}
	resolved, err := parseResolvedReference(version.GetName())
	if err != nil {
		return ResolvedReference{}, err
	}
	if resolved.ProjectNumber != project || resolved.Secret != secret {
		return ResolvedReference{}, core.ErrSecretStorePayload
	}
	return resolved, nil
}
