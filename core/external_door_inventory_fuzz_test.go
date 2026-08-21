package core

import (
	"bytes"
	"crypto/ed25519"
	"encoding"
	"encoding/hex"
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"hash/crc32"
	"os"
	"slices"
	"strings"
	"testing"
)

type coreJSONDoor uint8

const (
	coreJSONDoorUnknown coreJSONDoor = iota
	coreJSONDoorPlatform
	coreJSONDoorOperatingSystem
	coreJSONDoorCPUArchitecture
	coreJSONDoorOffering
	coreJSONDoorReleaseVersion
	coreJSONDoorBuildCommit
	coreJSONDoorBuildIdentity
	coreJSONDoorCatalogPageLimit
	coreJSONDoorCatalogSelectionKind
	coreJSONDoorCatalogPositionKind
	coreJSONDoorCatalogContinuationState
	coreJSONDoorErrorIdentity
	coreJSONDoorHTTPEndpoint
	coreJSONDoorPackageIdentity
	coreJSONDoorPackageKind
	coreJSONDoorHTTPStatusCode
	coreJSONDoorHTTPHeaderName
	coreJSONDoorHTTPMediaType
	coreJSONDoorSHA256Digest
	coreJSONDoorCRC32C
	coreJSONDoorEd25519PublicKey
	coreJSONDoorByteCount
	coreJSONDoorByteLength
	coreJSONDoorPathComponent
	coreJSONDoorAbsolutePath
	coreJSONDoorLimit
)

