package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/projectstandards"
)

const (
	ArtifactManifestMaximumEntries      = 256
	ArtifactChunkMaximumBytes           = 512 * 1024
	ArtifactChunkDocumentMaximumBytes   = 900 * 1024
	ArtifactChunkReceiptMaximumBytes    = 16 * 1024
	ArtifactManifestReceiptMaximumBytes = 16 * 1024
)

type ArtifactKind uint8

const (
	ArtifactKindUnknown ArtifactKind = iota
	ArtifactStdout
	ArtifactStderr
	ArtifactCoverage
	ArtifactCPUProfile
	ArtifactMemoryProfile
	ArtifactBlockProfile
	ArtifactMutexProfile
	ArtifactTrace
	ArtifactCrasher
	ArtifactReport
	ArtifactBinary
	artifactKindLimit
)

const (
	artifactStdoutToken        = "stdout"
	artifactStderrToken        = "stderr"
	artifactCoverageToken      = "coverage"
	artifactCPUProfileToken    = "cpu-profile"
	artifactMemoryProfileToken = "memory-profile"
	artifactBlockProfileToken  = "block-profile"
	artifactMutexProfileToken  = "mutex-profile"
	artifactTraceToken         = "trace"
	artifactCrasherToken       = "crasher"
	artifactReportToken        = "report"
	artifactBinaryToken        = "binary"
)

func (k ArtifactKind) Validate() error {
	if k <= ArtifactKindUnknown || k >= artifactKindLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k ArtifactKind) IsValid() bool { return k.Validate() == nil }

func (k ArtifactKind) String() string {
	names := [...]string{
		"", artifactStdoutToken, artifactStderrToken, artifactCoverageToken,
		artifactCPUProfileToken, artifactMemoryProfileToken, artifactBlockProfileToken,
		artifactMutexProfileToken, artifactTraceToken, artifactCrasherToken,
		artifactReportToken, artifactBinaryToken,
	}
	if !k.IsValid() {
		return ""
	}
	return names[k] // #nosec G602 -- IsValid proves the closed enum index is within names.
}

