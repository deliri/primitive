import argparse
import hashlib
import json
import pathlib
import tarfile

parser = argparse.ArgumentParser()
parser.add_argument('phase', choices=['history', 'final'])
args = parser.parse_args()
source = pathlib.Path('/private/tmp/primitive-cloudflare-proof')
destination = pathlib.Path('/private/tmp/primitive-cloudflare-evidence-stage')
destination.mkdir(exist_ok=True)

def digest(path):
    checksum = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            checksum.update(block)
    return checksum.hexdigest()

def final_path(path):
    name = path.name
    return name.startswith(('gate-03-', 'fuzz-02-', 'furnace-focused-race-02', 'furnace-fuzz-02'))

files = [p for p in sorted(source.rglob('*')) if p.is_file() and final_path(p) == (args.phase == 'final')]
entries = []
for path in files:
    entries.append({'path': path.relative_to(source).as_posix(), 'bytes': path.stat().st_size, 'sha256': digest(path)})
by_path = {entry['path']: entry for entry in entries}
attempts = []
for path in files:
    if not path.name.endswith('.receipt.json'):
        continue
    receipt = json.loads(path.read_text())
    for artifact in receipt.get('artifacts', []):
        artifact_path = (path.parent / artifact['path']).relative_to(source).as_posix()
        sealed = by_path[artifact_path]
        if any(sealed[key] != artifact[key] for key in ('bytes', 'sha256')):
            raise SystemExit('receipt/manifest/disk disagreement: ' + artifact_path)
    attempts.append({'receipt': path.relative_to(source).as_posix(), **{key: receipt.get(key) for key in (
        'revision', 'source_tree', 'started_unix_ns', 'finished_unix_ns', 'argv', 'cwd', 'toolchain',
        'cache_posture', 'filter', 'exit_status', 'signal', 'passed', 'failed', 'skipped',
        'packages_passed', 'packages_failed', 'packages_skipped', 'unavailable', 'timed_out', 'not_run', 'build_failed')}})
attempts.sort(key=lambda attempt: (attempt.get('started_unix_ns') or 0, attempt['receipt']))

archive = destination / (args.phase + '.tar.gz')
with tarfile.open(archive, 'w:gz', compresslevel=6) as bundle:
    for path, entry in zip(files, entries):
        info = tarfile.TarInfo(entry['path'])
        info.size = entry['bytes']
        info.mode = 0o600
        info.mtime = 0
        with path.open('rb') as stream:
            bundle.addfile(info, stream)

# A second streaming walk verifies the persisted archive rather than trusting
# the writer's input list. It also refuses extra, duplicate and missing entries.
seen = set()
with tarfile.open(archive, 'r|gz') as bundle:
    for member in bundle:
        if not member.isfile() or member.name not in by_path or member.name in seen:
            raise SystemExit('unexpected or duplicate archive member: ' + member.name)
        seen.add(member.name)
        checksum = hashlib.sha256()
        size = 0
        stream = bundle.extractfile(member)
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            checksum.update(block)
            size += len(block)
        entry = by_path[member.name]
        if size != entry['bytes'] or checksum.hexdigest() != entry['sha256']:
            raise SystemExit('archive/manifest disagreement: ' + member.name)
if seen != set(by_path):
    raise SystemExit('archive omitted retained evidence')

manifest = {'kind': 'local_execution_evidence_manifest', 'acceptance_issued': False,
            'archive': {'path': archive.name, 'bytes': archive.stat().st_size, 'sha256': digest(archive)},
            'files': entries}
(destination / (args.phase + '.manifest.json')).write_text(json.dumps(manifest, indent=2) + '\n')
(destination / (args.phase + '.attempts.json')).write_text(json.dumps(attempts, indent=2) + '\n')
print(json.dumps({'phase': args.phase, 'files': len(entries), 'attempts': len(attempts), 'archive': manifest['archive']}))
