package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	cgroupV2LimitName      = "memory.max"
	cgroupV1LimitName      = "memory.limit_in_bytes"
	cgroupMemoryController = "memory"
	cgroupV2MaxToken       = "max"
	cgroupV1UnlimitedMin   = uint64(1 << 62)

	virtualFileMaximumBytes = 1 << 20
	procLineMaximumBytes    = 64 << 10
	limitValueMaximumBytes  = 128
	streamBufferBytes       = 4 << 10
)

var (
	cgroupV2LimitComponent = sync.OnceValues(func() (core.PathComponent, error) {
		return core.ParsePathComponent(cgroupV2LimitName)
	})
	cgroupV1LimitComponent = sync.OnceValues(func() (core.PathComponent, error) {
		return core.ParsePathComponent(cgroupV1LimitName)
	})
)

type cgroupMembership struct {
	path   string
	source WorkloadMemoryLimitSource
}

func (m cgroupMembership) Validate() error {
	if m.source != WorkloadMemoryLimitSourceCgroupV1 &&
		m.source != WorkloadMemoryLimitSourceCgroupV2 {
		return core.ErrHostFactsObservation
	}
	if !validCgroupPath(m.path) {
		return core.ErrHostFactsObservation
	}
	return nil
}

type cgroupMount struct {
	root       string
	mountPoint core.AbsolutePath
	source     WorkloadMemoryLimitSource
}

func (m cgroupMount) Validate() error {
	if !validCgroupPath(m.root) || m.mountPoint.Validate() != nil ||
		(m.source != WorkloadMemoryLimitSourceCgroupV1 &&
			m.source != WorkloadMemoryLimitSourceCgroupV2) {
		return core.ErrHostFactsObservation
	}
	return nil
}

// cgroupMountAmbiguityCeiling saturates the equally-specific mount tally. The
// tally only has to distinguish "exactly one" from "more than one", so it stops
// counting rather than wrapping: an unsaturated eight-bit tally returns to one
// after 256 equally-specific mounts and reports a hostile mount table as unique.
const cgroupMountAmbiguityCeiling = uint8(2)

type cgroupMountSelection struct {
	selected cgroupMount
	count    uint8
}

func (s *cgroupMountSelection) consider(
	candidate cgroupMount,
	membership cgroupMembership,
) error {
	if err := errors.Join(candidate.Validate(), membership.Validate()); err != nil {
		return err
	}
	if !cgroupRootContains(candidate.root, membership.path) {
		return nil
	}
	if s.count == 0 || len(candidate.root) > len(s.selected.root) {
		s.selected, s.count = candidate, 1
		return nil
	}
	if len(candidate.root) == len(s.selected.root) && s.count < cgroupMountAmbiguityCeiling {
		s.count++
	}
	return nil
}

func cgroupRootContains(root, membership string) bool {
	return root == "/" || membership == root || strings.HasPrefix(membership, root+"/")
}

type cgroupLevelLimitState uint8

const (
	cgroupLevelLimitAbsent cgroupLevelLimitState = iota
	cgroupLevelLimitFinite
	cgroupLevelLimitUnlimited
	cgroupLevelLimitStateLimit
)

// cgroupLevelLimit is one cgroup level's memory declaration. The discriminator
// makes absence, a finite ceiling, and an unlimited declaration mutually
// exclusive.
type cgroupLevelLimit struct {
	value uint64
	state cgroupLevelLimitState
}

func (l cgroupLevelLimit) Validate() error {
	switch l.state {
	case cgroupLevelLimitAbsent, cgroupLevelLimitUnlimited:
		if l.value != 0 {
			return core.ErrHostFactsObservation
		}
		return nil
	case cgroupLevelLimitFinite:
		return nil
	default:
		return core.ErrHostFactsObservation
	}
}

// cgroupLimitFold folds the finite ceilings declared from the current cgroup
// through its mounted ancestors.
//
// It records the deepest level that declared an interface at all, because a
// hierarchy where no level declares one carries no observable limit fact, and
// under cgroup v2 that is the normal shape of the mount root: the kernel exposes
// memory.max only on non-root cgroups, so the topmost level walked almost always
// declares nothing.
type cgroupLimitFold struct {
	closest core.AbsolutePath
	path    core.AbsolutePath
	minimum uint64
}

func (f *cgroupLimitFold) consider(
	interfacePath core.AbsolutePath,
	level cgroupLevelLimit,
) error {
	if err := errors.Join(interfacePath.Validate(), level.Validate()); err != nil {
		return errors.Join(core.ErrHostFactsObservation, err)
	}
	if level.state == cgroupLevelLimitAbsent {
		return nil
	}
	if f.closest == (core.AbsolutePath{}) {
		f.closest = interfacePath
	}
	if level.state == cgroupLevelLimitFinite &&
		(f.path == (core.AbsolutePath{}) || level.value < f.minimum) {
		f.path, f.minimum = interfacePath, level.value
	}
	return nil
}

