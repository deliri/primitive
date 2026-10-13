# Native Git status completion

`ObserveStatus` now returns `gitrepo.StatusObservation`, retaining the native
`process.Result` privately. `Process` derives the receipt from that single
reaped result. An absent value refuses with the Git and native process contract
identities; it cannot become a completed, empty status. There is no public
constructor, mirrored completion flag, output body or repository inventory.

This is a clean API change. Callers obtain `Process()` and classify its native
byte counts only after validation. Primitive retains diagnostics as mechanical
facts; the product decides whether those facts permit clean-worktree authority.

Furnace evidence is under
`/data/evidence/raw/primitive/2026-10-13-git-status-completion`. The status race
scope passed 18 terminal events. Full Git package race passed 384, with no
failures or skips. Bypassing completed-status validation failed the absent-value
ratchet. The unmodified full scope supplies restoration evidence. Existing
native extent fuzz covers conservation through the real Git producer and an
independent native command; the retained receipt governs its exact outcome.

All test, cache, build, scratch and evidence files remain under `/data`.
This change concerns Primitive's streamed output and completion authority;
it makes no assertion about Git's internal repository memory.
