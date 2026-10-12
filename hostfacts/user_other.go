//go:build !linux && !darwin

package hostfacts

import "github.com/deliri/primitive/v2026/core"

func observeEffectiveUser() (string, error) { return "", core.ErrHostFactsUnsupported }
