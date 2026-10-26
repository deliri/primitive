//go:build !unix

package process

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
)

func observeGroupUsage(context.Context, GroupMemoryRequest) (GroupUsage, error) {
	return GroupUsage{}, core.ErrProcessUnsupported
}

func ownGroupIdentity() (ProcessIdentity, error) { return 0, core.ErrProcessUnsupported }
