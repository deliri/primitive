package release

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"errors"
	"hash/crc32"
	"io"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// BuiltArtifactMaximumBytes is the largest executable admitted by the
	// release inspection boundary. It preserves the established consumer
	// release ceiling while keeping every read and parser view finite.
	BuiltArtifactMaximumBytes      = 256 << 20
	artifactInspectionBufferBytes  = 64 << 10
	artifactInspectionPatternCount = 4 + linkerAssignmentMaximumCount
	debugSectionPrefix             = ".debug"
	compressedDebugSectionPrefix   = ".zdebug"
)

// ArtifactInspectionRequest supplies one bounded built executable and the
// exact compiler-owned facts that its build command injected.
type ArtifactInspectionRequest struct {
	Source            io.ReaderAt
	LinkerAssignments LinkerAssignments
	Extent            core.ByteCount
	Build             core.BuildIdentity
}

// Validate closes the inspection boundary before the source is read.
func (r ArtifactInspectionRequest) Validate() error {
	if r.Source == nil {
		return contractError(errors.New("artifact inspection source is nil"))
	}
	for _, err := range []error{
		r.Extent.Validate(), r.Build.Validate(), r.LinkerAssignments.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("artifact inspection request is invalid"), err)
		}
	}
	extent, err := r.Extent.Uint64()
	if err != nil || extent == 0 || extent > BuiltArtifactMaximumBytes {
		return contractError(errors.New("artifact inspection extent is outside its bound"), err)
	}
	return nil
}

// InspectBuiltArtifact proves the executable format, target architecture,
// stripping, exact extent, dual integrity, and embedded release stamps. The
// source is never retained and memory use is independent of its extent.
func InspectBuiltArtifact(request ArtifactInspectionRequest) (Artifact, error) {
	if err := request.Validate(); err != nil {
		return Artifact{}, err
	}
	extent, _ := request.Extent.Int64()
	bounded := io.NewSectionReader(request.Source, 0, extent)
	if err := inspectExecutable(bounded, request.Build.Platform()); err != nil {
		return Artifact{}, err
	}
	integrity, err := inspectArtifactBytes(bounded, request, extent)
	if err != nil {
		return Artifact{}, err
	}
	return NewArtifact(ArtifactRequest{
		Build: request.Build, Extent: request.Extent,
		SHA256: integrity.sha256, CRC32C: integrity.crc32c,
	})
}

func inspectExecutable(source io.ReaderAt, platform core.Platform) error {
	switch platform.OperatingSystem {
	case core.OperatingSystemDarwin:
		return inspectMachOExecutable(source, platform)
	case core.OperatingSystemLinux:
		return inspectELFExecutable(source, platform)
	case core.OperatingSystemWindows:
		return inspectPEExecutable(source, platform)
	default:
		return contractError(errors.New("artifact platform has no executable format"))
	}
}

func inspectMachOExecutable(source io.ReaderAt, platform core.Platform) error {
	binary, err := macho.NewFile(source)
	if err != nil {
		return contractError(errors.New("artifact is not mach-o"), err)
	}
	if binary.Type != macho.TypeExec || binary.Cpu != macho.CpuArm64 ||
		platform.Architecture != core.CPUArchitectureARM64 || machoHasDebug(binary) {
		return contractError(errors.New("mach-o artifact differs from its release target"))
	}
	return nil
}

// machoHasDebug reports retained DWARF or a retained static symbol table. A
// stripped Go Mach-O executable keeps only its undefined dynamic imports, so
// any local symbol proves the linker symbol table survived.
func machoHasDebug(binary *macho.File) bool {
	for _, section := range binary.Sections {
		if section.Seg == "__DWARF" || strings.HasPrefix(section.Name, "__debug_") {
			return true
		}
	}
	return binary.Dysymtab == nil || binary.Dysymtab.Nlocalsym != 0 ||
		binary.Dysymtab.Nextdefsym != 0
}

func inspectELFExecutable(source io.ReaderAt, platform core.Platform) error {
	binary, err := elf.NewFile(source)
	if err != nil {
		return contractError(errors.New("artifact is not elf"), err)
	}
	want, err := elfMachine(platform.Architecture)
	if err != nil {
		return err
	}
	if binary.Type != elf.ET_EXEC || binary.Machine != want || elfHasDebug(binary) {
		return contractError(errors.New("elf artifact differs from its release target"))
	}
	return nil
}

func elfMachine(architecture core.CPUArchitecture) (elf.Machine, error) {
	switch architecture {
	case core.CPUArchitectureAMD64:
		return elf.EM_X86_64, nil
	case core.CPUArchitectureARM64:
		return elf.EM_AARCH64, nil
	default:
		return elf.EM_NONE, contractError(errors.New("elf artifact architecture is unsupported"))
	}
}

