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
// can belong to an Anvil subject. It does not scan unrelated host state into
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
		return errors.Join(core.ErrPrimitiveContract, errors.New("Linux residue process user ID must be nonzero"))
	}
	return nil
}

// LinuxResidueSource streams the live Linux process and ownership surfaces
// under the configured, reviewed Anvil coordinates.
type LinuxResidueSource struct{ configuration LinuxResidueConfiguration }

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
	processes, descriptors, sockets, err := s.observeSubjectProcesses(ctx)
	if err != nil {
		return Residue{}, err
	}
	controlGroups, err := countPrefixedEntries(ctx, s.configuration.ControlGroupRoot, s.configuration.UnitPrefix.String())
	if err != nil {
		return Residue{}, fmt.Errorf("observe Anvil control-group residue: %w", err)
	}
	namespaces, err := countPrefixedEntries(ctx, s.configuration.NetworkNamespaceRoot, s.configuration.UnitPrefix.String())
	if err != nil {
		return Residue{}, fmt.Errorf("observe Anvil network-namespace residue: %w", err)
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
		Processes: processes, ControlGroups: controlGroups, Namespaces: namespaces, Mounts: mounts,
		Descriptors: descriptors, Sockets: sockets, CredentialCustody: credentials, SecretCustody: secrets,
	}, nil
}

func (s LinuxResidueSource) observeSubjectProcesses(ctx context.Context) (uint32, uint32, uint32, error) {
	directory, err := os.Open(s.configuration.ProcRoot.String())
	if err != nil {
		return 0, 0, 0, fmt.Errorf("open Linux process table: %w", err)
	}
	var processes uint32
	var descriptors uint32
	var sockets uint32
	for {
		if err := ctx.Err(); err != nil {
			return 0, 0, 0, errors.Join(err, directory.Close())
		}
		entries, readErr := directory.ReadDir(residueReadBatch)
		for _, entry := range entries {
			if !entry.IsDir() || !decimalName(entry.Name()) {
				continue
			}
			owned, ownershipErr := s.processOwnedBySubject(entry.Name())
			if errors.Is(ownershipErr, fs.ErrNotExist) {
				continue
			}
			if ownershipErr != nil {
				return 0, 0, 0, errors.Join(ownershipErr, directory.Close())
			}
			if !owned {
				continue
			}
			if incrementErr := incrementResidue(&processes); incrementErr != nil {
				return 0, 0, 0, errors.Join(incrementErr, directory.Close())
			}
			ownedDescriptors, ownedSockets, descriptorErr := s.observeProcessDescriptors(ctx, entry.Name())
			if errors.Is(descriptorErr, fs.ErrNotExist) {
				continue
			}
			descriptorCountErr := addResidue(&descriptors, ownedDescriptors)
			socketCountErr := addResidue(&sockets, ownedSockets)
			if err := errors.Join(descriptorErr, descriptorCountErr, socketCountErr); err != nil {
				return 0, 0, 0, errors.Join(err, directory.Close())
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return 0, 0, 0, errors.Join(readErr, directory.Close())
		}
	}
	if closeErr := directory.Close(); closeErr != nil {
		return 0, 0, 0, closeErr
	}
	return processes, descriptors, sockets, nil
}

func (s LinuxResidueSource) processOwnedBySubject(processID string) (bool, error) {
	status, err := os.Open(filepath.Join(s.configuration.ProcRoot.String(), processID, "status"))
	if err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(status)
	scanner.Buffer(make([]byte, 256), residueLineMaximum)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		uid, parseErr := ParseLinuxStatusUIDRow(line)
		return uid == s.configuration.ProcessUserID, errors.Join(parseErr, status.Close())
	}
	return false, errors.Join(core.ErrPrimitiveContract, scanner.Err(), status.Close(), errors.New("Linux process status has no Uid row"))
}

func (s LinuxResidueSource) observeProcessDescriptors(ctx context.Context, processID string) (uint32, uint32, error) {
	directory, err := os.Open(filepath.Join(s.configuration.ProcRoot.String(), processID, "fd"))
	if err != nil {
		return 0, 0, err
	}
	var descriptors uint32
	var sockets uint32
	for {
		if err := ctx.Err(); err != nil {
			return 0, 0, errors.Join(err, directory.Close())
		}
		entries, readErr := directory.ReadDir(residueReadBatch)
		for _, entry := range entries {
			target, linkErr := os.Readlink(filepath.Join(s.configuration.ProcRoot.String(), processID, "fd", entry.Name()))
			if errors.Is(linkErr, fs.ErrNotExist) {
				continue
			}
			if linkErr != nil {
				return 0, 0, errors.Join(linkErr, directory.Close())
			}
			if pathWithin(target, s.configuration.RunParent.String()) {
				if err := incrementResidue(&descriptors); err != nil {
					return 0, 0, errors.Join(err, directory.Close())
				}
			}
			if strings.HasPrefix(target, "socket:[") {
				if err := incrementResidue(&sockets); err != nil {
					return 0, 0, errors.Join(err, directory.Close())
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return 0, 0, errors.Join(readErr, directory.Close())
		}
	}
	return descriptors, sockets, directory.Close()
}

func (s LinuxResidueSource) observeRunMounts(ctx context.Context) (uint32, error) {
	mountInfo := filepath.Join(s.configuration.ProcRoot.String(), "self", "mountinfo")
	file, err := os.Open(mountInfo)
	if err != nil {
		return 0, fmt.Errorf("open Linux mount table: %w", err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), residueLineMaximum)
	var count uint32
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return 0, errors.Join(err, file.Close())
		}
		mountPoint, parseErr := ParseLinuxMountInfoPoint(scanner.Text())
		if parseErr != nil {
			return 0, errors.Join(parseErr, file.Close())
		}
		if pathWithin(mountPoint, s.configuration.RunParent.String()) {
			if err := incrementResidue(&count); err != nil {
				return 0, errors.Join(err, file.Close())
			}
		}
	}
	return count, errors.Join(scanner.Err(), file.Close())
}

func countPrefixedEntries(ctx context.Context, root core.AbsolutePath, prefix string) (uint32, error) {
	return countDirectoryEntries(ctx, root, func(name string) bool { return strings.HasPrefix(name, prefix) })
}

func countAllEntries(ctx context.Context, root core.AbsolutePath) (uint32, error) {
	return countDirectoryEntries(ctx, root, func(string) bool { return true })
}

func countDirectoryEntries(ctx context.Context, root core.AbsolutePath, include func(string) bool) (uint32, error) {
	directory, err := os.Open(root.String())
	if err != nil {
		return 0, err
	}
	var count uint32
	for {
		if err := ctx.Err(); err != nil {
			return 0, errors.Join(err, directory.Close())
		}
		entries, readErr := directory.ReadDir(residueReadBatch)
		for _, entry := range entries {
			if include(entry.Name()) {
				if err := incrementResidue(&count); err != nil {
					return 0, errors.Join(err, directory.Close())
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return 0, errors.Join(readErr, directory.Close())
		}
	}
	return count, directory.Close()
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
)