func (k ArtifactKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *ArtifactKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := ArtifactKindUnknown + 1; candidate < artifactKindLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type ArtifactManifestEntry struct {
	Kind       ArtifactKind                   `json:"kind"`
	Path       projectstandards.SourcePath    `json:"path"`
	MediaType  core.HTTPMediaType             `json:"media_type"`
	Digest     core.SHA256Digest              `json:"digest"`
	Bytes      core.ByteLength                `json:"bytes"`
	Experiment *projectstandards.ExperimentID `json:"experiment_id,omitempty"`
}

func (e ArtifactManifestEntry) Validate() error {
	if err := errors.Join(e.Kind.Validate(), e.Path.Validate(), e.MediaType.Validate(), e.Digest.Validate(), e.Bytes.Validate()); err != nil {
		return err
	}
	if e.Experiment != nil {
		return e.Experiment.Validate()
	}
	return nil
}

type ArtifactManifest struct {
	SchemaVersion uint16                  `json:"schema_version"`
	Run           projectstandards.RunID  `json:"run_id"`
	Fence         SchedulingFence         `json:"fence"`
	Members       MemberSet               `json:"member_set"`
	Entries       []ArtifactManifestEntry `json:"entries"`
	TotalBytes    core.ByteLength         `json:"total_bytes"`
}

func (m ArtifactManifest) Validate() error {
	if m.SchemaVersion != SchemaVersion || len(m.Entries) > ArtifactManifestMaximumEntries {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(m.Run.Validate(), m.Fence.Validate(), m.Members.Validate(), m.TotalBytes.Validate()); err != nil {
		return err
	}
	if err := validateMemberBinding(m.Members, m.Fence, m.Run); err != nil {
		return err
	}
	return m.validateEntries()
}

func (m ArtifactManifest) validateEntries() error {
	var total uint64
	previous := ""
	for index := range m.Entries {
		entry := m.Entries[index]
		if err := entry.Validate(); err != nil {
			return err
		}
		path := entry.Path.String()
		if index > 0 && previous >= path {
			return core.ErrPrimitiveContract
		}
		previous = path
		if ^uint64(0)-total < entry.Bytes.Uint64() {
			return core.ErrPrimitiveContract
		}
		total += entry.Bytes.Uint64()
	}
	if total != m.TotalBytes.Uint64() {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (m ArtifactManifest) Digest() (core.SHA256Digest, error) {
	encoded, err := m.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func (m ArtifactManifest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	digest, err := m.Digest()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	hex, err := digest.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("artifact-manifest:" + hex)
}

type artifactManifestWire ArtifactManifest

func (m ArtifactManifest) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(artifactManifestWire(m))
}

func (m *ArtifactManifest) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[artifactManifestWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ArtifactManifest(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*m = candidate
	return nil
}

type ArtifactChunk struct {
	SchemaVersion  uint16                 `json:"schema_version"`
	Run            projectstandards.RunID `json:"run_id"`
	Fence          SchedulingFence        `json:"fence"`
	Members        MemberSet              `json:"member_set"`
	ManifestDigest core.SHA256Digest      `json:"manifest_digest"`
	Entry          ArtifactManifestEntry  `json:"entry"`
	Offset         core.ByteLength        `json:"offset"`
	Data           []byte                 `json:"data"`
	Final          bool                   `json:"final"`
}

func (c ArtifactChunk) Validate() error {
	if c.SchemaVersion != SchemaVersion || len(c.Data) == 0 || len(c.Data) > ArtifactChunkMaximumBytes {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(c.Run.Validate(), c.Fence.Validate(), c.Members.Validate(), c.ManifestDigest.Validate(), c.Entry.Validate(), c.Offset.Validate()); err != nil {
		return err
	}
	if err := validateMemberBinding(c.Members, c.Fence, c.Run); err != nil {
		return err
	}
	return c.validateExtent()
}

func (c ArtifactChunk) validateExtent() error {
	end := c.Offset.Uint64() + uint64(len(c.Data))
	if end < c.Offset.Uint64() || end > c.Entry.Bytes.Uint64() || c.Final != (end == c.Entry.Bytes.Uint64()) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateMemberBinding(members MemberSet, fence SchedulingFence, run projectstandards.RunID) error {
	digest, err := members.Digest()
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if digest != fence.MemberSetDigest || !members.Contains(run) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (c ArtifactChunk) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := c.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	manifest, err := c.ManifestDigest.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	entry, err := c.Entry.Digest.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey(fmt.Sprintf("artifact:%s:%s:%d", manifest, entry, c.Offset.Uint64()))
}

type artifactChunkWire ArtifactChunk

func (c ArtifactChunk) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(artifactChunkWire(c))
	if err != nil || len(encoded) > ArtifactChunkDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (c *ArtifactChunk) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[artifactChunkWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ArtifactChunk(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = candidate
	return nil
}

type ArtifactChunkReceipt struct {
	SchemaVersion uint16                 `json:"schema_version"`
	Run           projectstandards.RunID `json:"run_id"`
	Manifest      core.SHA256Digest      `json:"manifest_digest"`
	Artifact      core.SHA256Digest      `json:"artifact_digest"`
	Committed     core.ByteLength        `json:"committed_bytes"`
	Complete      bool                   `json:"complete"`
}

type ArtifactManifestReceipt struct {
	SchemaVersion uint16                 `json:"schema_version"`
	Run           projectstandards.RunID `json:"run_id"`
	Digest        core.SHA256Digest      `json:"manifest_digest"`
	Bytes         core.ByteLength        `json:"manifest_bytes"`
}

func (r ArtifactManifestReceipt) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Run.Validate(), r.Digest.Validate(), r.Bytes.Validate())
}

type artifactManifestReceiptWire ArtifactManifestReceipt

func (r ArtifactManifestReceipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(artifactManifestReceiptWire(r))
}

func (r *ArtifactManifestReceipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[artifactManifestReceiptWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ArtifactManifestReceipt(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type ArtifactManifestRecord struct {
	Manifest  ArtifactManifest
	Canonical []byte
	Digest    core.SHA256Digest
	Bytes     core.ByteLength
}

func NewArtifactManifestRecord(manifest ArtifactManifest) (ArtifactManifestRecord, error) {
	canonical, err := manifest.MarshalJSON()
	if err != nil {
		return ArtifactManifestRecord{}, err
	}
	extent, err := core.NewByteLength(uint64(len(canonical)))
	if err != nil {
		return ArtifactManifestRecord{}, err
	}
	record := ArtifactManifestRecord{Manifest: manifest, Canonical: canonical, Digest: core.SHA256Of(canonical), Bytes: extent}
	return record, record.Validate()
}

func (r ArtifactManifestRecord) Validate() error {
	if err := errors.Join(r.Manifest.Validate(), r.Digest.Validate(), r.Bytes.Validate()); err != nil {
		return err
	}
	canonical, err := r.Manifest.MarshalJSON()
	if err != nil || string(canonical) != string(r.Canonical) || r.Digest != core.SHA256Of(r.Canonical) || r.Bytes.Uint64() != uint64(len(r.Canonical)) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type ArtifactManifestRepository interface {
	StoreArtifactManifest(context.Context, ArtifactManifestRecord) error
}

type ArtifactManifestClient struct{ socket exchange.ClientSocket }

func NewArtifactManifestClient(configuration exchange.ClientSocketConfiguration) (ArtifactManifestClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return ArtifactManifestClient{}, err
	}
	return ArtifactManifestClient{socket: socket}, nil
}

func (c ArtifactManifestClient) Submit(ctx context.Context, manifest ArtifactManifest) (exchange.JSONResponse[ArtifactManifestReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[ArtifactManifest, ArtifactManifestReceipt](ctx, c.socket, manifest)
}

type ArtifactManifestServer struct {
	socket     exchange.ServerSocket
	repository ArtifactManifestRepository
}

func NewArtifactManifestServer(contract exchange.JSONSocketContract, repository ArtifactManifestRepository) (ArtifactManifestServer, error) {
	if repository == nil {
		return ArtifactManifestServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return ArtifactManifestServer{}, err
	}
	return ArtifactManifestServer{socket: socket, repository: repository}, nil
}

func (s ArtifactManifestServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[ArtifactManifest, *ArtifactManifest](s.socket, request)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(request.Context(), received.Body.Fence.Machine.Machine, received.Body.Fence.Machine.Generation); err != nil {
		return err
	}
	record, err := NewArtifactManifestRecord(*received.Body)
	if err != nil {
		return err
	}
	if err := s.repository.StoreArtifactManifest(request.Context(), record); err != nil {
		return err
	}
	return exchange.WriteSocketJSON(s.socket, writer, ArtifactManifestReceipt{SchemaVersion: SchemaVersion, Run: record.Manifest.Run, Digest: record.Digest, Bytes: record.Bytes})
}

func ArtifactManifestSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(core.JSONDocumentMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(ArtifactManifestReceiptMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK()}
	return contract, contract.Validate()
}

func (r ArtifactChunkReceipt) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Run.Validate(), r.Manifest.Validate(), r.Artifact.Validate(), r.Committed.Validate())
}

type artifactChunkReceiptWire ArtifactChunkReceipt

func (r ArtifactChunkReceipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(artifactChunkReceiptWire(r))
}
func (r *ArtifactChunkReceipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[artifactChunkReceiptWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ArtifactChunkReceipt(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type ArtifactChunkRepository interface {
	StoreArtifactChunk(context.Context, ArtifactChunk) (ArtifactChunkReceipt, error)
}

type ArtifactClient struct{ socket exchange.ClientSocket }

func NewArtifactClient(configuration exchange.ClientSocketConfiguration) (ArtifactClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return ArtifactClient{}, err
	}
	return ArtifactClient{socket: socket}, nil
}
func (c ArtifactClient) Submit(ctx context.Context, chunk ArtifactChunk) (exchange.JSONResponse[ArtifactChunkReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[ArtifactChunk, ArtifactChunkReceipt](ctx, c.socket, chunk)
}

type ArtifactServer struct {
	socket     exchange.ServerSocket
	repository ArtifactChunkRepository
}

func NewArtifactServer(contract exchange.JSONSocketContract, repository ArtifactChunkRepository) (ArtifactServer, error) {
	if repository == nil {
		return ArtifactServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return ArtifactServer{}, err
	}
	return ArtifactServer{socket: socket, repository: repository}, nil
}
func (s ArtifactServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[ArtifactChunk, *ArtifactChunk](s.socket, request)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(request.Context(), received.Body.Fence.Machine.Machine, received.Body.Fence.Machine.Generation); err != nil {
		return err
	}
	receipt, err := s.repository.StoreArtifactChunk(request.Context(), *received.Body)
	if err != nil {
		return err
	}
	if receipt.Run != received.Body.Run || receipt.Manifest != received.Body.ManifestDigest || receipt.Artifact != received.Body.Entry.Digest || receipt.Committed.Uint64() != received.Body.Offset.Uint64()+uint64(len(received.Body.Data)) || receipt.Complete != received.Body.Final {
		return core.ErrPrimitiveContract
	}
	return exchange.WriteSocketJSON(s.socket, writer, receipt)
}
func ArtifactSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(ArtifactChunkDocumentMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(ArtifactChunkReceiptMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK()}
	return contract, contract.Validate()
}

func ArtifactChunkMatchesEntry(chunk ArtifactChunk, entry ArtifactManifestEntry) bool {
	if chunk.Entry.Path != entry.Path || chunk.Entry.Kind != entry.Kind || chunk.Entry.MediaType != entry.MediaType || chunk.Entry.Digest != entry.Digest || chunk.Entry.Bytes != entry.Bytes || !equalOptionalExperiment(chunk.Entry.Experiment, entry.Experiment) {
		return false
	}
	if chunk.Offset.Uint64() == 0 && chunk.Final {
		return uint64(len(chunk.Data)) == entry.Bytes.Uint64() && core.SHA256Of(chunk.Data) == entry.Digest
	}
	return true
}
func equalOptionalExperiment(left, right *projectstandards.ExperimentID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

var (
	_ core.Validatable            = ArtifactKindUnknown
	_ json.Unmarshaler            = (*ArtifactKind)(nil)
	_ core.Validatable            = ArtifactManifestEntry{}
	_ core.ValidatedJSONMarshaler = ArtifactManifest{}
	_ json.Unmarshaler            = (*ArtifactManifest)(nil)
	_ exchange.IdempotencyBound   = ArtifactManifest{}
	_ core.ValidatedJSONMarshaler = ArtifactChunk{}
	_ json.Unmarshaler            = (*ArtifactChunk)(nil)
	_ exchange.IdempotencyBound   = ArtifactChunk{}
	_ core.ValidatedJSONMarshaler = ArtifactChunkReceipt{}
	_ json.Unmarshaler            = (*ArtifactChunkReceipt)(nil)
	_ core.ValidatedJSONMarshaler = ArtifactManifestReceipt{}
	_ json.Unmarshaler            = (*ArtifactManifestReceipt)(nil)
	_ core.Validatable            = ArtifactManifestRecord{}
)
