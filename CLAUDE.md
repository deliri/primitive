# Primitive — read this before writing any code

## STOP. Read the law first.

Before writing or changing **production code** in this repository, read
`_docs/primitive_policy.md` **in full**. Before writing or changing **any test**, read
`_docs/testing_protocol.md` **in full**. Not skimmed, not grepped. They are
commit gates, not guidance, and work produced without them gets thrown away.

If context is short, read the law and do less work. Do not do more work with
less law.

## Status: not shipped (as of 2026-08-06)

Nothing here is shipped. No customer holds an installed binary, a released
artifact, a persisted bundle, or a signed document. So **every fix is a clean
break**: change the real contract and delete the superseded one in the same
commit. No compatibility layer, wrapper, alias, adapter, dual format, fallback
reader, or migration path. Rename and reshape freely. This is the cheapest this
work will ever be.

## The layering law — zero exceptions

```text
OS -> Go standard library (golang.org/x/sys only where stdlib cannot reach)
   -> Primitive
     -> this repository's policy
```

Each layer rides the one below it and never reaches around it.

- **Never call the standard library directly for anything Primitive owns.**
- Read `primitive/CAPABILITIES.md` **before** writing anything that touches the
  real world or crosses a wire. Ask "why can't the caller reach it?", not "does
  it exist?".
- A missing or awkward Primitive door is a **Primitive defect** to fix, prove,
  publish, and pull in. It is never a licence to take a local exception.
- Reading a value the stdlib already returned is riding it, not bypassing it.

## The ownership test

> If it crosses the wire it belongs in Primitive. If it never leaves one side
> it is policy.

Both ends run this identical stack, so a wire type has exactly one home. A
capability two repositories need is Primitive's, even if only one needs it
today. Product vocabulary, business rules, and thresholds stay here.

## The blink model

```text
B2 Value   B3 Mechanism   B4 Capability   B5 Agreement   B6 Decision   B7 Policy
```

Layers are fixed by the question each answers. **Import sideways or down, never
up.** Peer visibility is orthogonal to layer: a contract both ends import lives
in Primitive at whatever layer it sits.

## What this repository is for

**Primitive is the product-neutral Go primitive layer** used to build reliable command-line tools and services. It makes Go's standard library, OS primitives, documented protocols, and official SDKs typed, validated, bounded, streaming, and composable.

It owns every mechanism by which a consumer touches the real world, and every contract that crosses a wire. It owns the obligation to be reachable: a door that exists but a caller cannot reach is a defect equal to a missing one, and a type Primitive demands but will not produce is the same defect.

It never owns product policy, and never imports Kernel, Witness, Bug, or Peachfuzz.

**Where this meets the real world:**

Primitive *is* that layer. It rides the Go standard library and reaches the OS only through it, using `golang.org/x/sys` where the stdlib genuinely cannot reach.

A real-world capability with no named Primitive door is either a gap to close in
Primitive or a reach-past to delete.

## Hard rules

- Single source of truth. Compiler-owned. No raw strings, magic values,
  duplicated literals, loose maps, bool protocols, or hidden conventions.
- `Validate()` at ingress, package crossing, persistence, execution, recovery,
  and external output. Validation belongs to the type that owns the rule.
- Streaming and O(1) in input extent. No world building.
- Production `gocyclo <= 10`. Split aggressively.
- No shims, wrappers, aliases, or back-compat paths. Land changes coherently in
  one commit.
- `errors.Is` / `errors.As` only. Never match error strings.
- Tests are hostile. No stubs, fakes, mocks, simulators, or lookalikes. A test
  that cannot fail proves nothing: revert the fix and watch it go red.
- Run `gofmt`, `go vet`, `fieldalignment`, and `gocyclo` before calling a Go
  change complete.
- **Commit only on explicit approval.** Gates are the user's to run.
- No dashes or em dashes in prose, copy, comments, or commit messages.
