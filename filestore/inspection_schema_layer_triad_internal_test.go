package filestore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Private observation corruption belongs at this local schema boundary. Native
// observation tests separately exercise the real OS producer. Each corruption
// targets a named retained fact; successful construction cannot excuse a validator
// that later admits a contradictory package-owned observation.
func TestInspectionSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	one, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		base        PathKind
		mutate      func(Inspection) Inspection
		wantKind    PathKind
		wantErr     error
		wantCause   error
		wantPresent bool
		wantRegular bool
		wantOwner   bool
	}{
		{name: "complete regular observation retains every typed fact", wantKind: PathKindRegularFile, wantPresent: true, wantRegular: true, wantOwner: true},
		{name: "epoch is a real observed timestamp", mutate: func(i Inspection) Inspection { i.modified = temporal.InstantFromNanoseconds(0); return i }, wantKind: PathKindRegularFile, wantPresent: true, wantRegular: true, wantOwner: true},
		{name: "zero permission bits are observed denial rather than absence", mutate: func(i Inspection) Inspection { i.permissions = Permissions{set: true}; return i }, wantKind: PathKindRegularFile, wantPresent: true, wantRegular: true, wantOwner: true},
		{name: "unsupported numeric ownership stays unreported", mutate: func(i Inspection) Inspection { i.ownership = Ownership{}; return i }, wantKind: PathKindRegularFile, wantPresent: true, wantRegular: true},
		{name: "unsupported allocation stays distinct from reported zero", mutate: func(i Inspection) Inspection { i.allocation = Allocation{}; return i }, wantKind: PathKindRegularFile, wantPresent: true, wantRegular: true, wantOwner: true},
		{name: "absent path carries no file metadata", base: PathKindAbsent, wantKind: PathKindAbsent},
		{name: "unreachable path carries its distinct kind without metadata", base: PathKindUnreachable, wantKind: PathKindUnreachable},
		{name: "directory cannot claim regular-file extent or allocation", base: PathKindDirectory, wantKind: PathKindDirectory, wantPresent: true, wantOwner: true},
		{name: "missing timestamp cannot survive behind a valid kind", mutate: func(i Inspection) Inspection { i.modified = temporal.Instant{}; return i }, wantErr: core.ErrFilestoreContract, wantCause: core.ErrTemporalContract},
		{name: "unset permissions cannot survive behind a valid kind", mutate: func(i Inspection) Inspection { i.permissions = Permissions{}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "permission bits cannot smuggle file type", mutate: func(i Inspection) Inspection { i.permissions.value |= fs.ModeDir; return i }, wantErr: core.ErrFilestoreContract},
		{name: "unreported UID cannot retain an owner", mutate: func(i Inspection) Inspection { i.ownership = Ownership{uid: 1}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "unreported GID cannot retain a group", mutate: func(i Inspection) Inspection { i.ownership = Ownership{gid: 1}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "unreported allocation cannot retain bytes", mutate: func(i Inspection) Inspection { i.allocation = Allocation{bytes: one}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "directory cannot retain a file extent", base: PathKindDirectory, mutate: func(i Inspection) Inspection { i.size = one; return i }, wantErr: core.ErrFilestoreContract},
		{name: "directory cannot retain a file allocation claim", base: PathKindDirectory, mutate: func(i Inspection) Inspection { i.allocation = Allocation{reported: true}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "absent kind cannot hide an observed timestamp", base: PathKindAbsent, mutate: func(i Inspection) Inspection { i.modified = temporal.InstantFromNanoseconds(0); return i }, wantErr: core.ErrFilestoreContract},
		{name: "absent kind cannot hide file bytes", base: PathKindAbsent, mutate: func(i Inspection) Inspection { i.size = one; return i }, wantErr: core.ErrFilestoreContract},
		{name: "absent kind cannot hide permissions", base: PathKindAbsent, mutate: func(i Inspection) Inspection { i.permissions = Permissions{set: true}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "absent kind cannot hide numeric ownership", base: PathKindAbsent, mutate: func(i Inspection) Inspection { i.ownership = Ownership{set: true}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "absent kind cannot hide allocated storage", base: PathKindAbsent, mutate: func(i Inspection) Inspection { i.allocation = Allocation{reported: true}; return i }, wantErr: core.ErrFilestoreContract},
		{name: "unknown kind cannot expose a valid-looking file", mutate: func(i Inspection) Inspection { i.kind = PathKindUnknown; return i }, wantErr: core.ErrFilestoreContract},
		{name: "future kind cannot expose a valid-looking file", mutate: func(i Inspection) Inspection { i.kind = pathKindLimit; return i }, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "source")
			if err := os.WriteFile(name, []byte{0, 255}, 0o600); err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseAbsolutePath(name)
			if err != nil {
				t.Fatal(err)
			}
			in, err := Inspect(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			// Use a complete typed owner even on hosts that cannot observe
			// numeric ownership; native availability is tested at the producer.
			in.ownership = Ownership{uid: 1, gid: 2, set: true}
			in.allocation = Allocation{bytes: one, reported: true}
			switch tc.base {
			case PathKindAbsent, PathKindUnreachable:
				in = Inspection{kind: tc.base}
			case PathKindDirectory:
				in.kind = tc.base
				in.size = core.ByteLength{}
				in.allocation = Allocation{}
			}
			if tc.mutate != nil {
				before := in
				in = tc.mutate(in)
				if in == before {
					t.Fatalf("mutated schema = %+v, want distinct from %+v", in, before)
				}
			}
			gotErr := in.Validate()
			if !errors.Is(gotErr, tc.wantErr) || (tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause)) {
				t.Fatalf("Validate = %v, want %v and native cause %v", gotErr, tc.wantErr, tc.wantCause)
			}
			kind, kindErr := in.Kind()
			size, sizeErr := in.SizeBytes()
			modified, modifiedErr := in.ModifiedAt()
			permissions, permissionsErr := in.Permissions()
			owner, ownerErr := in.Ownership()
			allocation, allocationErr := in.Allocation()
			for _, result := range []struct {
				name      string
				gotErr    error
				wantValid bool
			}{
				{name: "kind", gotErr: kindErr, wantValid: tc.wantErr == nil},
				{name: "size", gotErr: sizeErr, wantValid: tc.wantRegular},
				{name: "timestamp", gotErr: modifiedErr, wantValid: tc.wantPresent},
				{name: "permissions", gotErr: permissionsErr, wantValid: tc.wantPresent},
				{name: "ownership", gotErr: ownerErr, wantValid: tc.wantOwner},
				{name: "allocation", gotErr: allocationErr, wantValid: tc.wantRegular},
			} {
				if result.wantValid && result.gotErr != nil {
					t.Fatalf("%s = %v, want admitted fact", result.name, result.gotErr)
				}
				if !result.wantValid && !errors.Is(result.gotErr, core.ErrFilestoreContract) {
					t.Fatalf("%s = %v, want typed refusal", result.name, result.gotErr)
				}
			}
			if tc.wantErr != nil {
				if kind != PathKindUnknown || size != (core.ByteLength{}) || modified != (temporal.Instant{}) || permissions != (Permissions{}) || owner != (Ownership{}) || allocation != (Allocation{}) {
					t.Fatalf("refused accessors = (%v,%v,%v,%v,%v,%v), want no leaked facts", kind, size, modified, permissions, owner, allocation)
				}
				return
			}
			if kind != tc.wantKind {
				t.Fatalf("kind = %v, want %v", kind, tc.wantKind)
			}
			if tc.wantRegular && (size != in.size || allocation != in.allocation) {
				t.Fatalf("regular facts = (%v,%v), want (%v,%v)", size, allocation, in.size, in.allocation)
			}
			if tc.wantPresent && (modified != in.modified || permissions != in.permissions) {
				t.Fatalf("entry facts = (%v,%v), want (%v,%v)", modified, permissions, in.modified, in.permissions)
			}
			if !tc.wantPresent && (modified != (temporal.Instant{}) || permissions != (Permissions{})) {
				t.Fatalf("absent timestamp/permissions = (%v,%v), want zero facts", modified, permissions)
			}
			if !tc.wantRegular && (size != (core.ByteLength{}) || allocation != (Allocation{})) {
				t.Fatalf("non-regular size/allocation = (%v,%v), want zero facts", size, allocation)
			}
			if !tc.wantOwner && owner != (Ownership{}) {
				t.Fatalf("unreported owner = %+v, want zero ownership", owner)
			}
			if tc.wantOwner && owner != in.ownership {
				t.Fatalf("owner = %v, want %v", owner, in.ownership)
			}
		})
	}
}

func TestKindOnlyInspectionConstructionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		in      PathKind
		want    Inspection
		wantErr error
	}{
		{name: "absent kind constructs exactly an empty observation", in: PathKindAbsent, want: Inspection{kind: PathKindAbsent}},
		{name: "unreachable kind preserves its distinct empty observation", in: PathKindUnreachable, want: Inspection{kind: PathKindUnreachable}},
		{name: "directory needs observed metadata", in: PathKindDirectory, wantErr: core.ErrFilestoreContract},
		{name: "regular file needs observed metadata", in: PathKindRegularFile, wantErr: core.ErrFilestoreContract},
		{name: "symbolic link needs observed metadata", in: PathKindSymbolicLink, wantErr: core.ErrFilestoreContract},
		{name: "other native entry needs observed metadata", in: PathKindOther, wantErr: core.ErrFilestoreContract},
		{name: "unknown kind cannot escape through an error result", in: PathKindUnknown, wantErr: core.ErrFilestoreContract},
		{name: "future kind cannot escape through an error result", in: pathKindLimit, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := newInspection(tc.in)
			if got != tc.want || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("kind-only construction = (%+v,%v), want (%+v,%v)", got, gotErr, tc.want, tc.wantErr)
			}
		})
	}
}
