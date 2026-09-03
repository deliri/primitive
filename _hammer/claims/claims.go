// Package claims owns Primitive's human-authored offline source claims.
// It is excluded from ordinary package discovery by the leading underscore;
// an offline source tool compiles and consumes Stream explicitly.
package claims

import (
	"crypto/ed25519"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
)

// Stream emits Primitive's authored claims in canonical subject and identity
// order without materializing or limiting the complete repository claim set.
func Stream(emit sourceclaim.Emit) error {
	if emit == nil {
		return core.ErrSourceClaimContract
	}
	builder := newBuilder()
	if builder.err != nil {
		return builder.err
	}
	if err := emitProjectClaims(builder, emit); err != nil {
		return err
	}
	return emitPackageClaims(builder, emit)
}

func emitProjectClaims(builder *builder, emit sourceclaim.Emit) error {
	claims := [...]sourceclaim.Claim{
		builder.projectClaim(projectClaimSpec{
			identity: "blind-typed-wall", problem: "Products otherwise scatter real-world mechanics through code that owns business meaning.",
			solution:       "Primitive exposes product-neutral typed sockets that validate, perform, observe, and receipt narrow effects.",
			benefit:        "Unrelated Go products reuse one mechanically exact boundary without sharing policy or a world model.",
			removal:        "Remove Primitive when Go and the operating system directly provide the complete owned, validated, and receipted boundary.",
			supportPackage: "exchange",
		}),
		builder.projectClaim(projectClaimSpec{
			identity: "bounded-unix-composition", problem: "Convenience APIs tend to load whole inputs, hide backpressure, and detach resource lifetimes.",
			solution:       "Primitive composes readers, writers, contexts, bounded buffers, iterators, and explicit cleanup one operation at a time.",
			benefit:        "Work remains proportional to admitted input and can be understood using ordinary Go and Unix mechanics.",
			removal:        "Remove the shared mechanism when its operation cannot remain narrow, streaming, and independently useful.",
			supportPackage: "filestore",
		}),
		builder.projectClaim(projectClaimSpec{
			identity: "compiler-owned-contracts", problem: "Prose, copied values, and naming conventions drift without breaking builds.",
			solution:       "Primitive owns nominal types, closed enums, validation, typed errors, constants, and compile-time witnesses for mechanical contracts.",
			benefit:        "Incorrect structural changes become compiler or validation failures instead of latent integration defects.",
			removal:        "Remove a Primitive contract when it is no longer shared or the standard library owns an equally strong nominal agreement.",
			supportPackage: "core",
		}),
		builder.projectClaim(projectClaimSpec{
			identity: "evidence-remains-distinct", problem: "Intent, attempts, observations, durable effects, and acceptance are easily collapsed into one unsupported success claim.",
			solution:       "Primitive represents each stage as a separate typed fact and binds durable evidence to exact source and mechanical results.",
			benefit:        "Products can apply their own policy without Primitive inventing what complete or accepted means.",
			removal:        "Remove an evidence mechanism when no consumer needs independent durable proof of that real-world boundary.",
			supportPackage: "proofledger",
		}),
		builder.projectClaim(projectClaimSpec{
			identity: "go-native-mechanics", problem: "Alternative runtimes and universal abstractions obscure the language, operating system, and network behavior that actually executes.",
			solution:       "Primitive rides context, I/O, HTTP, process, crypto, time, and operating-system semantics directly.",
			benefit:        "Consumers inherit Go improvements and retain familiar debugging, profiling, cancellation, and composition.",
			removal:        "Remove a wrapper when it adds no ownership, bound, validation, authentication, observation, or proof beyond Go itself.",
			supportPackage: "process",
		}),
	}
	for _, claim := range claims {
		if builder.err != nil {
			return builder.err
		}
		if err := emit(claim); err != nil {
			return err
		}
	}
	return nil
}

type builder struct {
	author  core.Ed25519PublicKey
	project core.SourceSubject
	err     error
}

func newBuilder() *builder {
	result := &builder{}
	result.author, result.err = core.NewEd25519PublicKey(ed25519.PublicKey{
		0x92, 0xac, 0x5e, 0x9d, 0x79, 0x20, 0xe1, 0xc4,
		0x2a, 0xdd, 0x10, 0xd0, 0xff, 0x15, 0xb6, 0xb2,
		0xdc, 0x25, 0xd8, 0x7e, 0x4a, 0xf4, 0x37, 0x95,
		0x9e, 0x33, 0x46, 0xf8, 0xf3, 0x8e, 0x96, 0x0d,
	})
	root, pathErr := core.ParseSourcePath(".")
	project, subjectErr := core.NewSourceSubject(core.SourceSubjectProject, root)
	result.project = project
	result.err = errors.Join(result.err, pathErr, subjectErr)
	return result
}

