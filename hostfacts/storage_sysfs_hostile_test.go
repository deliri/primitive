package hostfacts

import (
	"errors"
	"io/fs"
	"testing"
)

func TestResolveSysfsDeviceDirectoryRefusesEveryEscapeForm(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr error
		name    string
		target  string
		want    string
	}{
		{name: "relative kernel device target remains below sysfs", target: "../../devices/pci0000:00/block/sda", want: "/sys/devices/pci0000:00/block/sda"},
		{name: "absolute kernel device target remains below sysfs", target: "/sys/devices/virtual/block/loop0", want: "/sys/devices/virtual/block/loop0"},
		{name: "relative sibling below dev remains confined", target: "../devices/block/sda", want: "/sys/dev/devices/block/sda"},
		{name: "cleaned dot components remain confined", target: "../../devices/./block/../block/sda", want: "/sys/devices/block/sda"},
		{name: "sysfs root is refused because its fallback parent escapes", target: "/sys", wantErr: fs.ErrInvalid},
		{name: "empty target is refused", target: "", wantErr: fs.ErrInvalid},
		{name: "absolute temporary path is refused", target: "/tmp/x", wantErr: fs.ErrInvalid},
		{name: "relative traversal into temporary path is refused", target: "../../../../tmp/x", wantErr: fs.ErrInvalid},
		{name: "absolute sys prefix sibling is refused", target: "/sysfake/device", wantErr: fs.ErrInvalid},
		{name: "relative traversal to filesystem root is refused", target: "../../../..", wantErr: fs.ErrInvalid},
		{name: "relative traversal to sysfs root is refused", target: "../../..", wantErr: fs.ErrInvalid},
		{name: "relative traversal to etc is refused", target: "../../../../etc/passwd", wantErr: fs.ErrInvalid},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := resolveSysfsDeviceDirectory(testCase.target)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("resolveSysfsDeviceDirectory(%q) error = %v, want %v", testCase.target, gotErr, testCase.wantErr)
			}
			if got != testCase.want {
				t.Fatalf("resolveSysfsDeviceDirectory(%q) = %q, want %q", testCase.target, got, testCase.want)
			}
		})
	}
}

func TestResolveSysfsDeviceParentAppliesTheSameConfinementBeforeFallback(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr error
		name    string
		device  string
		want    string
	}{
		{name: "partition parent remains a strict sysfs descendant", device: "/sys/devices/pci/block/sda/sda1", want: "/sys/devices/pci/block/sda"},
		{name: "device directly below sysfs cannot fall back to root", device: "/sys/device", wantErr: fs.ErrInvalid},
		{name: "sysfs root cannot fall back to filesystem root", device: "/sys", wantErr: fs.ErrInvalid},
		{name: "outside device directory cannot enter fallback", device: "/tmp/device", wantErr: fs.ErrInvalid},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := resolveSysfsDeviceParent(testCase.device)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("resolveSysfsDeviceParent(%q) error = %v, want %v", testCase.device, gotErr, testCase.wantErr)
			}
			if got != testCase.want {
				t.Fatalf("resolveSysfsDeviceParent(%q) = %q, want %q", testCase.device, got, testCase.want)
			}
		})
	}
}
