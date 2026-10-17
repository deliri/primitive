package background_test

import (
	"github.com/deliri/primitive/v2026/background"
	"testing"
)

type wallGapIngressProof[Door any] struct {
	Door Door
	Fuzz func(*testing.F)
}

var (
	_ = wallGapIngressProof[func(background.WallGapWatchRequest) (*background.WallGapWatch, error)]{Door: background.OpenWallGapWatch, Fuzz: FuzzWallGapWatchAdmissionAndJoinedSnapshot}
	_ = wallGapIngressProof[func(background.WallGapWatchRequest) error]{Door: background.WallGapWatchRequest.Validate, Fuzz: FuzzWallGapWatchAdmissionAndJoinedSnapshot}
)
