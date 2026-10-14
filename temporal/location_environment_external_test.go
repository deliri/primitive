package temporal_test

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const temporalZoneChildEnvironment = "PRIMITIVE_TEMPORAL_ZONE_CHILD"
const temporalForeignZoneChildEnvironment = "PRIMITIVE_TEMPORAL_FOREIGN_ZONE_CHILD"

// This native child proves poisoned ZONEINFO/TZ cannot select a different
// database or casing policy. It does not claim the host's system timezone
// files were removed; the production loader's embedded bytes are its input.
func TestEmbeddedLocationIgnoresHostZoneInfoOverride(t *testing.T) {
	t.Parallel()
	if os.Getenv(temporalZoneChildEnvironment) == "1" {
		if os.Getenv(temporalForeignZoneChildEnvironment) == "1" {
			// Prove the hostile fixture is admitted by the actual Go ambient
			// loader before testing Primitive's independent embedded authority.
			native, err := time.LoadLocation("America/Toronto")
			if err != nil {
				t.Fatalf("native foreign timezone fixture = %v, want nil", err)
			}
			got := time.Date(2026, time.March, 8, 7, 0, 0, 0, time.UTC).In(native).Format(time.RFC3339)
			if got != "2026-03-08T16:00:00+09:00" {
				t.Fatalf("native foreign fixture = %q, want Tokyo data admitted under Toronto name", got)
			}
		}
		location, err := temporal.NewLocation("America/Toronto")
		if err != nil {
			t.Fatalf("embedded zone under override = %v, want nil", err)
		}
		instant, err := temporal.ParseRFC3339UTC("2026-03-08T07:00:00Z")
		if err != nil {
			t.Fatalf("owned instant = %v, want nil", err)
		}
		got, err := location.Format(instant, time.RFC3339)
		if err != nil || got != "2026-03-08T03:00:00-04:00" {
			t.Fatalf("embedded transition = %q/%v, want Toronto first post-gap clock", got, err)
		}
		bad, err := temporal.NewLocation("america/toronto")
		if bad != (temporal.Location{}) || !errors.Is(err, core.ErrTemporalContract) {
			t.Fatalf("embedded case refusal = %v/%v, want zero/contract", bad, err)
		}
		return
	}
	for _, name := range []string{"missing archive", "malformed archive", "empty directory", "foreign valid archive"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			owned := t.TempDir()
			zoneInfo := filepath.Join(owned, "zoneinfo.zip")
			if name == "malformed archive" {
				if err := os.WriteFile(zoneInfo, []byte("not a timezone archive"), 0600); err != nil {
					t.Fatalf("owned poison fixture = %v, want nil", err)
				}
			}
			if name == "empty directory" {
				zoneInfo = owned
			}
			if name == "foreign valid archive" {
				if err := writeForeignZoneArchive(zoneInfo); err != nil {
					t.Fatalf("owned foreign timezone archive = %v, want nil", err)
				}
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatalf("native test executable = %v, want nil", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestEmbeddedLocationIgnoresHostZoneInfoOverride$", "-test.count=1", "-test.timeout=20s")
			command.WaitDelay = 10 * time.Second
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "ZONEINFO=") && !strings.HasPrefix(entry, "TZ=") && !strings.HasPrefix(entry, temporalZoneChildEnvironment+"=") && !strings.HasPrefix(entry, temporalForeignZoneChildEnvironment+"=") {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, temporalZoneChildEnvironment+"=1", "ZONEINFO="+zoneInfo, "TZ=Not/A_Zone")
			if name == "foreign valid archive" {
				command.Env = append(command.Env, temporalForeignZoneChildEnvironment+"=1")
			}
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("owned child execution = %v with output %q, want native completion", err, output)
			}
		})
	}
}

func writeForeignZoneArchive(destination string) (resultErr error) {
	archive, err := zip.OpenReader("tzdata/zoneinfo.zip")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, archive.Close()) }()
	for _, member := range archive.File {
		if member.Name != "Asia/Tokyo" {
			continue
		}
		source, err := member.Open()
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, source.Close()) }()
		file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
		writer := zip.NewWriter(file)
		// Go's native timezone ZIP loader requires stored entries (method0).
		// Source: https://go.dev/src/time/zoneinfo_read.go
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: "America/Toronto", Method: zip.Store})
		if err != nil {
			return errors.Join(err, writer.Close())
		}
		_, err = io.Copy(entry, source)
		return errors.Join(err, writer.Close())
	}
	return core.ErrTemporalContract
}