func elfHasDebug(binary *elf.File) bool {
	for _, section := range binary.Sections {
		if section.Type == elf.SHT_SYMTAB || strings.HasPrefix(section.Name, debugSectionPrefix) ||
			strings.HasPrefix(section.Name, compressedDebugSectionPrefix) {
			return true
		}
	}
	return false
}

func inspectPEExecutable(source io.ReaderAt, platform core.Platform) error {
	binary, err := pe.NewFile(source)
	if err != nil {
		return contractError(errors.New("artifact is not pe"), err)
	}
	if platform.Architecture != core.CPUArchitectureAMD64 ||
		binary.Machine != pe.IMAGE_FILE_MACHINE_AMD64 ||
		binary.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE == 0 ||
		binary.Characteristics&pe.IMAGE_FILE_DLL != 0 || peHasDebug(binary) {
		return contractError(errors.New("pe artifact differs from its release target"))
	}
	return nil
}

// peHasDebug reports retained DWARF or a retained COFF symbol table. A
// stripped Go PE executable keeps the empty .symtab section header but no
// symbol records, so any record proves the symbol table survived.
func peHasDebug(binary *pe.File) bool {
	for _, section := range binary.Sections {
		if strings.HasPrefix(section.Name, debugSectionPrefix) ||
			strings.HasPrefix(section.Name, compressedDebugSectionPrefix) {
			return true
		}
	}
	return len(binary.COFFSymbols) != 0
}

type artifactByteInspection struct {
	sha256 core.SHA256Digest
	crc32c core.CRC32C
}

func inspectArtifactBytes(
	bounded *io.SectionReader,
	request ArtifactInspectionRequest,
	extent int64,
) (artifactByteInspection, error) {
	if _, err := bounded.Seek(0, io.SeekStart); err != nil {
		return artifactByteInspection{}, contractError(errors.New("rewind artifact"), err)
	}
	patterns := artifactInspectionPatterns(request)
	finder := newArtifactPatternFinder(patterns)
	sha := core.NewDigestWriter()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	written, err := io.CopyBuffer(io.MultiWriter(sha, crc, finder), bounded,
		make([]byte, artifactInspectionBufferBytes))
	if err != nil || written != extent {
		return artifactByteInspection{}, contractError(errors.New("stream artifact bytes"), err)
	}
	var extra [1]byte
	read, readErr := request.Source.ReadAt(extra[:], extent)
	if read != 0 || !errors.Is(readErr, io.EOF) {
		return artifactByteInspection{}, contractError(errors.New("artifact exceeds its declared extent"), readErr)
	}
	if err := finder.Validate(); err != nil {
		return artifactByteInspection{}, err
	}
	shaDigest, _, err := sha.Seal()
	if err != nil {
		return artifactByteInspection{}, contractError(errors.New("stream artifact bytes"), err)
	}
	return artifactByteInspection{
		sha256: shaDigest, crc32c: core.NewCRC32C(crc.Sum32()),
	}, nil
}

func artifactInspectionPatterns(request ArtifactInspectionRequest) [artifactInspectionPatternCount]string {
	build := request.Build
	patterns := [artifactInspectionPatternCount]string{
		build.Offering().String(), build.Version().String(),
		build.Commit().String(), build.Platform().String(),
	}
	for index := range request.LinkerAssignments.count {
		patternIndex := index + 4
		if patternIndex >= len(patterns) {
			break
		}
		// #nosec G602 -- request validation proves count <= the fixed assignment
		// array and patternIndex is checked against the fixed destination above.
		patterns[patternIndex] = request.LinkerAssignments.values[index].value
	}
	return patterns
}

type artifactPatternFinder struct {
	patterns [artifactInspectionPatternCount]string
	found    [artifactInspectionPatternCount]bool
	tail     [linkerValueMaximumBytes - 1]byte
	tailSize int
	longest  int
}

func newArtifactPatternFinder(patterns [artifactInspectionPatternCount]string) *artifactPatternFinder {
	finder := &artifactPatternFinder{patterns: patterns, longest: 1}
	for _, pattern := range patterns {
		if len(pattern) > finder.longest {
			finder.longest = len(pattern)
		}
	}
	return finder
}

func (f *artifactPatternFinder) Write(data []byte) (int, error) {
	window := make([]byte, f.tailSize+len(data))
	copy(window, f.tail[:f.tailSize])
	copy(window[f.tailSize:], data)
	for index, pattern := range f.patterns {
		if pattern != "" && !f.found[index] && bytes.Contains(window, []byte(pattern)) {
			f.found[index] = true
		}
	}
	keep := min(f.longest-1, len(window))
	copy(f.tail[:], window[len(window)-keep:])
	f.tailSize = keep
	return len(data), nil
}

func (f *artifactPatternFinder) Validate() error {
	for index, pattern := range f.patterns {
		if pattern != "" && !f.found[index] {
			return contractError(errors.New("artifact is missing embedded build value slot " +
				strconv.Itoa(index)))
		}
	}
	return nil
}
