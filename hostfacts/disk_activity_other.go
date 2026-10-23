//go:build !linux

package hostfacts

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
)

func observeDiskActivity(context.Context, DiskActivityRequest) (DiskActivity, error) {
	return DiskActivity{}, core.ErrHostFactsUnsupported
}
