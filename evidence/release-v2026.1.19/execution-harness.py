from pathlib import Path
import subprocess,json,hashlib,sys,time,os
root=Path('/Users/d/code/primitive')
out=root/'.artifacts/release-v2026.1.19';out.mkdir(parents=True,exist_ok=True)
def descriptor(p):
 b=p.read_bytes();return dict(path=str(p.relative_to(root)),bytes=len(b),sha256=hashlib.sha256(b).hexdigest())
def sources():
 names=subprocess.check_output(['rg','--files','--hidden','-g','*.go','-g','go.mod','-g','go.sum','-g','compass/config.json','-g','_docs/testing_protocol.md','-g','!**/testdata/**','-g','!.git/**'],cwd=root,text=True).splitlines()
 names += subprocess.check_output(['rg','--files','scripts',*[str(p.relative_to(root)) for p in (root/'exchange/testdata/fuzz',root/'filestore/testdata/fuzz') if p.exists()]],cwd=root,text=True).splitlines()
 rows=[descriptor(root/n) for n in sorted(set(names))]
 data=json.dumps(rows,sort_keys=True,indent=2).encode()+b'\n';sha=hashlib.sha256(data).hexdigest();p=out/('sources-'+sha+'.json')
 if not p.exists():p.write_bytes(data)
 return descriptor(p)
label=sys.argv[1];argv=sys.argv[2:];path=out/(label+'.json');assert not path.exists(),path
before=sources();start=time.time()
with (out/(label+'.stdout.txt')).open('wb') as stdout,(out/(label+'.stderr.txt')).open('wb') as stderr:
 proc=subprocess.run(argv,cwd=root,stdin=subprocess.DEVNULL,stdout=stdout,stderr=stderr)
record=dict(label=label,argv=argv,cwd=str(root),revision=subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),tree=subprocess.check_output(['git','status','--porcelain'],cwd=root,text=True),toolchain=subprocess.check_output(['go','version'],text=True).strip(),environment=subprocess.check_output(['go','env','GOOS','GOARCH','GOWORK','CGO_ENABLED','GOFLAGS'],text=True).splitlines(),started_unix=start,elapsed_seconds=time.time()-start,exit_status=proc.returncode,sources_before=before,sources_after=sources(),artifacts=[descriptor(out/(label+'.'+stream+'.txt')) for stream in ('stdout','stderr')],cache_posture='test result cache disabled by -count=1' if '-count=1' in argv else 'normal build/analyzer cache; no test execution unless specified')
if argv[:2]==['go','test']:
 events=[]
 for line in (out/(label+'.stdout.txt')).read_text().splitlines():
  try:events.append(json.loads(line))
  except ValueError:pass
 record['test_events']={a:sum(e.get('Action')==a and 'Test' in e for e in events) for a in ('pass','fail','skip')}
 record['package_events']=[e for e in events if 'Test' not in e and e.get('Action') in ('pass','fail','skip')]
path.write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps({k:record[k] for k in ('label','argv','elapsed_seconds','exit_status')},indent=2))
for stream in ('stdout','stderr'):
 p=out/(label+'.'+stream+'.txt');s=p.read_text();print(stream+': '+s[:5000]+('\n[remaining output retained]' if len(s)>5000 else ''))
