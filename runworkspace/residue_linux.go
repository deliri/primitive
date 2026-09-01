//go:build linux

package runworkspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	residueHostEntryMaximum = 1 << 16
	residueReadBatch        = 128
	residueLineMaximum      = 64 * 1024
)

// LinuxResidueConfiguration binds the fixed host coordinates whose residue
// can belong to an isolated subject. It does not scan unrelated host state into
// a world model.
type LinuxResidueConfiguration struct {
	ProcRoot             core.AbsolutePath
	RunParent            core.AbsolutePath
	ControlGroupRoot     core.AbsolutePath
	NetworkNamespaceRoot core.AbsolutePath
	CredentialRoot       core.AbsolutePath
	SecretRoot           core.AbsolutePath
	UnitPrefix           core.PathComponent
	ProcessUserID        uint32
}

func (c LinuxResidueConfiguration) Validate() error {
	if err := errors.Join(c.ProcRoot.Validate(), c.RunParent.Validate(), c.ControlGroupRoot.Validate(), c.NetworkNamespaceRoot.Validate(), c.CredentialRoot.Validate(), c.SecretRoot.Validate(), c.UnitPrefix.Validate()); err != nil {
		return err
	}
	if c.ProcessUserID == 0 {
		return errors.Join(core.ErrPrimitiveContract, errors.New("linux residue process user id must be nonzero"))
	}
	return nil
}

// LinuxResidueSource streams the live Linux process and ownership surfaces
// under the configured, reviewed execution coordinates.
type LinuxResidueSource struct{ configuration LinuxResidueConfiguration }

type residueCounts struct {
	processes   uint32
	descriptors uint32
	sockets     uint32
}

func (residueCounts) runWorkspaceInternalFlow() {}

func NewLinuxResidueSource(configuration LinuxResidueConfiguration) (LinuxResidueSource, error) {
	if err := configuration.Validate(); err != nil {
		return LinuxResidueSource{}, err
	}
	return LinuxResidueSource{configuration: configuration}, nil
}

func (s LinuxResidueSource) Validate() error { return s.configuration.Validate() }

func (s LinuxResidueSource) ObserveResidue(ctx context.Context) (Residue, error) {
	if err := s.Validate(); err != nil {
		return Residue{}, err
	}
	processResidue, err := s.observeSubjectProcesses(ctx)
	if err != nil {
		return Residue{}, err
	}
	controlGroups, err := countPrefixedEntries(ctx, s.configuration.ControlGroupRoot, s.configuration.UnitPrefix.String())
	if err != nil {
		return Residue{}, fmt.Errorf("observe control-group residue: %w", err)
	}
	namespaces, err := countPrefixedEntries(ctx, s.configuration.NetworkNamespaceRoot, s.configuration.UnitPrefix.String())
	if err != nil {
		return Residue{}, fmt.Errorf("observe network-namespace residue: %w", err)
	}
	mounts, err := s.observeRunMounts(ctx)
	if err != nil {
		return Residue{}, err
	}
	credentials, err := countAllEntries(ctx, s.configuration.CredentialRoot)
	if err != nil {
		return Residue{}, fmt.Errorf("observe credential custody residue: %w", err)
	}
	secrets, err := countAllEntries(ctx, s.configuration.SecretRoot)
	if err != nil {
		return Residue{}, fmt.Errorf("observe secret custody residue: %w", err)
	}
	return Residue{
		Processes: processResidue.processes, ControlGroups: controlGroups, Namespaces: namespaces, Mounts: mounts,
		Descriptors: processResidue.descriptors, Sockets: processResidue.sockets, CredentialCustody: credentials, SecretCustody: secrets,
	}, nil
}

func (s LinuxResidueSource) observeSubjectProcesses(ctx context.Context) (counts residueCounts, resultErr error) {
	directory, err := os.Open(s.configuration.ProcRoot.String())
	if err != nil {
		return residueCounts{}, fmt.Errorf("open Linux process table: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	emptyReads := 0
	for {
		if err := ctx.Err(); err != nil {
			return residueCounts{}, err
		}
		entries, readErr := readResidueDirectoryBatch(directory, &emptyReads)
		for _, entry := range entries {
			contribution, observeErr := s.observeProcessEntry(ctx, entry)
			if observeErr != nil {
				return residueCounts{}, observeErr
			}
			if err := counts.add(contribution); err != nil {
				return residueCounts{}, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return residueCounts{}, readErr
		}
	}
	return counts, nil
}

func (s LinuxResidueSource) observeProcessEntry(ctx context.Context, entry fs.DirEntry) (residueCounts, error) {
	if !entry.IsDir() || !decimalName(entry.Name()) {
		return residueCounts{}, nil
	}
	owned, err := s.processOwnedBySubject(entry.Name())
	if errors.Is(err, fs.ErrNotExist) {
		return residueCounts{}, nil
	}
	if err != nil || !owned {
		return residueCounts{}, err
	}
	counts := residueCounts{processes: 1}
	descriptors, descriptorErr := s.observeProcessDescriptors(ctx, entry.Name())
	if errors.Is(descriptorErr, fs.ErrNotExist) {
		return counts, nil
	}
	if descriptorErr != nil {
		return residueCounts{}, descriptorErr
	}
	counts.descriptors = descriptors.descriptors
	counts.sockets = descriptors.sockets
	return counts, nil
}

func (c *residueCounts) add(other residueCounts) error {
	return errors.Join(
		addResidue(&c.processes, other.processes),
		addResidue(&c.descriptors, other.descriptors),
		addResidue(&c.sockets, other.sockets),
	)
}

func (s LinuxResidueSource) processOwnedBySubject(processID string) (owned bool, resultErr error) {
	status, err := os.Open(filepath.Join(s.configuration.ProcRoot.String(), processID, "status"))
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, status.Close()) }()
	scanner := bufio.NewScanner(status)
	scanner.Buffer(make([]byte, 256), residueLineMaximum)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		uid, parseErr := ParseLinuxStatusUIDRow(line)
		return uid == s.configuration.ProcessUserID, parseErr
	}
	return false, errors.Join(core.ErrPrimitiveContract, scanner.Err(), errors.New("linux process status has no uid row"))
}