// close projects the fold. The interface path reported for an unlimited result
// is the closest present declaration, not the topmost level the walk happened
// to end on.
func (f cgroupLimitFold) close(
	source WorkloadMemoryLimitSource,
) (WorkloadMemoryLimit, error) {
	if f.closest == (core.AbsolutePath{}) {
		return unavailableWorkloadMemoryLimit()
	}
	if f.path != (core.AbsolutePath{}) {
		return validateWorkloadMemoryLimit(WorkloadMemoryLimit{
			state: WorkloadMemoryLimitLimited, source: source,
			limit: core.NewByteLength(f.minimum), path: f.path,
		})
	}
	return validateWorkloadMemoryLimit(WorkloadMemoryLimit{
		state: WorkloadMemoryLimitUnlimited, source: source, path: f.closest,
	})
}

type virtualFileRequest struct {
	Path         core.AbsolutePath
	MaximumBytes uint64
}

func (r virtualFileRequest) Validate() error {
	if err := r.Path.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	if r.MaximumBytes == 0 || r.MaximumBytes > virtualFileMaximumBytes {
		return core.ErrHostFactsContract
	}
	return nil
}

// parseCgroupMembershipLine projects one /proc/self/cgroup line. The membership's
// own source discriminates the outcome, so a line that names no memory-capable
// hierarchy returns the zero membership with an unknown source rather than a pair
// of parallel booleans that could both be set at once.
func parseCgroupMembershipLine(line []byte) (cgroupMembership, error) {
	parts := bytes.SplitN(line, []byte{':'}, 3)
	if len(parts) != 3 || len(parts[2]) == 0 {
		return cgroupMembership{}, core.ErrHostFactsObservation
	}
	membership := cgroupMembership{path: string(parts[2])}
	if string(parts[0]) == "0" && len(parts[1]) == 0 {
		membership.source = WorkloadMemoryLimitSourceCgroupV2
		return membership, membership.Validate()
	}
	if commaTokenContains(string(parts[1]), cgroupMemoryController) {
		membership.source = WorkloadMemoryLimitSourceCgroupV1
		return membership, membership.Validate()
	}
	return cgroupMembership{}, nil
}

func parseMountInfoLine(line []byte, source WorkloadMemoryLimitSource) (cgroupMount, bool, error) {
	fields := strings.Fields(string(line))
	separator := mountInfoSeparator(fields)
	if separator < 6 || separator+3 >= len(fields) {
		return cgroupMount{}, false, core.ErrHostFactsObservation
	}
	filesystem := fields[separator+1]
	if !mountFilesystemMatches(filesystem, fields[separator+3], source) {
		return cgroupMount{}, false, nil
	}
	root, err := decodeMountInfoPath(fields[3])
	if err != nil || !validCgroupPath(root) {
		return cgroupMount{}, false, errors.Join(core.ErrHostFactsObservation, err)
	}
	mountPointText, err := decodeMountInfoPath(fields[4])
	if err != nil {
		return cgroupMount{}, false, err
	}
	mountPoint, err := core.ParseAbsolutePath(mountPointText)
	if err != nil {
		return cgroupMount{}, false, err
	}
	mount := cgroupMount{root: root, mountPoint: mountPoint, source: source}
	return mount, true, mount.Validate()
}

func mountInfoSeparator(fields []string) int {
	for index, field := range fields {
		if field == "-" {
			return index
		}
	}
	return -1
}

func mountFilesystemMatches(filesystem, superOptions string, source WorkloadMemoryLimitSource) bool {
	switch source {
	case WorkloadMemoryLimitSourceCgroupV2:
		return filesystem == "cgroup2"
	case WorkloadMemoryLimitSourceCgroupV1:
		return filesystem == "cgroup" && commaTokenContains(superOptions, cgroupMemoryController)
	default:
		return false
	}
}

func foldCgroupLimits(
	ctx context.Context,
	membership cgroupMembership,
	mount cgroupMount,
) (WorkloadMemoryLimit, error) {
	current, err := resolveCgroupDirectory(membership.path, mount)
	if err != nil {
		return WorkloadMemoryLimit{}, err
	}
	component, err := limitComponent(membership.source)
	if err != nil {
		return WorkloadMemoryLimit{}, err
	}
	var folded cgroupLimitFold
	for range core.FilesystemPathMaximumComponents {
		if err = foldOneCgroupLevel(ctx, &folded, cgroupLevelRequest{
			directory: current, component: component, source: membership.source,
		}); err != nil {
			return WorkloadMemoryLimit{}, err
		}
		if current == mount.mountPoint {
			return folded.close(membership.source)
		}
		var parent core.AbsolutePath
		parent, err = current.Parent()
		if err != nil || parent == current {
			return WorkloadMemoryLimit{}, errors.Join(core.ErrHostFactsObservation, err)
		}
		current = parent
	}
	return WorkloadMemoryLimit{}, core.ErrHostFactsObservation
}

