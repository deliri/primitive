//go:build !unix

package process

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
)

func observeGroupMemory(context.Context, GroupObservationRequest) (core.ByteLength, error) {
	return core.ByteLength{}, core.ErrProcessUnsupported
}

func ownGroupIdentity() (ProcessIdentity, error) { return 0, core.ErrProcessUnsupported }
