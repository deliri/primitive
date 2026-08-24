# Taskmanager consumer evidence

Date: 2026-08-24

This interview records consumer evidence for a product-neutral task-management
socket. It is evidence for the typed Go contract, not a second specification.

## Observed implementation

Blink Kernel currently owns the task server under `adminapi` and the task
application vocabulary under its local `core`. Its working command proves the
remote workflow: a real cookie session, strict JSON, replay-bound mutations,
optimistic revisions, evidence, Git commits, and atomic completion.

The rejected Primitive attempt had two independent parts. The `taskprogress`
package was an exact package-name-only copy of Blink's aggregate model and did
not build outside Blink's `core`. The command did build, but carried private
copies of Blink routes, authentication documents, task documents, limits, and
response projections. It loaded the complete project registry to address one
project and supported only task updates.

## Consumer statements

Primitive is the shared floor imported by Blink Kernel, every distro made from
it, Witness, Bug, Peachfuzz, and future repositories. Each repository should
load one `task_config.json` and use the same Primitive client to query and
mutate its configured project. Blink Kernel should plug the other end of that
same socket into `adminapi`.

Primitive must remain blind. It must not know which product or CLI imports it,
which database persists the records, which deployment hosts the peer, or what
a project means. Project, phase, task, evidence, commit, and completion facts
are shared wire vocabulary; application authorization, storage layout, UI,
and business policy remain consumer-owned.

## Ownership inventory

Taskmanager owns:

- closed task-manager operation, lifecycle, task-kind, and page-order enums;
- bounded scalar constructors and canonical JSON projections;
- direct project, phase, and task identities using Primitive UUIDv7 values;
- bounded page requests and responses with explicit continuation cursors;
- entity-local optimistic revisions and mutation identities;
- append-only evidence and Git commit documents;
- atomic task-completion request and receipt documents;
- compiler-owned route construction and method/replay semantics;
- paired client send, server receive, and server response-write sockets over
  Exchange; and
- the reusable typed `task_config.json` document consumed by the command leaf.

Taskmanager does not own:

- Firestore, SQL, collection names, indexes, transactions, or migrations;
- a global tracker document or an in-memory model of all project history;
- authentication policy, roles, admin middleware, cookies, or credential
  verification;
- UI tabs, templates, browser navigation, or application routing outside its
  exact API socket;
- repository catalogs, product switches, deployment names, or fixed domains;
- retry, redirect, timeout, buffering, time, entropy, identifier generation,
  filesystem access, process state, or secret-provider mechanics already owned
  by other Primitive packages; or
- compatibility with the rejected Blink-local aggregate wire.

## Retained mechanics

- strict typed JSON ingress and canonical validated JSON output;
- a real standard-library cookie jar through Exchange where a consumer's
  authentication plug-in requires one;
- compiler-bound agreement between mutation documents and HTTP idempotency
  keys;
- bounded request and response bodies;
- Google Secret Manager references through Secretstore in the CLI leaf, with
  values destroyed after explicit projection;
- UUIDv7 mutation identities minted from Primitive time and entropy; and
- server-owned persistence and commitment.

## Scale rulings

- Project, phase, task, evidence, and commit listings are independently paged.
- Active and completed project queries are separate requests; completed data is
  never implied by the default active query.
- Task detail does not embed unbounded evidence, commit, activity, phase, or
  sibling-task collections.
- Mutating one task compares only that task's revision. Creating or appending
  child records does not contend on one application-wide revision.
- Every response has a compiler-owned item ceiling and a continuation cursor;
  there is no total project-count ceiling and no response proportional to ten
  years of history.

## Later consumer surgery

After Primitive's package and command are reviewed, Blink Kernel replaces its
local task wire vocabulary with this contract and mounts the server socket
behind its existing admin authorization. Its persistence adapter remains
Blink-owned. Witness, Bug, Peachfuzz, and future repositories then consume the
same command and supply only their own typed configuration.