func (d coreJSONDoor) receiverName() string {
	switch d {
	case coreJSONDoorPlatform:
		return "Platform"
	case coreJSONDoorOperatingSystem:
		return "OperatingSystem"
	case coreJSONDoorCPUArchitecture:
		return "CPUArchitecture"
	case coreJSONDoorOffering:
		return "Offering"
	case coreJSONDoorReleaseVersion:
		return "ReleaseVersion"
	case coreJSONDoorBuildCommit:
		return "BuildCommit"
	case coreJSONDoorBuildIdentity:
		return "BuildIdentity"
	case coreJSONDoorCatalogPageLimit:
		return "CatalogPageLimit"
	case coreJSONDoorCatalogSelectionKind:
		return "CatalogSelectionKind"
	case coreJSONDoorCatalogPositionKind:
		return "CatalogPositionKind"
	case coreJSONDoorCatalogContinuationState:
		return "CatalogContinuationState"
	case coreJSONDoorErrorIdentity:
		return "ErrorIdentity"
	case coreJSONDoorHTTPEndpoint:
		return "HTTPEndpoint"
	case coreJSONDoorPackageIdentity:
		return "PackageIdentity"
	case coreJSONDoorPackageKind:
		return "PackageKind"
	case coreJSONDoorHTTPStatusCode:
		return "HTTPStatusCode"
	case coreJSONDoorHTTPHeaderName:
		return "HTTPHeaderName"
	case coreJSONDoorHTTPMediaType:
		return "HTTPMediaType"
	case coreJSONDoorSHA256Digest:
		return "SHA256Digest"
	case coreJSONDoorCRC32C:
		return "CRC32C"
	case coreJSONDoorEd25519PublicKey:
		return "Ed25519PublicKey"
	case coreJSONDoorByteCount:
		return "ByteCount"
	case coreJSONDoorByteLength:
		return "ByteLength"
	case coreJSONDoorPathComponent:
		return "PathComponent"
	case coreJSONDoorAbsolutePath:
		return "AbsolutePath"
	case coreJSONDoorUnknown, coreJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type coreJSONFixtures struct {
	relativePath    RelativePath
	absolutePath    AbsolutePath
	component       PathComponent
	mediaType       HTTPMediaType
	header          HTTPHeaderName
	offering        Offering
	endpoint        HTTPEndpoint
	build           BuildIdentity
	byteLength      ByteLength
	byteCount       ByteCount
	version         ReleaseVersion
	crc32c          CRC32C
	status          HTTPStatusCode
	pageLimit       CatalogPageLimit
	errorIdentity   ErrorIdentity
	sha256          SHA256Digest
	commit          BuildCommit
	publicKey       Ed25519PublicKey
	platform        Platform
	packageKind     PackageKind
	packageIdentity PackageIdentity
	selection       CatalogSelectionKind
	continuation    CatalogContinuationState
	position        CatalogPositionKind
	architecture    CPUArchitecture
	operatingSystem OperatingSystem
}

type coreJSONSeed struct {
	document []byte
	door     coreJSONDoor
}

func FuzzCoreExternalJSONDoorInventory(f *testing.F) {
	fixtures := coreFixturesForFuzz(f)
	for _, seed := range coreJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(coreJSONDoorBuildIdentity), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch coreJSONDoor(rawDoor) {
		case coreJSONDoorPlatform:
			fuzzCoreJSONValue(t, data, fixtures.platform)
		case coreJSONDoorOperatingSystem:
			fuzzCoreJSONValue(t, data, fixtures.operatingSystem)
		case coreJSONDoorCPUArchitecture:
			fuzzCoreJSONValue(t, data, fixtures.architecture)
		case coreJSONDoorOffering:
			fuzzCoreJSONValue(t, data, fixtures.offering)
		case coreJSONDoorReleaseVersion:
			fuzzCoreJSONValue(t, data, fixtures.version)
		case coreJSONDoorBuildCommit:
			fuzzCoreJSONValue(t, data, fixtures.commit)
		case coreJSONDoorBuildIdentity:
			fuzzCoreJSONValue(t, data, fixtures.build)
		case coreJSONDoorCatalogPageLimit:
			fuzzCoreJSONValue(t, data, fixtures.pageLimit)
		case coreJSONDoorCatalogSelectionKind:
			fuzzCoreJSONValue(t, data, fixtures.selection)
		case coreJSONDoorCatalogPositionKind:
			fuzzCoreJSONValue(t, data, fixtures.position)
		case coreJSONDoorCatalogContinuationState:
			fuzzCoreJSONValue(t, data, fixtures.continuation)
		case coreJSONDoorErrorIdentity:
			fuzzCoreJSONValue(t, data, fixtures.errorIdentity)
		case coreJSONDoorHTTPEndpoint:
			fuzzCoreJSONValue(t, data, fixtures.endpoint)
		case coreJSONDoorPackageIdentity:
			fuzzCoreJSONValue(t, data, fixtures.packageIdentity)
		case coreJSONDoorPackageKind:
			fuzzCoreJSONValue(t, data, fixtures.packageKind)
		case coreJSONDoorHTTPStatusCode:
			fuzzCoreJSONValue(t, data, fixtures.status)
		case coreJSONDoorHTTPHeaderName:
			fuzzCoreJSONValue(t, data, fixtures.header)
		case coreJSONDoorHTTPMediaType:
			fuzzCoreJSONValue(t, data, fixtures.mediaType)
		case coreJSONDoorSHA256Digest:
			fuzzCoreJSONValue(t, data, fixtures.sha256)
		case coreJSONDoorCRC32C:
			fuzzCoreJSONValue(t, data, fixtures.crc32c)
		case coreJSONDoorEd25519PublicKey:
			fuzzCoreJSONValue(t, data, fixtures.publicKey)
		case coreJSONDoorByteCount:
			fuzzCoreJSONValue(t, data, fixtures.byteCount)
		case coreJSONDoorByteLength:
			fuzzCoreJSONValue(t, data, fixtures.byteLength)
		case coreJSONDoorPathComponent:
			fuzzCoreJSONValue(t, data, fixtures.component)
		case coreJSONDoorAbsolutePath:
			fuzzCoreJSONValue(t, data, fixtures.absolutePath)
		case coreJSONDoorUnknown, coreJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type coreTextDoor uint8

const (
	coreTextDoorUnknown coreTextDoor = iota
	coreTextDoorPlatform
	coreTextDoorOffering
	coreTextDoorReleaseVersion
	coreTextDoorSHA256Digest
	coreTextDoorCRC32C
	coreTextDoorEd25519PublicKey
	coreTextDoorBuildCommit
	coreTextDoorHTTPEndpoint
	coreTextDoorPackageIdentity
	coreTextDoorHTTPHeaderName
	coreTextDoorHTTPMediaType
	coreTextDoorPathComponent
	coreTextDoorRelativePath
	coreTextDoorAbsolutePath
	coreTextDoorLimit
)

func FuzzCoreExternalTextDoorInventory(f *testing.F) {
	fixtures := coreFixturesForFuzz(f)
	for _, seed := range coreTextSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.text)
	}
	for _, hostile := range []string{"", " ", "unknown", "\x00", "\xff"} {
		f.Add(uint8(coreTextDoorReleaseVersion), hostile)
		f.Add(uint8(coreTextDoorAbsolutePath), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		switch coreTextDoor(rawDoor) {
		case coreTextDoorPlatform:
			fuzzCoreTextUnmarshal(t, coreTextDecodeRequest[Platform]{
				text: []byte(value), seed: fixtures.platform,
				projection: func(got Platform) (string, error) { return got.String(), nil },
			})
		case coreTextDoorOffering:
			fuzzCoreTextUnmarshal(t, coreTextDecodeRequest[Offering]{
				text: []byte(value), seed: fixtures.offering,
				projection: func(got Offering) (string, error) { return got.String(), nil },
			})
		case coreTextDoorReleaseVersion:
			fuzzCoreTextUnmarshal(t, coreTextDecodeRequest[ReleaseVersion]{
				text: []byte(value), seed: fixtures.version,
				projection: func(got ReleaseVersion) (string, error) { return got.String(), nil },
			})
		case coreTextDoorSHA256Digest:
			fuzzCoreTextUnmarshal(t, coreTextDecodeRequest[SHA256Digest]{
				text: []byte(value), seed: fixtures.sha256, projection: SHA256Digest.Hex,
			})
		case coreTextDoorCRC32C:
			fuzzCoreTextUnmarshal(t, coreTextDecodeRequest[CRC32C]{
				text: []byte(value), seed: fixtures.crc32c, projection: CRC32C.Base64,
			})
		case coreTextDoorEd25519PublicKey:
			fuzzCoreTextUnmarshal(t, coreTextDecodeRequest[Ed25519PublicKey]{
				text: []byte(value), seed: fixtures.publicKey, projection: Ed25519PublicKey.Hex,
			})
		case coreTextDoorBuildCommit:
			got, err := ParseBuildCommit(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[BuildCommit]{
				input: value, value: got, err: err, requiresExact: true, parse: ParseBuildCommit,
			}))
		case coreTextDoorHTTPEndpoint:
			got, err := ParseHTTPEndpoint(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[HTTPEndpoint]{
				input: value, value: got, err: err, parse: ParseHTTPEndpoint,
			}))
		case coreTextDoorPackageIdentity:
			got, err := ParsePackageIdentity(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[PackageIdentity]{
				input: value, value: got, err: err, requiresExact: true, parse: ParsePackageIdentity,
			}))
		case coreTextDoorHTTPHeaderName:
			got, err := ParseHTTPHeaderName(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[HTTPHeaderName]{
				input: value, value: got, err: err, parse: ParseHTTPHeaderName,
			}))
		case coreTextDoorHTTPMediaType:
			got, err := ParseHTTPMediaType(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[HTTPMediaType]{
				input: value, value: got, err: err, parse: ParseHTTPMediaType,
			}))
		case coreTextDoorPathComponent:
			got, err := ParsePathComponent(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[PathComponent]{
				input: value, value: got, err: err, requiresExact: true, parse: ParsePathComponent,
			}))
		case coreTextDoorRelativePath:
			got, err := ParseRelativePath(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[RelativePath]{
				input: value, value: got, err: err, requiresExact: true, parse: ParseRelativePath,
			}))
		case coreTextDoorAbsolutePath:
			got, err := ParseAbsolutePath(value)
			fuzzCoreParseOutcome(t, coreParseOutcomeFor(coreParseRequest[AbsolutePath]{
				input: value, value: got, err: err, requiresExact: true, parse: ParseAbsolutePath,
			}))
		case coreTextDoorUnknown, coreTextDoorLimit:
			return
		default:
			return
		}
	})
}

type coreJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzCoreJSONValue[T coreJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("core seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("core JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, ErrJSONContract) {
			t.Fatalf("core JSON door error = %v, want %v", decodeErr, ErrJSONContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected core JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted core JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > JSONDocumentMaximumBytes {
		t.Fatalf("core canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("core round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("core canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("core JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

type coreTextDecodeRequest[T coreJSONValue] struct {
	seed       T
	projection func(T) (string, error)
	text       []byte
}

func fuzzCoreTextUnmarshal[T coreJSONValue](t *testing.T, request coreTextDecodeRequest[T]) {
	t.Helper()
	before, err := request.seed.MarshalJSON()
	if err != nil {
		t.Fatalf("core text seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := request.seed
	decoder, ok := any(&candidate).(encoding.TextUnmarshaler)
	if !ok {
		t.Fatalf("core text receiver %T lacks encoding.TextUnmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalText(request.text)
	if decodeErr != nil {
		if !errors.Is(decodeErr, ErrPrimitiveContract) {
			t.Fatalf("core text door error = %v, want %v", decodeErr, ErrPrimitiveContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected core text door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted core text validation error = %v, want nil", err)
	}
	canonical, err := request.projection(candidate)
	if err != nil || canonical != string(request.text) {
		t.Fatalf("core accepted text = (%q, %v), want exact canonical input", canonical, err)
	}
	var roundTrip T
	roundTripDecoder := any(&roundTrip).(encoding.TextUnmarshaler)
	if err := roundTripDecoder.UnmarshalText([]byte(canonical)); err != nil {
		t.Fatalf("core canonical text decode error = %v, want nil", err)
	}
	second, err := request.projection(roundTrip)
	if err != nil || second != canonical {
		t.Fatalf("core text door lacks a canonical fixed point: marshal error %v", err)
	}
}

type coreParseOutcome struct {
	err           error
	validate      func() error
	roundTrip     func(string) (string, error)
	input         string
	projection    string
	requiresExact bool
}

type coreParseRequest[T interface {
	Validate() error
	String() string
}] struct {
	value         T
	err           error
	parse         func(string) (T, error)
	input         string
	requiresExact bool
}

func coreParseOutcomeFor[T interface {
	Validate() error
	String() string
}](request coreParseRequest[T]) coreParseOutcome {
	return coreParseOutcome{
		input: request.input, projection: request.value.String(), err: request.err,
		validate: request.value.Validate, requiresExact: request.requiresExact,
		roundTrip: func(value string) (string, error) {
			got, parseErr := request.parse(value)
			return got.String(), parseErr
		},
	}
}

func fuzzCoreParseOutcome(t *testing.T, outcome coreParseOutcome) {
	t.Helper()
	if outcome.err != nil {
		if !errors.Is(outcome.err, ErrPrimitiveContract) || outcome.projection != "" {
			t.Fatalf("core text parse refusal = (%q, %v), want empty and %v",
				outcome.projection, outcome.err, ErrPrimitiveContract)
		}
		return
	}
	if outcome.validate() != nil || outcome.projection == "" ||
		outcome.requiresExact && outcome.projection != outcome.input {
		t.Fatalf("core text parse acceptance = (%q, %v), input %q",
			outcome.projection, outcome.validate(), outcome.input)
	}
	second, err := outcome.roundTrip(outcome.projection)
	if err != nil || second != outcome.projection {
		t.Fatalf("core text parser lacks canonical fixed point: (%q, %v)", second, err)
	}
}

type coreTextSeed struct {
	text string
	door coreTextDoor
}

func coreTextSeedsForFuzz(t testing.TB, fixtures coreJSONFixtures) []coreTextSeed {
	t.Helper()
	sha256Text, err := fixtures.sha256.Hex()
	if err != nil {
		t.Fatalf("SHA256Digest.Hex(seed) error = %v, want nil", err)
	}
	crc32cText, err := fixtures.crc32c.Base64()
	if err != nil {
		t.Fatalf("CRC32C.Base64(seed) error = %v, want nil", err)
	}
	publicKeyText, err := fixtures.publicKey.Hex()
	if err != nil {
		t.Fatalf("Ed25519PublicKey.Hex(seed) error = %v, want nil", err)
	}
	seeds := []coreTextSeed{
		{door: coreTextDoorPlatform, text: fixtures.platform.String()},
		{door: coreTextDoorReleaseVersion, text: fixtures.version.String()},
		{door: coreTextDoorSHA256Digest, text: sha256Text},
		{door: coreTextDoorCRC32C, text: crc32cText},
		{door: coreTextDoorEd25519PublicKey, text: publicKeyText},
		{door: coreTextDoorBuildCommit, text: fixtures.commit.String()},
		{door: coreTextDoorHTTPEndpoint, text: fixtures.endpoint.String()},
		{door: coreTextDoorPackageIdentity, text: fixtures.packageIdentity.String()},
		{door: coreTextDoorHTTPHeaderName, text: fixtures.header.String()},
		{door: coreTextDoorHTTPMediaType, text: fixtures.mediaType.String()},
		{door: coreTextDoorPathComponent, text: fixtures.component.String()},
		{door: coreTextDoorRelativePath, text: fixtures.relativePath.String()},
		{door: coreTextDoorAbsolutePath, text: fixtures.absolutePath.String()},
	}
	seeds = append(seeds, coreTextSeed{door: coreTextDoorOffering, text: fixtures.offering.String()})
	return seeds
}

func coreFixturesForFuzz(t testing.TB) coreJSONFixtures {
	t.Helper()
	platform := Platform{OperatingSystem: OperatingSystemDarwin, Architecture: CPUArchitectureARM64}
	commit, err := ParseBuildCommit(strings.Repeat("a", buildCommitSHA1Bytes*2))
	if err != nil {
		t.Fatalf("ParseBuildCommit(seed) error = %v, want nil", err)
	}
	version := NewReleaseVersion(2026, 0, 76)
	offering, err := parseOffering("core-fuzz-fixture")
	if err != nil {
		t.Fatalf("ParseOffering(seed) error = %v, want nil", err)
	}
	build, err := NewBuildIdentity(BuildIdentityRequest{
		Offering: offering, Version: version, Commit: commit, Platform: platform,
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity(seed) error = %v, want nil", err)
	}
	pageLimit, err := NewCatalogPageLimit(2)
	if err != nil {
		t.Fatalf("NewCatalogPageLimit(seed) error = %v, want nil", err)
	}
	endpoint, err := ParseHTTPEndpoint(SchemeHTTPS + "://example.com/evidence")
	if err != nil {
		t.Fatalf("ParseHTTPEndpoint(seed) error = %v, want nil", err)
	}
	header, err := ParseHTTPHeaderName("content-type")
	if err != nil {
		t.Fatalf("ParseHTTPHeaderName(seed) error = %v, want nil", err)
	}
	mediaType, err := ParseHTTPMediaType("application/json; charset=utf-8")
	if err != nil {
		t.Fatalf("ParseHTTPMediaType(seed) error = %v, want nil", err)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize))
	publicKey, err := NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey(seed) error = %v, want nil", err)
	}
	byteCount, err := NewByteCount(1)
	if err != nil {
		t.Fatalf("NewByteCount(seed) error = %v, want nil", err)
	}
	byteLength, err := NewByteLength(0)
	if err != nil {
		t.Fatalf("NewByteLength(seed) error = %v, want nil", err)
	}
	component, err := ParsePathComponent("evidence.json")
	if err != nil {
		t.Fatalf("ParsePathComponent(seed) error = %v, want nil", err)
	}
	absolute, err := ParseAbsolutePath("/tmp/primitive-core-fuzz")
	if err != nil {
		t.Fatalf("ParseAbsolutePath(seed) error = %v, want nil", err)
	}
	relative, err := ParseRelativePath("evidence/run.json")
	if err != nil {
		t.Fatalf("ParseRelativePath(seed) error = %v, want nil", err)
	}
	return coreJSONFixtures{
		platform: platform, operatingSystem: OperatingSystemDarwin,
		architecture: CPUArchitectureARM64, offering: offering,
		version: version, commit: commit, build: build, pageLimit: pageLimit,
		selection: CatalogSelectionAll, position: CatalogPositionStart,
		continuation: CatalogContinuationEnd, errorIdentity: ErrPrimitiveContract,
		endpoint: endpoint, packageIdentity: PackageCore, packageKind: PackageKindProduction,
		status: HTTPStatusOK(), header: header, mediaType: mediaType,
		sha256:    SHA256Of([]byte("core fuzz digest")),
		crc32c:    NewCRC32C(crc32.Checksum([]byte("core fuzz crc32c"), crc32.MakeTable(crc32.Castagnoli))),
		publicKey: publicKey, byteCount: byteCount, byteLength: byteLength,
		component: component, absolutePath: absolute, relativePath: relative,
	}
}

func coreJSONSeedsForFuzz(t testing.TB, fixtures coreJSONFixtures) []coreJSONSeed {
	t.Helper()
	seeds := []coreJSONSeed{
		coreJSONSeedForFuzz(t, coreJSONDoorPlatform, fixtures.platform),
		coreJSONSeedForFuzz(t, coreJSONDoorOperatingSystem, fixtures.operatingSystem),
		coreJSONSeedForFuzz(t, coreJSONDoorCPUArchitecture, fixtures.architecture),
		coreJSONSeedForFuzz(t, coreJSONDoorReleaseVersion, fixtures.version),
		coreJSONSeedForFuzz(t, coreJSONDoorBuildCommit, fixtures.commit),
		coreJSONSeedForFuzz(t, coreJSONDoorBuildIdentity, fixtures.build),
		coreJSONSeedForFuzz(t, coreJSONDoorCatalogPageLimit, fixtures.pageLimit),
		coreJSONSeedForFuzz(t, coreJSONDoorCatalogSelectionKind, fixtures.selection),
		coreJSONSeedForFuzz(t, coreJSONDoorCatalogPositionKind, fixtures.position),
		coreJSONSeedForFuzz(t, coreJSONDoorCatalogContinuationState, fixtures.continuation),
		coreJSONSeedForFuzz(t, coreJSONDoorErrorIdentity, fixtures.errorIdentity),
		coreJSONSeedForFuzz(t, coreJSONDoorHTTPEndpoint, fixtures.endpoint),
		coreJSONSeedForFuzz(t, coreJSONDoorPackageIdentity, fixtures.packageIdentity),
		coreJSONSeedForFuzz(t, coreJSONDoorPackageKind, fixtures.packageKind),
		coreJSONSeedForFuzz(t, coreJSONDoorHTTPStatusCode, fixtures.status),
		coreJSONSeedForFuzz(t, coreJSONDoorHTTPHeaderName, fixtures.header),
		coreJSONSeedForFuzz(t, coreJSONDoorHTTPMediaType, fixtures.mediaType),
		coreJSONSeedForFuzz(t, coreJSONDoorSHA256Digest, fixtures.sha256),
		coreJSONSeedForFuzz(t, coreJSONDoorCRC32C, fixtures.crc32c),
		coreJSONSeedForFuzz(t, coreJSONDoorEd25519PublicKey, fixtures.publicKey),
		coreJSONSeedForFuzz(t, coreJSONDoorByteCount, fixtures.byteCount),
		coreJSONSeedForFuzz(t, coreJSONDoorByteLength, fixtures.byteLength),
		coreJSONSeedForFuzz(t, coreJSONDoorPathComponent, fixtures.component),
		coreJSONSeedForFuzz(t, coreJSONDoorAbsolutePath, fixtures.absolutePath),
	}
	seeds = append(seeds, coreJSONSeedForFuzz(t, coreJSONDoorOffering, fixtures.offering))
	return seeds
}

func coreJSONSeedForFuzz(t testing.TB, door coreJSONDoor, value coreJSONValue) coreJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("core fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return coreJSONSeed{door: door, document: document}
}

func FuzzCoreDecodeJSONStringTokenSemanticClosure(f *testing.F) {
	canonical, err := MarshalCanonicalJSONString("typed core seed")
	if err != nil {
		f.Fatalf("MarshalCanonicalJSONString(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	for _, hostile := range [][]byte{nil, {}, []byte(`null`), []byte(`"`), []byte(`"x" 0`), {0xff}} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, gotErr := DecodeJSONStringToken(data)
		if len(data) > JSONDocumentMaximumBytes {
			if !errors.Is(gotErr, ErrJSONContract) || got != "" {
				t.Fatalf("oversized DecodeJSONStringToken() = (length %d, %v), want empty typed refusal", len(got), gotErr)
			}
			return
		}
		if gotErr != nil {
			if !errors.Is(gotErr, ErrJSONContract) || got != "" {
				t.Fatalf("DecodeJSONStringToken() = (%q, %v), want empty typed refusal", got, gotErr)
			}
			return
		}
		if !jsontext.Value(data).IsValid() {
			t.Fatalf("DecodeJSONStringToken accepted invalid JSON")
		}
		encoded, err := MarshalCanonicalJSONString(got)
		if err != nil {
			t.Fatalf("MarshalCanonicalJSONString(accepted) error = %v, want nil", err)
		}
		second, err := DecodeJSONStringToken(encoded)
		if err != nil || second != got {
			t.Fatalf("JSON string canonical fixed point = (%q, %v), want %q and nil", second, err, got)
		}
	})
}

func FuzzCoreDecodeCanonicalHexSemanticClosure(f *testing.F) {
	digest := SHA256Of([]byte("typed canonical hex seed"))
	raw, err := digest.Bytes()
	if err != nil {
		f.Fatalf("SHA256Digest.Bytes(seed) error = %v, want nil", err)
	}
	f.Add(hex.EncodeToString(raw[:]))
	for _, hostile := range []string{"", "0", "gg", strings.ToUpper(hex.EncodeToString(raw[:])), "\x00", "\xff"} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, value string) {
		before := bytes.Repeat([]byte{0xa5}, len(raw))
		destination := bytes.Clone(before)
		gotErr := DecodeCanonicalHex(destination, value)
		if gotErr != nil {
			if !errors.Is(gotErr, ErrPrimitiveContract) || !bytes.Equal(destination, before) {
				t.Fatalf("DecodeCanonicalHex refusal = (%x, %v), want preserved typed refusal", destination, gotErr)
			}
			return
		}
		if hex.EncodeToString(destination) != value || len(value) != hex.EncodedLen(len(destination)) {
			t.Fatalf("DecodeCanonicalHex accepted noncanonical value %q", value)
		}
	})
}

func FuzzCoreHTTPStatusAdmitIntSemanticClosure(f *testing.F) {
	for _, seed := range []int{99, 100, 199, 200, 299, 300, 399, 400, 499, 500, 599, 600} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value int) {
		status := HTTPStatusOK()
		err := status.AdmitInt(value)
		if err != nil {
			if !errors.Is(err, ErrPrimitiveContract) || status != HTTPStatusOK() {
				t.Fatalf("HTTPStatusCode.AdmitInt(%d) = (%v, %v), want preserved typed refusal", value, status, err)
			}
			return
		}
		got, gotErr := status.Int()
		if gotErr != nil || got != value || value < httpStatusCodeMinimum || value > httpStatusCodeMaximum {
			t.Fatalf("HTTPStatusCode.AdmitInt(%d) = (%d, %v), invalid acceptance", value, got, gotErr)
		}
	})
}

func TestCoreExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	gotJSON, err := coreExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("coreExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := coreJSONDoorUnknown + 1; door < coreJSONDoorLimit; door++ {
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
	_ = FuzzDecodeStrictJSONAbsolutePathPublicBoundary
	_ = FuzzCoreDecodeJSONStringTokenSemanticClosure
	_ = FuzzCoreDecodeCanonicalHexSemanticClosure
	_ = FuzzCoreHTTPStatusAdmitIntSemanticClosure
}

func coreExportedJSONReceiverNames() ([]string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, file.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "UnmarshalJSON" || function.Recv == nil || len(function.Recv.List) != 1 {
				continue
			}
			pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			receiver, ok := pointer.X.(*ast.Ident)
			if ok && receiver.IsExported() {
				names = append(names, receiver.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}
