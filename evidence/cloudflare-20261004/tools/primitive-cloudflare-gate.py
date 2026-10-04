import pathlib, subprocess, sys, os, json
repo='/Users/d/code/primitive'
root='/private/tmp/primitive-cloudflare-proof'
phase=sys.argv[1]
env=os.environ.copy()
# JSON is a test argument, never GOFLAGS: child go-list invocations must retain their own output contract.
env['GOFLAGS']=''
recorder=['python3','/private/tmp/primitive-cloudflare-run.py','--root',root,'--cwd',repo]
commands=[('fix',['go','fix','./...']),('vet',['go','vet','./...']),('fieldalignment',['fieldalignment','./...']),('gocyclo',None),('goconst',['goconst','-ignore','vendor','-min-occurrences','3','-min-length','4','-ignore-tests','./...']),('nilaway',None),('errcheck',['errcheck','./...']),('staticcheck',['staticcheck','./...']),('deadcode',['deadcode','./...']),('deadcode-test',['deadcode','-test','./...']),('govulncheck',['govulncheck','./...']),('gosec',['gosec','./...']),('witness-lint',['witness-lint','./...']),('test',['go','test','-json','-count=1','./...']),('race',['go','test','-json','-race','-shuffle=on','-count=2','./...'])]
failed = False
for name,cmd in commands:
 if name=='gocyclo':
  files=subprocess.check_output(['find','.','-path','./vendor','-prune','-o','-name','*_test.go','-prune','-o','-name','*.go','-print'],cwd=repo,text=True).splitlines()
  cmd=['gocyclo','-over','10',*files]
 if name=='nilaway':
  discovery=subprocess.run(recorder+['--name',phase+'-package-scope','--','go','list','./...'],env=env)
  if discovery.returncode: raise SystemExit(discovery.returncode)
  packages=[x for x in pathlib.Path(root,phase+'-package-scope.stdout').read_text().splitlines() if '/vendor/' not in x]
  cmd=['nilaway',*packages]
 env['GOFLAGS']=''
 print('START '+phase+'-'+name,flush=True)
 result=subprocess.run(recorder+['--name',phase+'-'+name,'--',*cmd],env=env)
 print('FINISH '+name+' exit='+str(result.returncode),flush=True)
 failed = failed or result.returncode != 0

raise SystemExit(1 if failed else 0)
