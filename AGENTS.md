# Workspace instructions

Read the shared [workspace agreement](../AGENTS.md) completely and preserve
its architecture, standing permissions, focused-test and evidence rules.

## Stoker test orchestration

Use installed `stoker` for supported tests on exact committed revisions and
for retrieving their evidence. Follow the Stoker section of the shared
[workspace agreement](../AGENTS.md) and read `stoker man run` before submitting.
Stoker handles live resource admission, host selection, native execution and
SSH/SCP evidence delivery to Furnace. Keep test scope focused and preserve any
project requirement to use Furnace with `--system furnace`. Inspect
`stoker status RUN_ID` and the terminal receipt, then use
`stoker evidence RUN_ID --artifact stdout`; submission alone is not a pass.
Standing permission covers authorized configured execution and evidence return,
subject to mandatory platform sandbox restrictions.

