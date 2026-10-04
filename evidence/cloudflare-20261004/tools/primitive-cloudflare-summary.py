import json
import pathlib
import re

root = pathlib.Path('/private/tmp/primitive-cloudflare-proof')
stage = pathlib.Path('/private/tmp/primitive-cloudflare-evidence-stage')
revision = '0e6c118e1d0c5db042da9054a3eee8a61e766254'

def receipt(name, directory=root):
    value = json.loads((directory / (name + '.receipt.json')).read_text())
    if value['revision'] != revision or value['source_tree'] != 'clean':
        raise SystemExit('final execution is not bound to clean committed source: ' + name)
    return value

def fuzz_summary(prefix, directory):
    rows = []
    for path in sorted(directory.glob(prefix + '-Fuzz*.receipt.json')):
        name = path.name.removesuffix('.receipt.json')
        value = receipt(name, directory)
        samples = re.findall(r'execs: (\d+)', (directory / (name + '.stdout')).read_text())
        rows.append({'attempt': name, 'exit_status': value['exit_status'], 'executions': int(samples[-1]) if samples else None})
    if len(rows) != 12 or any(row['executions'] is None for row in rows):
        raise SystemExit('fuzz target inventory or execution count incomplete: ' + prefix)
    return {'targets': rows, 'failed_targets': sum(row['exit_status'] != 0 for row in rows), 'executions': sum(row['executions'] for row in rows)}

names = ['fix', 'vet', 'fieldalignment', 'gocyclo', 'goconst', 'nilaway', 'errcheck', 'staticcheck', 'deadcode', 'deadcode-test', 'govulncheck', 'gosec', 'witness-lint', 'test', 'race']
gate = {name: receipt('gate-03-' + name) for name in names}
mac_fuzz = fuzz_summary('fuzz-02', root)
furnace_fuzz = fuzz_summary('furnace-fuzz-02', root / 'furnace')
furnace_race = receipt('furnace-focused-race-02', root / 'furnace')
expected_names = {'zero declaration', 'zero hazard', 'zero scope', 'future hazard', 'future scope', 'process hazard with sibling scope', 'sibling hazard with process scope'}
event_classifications = {}
for phase in ('test', 'race'):
    expected = []
    unexpected = []
    skipped = []
    with (root / ('gate-03-' + phase + '.stdout')).open() as stream:
        for line in stream:
            try:
                event = json.loads(line)
            except ValueError:
                continue
            if event.get('Action') == 'fail' and event.get('Test'):
                destination = expected if event.get('Package', '').endswith('/testserial') and event['Test'] in expected_names else unexpected
                destination.append({'package': event['Package'], 'test': event['Test']})
            if event.get('Action') == 'skip':
                skipped.append({'package': event['Package'], 'test': event.get('Test')})
    event_classifications[phase] = {'expected_nested_rejections': expected, 'unexpected_test_failures': unexpected, 'skipped': skipped}

summary = {'kind': 'implementation_execution_summary', 'source_revision': revision, 'acceptance_issued': False,
           'required_gate_passed': all(value['exit_status'] == 0 for value in gate.values()),
           'gate': {name: {key: value.get(key) for key in ('argv', 'exit_status', 'passed', 'failed', 'skipped', 'packages_passed', 'packages_failed')} for name, value in gate.items()},
           'event_classifications': event_classifications, 'mac_fuzz': mac_fuzz, 'furnace_fuzz': furnace_fuzz,
           'furnace_focused_race': {key: furnace_race[key] for key in ('argv', 'exit_status', 'passed', 'failed', 'skipped', 'packages_passed', 'packages_failed')}}
(stage / 'final-summary.json').write_text(json.dumps(summary, indent=2) + '\n')

notes = {'fieldalignment': 'Existing layout findings outside the new SDK/signing files.',
         'gocyclo': 'Existing Plunk functions at complexity 15, 11 and 11.',
         'goconst': 'Exit 0 with existing repeated-token diagnostics; output is retained.',
         'deadcode': 'Library module: tool exits 1 with “no main packages”; no fake entrypoint was added.',
         'witness-lint': '16 existing findings across 8 unchanged files; existing waiver counts are retained, no waiver added.',
         'test': '68 packages pass; 3 live GCS smoke skips; 7 expected nested rejection events.',
         'race': '68 packages pass; 6 live GCS smoke skips over two passes; 28 expected nested rejection events.',
         'govulncheck': '0 reachable vulnerabilities; unused dependency findings remain in raw output.'}
lines = ['# Final execution results', '', 'Source: `' + revision + '`; clean Mac and Furnace checkouts, Go 1.27.1.', '',
         '**The complete required gate is not passing.** Existing static findings and the library-only deadcode invocation remain explicit. Passing package tests do not erase these results.', '',
         '| Gate command | Exit | Result detail |', '| --- | ---: | --- |']
for name in names:
    value = gate[name]
    argv = value['argv']
    command = ' '.join(argv)
    if name in ('gocyclo', 'nilaway'):
        command = name + ' [complete discovered source/package scope; see receipt]'
    lines.append('| `' + command + '` | ' + str(value['exit_status']) + ' | ' + notes.get(name, 'Passed.') + ' |')
lines += ['', '## Behavioral results', '',
          '- Mac full ordinary suite: 68 passing packages; 90,794 pass events, 7 expected nested rejection events, 3 skips.',
          '- Mac full race/shuffle/count=2 suite: 68 passing packages; 181,600 pass events, 28 expected nested rejection events, 6 skips.',
          '- Furnace focused race/shuffle/count=2 suite: ' + str(furnace_race['passed']) + ' pass events, 0 failures and 0 skips across Cloudflare, Exchange, Core, Objectstore and authored claims.',
          '- Mac semantic fuzz: 12 targets, ' + str(mac_fuzz['executions']) + ' executions, ' + str(mac_fuzz['failed_targets']) + ' failed targets.',
          '- Furnace semantic fuzz: 12 targets, ' + str(furnace_fuzz['executions']) + ' executions, ' + str(furnace_fuzz['failed_targets']) + ' failed targets.', '',
          'Counts include subtests and seed executions, not unique requirements. The exact skipped tests and all raw failure events are retained in `final-summary.json` and the archived logs. Three unavailable live GCS smoke tests are not represented as passed. No live Cloudflare provider smoke is claimed.', '',
          '## Existing gate findings', '',
          'The Witness findings are in `_tools/fieldalignmentgate`, Controlplane registration, Filestore copy/sort helper signatures, Google Identity IAM, Plunk Operation enum contracts, and Runnercontrol enum switches. `gate-findings-baseline-bindings.json` in the history archive proves these production files are byte-identical to baseline `64264429f65417c96ea1e38c481c4f6ff9f9aeb1`.', '',
          'These remain open proof surfaces, not waivers or successful checks. Raw fieldalignment output also retains existing protocol/layout findings. No blanket layout rewrite was used to change unrelated signed wire structures. The direct `deadcode ./...` invocation is unavailable for this library shape; `deadcode -test ./...` completed successfully.', '',
          'The final source checkpoint adds no Witness waiver. The existing repository waiver inventory printed by Witness remains visible. All final command scopes and complete expanded argument vectors are in `final.attempts.json` and the individual archived receipts.', '',
          'The final gate launcher exits 1, matching the failed child commands. This is implementation evidence for review, not independent acceptance.', '']
(stage / 'FINAL_RESULTS.md').write_text('\n'.join(lines))
print(json.dumps({'gate_passed': summary['required_gate_passed'], 'mac_fuzz_executions': mac_fuzz['executions'], 'furnace_fuzz_executions': furnace_fuzz['executions']}))
