import pathlib, re, subprocess, sys
repo=pathlib.Path('/data/repos/primitive')
phase=sys.argv[1]
failed = False
for package,glob in [('cloudflare','*_test.go'),('exchange','signing_internal_test.go')]:
 for path in sorted((repo/package).glob(glob)):
  for target in re.findall(r'^func (Fuzz\w+)\(',path.read_text(),re.M):
   argv=['python3','/work/runtime/primitive-cloudflare-proof/primitive-cloudflare-run.py','--root','/data/evidence/primitive/cloudflare-20261004','--name',phase+'-'+target,'--cwd',str(repo),'--','go','test','-json','-count=1','./'+package,'-run','^$','-fuzz','^'+target+'$','-fuzztime=2s','-fuzzminimizetime=1s','-parallel=2']
   result = subprocess.run(argv,check=False)
   failed = failed or result.returncode != 0

raise SystemExit(1 if failed else 0)
