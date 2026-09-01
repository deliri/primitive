package runworkspace_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runworkspace"
)

func TestLinuxResidueIngressLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive canonical process identity and mount point retain exact host facts", func(t *testing.T) {
		t.Parallel()
		uid, uidErr := runworkspace.ParseLinuxStatusUIDRow("Uid:\t1000\t1000\t1000\t1000")
		mount, mountErr := runworkspace.ParseLinuxMountInfoPoint("29 23 0:26 / /var/lib/primitive/runs rw,nosuid - tmpfs tmpfs rw")
		if uidErr != nil || uid != 1000 || mountErr != nil || mount != "/var/lib/primitive/runs" {
			t.Fatalf("Linux residue parsers = (uid %d, uid error %v, mount %q, mount error %v), want (1000, nil, /var/lib/primitive/runs, nil)", uid, uidErr, mount, mountErr)
		}
	})

	t.Run("negative truncated host rows retain typed refusal identities", func(t *testing.T) {
		t.Parallel()
		uid, uidErr := runworkspace.ParseLinuxStatusUIDRow("Uid:\t1000")
		mount, mountErr := runworkspace.ParseLinuxMountInfoPoint("29 23 0:26 / /var/lib/primitive/runs")
		if uid != 0 || !errors.Is(uidErr, core.ErrPrimitiveContract) || mount != "" || !errors.Is(mountErr, core.ErrPrimitiveContract) {
			t.Fatalf("Linux residue truncated rows = (uid %d, uid error %v, mount %q, mount error %v), want zero values and errors.Is(..., %v)", uid, uidErr, mount, mountErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral root identity and unrelated root mount remain exact rather than becoming residue", func(t *testing.T) {
		t.Parallel()
		uid, uidErr := runworkspace.ParseLinuxStatusUIDRow("Uid:\t0\t0\t0\t0")
		mount, mountErr := runworkspace.ParseLinuxMountInfoPoint("1 0 8:1 / / rw - ext4 /dev/root rw")
		if uidErr != nil || uid != 0 || mountErr != nil || mount != "/" {
			t.Fatalf("Linux residue neutral rows = (uid %d, uid error %v, mount %q, mount error %v), want (0, nil, /, nil)", uid, uidErr, mount, mountErr)
		}
	})
}

func FuzzLinuxStatusUIDRowSemanticClosure(f *testing.F) {
	for _, uid := range []uint32{0, 1, 1000, 65534, ^uint32(0)} {
		value := strconv.FormatUint(uint64(uid), 10)
		f.Add("Uid:\t" + value + "\t" + value + "\t" + value + "\t" + value)
	}
	for _, malformed := range []string{"", "Uid:", "Uid:\t1", "Gid:\t1\t1\t1\t1", "Uid:\t-1\t0\t0\t0", "Uid:\t01\t1\t1\t1"} {
		f.Add(malformed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := runworkspace.ParseLinuxStatusUIDRow(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("ParseLinuxStatusUIDRow(%q) error = %v, want errors.Is(..., %v)", input, gotErr, core.ErrPrimitiveContract)
			}
			return
		}
		value := strconv.FormatUint(uint64(got), 10)
		canonical := "Uid:\t" + strings.Join([]string{value, value, value, value}, "\t")
		roundTrip, roundTripErr := runworkspace.ParseLinuxStatusUIDRow(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("Linux Uid canonical closure for %q = (%d, %v), want (%d, nil)", input, roundTrip, roundTripErr, got)
		}
	})
}

func FuzzLinuxMountInfoPointSemanticClosure(f *testing.F) {
	for _, mount := range []string{"/", "/var/lib/primitive/runs", "/run/netns"} {
		f.Add("29 23 0:26 / " + mount + " rw,nosuid - tmpfs tmpfs rw")
	}
	for _, malformed := range []string{"", "29 23", "29 23 0:26 / relative rw - tmpfs tmpfs rw", "29 23 0:26 / /run rw tmpfs tmpfs rw"} {
		f.Add(malformed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := runworkspace.ParseLinuxMountInfoPoint(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("ParseLinuxMountInfoPoint(%q) error = %v, want errors.Is(..., %v)", input, gotErr, core.ErrPrimitiveContract)
			}
			return
		}
		canonical := "29 23 0:26 / " + got + " rw,nosuid - tmpfs tmpfs rw"
		roundTrip, roundTripErr := runworkspace.ParseLinuxMountInfoPoint(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("Linux mountinfo canonical closure for %q = (%q, %v), want (%q, nil)", input, roundTrip, roundTripErr, got)
		}
	})
}
