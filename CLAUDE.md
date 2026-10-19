# Agent instructions

Read [AGENTS.md](AGENTS.md) and the shared [workspace agreement](../AGENTS.md)
completely before work and after context recovery. Existing project rules apply.

## Stoker test orchestration

Default to `stoker run --project PROJECT --revision COMMIT --plan PLAN.json --wait`
for supported build, vet, lint, test, fuzz and mutation work. Commit the plan in
the owning project and keep its scope focused. Stoker's
`compass/example-plan.json` and `_testdata/wait-proof-plan.json` show the typed
shapes; reference examples do not authorize their unrelated test scope.
A single focused Go test can use `stoker run --project PROJECT --revision COMMIT --wait -- ./affected/package '-run=^TestName$'`.

Project profiles own preparation, allowed compiler settings and actual budgets.
Do not repeat their setup in shell scripts or override Stoker's resource
environment. Use typed steps instead of `--kind tool`, inline shell or Python
for supported operations. Let Stoker manage resource waits, source fetching,
worker selection, private scratch cleanup and verified evidence delivery.

Invoke `stoker` from PATH directly. Omit `--system` unless an existing native-host
requirement applies; `--server` is an administrative coordinator selection,
never an evidence destination. Retrieve with
`stoker evidence RUN_ID --artifact stdout`, without host flags or shell pipes.
Standing permission covers configured execution and evidence return; mandatory
platform restrictions still apply. Inspect the terminal result and actual test
events before claiming success. The complete batch wait is unlimited by default;
`--wait-timeout-seconds N` limits caller patience without cancelling the order.

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

