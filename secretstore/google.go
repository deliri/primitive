package secretstore

import (
	"context"
	"errors"
	"hash/crc32"
	"math"
	"strconv"
	"strings"
	"sync"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

var googleCRC32CTable = crc32.MakeTable(crc32.Castagnoli)

const (
	// GoogleAccessResponseEnvelopeMaximumBytes is Primitive's bounded allowance
	// for the protobuf resource name, checksum, field framing, and transport
	// evolution around Google's published 64-KiB secret payload maximum.
	GoogleAccessResponseEnvelopeMaximumBytes = 4 * 1024
	// GoogleAccessResponseMaximumBytes bounds decoding before the official SDK
	// can allocate a complete AccessSecretVersion response.
	GoogleAccessResponseMaximumBytes = PayloadMaximumBytes + GoogleAccessResponseEnvelopeMaximumBytes
)

// GoogleReader is a bounded authenticated capability over the official Google
// Cloud Secret Manager SDK.
type GoogleReader struct {
	client *secretmanager.Client
	mu     sync.RWMutex
}

// NewGoogleReader constructs one official-SDK client using ambient application
// default credentials.
func NewGoogleReader(ctx context.Context) (*GoogleReader, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, errors.Join(core.ErrSecretStoreContract, err)
	}
	client, err := secretmanager.NewClient(ctx, option.WithGRPCDialOption(
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(GoogleAccessResponseMaximumBytes)),
	))
	if err != nil {
		return nil, googleAccessError(err)
	}
	return &GoogleReader{client: client}, nil
}

// Validate rejects an unconstructed or closed reader.
func (r *GoogleReader) Validate() error {
	if r == nil {
		return core.ErrSecretStoreContract
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.client == nil {
		return core.ErrSecretStoreContract
	}
	return nil
}

// Access reads one exact reference and verifies the provider checksum before
// releasing bounded destroyable material.
func (r *GoogleReader) Access(ctx context.Context, request AccessRequest) (AccessResult, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return AccessResult{}, errors.Join(core.ErrSecretStoreContract, err)
	}
	name, err := request.resourceName()
	if err != nil {
		return AccessResult{}, err
	}
	if r == nil {
		return AccessResult{}, core.ErrSecretStoreContract
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.client == nil {
		return AccessResult{}, core.ErrSecretStoreContract
	}
	response, err := r.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		return AccessResult{}, googleAccessError(err)
	}
	return accessResultFromGoogleResponse(request, response)
}

// Close releases the official SDK client's resources exactly once.
func (r *GoogleReader) Close() error {
	if r == nil {
		return core.ErrSecretStoreContract
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client == nil {
		return core.ErrSecretStoreContract
	}
	err := r.client.Close()
	r.client = nil
	if err != nil {
		return googleAccessError(err)
	}
	return nil
}

func accessResultFromGoogleResponse(request AccessRequest, response *secretmanagerpb.AccessSecretVersionResponse) (AccessResult, error) {
	if err := request.Validate(); err != nil {
		return AccessResult{}, err
	}
	if response == nil || response.Payload == nil {
		return AccessResult{}, payloadError("Google Secret Manager response payload is absent")
	}
	payload := response.Payload.Data
	defer clear(payload)
	resolved, err := parseResolvedReference(response.Name)
	if err != nil {
		return AccessResult{}, err
	}
	if !resolved.matches(request) {
		return AccessResult{}, payloadError("Google Secret Manager response does not match the request")
	}
	if err := validateGooglePayload(payload, response.Payload.DataCrc32C); err != nil {
		return AccessResult{}, err
	}
	return newAccessResult(request, resolved, payload)
}

func validateGooglePayload(payload []byte, checksum *int64) error {
	if len(payload) > PayloadMaximumBytes || checksum == nil {
		return payloadError("Google Secret Manager response payload contract is invalid")
	}
	value := *checksum
	if value < 0 || value > math.MaxUint32 || uint32(value) != crc32.Checksum(payload, googleCRC32CTable) {
		return payloadError("Google Secret Manager response checksum is invalid")
	}
	return nil
}

func newAccessResult(request AccessRequest, resolved ResolvedReference, payload []byte) (AccessResult, error) {
	value, err := NewValue(payload)
	if err != nil {
		return AccessResult{}, err
	}
	result := AccessResult{Request: request, Reference: resolved, Value: value}
	if err := result.Validate(); err != nil {
		_ = value.Destroy()
		return AccessResult{}, err
	}
	return result, nil
}

func parseResolvedReference(name string) (ResolvedReference, error) {
	if len(name) == 0 || len(name) > googleResolvedNameMaximumBytes {
		return ResolvedReference{}, payloadError("Google Secret Manager resolved name extent is invalid")
	}
	remainder, ok := strings.CutPrefix(name, googleProjectResourcePrefix)
	if !ok {
		return ResolvedReference{}, payloadError("Google Secret Manager resolved name prefix is invalid")
	}
	projectText, remainder, ok := strings.Cut(remainder, googleSecretResourceSegment)
	if !ok || !decimalDigitsOnly(projectText) {
		return ResolvedReference{}, payloadError("Google Secret Manager resolved project segment is invalid")
	}
	secretText, versionText, ok := strings.Cut(remainder, googleVersionResourceSegment)
	if !ok || !decimalDigitsOnly(versionText) {
		return ResolvedReference{}, payloadError("Google Secret Manager resolved version segment is invalid")
	}
	projectValue, projectParseErr := strconv.ParseUint(projectText, 10, 64)
	project, projectErr := NewGoogleProjectNumber(projectValue)
	secret, secretErr := ParseGoogleSecretID(secretText)
	versionValue, versionParseErr := strconv.ParseUint(versionText, 10, 64)
	version, versionErr := NewGoogleVersionNumber(versionValue)
	resolved := ResolvedReference{ProjectNumber: project, Secret: secret, Version: version}
	if err := errors.Join(projectParseErr, projectErr, secretErr, versionParseErr, versionErr, resolved.Validate()); err != nil {
		return ResolvedReference{}, errors.Join(core.ErrSecretStorePayload, err)
	}
	return resolved, nil
}

func decimalDigitsOnly(value string) bool {
	if value == "" || len(value) > 1 && value[0] == '0' {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func payloadError(detail string) error {
	return errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract, errors.New(detail))
}

func googleAccessError(err error) error {
	return errors.Join(core.ErrSecretStoreAccess, err)
}