// cgroupLevelRequest binds one cgroup directory to the interface it is read
// through.
type cgroupLevelRequest struct {
	directory core.AbsolutePath
	component core.PathComponent
	source    WorkloadMemoryLimitSource
}

func foldOneCgroupLevel(
	ctx context.Context,
	folded *cgroupLimitFold,
	request cgroupLevelRequest,
) error {
	interfacePath, err := request.directory.Join(request.component)
	if err != nil {
		return err
	}
	level, err := readCgroupLevelLimit(ctx, interfacePath, request.source)
	if err != nil {
		return err
	}
	return folded.consider(interfacePath, level)
}

// readCgroupLevelLimit reads one level's declaration. A missing interface file is
// the kernel reporting that this level declares no memory ceiling, so it folds in
// as an absent declaration instead of failing the whole observation.
func readCgroupLevelLimit(
	ctx context.Context,
	interfacePath core.AbsolutePath,
	source WorkloadMemoryLimitSource,
) (cgroupLevelLimit, error) {
	value, unlimited, err := readCgroupLimit(ctx, interfacePath, source)
	if errors.Is(err, fs.ErrNotExist) {
		return cgroupLevelLimit{state: cgroupLevelLimitAbsent}, nil
	}
	if err != nil {
		return cgroupLevelLimit{}, err
	}
	state := cgroupLevelLimitFinite
	if unlimited {
		state = cgroupLevelLimitUnlimited
	}
	return cgroupLevelLimit{value: value, state: state}, nil
}

func resolveCgroupDirectory(membershipPath string, mount cgroupMount) (core.AbsolutePath, error) {
	relative := ""
	if mount.root == "/" {
		relative = strings.TrimPrefix(membershipPath, "/")
	} else if membershipPath == mount.root {
		relative = ""
	} else if strings.HasPrefix(membershipPath, mount.root+"/") {
		relative = strings.TrimPrefix(membershipPath, mount.root+"/")
	} else {
		return core.AbsolutePath{}, core.ErrHostFactsObservation
	}
	resolved := filepath.Join(mount.mountPoint.String(), filepath.FromSlash(relative))
	return core.ParseAbsolutePath(resolved)
}

func limitComponent(source WorkloadMemoryLimitSource) (core.PathComponent, error) {
	switch source {
	case WorkloadMemoryLimitSourceCgroupV2:
		return cgroupV2LimitComponent()
	case WorkloadMemoryLimitSourceCgroupV1:
		return cgroupV1LimitComponent()
	default:
		return core.PathComponent{}, core.ErrHostFactsContract
	}
}

func readCgroupLimit(
	ctx context.Context,
	interfacePath core.AbsolutePath,
	source WorkloadMemoryLimitSource,
) (uint64, bool, error) {
	data, err := readVirtualValue(ctx, virtualFileRequest{
		Path: interfacePath, MaximumBytes: limitValueMaximumBytes,
	})
	if err != nil {
		return 0, false, err
	}
	token, err := canonicalLimitToken(data)
	if err != nil {
		return 0, false, err
	}
	if source == WorkloadMemoryLimitSourceCgroupV2 && token == cgroupV2MaxToken {
		return 0, true, nil
	}
	value, err := strconv.ParseUint(token, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != token {
		return 0, false, errors.Join(core.ErrHostFactsObservation, err)
	}
	if source == WorkloadMemoryLimitSourceCgroupV1 && value >= cgroupV1UnlimitedMin {
		return 0, true, nil
	}
	return value, false, nil
}

func canonicalLimitToken(data []byte) (string, error) {
	token := strings.TrimSuffix(string(data), "\n")
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", core.ErrHostFactsObservation
	}
	return token, nil
}

type boundedLineScan struct {
	reader  io.Reader
	visit   func([]byte) error
	maximum uint64
}

