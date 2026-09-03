package sourceobservation_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceobservation"
)

func TestFileMembershipRejectsDuplicateReferencesInsteadOfLosingAccounting(t *testing.T) {
	t.Parallel()

	filePath := observedPath(t, "exchange/client.go")
	reference := sourceobservation.FileReference{Path: filePath, ObservationDigest: core.SHA256Of([]byte("package exchange"))}
	_, gotErr := sourceobservation.ConsumeFileReferences(func(emit sourceobservation.EmitFileReference) error {
		if err := emit(reference); err != nil {
			return err
		}
		return emit(reference)
	}, func(sourceobservation.FileReference) error { return nil })
	if !errors.Is(gotErr, core.ErrSourceObservationConflict) {
		t.Fatalf("ConsumeFileReferences(duplicate path) error = %v, want %v", gotErr, core.ErrSourceObservationConflict)
	}
}

func TestFileMembershipStreamsBeyondStrictJSONArrayCeiling(t *testing.T) {
	t.Parallel()

	wantCount := uint64(core.DefaultStrictJSONLimits().ArrayItemMaximum) + 1
	got, gotErr := sourceobservation.ConsumeFileReferences(func(emit sourceobservation.EmitFileReference) error {
		for index := range wantCount {
			path := observedPath(t, fmt.Sprintf("package/file-%020d.go", index))
			reference := sourceobservation.FileReference{Path: path, ObservationDigest: core.SHA256Of([]byte(path.String()))}
			if err := emit(reference); err != nil {
				return err
			}
		}
		return nil
	}, func(sourceobservation.FileReference) error { return nil })
	if gotErr != nil || got.Count != wantCount {
		t.Fatalf("ConsumeFileReferences(beyond JSON array ceiling) = (%+v, %v), want count %d and nil", got, gotErr, wantCount)
	}
}

func TestMembershipStreamsCannotEraseIgnoredDestinationFailures(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("membership destination unavailable")
	filePath := observedPath(t, "exchange/client.go")
	packagePath := observedPath(t, "exchange")
	fileReference := sourceobservation.FileReference{
		Path: filePath, Package: &packagePath,
		ObservationDigest: core.SHA256Of([]byte("file observation")),
	}
	packageReference := sourceobservation.PackageReference{
		Path: packagePath, ObservationDigest: core.SHA256Of([]byte("package observation")),
	}

	t.Run("file membership destination refusal remains terminal", func(t *testing.T) {
		t.Parallel()

		got, gotErr := sourceobservation.ConsumeFileReferences(func(emit sourceobservation.EmitFileReference) error {
			_ = emit(fileReference)
			return nil
		}, func(sourceobservation.FileReference) error { return wantErr })
		if !errors.Is(gotErr, wantErr) || got != (sourceobservation.FileMembership{}) {
			t.Fatalf("ConsumeFileReferences(ignored destination failure) = (%+v, %v), want (zero, %v)", got, gotErr, wantErr)
		}
	})

	t.Run("package membership destination refusal remains terminal", func(t *testing.T) {
		t.Parallel()

		got, gotErr := sourceobservation.ConsumePackageReferences(func(emit sourceobservation.EmitPackageReference) error {
			_ = emit(packageReference)
			return nil
		}, func(sourceobservation.PackageReference) error { return wantErr })
		if !errors.Is(gotErr, wantErr) || got != (sourceobservation.PackageMembership{}) {
			t.Fatalf("ConsumePackageReferences(ignored destination failure) = (%+v, %v), want (zero, %v)", got, gotErr, wantErr)
		}
	})
}

func TestFileRejectsNonCanonicalMechanicalFactsBeforeDigesting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutate func(testing.TB, *sourceobservation.File)
		name   string
	}{
		{
			name: "declaration source coordinates run backward",
			mutate: func(t testing.TB, file *sourceobservation.File) {
				file.Declarations = []sourceobservation.Declaration{
					observedDeclaration(t, "Second", sourceobservation.DeclarationFunction, 20),
					observedDeclaration(t, "First", sourceobservation.DeclarationFunction, 10),
				}
			},
		},
		{
			name: "duplicate effect coordinate and identity cannot fork one fact",
			mutate: func(t testing.TB, file *sourceobservation.File) {
				effectName, effectErr := sourceobservation.NewEffectName("filesystem-write")
				symbol, symbolErr := sourceobservation.NewSymbol("WriteFile")
				if err := errors.Join(effectErr, symbolErr); err != nil {
					t.Fatalf("effect fixture error = %v, want nil", err)
				}
				effect := sourceobservation.Effect{Name: effectName, Symbol: symbol, Line: 12, Column: 4}
				file.Effects = []sourceobservation.Effect{effect, effect}
			},
		},
		{
			name: "reference source coordinates run backward",
			mutate: func(t testing.TB, file *sourceobservation.File) {
				from, fromErr := sourceobservation.NewSymbol("Caller")
				first, firstErr := sourceobservation.NewSymbol("First")
				second, secondErr := sourceobservation.NewSymbol("Second")
				if err := errors.Join(fromErr, firstErr, secondErr); err != nil {
					t.Fatalf("reference fixture error = %v, want nil", err)
				}
				file.References = []sourceobservation.Reference{
					{From: from, To: second, Kind: sourceobservation.ReferencePackage, Line: 20, Column: 1},
					{From: from, To: first, Kind: sourceobservation.ReferencePackage, Line: 10, Column: 1},
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			context := observationContext(t)
			file := observedFile(t, "exchange/client.go", nil, context.ID, nil)
			tc.mutate(t, &file)
			gotDigest, gotErr := file.ObservationDigest()
			if !errors.Is(gotErr, core.ErrSourceObservationConflict) || gotDigest != (core.SHA256Digest{}) {
				t.Fatalf("File.ObservationDigest(non-canonical facts) = (%v, %v), want (zero, %v)", gotDigest, gotErr, core.ErrSourceObservationConflict)
			}
		})
	}
}

func TestFileRejectsAncestorPackageThatDoesNotOwnItsDirectory(t *testing.T) {
	t.Parallel()

	context := observationContext(t)
	packagePath := observedPath(t, "exchange")
	file := observedFile(t, "exchange/internal/client.go", &packagePath, context.ID, nil)
	gotErr := file.Validate()
	if !errors.Is(gotErr, core.ErrSourceObservationConflict) {
		t.Fatalf("File.Validate(ancestor package) error = %v, want %v", gotErr, core.ErrSourceObservationConflict)
	}
}

func observedPath(t testing.TB, value string) core.SourcePath {
	t.Helper()

	got, err := core.ParseSourcePath(value)
	if err != nil {
		t.Fatalf("core.ParseSourcePath(%q) error = %v, want nil", value, err)
	}
	return got
}
