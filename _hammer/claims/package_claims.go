package claims

import "github.com/deliri/primitive/v2026/sourceclaim"

type packageClaimSpec struct {
	path     string
	title    string
	problem  string
	solution string
	benefit  string
	removal  string
	owns     string
	excludes string
}

func emitPackageClaims(builder *builder, emit sourceclaim.Emit) error {
	var emitErr error
	emitPackageClaimSpecs(func(spec packageClaimSpec) bool {
		claim := builder.packageClaim(spec)
		if builder.err != nil {
			emitErr = builder.err
			return false
		}
		emitErr = emit(claim)
		return emitErr == nil
	})
	return emitErr
}

// emitPackageClaimSpecs keeps the authored catalog streaming. Adding a
// package adds authored source here; it never causes Primitive to synthesize a
// human reason from the mechanical core catalog.
func emitPackageClaimSpecs(emit func(packageClaimSpec) bool) {
	if !emitPackageClaimSpecsAThroughG(emit) {
		return
	}
	if !emitPackageClaimSpecsHThroughR(emit) {
		return
	}
	emitPackageClaimSpecsSThroughZ(emit)
}
