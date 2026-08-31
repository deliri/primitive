package runworkspace

import (
	"errors"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// ParseLinuxStatusUIDRow admits exactly one Linux /proc status Uid row and
// returns its real-user identity. Effective, saved, and filesystem identities
// remain required so truncated rows cannot be mistaken for authority.
func ParseLinuxStatusUIDRow(line string) (uint32, error) {
	if !strings.HasPrefix(line, "Uid:") {
		return 0, errors.Join(core.ErrPrimitiveContract, errors.New("Linux process status row is not Uid"))
	}
	fields := strings.Fields(strings.TrimPrefix(line, "Uid:"))
	if len(fields) != 4 {
		return 0, errors.Join(core.ErrPrimitiveContract, errors.New("Linux process Uid row does not contain four identities"))
	}
	for _, field := range fields {
		if field == "" || (len(field) > 1 && field[0] == '0') {
			return 0, errors.Join(core.ErrPrimitiveContract, errors.New("Linux process Uid identity is not canonical decimal"))
		}
		for _, character := range field {
			if character < '0' || character > '9' {
				return 0, errors.Join(core.ErrPrimitiveContract, errors.New("Linux process Uid identity contains a non-decimal character"))
			}
		}
	}
	uid, err := strconv.ParseUint(fields[0], 10, 32)
	if err != nil {
		return 0, errors.Join(core.ErrPrimitiveContract, err)
	}
	return uint32(uid), nil
}

// ParseLinuxMountInfoPoint admits one complete Linux mountinfo row and
// returns its kernel-escaped mount-point field. The caller compares it only
// with compiler-owned paths that use the same no-whitespace path contract.
func ParseLinuxMountInfoPoint(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return "", errors.Join(core.ErrPrimitiveContract, errors.New("Linux mountinfo row is incomplete"))
	}
	separator := -1
	for index := 6; index < len(fields); index++ {
		if fields[index] == "-" {
			separator = index
			break
		}
	}
	if separator < 6 || separator+3 > len(fields) || fields[4] == "" || fields[4][0] != '/' {
		return "", errors.Join(core.ErrPrimitiveContract, errors.New("Linux mountinfo row has no complete separator or absolute mount point"))
	}
	return fields[4], nil
}
