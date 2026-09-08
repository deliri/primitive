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
	"path/filepath"
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
	coreJSONDoorPackageRole
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
	coreJSONDoorSourcePath
	coreJSONDoorRepositoryIdentity
	coreJSONDoorSourceSnapshot
	coreJSONDoorSourceSubjectKind
	coreJSONDoorSourceSubject
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
	case coreJSONDoorPackageRole:
		return "PackageRole"
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
	case coreJSONDoorSourcePath:
		return "SourcePath"
	case coreJSONDoorRepositoryIdentity:
		return "RepositoryIdentity"
	case coreJSONDoorSourceSnapshot:
		return "SourceSnapshot"
	case coreJSONDoorSourceSubjectKind:
		return "SourceSubjectKind"
	case coreJSONDoorSourceSubject:
		return "SourceSubject"
	case coreJSONDoorUnknown, coreJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type coreJSONFixtures struct {
	relativePath    RelativePath
	absolutePath    AbsolutePath
	sourcePath      SourcePath
	repository      RepositoryIdentity
	snapshot        SourceSnapshot
	subjectKind     SourceSubjectKind
	subject         SourceSubject
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
	packageRole     PackageRole
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
		door := coreJSONDoor(rawDoor)
		if door <= coreJSONDoorUnknown || door >= coreJSONDoorLimit {
			door = coreJSONDoor(rawDoor%uint8(coreJSONDoorLimit-1) + 1)
		}
		var candidate, roundTrip coreJSONReceiver
		switch door {
		case coreJSONDoorPlatform:
			candidate, roundTrip = coreJSONReceivers(fixtures.platform)
		case coreJSONDoorOperatingSystem:
			candidate, roundTrip = coreJSONReceivers(fixtures.operatingSystem)
		case coreJSONDoorCPUArchitecture:
			candidate, roundTrip = coreJSONReceivers(fixtures.architecture)
		case coreJSONDoorOffering:
			candidate, roundTrip = coreJSONReceivers(fixtures.offering)
		case coreJSONDoorReleaseVersion:
			candidate, roundTrip = coreJSONReceivers(fixtures.version)
		case coreJSONDoorBuildCommit:
			candidate, roundTrip = coreJSONReceivers(fixtures.commit)
		case coreJSONDoorBuildIdentity:
			candidate, roundTrip = coreJSONReceivers(fixtures.build)
		case coreJSONDoorCatalogPageLimit:
			candidate, roundTrip = coreJSONReceivers(fixtures.pageLimit)
		case coreJSONDoorCatalogSelectionKind:
			candidate, roundTrip = coreJSONReceivers(fixtures.selection)
		case coreJSONDoorCatalogPositionKind:
			candidate, roundTrip = coreJSONReceivers(fixtures.position)
		case coreJSONDoorCatalogContinuationState:
			candidate, roundTrip = coreJSONReceivers(fixtures.continuation)
		case coreJSONDoorErrorIdentity:
			candidate, roundTrip = coreJSONReceivers(fixtures.errorIdentity)
		case coreJSONDoorHTTPEndpoint:
			candidate, roundTrip = coreJSONReceivers(fixtures.endpoint)
		case coreJSONDoorPackageIdentity:
			candidate, roundTrip = coreJSONReceivers(fixtures.packageIdentity)
		case coreJSONDoorPackageKind:
			candidate, roundTrip = coreJSONReceivers(fixtures.packageKind)
		case coreJSONDoorPackageRole:
			candidate, roundTrip = coreJSONReceivers(fixtures.packageRole)
		case coreJSONDoorHTTPStatusCode:
			candidate, roundTrip = coreJSONReceivers(fixtures.status)
		case coreJSONDoorHTTPHeaderName:
			candidate, roundTrip = coreJSONReceivers(fixtures.header)
		case coreJSONDoorHTTPMediaType:
			candidate, roundTrip = coreJSONReceivers(fixtures.mediaType)
		case coreJSONDoorSHA256Digest:
			candidate, roundTrip = coreJSONReceivers(fixtures.sha256)
		case coreJSONDoorCRC32C:
			candidate, roundTrip = coreJSONReceivers(fixtures.crc32c)
		case coreJSONDoorEd25519PublicKey:
			candidate, roundTrip = coreJSONReceivers(fixtures.publicKey)
		case coreJSONDoorByteCount:
			candidate, roundTrip = coreJSONReceivers(fixtures.byteCount)
		case coreJSONDoorByteLength:
			candidate, roundTrip = coreJSONReceivers(fixtures.byteLength)
		case coreJSONDoorPathComponent:
			candidate, roundTrip = coreJSONReceivers(fixtures.component)
		case coreJSONDoorAbsolutePath:
			candidate, roundTrip = coreJSONReceivers(fixtures.absolutePath)
		case coreJSONDoorSourcePath:
			candidate, roundTrip = coreJSONReceivers(fixtures.sourcePath)
		case coreJSONDoorRepositoryIdentity:
			candidate, roundTrip = coreJSONReceivers(fixtures.repository)
		case coreJSONDoorSourceSnapshot:
			candidate, roundTrip = coreJSONReceivers(fixtures.snapshot)
		case coreJSONDoorSourceSubjectKind:
			candidate, roundTrip = coreJSONReceivers(fixtures.subjectKind)
		case coreJSONDoorSourceSubject:
			candidate, roundTrip = coreJSONReceivers(fixtures.subject)
		case coreJSONDoorUnknown, coreJSONDoorLimit:
			t.Fatalf("normalized JSON door=%d from selector=%d; want admitted domain", door, rawDoor)
		default:
			t.Fatalf("JSON door=%d has no compiler-bound receiver", door)
		}
		before, beforeErr := candidate.MarshalJSON()
		if beforeErr != nil {
			t.Fatal(beforeErr)
		}
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if bytes.Equal(data, before) {
				t.Fatalf("producer's exact JSON was refused: %v", decodeErr)
			}
			after, marshalErr := candidate.MarshalJSON()
			if !errors.Is(decodeErr, ErrJSONContract) || marshalErr != nil || !bytes.Equal(after, before) {
				t.Fatalf("refused JSON error=%v, marshal=%v, receiver unchanged=%t; want typed refusal and unchanged receiver", decodeErr, marshalErr, bytes.Equal(after, before))
			}
			return
		}
		if err := candidate.Validate(); err != nil {
			t.Fatalf("accepted JSON validation=%v; want nil", err)
		}
		canonical, err := candidate.MarshalJSON()
		if err != nil || len(canonical) > JSONDocumentMaximumBytes {
			t.Fatalf("canonical=%d bytes, %v; want bounded valid JSON", len(canonical), err)
		}
		if err := roundTrip.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("canonical decode=%v; want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("republication=%d bytes, %v; want exact canonical %d bytes", len(second), err, len(canonical))
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
	coreTextDoorSourceSnapshot
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
	seeds := coreTextSeedsForFuzz(f, fixtures)
	for _, seed := range seeds {
		f.Add(uint8(seed.door), seed.text)
	}
	for _, hostile := range []string{"", " ", "unknown", "\x00", "\xff"} {
		f.Add(uint8(coreTextDoorReleaseVersion), hostile)
		f.Add(uint8(coreTextDoorAbsolutePath), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		door := coreTextDoor(rawDoor)
		if door <= coreTextDoorUnknown || door >= coreTextDoorLimit {
			door = coreTextDoor(rawDoor%uint8(coreTextDoorLimit-1) + 1)
		}
		var receivers coreTextReceivers
		var outcome coreParseOutcome
		switch door {
		case coreTextDoorPlatform:
			receivers = coreTextReceiversFor(coreTextDecodeRequest[Platform]{
				text: []byte(value), seed: fixtures.platform,
				projection: func(got Platform) (string, error) { return got.String(), nil },
			})
		case coreTextDoorOffering:
			receivers = coreTextReceiversFor(coreTextDecodeRequest[Offering]{
				text: []byte(value), seed: fixtures.offering,
				projection: func(got Offering) (string, error) { return got.String(), nil },
			})
		case coreTextDoorReleaseVersion:
			receivers = coreTextReceiversFor(coreTextDecodeRequest[ReleaseVersion]{
				text: []byte(value), seed: fixtures.version,
				projection: func(got ReleaseVersion) (string, error) { return got.String(), nil },
			})
		case coreTextDoorSHA256Digest:
			receivers = coreTextReceiversFor(coreTextDecodeRequest[SHA256Digest]{
				text: []byte(value), seed: fixtures.sha256, projection: SHA256Digest.Hex,
			})
		case coreTextDoorCRC32C:
			receivers = coreTextReceiversFor(coreTextDecodeRequest[CRC32C]{
				text: []byte(value), seed: fixtures.crc32c, projection: CRC32C.Base64,
			})
		case coreTextDoorEd25519PublicKey:
			receivers = coreTextReceiversFor(coreTextDecodeRequest[Ed25519PublicKey]{
				text: []byte(value), seed: fixtures.publicKey, projection: Ed25519PublicKey.Hex,
			})
		case coreTextDoorBuildCommit:
			got, err := ParseBuildCommit(value)
			outcome = coreParseOutcomeFor(coreParseRequest[BuildCommit]{
				input: value, value: got, err: err, requiresExact: true, parse: ParseBuildCommit,
			})
		case coreTextDoorSourceSnapshot:
			receivers = coreTextReceiversFor(coreTextDecodeRequest[SourceSnapshot]{
				text: []byte(value), seed: fixtures.snapshot,
				projection: func(got SourceSnapshot) (string, error) {
					if err := got.Validate(); err != nil {
						return "", err
					}
					return got.String(), nil
				},
			})
		case coreTextDoorHTTPEndpoint:
			got, err := ParseHTTPEndpoint(value)
			outcome = coreParseOutcomeFor(coreParseRequest[HTTPEndpoint]{
				input: value, value: got, err: err, parse: ParseHTTPEndpoint,
			})
		case coreTextDoorPackageIdentity:
			got, err := ParsePackageIdentity(value)
			outcome = coreParseOutcomeFor(coreParseRequest[PackageIdentity]{
				input: value, value: got, err: err, requiresExact: true, parse: ParsePackageIdentity,
			})
		case coreTextDoorHTTPHeaderName:
			got, err := ParseHTTPHeaderName(value)
			outcome = coreParseOutcomeFor(coreParseRequest[HTTPHeaderName]{
				input: value, value: got, err: err, parse: ParseHTTPHeaderName,
			})
		case coreTextDoorHTTPMediaType:
			got, err := ParseHTTPMediaType(value)
			outcome = coreParseOutcomeFor(coreParseRequest[HTTPMediaType]{
				input: value, value: got, err: err, parse: ParseHTTPMediaType,
			})
		case coreTextDoorPathComponent:
			got, err := ParsePathComponent(value)
			outcome = coreParseOutcomeFor(coreParseRequest[PathComponent]{
				input: value, value: got, err: err, requiresExact: true, parse: ParsePathComponent,
			})
		case coreTextDoorRelativePath:
			got, err := ParseRelativePath(value)
			outcome = coreParseOutcomeFor(coreParseRequest[RelativePath]{
				input: value, value: got, err: err, requiresExact: true, parse: ParseRelativePath,
			})
		case coreTextDoorAbsolutePath:
			got, err := ParseAbsolutePath(value)
			outcome = coreParseOutcomeFor(coreParseRequest[AbsolutePath]{
				input: value, value: got, err: err, requiresExact: true, parse: ParseAbsolutePath,
			})
		case coreTextDoorUnknown, coreTextDoorLimit:
			t.Fatalf("normalized text door=%d from selector=%d; want admitted domain", door, rawDoor)
		default:
			t.Fatalf("text door=%d has no compiler-bound operation", door)
		}
		if receivers.candidate != nil {
			before, err := receivers.candidate.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			seedText, err := receivers.projection()
			if err != nil {
				t.Fatal(err)
			}
			decodeErr := receivers.candidate.UnmarshalText([]byte(value))
			if decodeErr != nil {
				if value == seedText {
					t.Fatalf("producer's exact text was refused: %v", decodeErr)
				}
				after, marshalErr := receivers.candidate.MarshalJSON()
				if !errors.Is(decodeErr, ErrPrimitiveContract) || marshalErr != nil || !bytes.Equal(after, before) {
					t.Fatalf("text refusal=%v, marshal=%v, receiver unchanged=%t; want typed refusal and no mutation", decodeErr, marshalErr, bytes.Equal(after, before))
				}
				return
			}
			canonical, err := receivers.projection()
			if err != nil || canonical != value || receivers.candidate.Validate() != nil {
				t.Fatalf("accepted text=%q, %v; want exact valid input %q", canonical, err, value)
			}
			if err := receivers.roundTrip.UnmarshalText([]byte(canonical)); err != nil {
				t.Fatalf("text reparse=%v; want nil", err)
			}
			second, err := receivers.secondProjection()
			if err != nil || second != canonical {
				t.Fatalf("text republication=%q, %v; want %q", second, err, canonical)
			}
			return
		}
		if outcome.err != nil {
			for _, seed := range seeds {
				if seed.door == door && seed.text == value {
					t.Fatalf("known producer text was refused: %v", outcome.err)
				}
			}
			if !errors.Is(outcome.err, ErrPrimitiveContract) || outcome.projection != "" {
				t.Fatalf("text parse=%q, %v; want zero and typed refusal", outcome.projection, outcome.err)
			}
			return
		}
		if err := outcome.validate(); err != nil || outcome.projection == "" || outcome.requiresExact && outcome.projection != value {
			t.Fatalf("text projection=%q, %v; input=%q; want exact admitted projection", outcome.projection, err, value)
		}
		second, err := outcome.roundTrip(outcome.projection)
		if err != nil || second != outcome.projection {
			t.Fatalf("text reparse=%q, %v; want %q", second, err, outcome.projection)
		}

	})
}

type coreJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

type coreJSONReceiver interface {
	ValidatedJSONMarshaler
	json.Unmarshaler
}

// This helper constructs fresh typed receivers; every verdict stays in Fuzz.
func coreJSONReceivers[T ValidatedJSONMarshaler, P interface {
	*T
	coreJSONReceiver
}](seed T) (P, P) {
	candidate := seed
	return P(&candidate), P(new(T))
}

type coreTextDecodeRequest[T coreJSONValue] struct {
	seed       T
	projection func(T) (string, error)
	text       []byte
}

type coreTextReceiver interface {
	ValidatedJSONMarshaler
	encoding.TextUnmarshaler
}
type coreTextReceivers struct {
	candidate        coreTextReceiver
	roundTrip        coreTextReceiver
	projection       func() (string, error)
	secondProjection func() (string, error)
}

func coreTextReceiversFor[T coreJSONValue, P interface {
	*T
	coreTextReceiver
}](request coreTextDecodeRequest[T]) coreTextReceivers {
	candidate := request.seed
	var roundTrip T
	return coreTextReceivers{candidate: P(&candidate), roundTrip: P(&roundTrip), projection: func() (string, error) { return request.projection(candidate) }, secondProjection: func() (string, error) { return request.projection(roundTrip) }}
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
		{door: coreTextDoorSourceSnapshot, text: fixtures.snapshot.String()},
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
	absolute, err := ParseAbsolutePath(filepath.Join(filepath.VolumeName(t.TempDir())+string(filepath.Separator), "primitive-core-fuzz"))
	if err != nil {
		t.Fatalf("ParseAbsolutePath(seed) error = %v, want nil", err)
	}
	relative, err := ParseRelativePath("evidence/run.json")
	if err != nil {
		t.Fatalf("ParseRelativePath(seed) error = %v, want nil", err)
	}
	sourcePath, err := ParseSourcePath("source/file.go")
	if err != nil {
		t.Fatalf("ParseSourcePath(seed) error = %v, want nil", err)
	}
	repository, err := NewRepositoryIdentity("example.com/owner/project")
	if err != nil {
		t.Fatalf("NewRepositoryIdentity(seed) error = %v, want nil", err)
	}
	snapshot := SourceSnapshot{Digest: SHA256Of([]byte("core source snapshot fuzz seed"))}
	subject, err := NewSourceSubject(SourceSubjectFile, sourcePath)
	if err != nil {
		t.Fatalf("NewSourceSubject(seed) error = %v, want nil", err)
	}
	return coreJSONFixtures{
		platform: platform, operatingSystem: OperatingSystemDarwin,
		architecture: CPUArchitectureARM64, offering: offering,
		version: version, commit: commit, build: build, pageLimit: pageLimit,
		selection: CatalogSelectionAll, position: CatalogPositionStart,
		continuation: CatalogContinuationEnd, errorIdentity: ErrPrimitiveContract,
		endpoint: endpoint, packageIdentity: PackageCore, packageKind: PackageKindProduction, packageRole: PackageRoleValueContract,
		status: HTTPStatusOK(), header: header, mediaType: mediaType,
		sha256:    SHA256Of([]byte("core fuzz digest")),
		crc32c:    NewCRC32C(crc32.Checksum([]byte("core fuzz crc32c"), crc32.MakeTable(crc32.Castagnoli))),
		publicKey: publicKey, byteCount: byteCount, byteLength: byteLength,
		component: component, absolutePath: absolute, relativePath: relative,
		sourcePath: sourcePath, repository: repository, snapshot: snapshot,
		subjectKind: SourceSubjectFile, subject: subject,
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
		coreJSONSeedForFuzz(t, coreJSONDoorPackageRole, fixtures.packageRole),
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
		coreJSONSeedForFuzz(t, coreJSONDoorSourcePath, fixtures.sourcePath),
		coreJSONSeedForFuzz(t, coreJSONDoorRepositoryIdentity, fixtures.repository),
		coreJSONSeedForFuzz(t, coreJSONDoorSourceSnapshot, fixtures.snapshot),
		coreJSONSeedForFuzz(t, coreJSONDoorSourceSubjectKind, fixtures.subjectKind),
		coreJSONSeedForFuzz(t, coreJSONDoorSourceSubject, fixtures.subject),
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
		var want string
		nativeErr := json.Unmarshal(data, &want)
		wantOK := len(data) <= JSONDocumentMaximumBytes && nativeErr == nil && !bytes.Equal(bytes.TrimSpace(data), []byte(jsonNullLiteralText))
		if (gotErr == nil) != wantOK || wantOK && got != want {
			t.Fatalf("JSON string=%q, %v; want Go string %q, admission %t", got, gotErr, want, wantOK)
		}
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
		native, nativeErr := hex.DecodeString(value)
		wantOK := nativeErr == nil && len(native) == len(destination) && hex.EncodeToString(native) == value
		if (gotErr == nil) != wantOK || wantOK && !bytes.Equal(destination, native) {
			t.Fatalf("hex=%x, %v; want exact Go bytes %x with canonical admission %t", destination, gotErr, native, wantOK)
		}
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
		wantOK := value >= httpStatusCodeMinimum && value <= httpStatusCodeMaximum
		if (err == nil) != wantOK {
			t.Fatalf("status admission=%v; want acceptance %t for %d", err, wantOK, value)
		}
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
	for door := range coreJSONDoorLimit {
		if door < coreJSONDoorUnknown+1 {
			continue
		}
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
	_ = FuzzDecodeStrictJSONAbsolutePathPublicBoundary
	_ = FuzzDecodeStrictJSONReaderAbsolutePathPublicBoundary
	_ = FuzzCoreDecodeJSONStringTokenSemanticClosure
	_ = FuzzCoreDecodeCanonicalHexSemanticClosure
	_ = FuzzCoreHTTPStatusAdmitIntSemanticClosure
	_ = FuzzDigestWriterStreamAndReset
	_ = FuzzHTTPEndpointCanonicalProjection
	_ = FuzzAbsolutePathResolveTextIngress
	_ = FuzzIssueProjectionStructuralLimits
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
