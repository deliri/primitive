//go:build windows

package process

import (
	"context"
	"errors"
	"reflect"

	"golang.org/x/sys/windows"

	"github.com/deliri/primitive/v2026/core"
)

// observeProcesses walks the kernel Toolhelp snapshot. Entries whose recorded
// image or identity fails admission are skipped rather than surfaced: the
// snapshot reports the kernel's bookkeeping, and a row this stack cannot type
// is not a fact a caller can act on.
func observeProcesses(ctx context.Context, visit ProcessVisit) (resultErr error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return errors.Join(core.ErrProcessObservation, err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, windows.CloseHandle(snapshot))
	}()
	var entry windows.ProcessEntry32
	entry.Size = uint32(reflect.TypeFor[windows.ProcessEntry32]().Size()) // #nosec G115 -- a fixed kernel struct's size fits the field the kernel defined for it.
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return errors.Join(core.ErrProcessObservation, err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visitSnapshotEntry(entry, visit); err != nil {
			return err
		}
		nextErr := windows.Process32Next(snapshot, &entry)
		if errors.Is(nextErr, windows.ERROR_NO_MORE_FILES) {
			return nil
		}
		if nextErr != nil {
			return errors.Join(core.ErrProcessObservation, nextErr)
		}
	}
}

func visitSnapshotEntry(entry windows.ProcessEntry32, visit ProcessVisit) error {
	image, err := core.ParsePathComponent(windows.UTF16ToString(entry.ExeFile[:]))
	if err != nil {
		return nil
	}
	sighting := ProcessSighting{
		Identity: ProcessIdentity(entry.ProcessID), // #nosec G115 -- the kernel snapshot reports identifiers inside the platform pid domain.
		Image:    image,
	}
	if sighting.Validate() != nil {
		return nil
	}
	return visit(sighting)
}
