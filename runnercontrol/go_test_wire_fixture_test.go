package runnercontrol

// Typed fixtures for the external cmd/go wire; production uses a streaming projection.
type goTestEventWire struct {
	Time        string  `json:"Time"`
	Action      string  `json:"Action"`
	Package     string  `json:"Package"`
	Test        string  `json:"Test"`
	Output      string  `json:"Output"`
	OutputType  string  `json:"OutputType"`
	FailedBuild string  `json:"FailedBuild"`
	Elapsed     float64 `json:"Elapsed"`
}

type goBuildEventWire struct {
	ImportPath string `json:"ImportPath"`
	Action     string `json:"Action"`
	Output     string `json:"Output"`
}
