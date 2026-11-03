# Typed Plunk attachment checkpoint

Primitive 473 collects Claude's exact `9856c5507055e8e43cc0e8a054b5194fbae35f64`
attachment capability. `SendRequest.Attachments` emits the documented filename,
contentType and base64 content objects. Absent attachments omit the member.
The SDK bounds count, aggregate bytes, safe filenames and native MIME grammar.
The current provider reference documents ten attachments/ten MB; the older guide
admits five. This SDK conservatively admits five and ten MiB of decoded content.
The filename/media byte ceilings are local SDK admission policy, not a claim
about additional provider limits.

Retained native evidence (An, race enabled, no skips): original `8fa4da03` passed
23 selected nodes; exact-boundary `04c491b4` passed 29 new nodes. One-fact wire
member omission `a8dbd373` failed the existing encoding assertion in `3516592f`;
the mutation is retained only at `refs/archive/controls/plunk-attachment-omission`
and is not a main ancestor. Restored source `3cd059e3` passed a configured three
second semantic fuzz run `cfeca62f`: 1,142 reported executions, ten interesting.

Affected Core/Plunk vet, Staticcheck and errcheck have zero findings. Final
Plunk Witness `85700131` has zero attachment-owned findings and 61 unchanged
472 findings; Core has seven unchanged findings. Those remain open, not waived.
The earlier two owned diagnostic-only repair attempts are retained. Compass
metadata `e1d3574d` passed three nodes in `f51fa291`. Original receipt/execution
seals, complete commands, positive reader joins and byte-exact baseline bindings
are indexed in `/private/tmp/primitive473-SOURCE-HANDOFF.json`.

This is SDK encoding/admission proof using local fixtures, not provider delivery,
inbox/calendar import, live account verification or complete TimeMatch readiness.
