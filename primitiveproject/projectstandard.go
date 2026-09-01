// Package primitiveproject owns Primitive's authored project policy.
// Observed source, execution, and evidence facts remain inputs from their
// owning producers and are never manufactured here.
package primitiveproject

import (
	"errors"

	"github.com/deliri/primitive/v2026/projectstandards"
)

const projectStandardSourcePath = "primitiveproject/projectstandard.go"

// ProjectStandardKnowledge returns Primitive's compiler-owned authored
// project meaning. The caller supplies Git origins because history is an
// observed source fact rather than authored policy.
func ProjectStandardKnowledge(created projectstandards.OptionalGitOrigin, changed projectstandards.GitOrigin) (projectstandards.ProductKnowledge, error) {
	builder := projectStandardBuilder{}
	knowledge := projectstandards.ProductKnowledge{
		Created:        created,
		Changed:        changed,
		AuthorTitle:    builder.name("Primitive"),
		AuthorProblem:  builder.text("Go projects need real-world effects without scattering operating-system calls, provider mechanics, hidden conventions, and duplicated low-level contracts throughout product policy."),
		AuthorPurpose:  builder.text("Execute project-declared policy through small typed capabilities backed by Go's standard library, operating-system primitives, documented protocols, and official provider SDKs."),
		AuthorAudience: builder.text("Authors of Go command-line tools and services that want direct, understandable, bounded access to the real world."),
		AuthorPromise:  builder.text("Projects keep ownership of meaning while Primitive supplies compiler-owned, validated, bounded, streaming effect execution that advances cleanly with Go and the host platform."),
		SourcePath:     builder.path(projectStandardSourcePath),
		AuthorReasons: []projectstandards.Reason{
			builder.reason("Keep policy with the project", "The product decides what must happen, which states are valid, and what evidence is sufficient; a reusable effect package must not acquire that domain authority."),
			builder.reason("Stay native to Go and Unix", "Thin ownership boundaries over the standard library and operating system preserve language upgrades, familiar debugging, and code that can be understood without learning a second runtime."),
			builder.reason("Make correctness compiler-visible", "Typed intents, observations, errors, limits, package identities, and capability requirements let ordinary Go compilation and validation reject architectural drift."),
			builder.reason("Bound work instead of modeling the world", "Streaming operations, explicit resource ceilings, and narrow receipts keep memory, concurrency, retries, output, and lifetimes proportional to the admitted operation."),
		},
		AuthorOwns: []projectstandards.Boundary{
			builder.boundary("Real-world effect execution", "Filesystem, process, transport, time, entropy, secret, host, and signal interaction through their exact Primitive capability owners."),
			builder.boundary("Product-neutral low-level contracts", "Validated values, closed enums, stable error identities, explicit limits, and protocol facts reusable across unrelated Go products."),
			builder.boundary("Effect observations and receipts", "Typed bounded facts describing what was requested, what the available machine and provider admitted, what actually ran, and what durable result was observed."),
			builder.boundary("Compiler-owned capability architecture", "The complete Primitive package graph, effect ownership, production and test scope, and deterministic requirement resolution derived from code."),
		},
		AuthorNonGoals: []projectstandards.Boundary{
			builder.boundary("Product policy", "Primitive does not decide a product's routes, permissions, workflows, acceptance thresholds, persistence meaning, or valid domain transitions."),
			builder.boundary("Application world models", "Primitive does not build repository-sized object graphs, orchestration frameworks, generic state machines, service locators, or alternate runtimes above Go."),
			builder.boundary("Hidden compatibility architecture", "Primitive does not retain obsolete APIs, fallback paths, aliases, wrappers, or duplicate protocols merely to preserve old callers."),
			builder.boundary("Provider-specific product meaning", "Official SDKs may implement a product-neutral effect, but provider brands, business rules, and application records do not escape through the capability contract."),
		},
		AuthorFeatures: []projectstandards.Feature{
			{
				ID: builder.identifier("stdlib-effect-boundaries"), Title: builder.name("Standard-library effect boundaries"),
				Technical:        builder.text("Small typed capabilities execute admitted filesystem, process, transport, time, entropy, secret, host, and signal intents through Go and the operating system."),
				Benefit:          builder.text("Projects use the real platform without duplicating unsafe or inconsistent low-level mechanics."),
				ProofRequirement: builder.text("Exact-revision evidence must prove validation, bounds, cancellation, cleanup, and returned observations for each owned effect."),
				Delivery:         projectstandards.DeliveryDelivered,
			},
			{
				ID: builder.identifier("compiler-owned-capabilities"), Title: builder.name("Compiler-owned capability catalog"),
				Technical:        builder.text("A closed package architecture derives deterministic capability enumeration and resolves typed package or effect requirements by production or test scope."),
				Benefit:          builder.text("Projects can discover and require the correct Primitive socket without relying on prose, copied paths, or runtime registration."),
				ProofRequirement: builder.text("Architecture evidence must prove landed-package closure, unique effect ownership, allowed import edges, deterministic enumeration, and fail-closed resolution."),
				Delivery:         projectstandards.DeliveryDelivered,
			},
			{
				ID: builder.identifier("bounded-unix-composition"), Title: builder.name("Bounded Unix-like composition"),
				Technical:        builder.text("Readers, writers, iterators, argv, contexts, and narrow typed results compose one operation at a time with explicit resource ceilings."),
				Benefit:          builder.text("Systems remain understandable and efficient without an in-memory world model or a framework-owned lifecycle."),
				ProofRequirement: builder.text("Evidence must prove streaming behavior, bounded memory and concurrency, owned cleanup, deterministic output, and explicit saturation outcomes."),
				Delivery:         projectstandards.DeliveryDelivered,
			},
			{
				ID: builder.identifier("evidence-bearing-execution"), Title: builder.name("Evidence-bearing execution"),
				Technical:        builder.text("Effect results preserve typed requested, admitted, acted-on, timing, machine, artifact, completion, and source facts when the operation requires proof."),
				Benefit:          builder.text("Products can distinguish an attempt from a completed effect and retain exact operational truth without treating logs as proof."),
				ProofRequirement: builder.text("Independent exact-revision evidence must close intent, execution, output, receipt, and source coordinates without hiding retries, skips, cancellation, or partial availability."),
				Delivery:         projectstandards.DeliveryDelivered,
			},
		},
	}
	if builder.err != nil {
		return projectstandards.ProductKnowledge{}, builder.err
	}
	if err := knowledge.Validate(); err != nil {
		return projectstandards.ProductKnowledge{}, err
	}
	return knowledge, nil
}

type projectStandardBuilder struct {
	err error
}

func (b *projectStandardBuilder) identifier(value string) projectstandards.Identifier {
	result, err := projectstandards.NewIdentifier(value)
	b.err = errors.Join(b.err, err)
	return result
}

func (b *projectStandardBuilder) name(value string) projectstandards.Name {
	result, err := projectstandards.NewName(value)
	b.err = errors.Join(b.err, err)
	return result
}

func (b *projectStandardBuilder) text(value string) projectstandards.Text {
	result, err := projectstandards.NewText(value)
	b.err = errors.Join(b.err, err)
	return result
}

func (b *projectStandardBuilder) path(value string) projectstandards.SourcePath {
	result, err := projectstandards.ParseSourcePath(value)
	b.err = errors.Join(b.err, err)
	return result
}

func (b *projectStandardBuilder) reason(title, detail string) projectstandards.Reason {
	return projectstandards.Reason{Title: b.name(title), Detail: b.text(detail)}
}

func (b *projectStandardBuilder) boundary(title, detail string) projectstandards.Boundary {
	return projectstandards.Boundary{Title: b.name(title), Detail: b.text(detail)}
}
