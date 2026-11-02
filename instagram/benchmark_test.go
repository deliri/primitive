package instagram

import (
	"encoding/json"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

var benchmarkPageSink MediaPage

// BenchmarkMediaListProjection measures the hot path a feed refresh pays: a
// full twelve-reel page from bytes to validated media.
func BenchmarkMediaListProjection(b *testing.B) {
	list := make([]mediaWire, 12)
	for i := range list {
		list[i] = reel(string(rune('1'+i%9)) + "0" + string(rune('1'+i/9)))
	}
	raw, err := json.Marshal(mediaListWire{Data: list})
	if err != nil {
		b.Fatal(err)
	}
	if page, err := decodeAndProject(raw); err != nil || len(page.Media) != 12 {
		b.Fatalf("fixture is vacuous: %v %d", err, len(page.Media))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		page, err := decodeAndProject(raw)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPageSink = page
	}
	b.StopTimer()
	if len(benchmarkPageSink.Media) != 12 {
		b.Fatal("the timed work did not happen")
	}
}

func decodeAndProject(raw []byte) (MediaPage, error) {
	wire, err := core.DecodeStrictJSONBytes[mediaListWire](raw, core.ExtensibleJSONLimits())
	if err != nil {
		return MediaPage{}, err
	}
	return wire.page(core.InstagramMediaPageMaximum)
}