type projectClaimSpec struct {
	identity       string
	problem        string
	solution       string
	benefit        string
	removal        string
	supportPackage string
}

func (b *builder) projectClaim(spec projectClaimSpec) sourceclaim.Claim {
	packagePath := b.path(spec.supportPackage)
	packageSubject, err := core.NewSourceSubject(core.SourceSubjectPackage, packagePath)
	b.err = errors.Join(b.err, err)
	return sourceclaim.Claim{
		ID:       b.id(spec.identity),
		Author:   b.author,
		Subject:  b.project,
		Title:    b.text(spec.identity),
		Problem:  sourceclaim.Narrative{Summary: b.text(spec.problem)},
		Solution: sourceclaim.Narrative{Summary: b.text(spec.solution)},
		Benefit:  sourceclaim.Narrative{Summary: b.text(spec.benefit)},
		Removal:  sourceclaim.Narrative{Summary: b.text(spec.removal)},
		Owns: []sourceclaim.Boundary{{
			ID: b.id("mechanical-boundary"), Detail: b.text("Primitive owns the reusable typed mechanics named by this claim."),
		}},
		DoesNotOwn: []sourceclaim.Boundary{{
			ID: b.id("product-meaning"), Detail: b.text("Products own permission, meaning, state transitions, and completion policy."),
		}},
		Requirements: []sourceclaim.Requirement{
			{
				ID: b.id("compiler-support-package"), Statement: b.text("The supporting Primitive package exists in the exact observed source tree."), Mode: sourceclaim.RequirementCompiler,
				Compiler: &sourceclaim.CompilerRequirement{Target: packageSubject, Predicate: sourceclaim.CompilerSubjectPresent},
			},
			{
				ID: b.id("human-value-review"), Statement: b.text("A human confirms the stated problem, solution, benefit, boundary, and removal condition remain true."), Mode: sourceclaim.RequirementHumanReview,
			},
		},
	}
}

func (b *builder) packageClaim(spec packageClaimSpec) sourceclaim.Claim {
	packagePath := b.path(spec.path)
	packageSubject, err := core.NewSourceSubject(core.SourceSubjectPackage, packagePath)
	b.err = errors.Join(b.err, err)
	return sourceclaim.Claim{
		ID:       b.id("package-mechanism"),
		Author:   b.author,
		Subject:  packageSubject,
		Title:    b.text(spec.title),
		Problem:  sourceclaim.Narrative{Summary: b.text(spec.problem)},
		Solution: sourceclaim.Narrative{Summary: b.text(spec.solution)},
		Benefit:  sourceclaim.Narrative{Summary: b.text(spec.benefit)},
		Removal:  sourceclaim.Narrative{Summary: b.text(spec.removal)},
		Owns: []sourceclaim.Boundary{{
			ID: b.id("mechanical-boundary"), Detail: b.text(spec.owns),
		}},
		DoesNotOwn: []sourceclaim.Boundary{{
			ID: b.id("product-policy"), Detail: b.text(spec.excludes),
		}},
		Requirements: []sourceclaim.Requirement{
			{
				ID: b.id("compiler-subject-present"), Statement: b.text("The package exists in the exact observed source tree."), Mode: sourceclaim.RequirementCompiler,
				Compiler: &sourceclaim.CompilerRequirement{Target: packageSubject, Predicate: sourceclaim.CompilerSubjectPresent},
			},
			{
				ID: b.id("human-value-review"), Statement: b.text("A human confirms this package still solves the stated problem within its boundary."), Mode: sourceclaim.RequirementHumanReview,
			},
		},
	}
}

func (b *builder) id(value string) sourceclaim.ID {
	result, err := sourceclaim.NewID(value)
	b.err = errors.Join(b.err, err)
	return result
}

func (b *builder) text(value string) sourceclaim.Text {
	result, err := sourceclaim.NewText(value)
	b.err = errors.Join(b.err, err)
	return result
}

func (b *builder) path(value string) core.SourcePath {
	result, err := core.ParseSourcePath(value)
	b.err = errors.Join(b.err, err)
	return result
}
