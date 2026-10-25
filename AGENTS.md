## Anvil is GA

All agents use `/Users/d/code/anvil/_release/anvil-darwin-arm64` on the Mac.
Anvil owns admission, execution, server access and evidence transport. No direct
SSH/SCP/rsync, native Furnace runners or Stoker. This overrides older runner
instructions below. Read `/Users/d/code/anvil/_docs/agent-quickstart.md`.
File complaints in `/Users/d/code/anvil/dog_fooding.md` with the commit,
request, run ID, expected/actual result and evidence. Never include secrets.

Optional project-config subset: callback and evidence destination only. Omit it for fleet-local SSD/HDD. No fleet-policy overrides. Implementation pending; see the quickstart.

Solve this using first principles thinking. First, list every assumption baked into the
problem and how I'm framing it. Then break it down to the fundamental truths we know
for certain. Finally, rebuild a solution from those truths alone, ignoring how others
usually do it. Flag any assumption you couldn't verify.

We don’t add complexity. We don’t over engineer. 
We look for the simplest solution so it can scale. 
Simple doesn’t mean naive. Extremely well tested but simple code works best and is easiest to maintain. 
Every project not primitive does the following thing;
1) unix philosophy grep like O(1) streaming
2) no world building no state machines
3) passes all execution down to primitive which passes down execution to go std lib 100%
4) struct to structure, enums and no raw strings, maps, or ‘any’
5) even errors are defined as constants in core so the compiler is always in charge 

That’s our difference compared to every other shop

After a fix, rerun only the failed test, never the whole package.

# Workspace instructions

Read the shared [workspace agreement](../AGENTS.md) completely and preserve
its architecture, standing permissions, focused-test and evidence rules.

## Stoker is retired

Do not use `st`, `stoker` or any Stoker binary (retired 2026-10-21). Run
focused Go tests through the native runners in the shared
[workspace agreement](../AGENTS.md#stoker-is-retired). Use Bun's
`bun.webview()` for screenshots and browser work.


Shared agreement for Claude and Codex: read `/Users/d/code/AGENTS.md` (`/Users/d/code/CLAUDE.md` is a symlink to it) completely before work and after context recovery. This project's `CLAUDE.md` is a symlink to this file.