func (s LinuxResidueSource) observeProcessDescriptors(ctx context.Context, processID string) (counts residueCounts, resultErr error) {
	directory, err := os.Open(filepath.Join(s.configuration.ProcRoot.String(), processID, "fd"))
	if err != nil {
		return residueCounts{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	emptyReads := 0
	for {
		if err := ctx.Err(); err != nil {
			return residueCounts{}, err
		}
		entries, readErr := readResidueDirectoryBatch(directory, &emptyReads)
		for _, entry := range entries {
			contribution, observeErr := s.observeDescriptor(processID, entry.Name())
			if observeErr != nil {
				return residueCounts{}, observeErr
			}
			if err := counts.add(contribution); err != nil {
				return residueCounts{}, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return residueCounts{}, readErr
		}
	}
	return counts, nil
}

func (s LinuxResidueSource) observeDescriptor(processID, descriptorName string) (residueCounts, error) {
	target, err := os.Readlink(filepath.Join(s.configuration.ProcRoot.String(), processID, "fd", descriptorName))
	if errors.Is(err, fs.ErrNotExist) {
		return residueCounts{}, nil
	}
	if err != nil {
		return residueCounts{}, err
	}
	var counts residueCounts
	if pathWithin(target, s.configuration.RunParent.String()) {
		counts.descriptors = 1
	}
	if strings.HasPrefix(target, "socket:[") {
		counts.sockets = 1
	}
	return counts, nil
}

func (s LinuxResidueSource) observeRunMounts(ctx context.Context) (count uint32, resultErr error) {
	mountInfo := filepath.Join(s.configuration.ProcRoot.String(), "self", "mountinfo")
	file, err := os.Open(mountInfo)
	if err != nil {
		return 0, fmt.Errorf("open Linux mount table: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), residueLineMaximum)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		mountPoint, parseErr := ParseLinuxMountInfoPoint(scanner.Text())
		if parseErr != nil {
			return 0, parseErr
		}
		if pathWithin(mountPoint, s.configuration.RunParent.String()) {
			if err := incrementResidue(&count); err != nil {
				return 0, err
			}
		}
	}
	return count, scanner.Err()
}

func countPrefixedEntries(ctx context.Context, root core.AbsolutePath, prefix string) (uint32, error) {
	return countDirectoryEntries(ctx, root, func(name string) bool { return strings.HasPrefix(name, prefix) })
}

func countAllEntries(ctx context.Context, root core.AbsolutePath) (uint32, error) {
	return countDirectoryEntries(ctx, root, func(string) bool { return true })
}

func countDirectoryEntries(ctx context.Context, root core.AbsolutePath, include func(string) bool) (count uint32, resultErr error) {
	directory, err := os.Open(root.String())
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	emptyReads := 0
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		entries, readErr := readResidueDirectoryBatch(directory, &emptyReads)
		for _, entry := range entries {
			if include(entry.Name()) {
				if err := incrementResidue(&count); err != nil {
					return 0, err
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return 0, readErr
		}
	}
	return count, nil
}

func readResidueDirectoryBatch(directory *os.File, emptyReads *int) ([]fs.DirEntry, error) {
	entries, err := directory.ReadDir(residueReadBatch)
	if len(entries) != 0 || err != nil {
		*emptyReads = 0
		return entries, err
	}
	*emptyReads++
	if *emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
		return nil, io.ErrNoProgress
	}
	return nil, nil
}

func decimalName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func pathWithin(value, root string) bool {
	return value == root || strings.HasPrefix(value, root+string(os.PathSeparator))
}

func incrementResidue(value *uint32) error {
	if *value >= residueHostEntryMaximum || *value == math.MaxUint32 {
		return errors.Join(core.ErrPrimitiveContract, errors.New("host residue exceeds the bounded observation ceiling"))
	}
	*value++
	return nil
}

func addResidue(value *uint32, addition uint32) error {
	if addition > residueHostEntryMaximum-*value {
		return errors.Join(core.ErrPrimitiveContract, errors.New("host residue aggregate exceeds the bounded observation ceiling"))
	}
	*value += addition
	return nil
}

var (
	_ core.Validatable = LinuxResidueConfiguration{}
	_ core.Validatable = LinuxResidueSource{}
	_ ResidueSource    = LinuxResidueSource{}
	_ internalFlow     = residueCounts{}
)
