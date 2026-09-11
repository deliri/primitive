package github

type treeEntryFixture struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
	Size uint64 `json:"size,omitzero"`
}
