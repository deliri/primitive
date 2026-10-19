package gcsobjects

import (
	"net"
	"testing"
)

// gcsIPAddressSpelling skips net/netip only for spellings Go cannot parse. The
// oracle is net.ParseIP itself, so a prefilter that wrongly skipped a parseable
// address, or admitted a zoned one, would disagree here.
func TestGCSIPAddressSpellingMatchesNetParseIP(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", "1", "123", "1.2.3", "1.2.3.4", "192.168.5.4", "01.2.3.4", "256.1.1.1", "1.2.3.4.",
		".1.2.3", "1..2.3", "::", "::1", "fe80::1", "fe80::1%eth0", "::ffff:1.2.3.4", "1:2:3:4:5:6:7:8",
		"1:2:3:4:5:6:7:8:9", "abc", "my.bucket", "a.b.c.d", "1.2.3.4a", "1.2.3.4%x", "%1.2.3.4", "1.2.3.4:80",
		"bucket-with-dash", "g00gle", "ab:cd", "0.0.0.0", "255.255.255.255",
	} {
		if got, want := gcsIPAddressSpelling(value), net.ParseIP(value) != nil; got != want {
			t.Errorf("gcsIPAddressSpelling(%q) = %t, net.ParseIP admits = %t", value, got, want)
		}
	}
}

func FuzzGCSIPAddressSpellingMatchesNetParseIP(f *testing.F) {
	for _, seed := range []string{"1.2.3.4", "::1", "fe80::1%eth0", "my.bucket", "01.2.3.4"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if got, want := gcsIPAddressSpelling(value), net.ParseIP(value) != nil; got != want {
			t.Fatalf("gcsIPAddressSpelling(%q) = %t, net.ParseIP admits = %t", value, got, want)
		}
	})
}