func (s boundedLineScan) run(ctx context.Context) error {
	var readBuffer [streamBufferBytes]byte
	line := make([]byte, 0, procLineMaximumBytes)
	total, emptyReads := uint64(0), 0
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		count, readErr := s.reader.Read(readBuffer[:])
		if err := validateReadCount(count, len(readBuffer)); err != nil {
			return err
		}
		countBytes, err := core.CheckedUint64FromInt64(int64(count))
		if err != nil {
			return err
		}
		total += countBytes
		if total > s.maximum {
			return core.ErrHostFactsObservation
		}
		var consumeErr error
		line, consumeErr = consumeLines(line, readBuffer[:count], s.visit)
		if consumeErr != nil {
			return consumeErr
		}
		emptyReads = nextEmptyReads(emptyReads, count)
		if done, err := finishLineRead(line, s.visit, readErr); done {
			return err
		}
		if emptyReads >= zeroReadMaximum {
			return io.ErrNoProgress
		}
	}
}

func validateReadCount(count, maximum int) error {
	if count < 0 || count > maximum {
		return core.ErrHostFactsObservation
	}
	return nil
}

func nextEmptyReads(current, count int) int {
	if count != 0 {
		return 0
	}
	return current + 1
}

func finishLineRead(
	line []byte,
	visit func([]byte) error,
	readErr error,
) (bool, error) {
	if errors.Is(readErr, io.EOF) {
		return true, finishLines(line, visit)
	}
	if readErr != nil {
		return true, readErr
	}
	return false, nil
}

func consumeLines(line, data []byte, visit func([]byte) error) ([]byte, error) {
	for len(data) > 0 {
		newline := bytes.IndexByte(data, '\n')
		if newline < 0 {
			if len(line)+len(data) > procLineMaximumBytes {
				return line, core.ErrHostFactsObservation
			}
			return append(line, data...), nil
		}
		if len(line)+newline > procLineMaximumBytes {
			return line, core.ErrHostFactsObservation
		}
		line = append(line, data[:newline]...)
		if len(line) == 0 {
			return line, core.ErrHostFactsObservation
		}
		if err := visit(line); err != nil {
			return line, err
		}
		line = line[:0]
		data = data[newline+1:]
	}
	return line, nil
}

func finishLines(line []byte, visit func([]byte) error) error {
	if len(line) == 0 {
		return nil
	}
	return visit(line)
}

func readVirtualValue(
	ctx context.Context,
	request virtualFileRequest,
) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	closed := false
	file, err := os.Open(request.Path.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	data, readErr := readBoundedValue(ctx, file, request.MaximumBytes)
	closeErr := file.Close()
	closed = true
	return data, errors.Join(readErr, closeErr)
}

func readBoundedValue(ctx context.Context, reader io.Reader, maximum uint64) ([]byte, error) {
	storage := make([]byte, maximum+1)
	written, emptyReads := 0, 0
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return nil, err
		}
		count, readErr := reader.Read(storage[written:])
		if err := validateReadCount(count, len(storage)-written); err != nil {
			return nil, err
		}
		written += count
		writtenBytes, err := core.CheckedUint64FromInt64(int64(written))
		if err != nil || writtenBytes > maximum {
			return nil, core.ErrHostFactsObservation
		}
		emptyReads = nextEmptyReads(emptyReads, count)
		if done, err := finishValueRead(readErr); done {
			return storage[:written], err
		}
		if emptyReads >= zeroReadMaximum {
			return nil, io.ErrNoProgress
		}
	}
}

func finishValueRead(readErr error) (bool, error) {
	if errors.Is(readErr, io.EOF) {
		return true, nil
	}
	if readErr != nil {
		return true, readErr
	}
	return false, nil
}

func commaTokenContains(value, token string) bool {
	if token == "" {
		return false
	}
	for item := range strings.SplitSeq(value, ",") {
		if item == token {
			return true
		}
	}
	return false
}

func validCgroupPath(value string) bool {
	return value != "" && utf8.ValidString(value) && strings.HasPrefix(value, "/") &&
		path.Clean(value) == value && !strings.ContainsRune(value, '\x00') &&
		!strings.HasSuffix(value, " (deleted)")
}

func decodeMountInfoPath(value string) (string, error) {
	decoded := make([]byte, 0, len(value))
	for index := 0; index < len(value); {
		if value[index] != '\\' {
			decoded = append(decoded, value[index])
			index++
			continue
		}
		if index+3 >= len(value) {
			return "", core.ErrHostFactsObservation
		}
		octal := value[index+1 : index+4]
		if octal != "040" && octal != "011" && octal != "012" && octal != "134" {
			return "", core.ErrHostFactsObservation
		}
		parsed, err := strconv.ParseUint(octal, 8, 8)
		if err != nil {
			return "", errors.Join(core.ErrHostFactsObservation, err)
		}
		decoded = append(decoded, byte(parsed))
		index += 4
	}
	if !utf8.Valid(decoded) {
		return "", core.ErrHostFactsObservation
	}
	return string(decoded), nil
}
