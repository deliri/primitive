import argparse, hashlib, json, os, pathlib, platform, subprocess, time, re

p = argparse.ArgumentParser()
p.add_argument('--name', required=True)
p.add_argument('--cwd', required=True)
p.add_argument('--root')
p.add_argument('command', nargs=argparse.REMAINDER)
a = p.parse_args()
argv = a.command[1:] if a.command[:1] == ['--'] else a.command
root = pathlib.Path(a.root or os.environ.get('PEACHFUZZ_TEST_EVIDENCE_ROOT','/private/tmp/peachfuzz-v22-proof' if a.name.startswith('v22-') else '/private/tmp/peachfuzz-v21-proof'))
root.mkdir(parents=True,exist_ok=True)
base = root / a.name
if base.with_suffix('.receipt.json').exists():
    raise SystemExit('attempt name already retained; choose a new name')
env = os.environ.copy()
env.update(GOWORK='off', GOCACHE=os.environ.get('PEACHFUZZ_TEST_GOCACHE','/private/tmp/peachfuzz-go-cache'))
if a.name in ('required-go-test','required-go-test-race'):
    env['GOFLAGS']=env.get('GOFLAGS','')+' -v'
revision = subprocess.check_output(['git','rev-parse','HEAD'],cwd=a.cwd,text=True).strip()
status = subprocess.check_output(['git','status','--porcelain=v1'],cwd=a.cwd,text=True)
source_bindings = []
paths = set(subprocess.check_output(['git','ls-files','--modified','--others','--exclude-standard','-z'],cwd=a.cwd).decode().split('\0'))
for name in sorted(paths - {''}):
    path = pathlib.Path(a.cwd) / name
    if path.is_file():
        h = hashlib.sha256()
        with path.open('rb') as stream:
            for block in iter(lambda:stream.read(65536),b''): h.update(block)
        source_bindings.append({'path':name,'sha256':h.hexdigest(),'bytes':path.stat().st_size})
start = time.time_ns()
superseded = False
with open(str(base)+'.stdout','wb') as out, open(str(base)+'.stderr','wb') as err:
    if superseded:
        err.write(b'Planned phase superseded by the user-required complete final gate; no Go tests executed in this phase.\n')
        result = subprocess.CompletedProcess(argv,75)
    else:
        result = subprocess.run(argv,cwd=a.cwd,env=env,stdout=out,stderr=err)
facts = {'revision':revision,'source_tree':'dirty' if status else 'clean','source_status':status.splitlines(),'source_bindings':source_bindings,
         'argv':argv,'cwd':a.cwd,'toolchain':subprocess.check_output(['go','version'],env=env,text=True).strip(),
         'machine':platform.platform(),'environment':{key:env[key] for key in ('GOWORK','GOCACHE','GOMODCACHE','GOPROXY','GOPRIVATE','GONOPROXY','GONOSUMDB','GOSUMDB','GOFLAGS','GOMAXPROCS') if key in env},
         'started_unix_ns':start,'finished_unix_ns':time.time_ns(),'exit_status':result.returncode,
         'signal':-result.returncode if result.returncode < 0 else None,
         'execution_state':'superseded_not_run' if superseded else 'executed',
         'cache_posture':'bypassed' if '-count=1' in argv or '-count=2' in argv or '-count=1' in env.get('GOFLAGS','') else 'not asserted',
         'filter':argv[argv.index('-run')+1] if '-run' in argv else None,
         'counts_basis':'Go JSON action events include subtests and fuzz seeds. Verbose fallback counts are raw log events and may include expected child-test failures. Unknown availability and not-run counts remain explicit',
         'passed':0,'failed':0,'skipped':0,'packages_passed':0,'packages_failed':0,'packages_skipped':0,
         'unavailable':None,'timed_out':None,'not_run':None,'build_failed':0,'artifacts':[]}
with open(str(base)+'.stdout','rb') as stream:
    for line in stream:
        try: event = json.loads(line)
        except (ValueError,UnicodeDecodeError):
            decoded=line.decode('utf-8','replace')
            match=re.match(r'^\s*--- (PASS|FAIL|SKIP):',decoded)
            if match:
                facts[{'PASS':'passed','FAIL':'failed','SKIP':'skipped'}[match.group(1)]]+=1
            elif decoded.startswith('ok\t') or decoded.startswith('ok  \t'):
                facts['packages_passed']+=1
            elif decoded.startswith('FAIL\t'):
                facts['packages_failed']+=1
            elif decoded.startswith('?') and '[no test files]' in decoded:
                facts['packages_skipped']+=1
            continue
        if not isinstance(event, dict):
            continue
        action=event.get('Action')
        if action == 'build-fail': facts['build_failed'] += 1
        if action in ('pass','fail','skip'):
            key={'pass':'passed','fail':'failed','skip':'skipped'}[action]
            if not event.get('Test'): key='packages_'+key
            facts[key]+=1
for suffix in ('.stdout','.stderr'):
    path=pathlib.Path(str(base)+suffix)
    h=hashlib.sha256(); size=0
    with path.open('rb') as stream:
        for block in iter(lambda:stream.read(65536),b''): h.update(block); size+=len(block)
    facts['artifacts'].append({'path':path.name,'bytes':size,'sha256':h.hexdigest()})
receipt_tmp=pathlib.Path(str(base)+'.receipt.stage')
receipt_tmp.write_text(json.dumps(facts,indent=2)+'\n')
receipt_tmp.replace(pathlib.Path(str(base)+'.receipt.json'))
print(json.dumps({'attempt':a.name,'revision':revision,'exit_status':result.returncode,'build_failures':facts['build_failed'],'passed_events':facts['passed'],'failed_events':facts['failed'],'skipped_events':facts['skipped']}))
raise SystemExit(result.returncode)
