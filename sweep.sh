#!/usr/bin/env bash
# sweep.sh - scope and scale for the Primitive SSOT migration.
#
# Prints one row per (repo, package, capability, open sites). Every row is a
# door somebody has to open and read. This tool does NOT rewrite anything:
# the migration is done by hand, package by package. Its only job is to stop
# the inventory from lying, which it has done three ways in one session:
#
#   1. vendor/ leaking in. `grep -rn x .` prints paths as "vendor/..." with no
#      "./" prefix, so a "^./vendor/" filter passes every vendored file and
#      Primitive's own sha256.New() gets counted as a product violation.
#   2. Name collisions. "exec.Command" also matches a product's own
#      bugexec.Command. Import facts here come from `go list`, never a name,
#      and a pattern is only counted inside a package that really imports the
#      stdlib package it would be bypassing.
#   3. BSD vs GNU. \b is unsupported by macOS sed/grep BRE and fails silently,
#      changing nothing while appearing to succeed. Everything here is -E.
#
# Usage:
#   ./sweep.sh                 human-readable inventory
#   ./sweep.sh --tsv doors.tsv also emit repo<TAB>package<TAB>capability<TAB>n
#   SWEEP_ROOT=/path ./sweep.sh
set -uo pipefail

ROOT="${SWEEP_ROOT:-$HOME/code}"
REPOS=(kernel witness bug peachfuzz)

TSV=""
if [[ "${1:-}" == "--tsv" ]]; then
  TSV="${2:?--tsv needs a path}"
  : >"$TSV"
fi

# capability : stdlib import that proves the bypass is possible : call sites
# Keep the import narrow. A package that never imports os/exec cannot contain
# an os/exec site no matter what its identifiers spell.
RULES=(
  "filestore:os:os\.(Stat|Lstat|Open|OpenFile|Create|ReadFile|WriteFile|Remove|RemoveAll|MkdirAll|Rename|OpenRoot|ReadDir)\("
  "core.DigestWriter:crypto/sha256:sha256\.New\("
  "keygen:crypto/ed25519:ed25519\.(NewKeyFromSeed|GenerateKey|Sign|Verify)\("
  "keygen:crypto/rand:rand\.(Read\(|Reader)"
  "filelock:syscall:syscall\.Flock\("
  "process:os/exec:exec\.(Command|CommandContext|LookPath)\("
  "exchange:net/http:http\.(Client\{|DefaultClient|NewRequest|Get\(|Post\()"
  "temporal:time:time\.(Now|Since|After|Sleep)\("
)

grand=0
for repo in "${REPOS[@]}"; do
  dir="$ROOT/$repo"
  if [[ ! -d "$dir" ]]; then
    printf '\n===== %s ===== SKIP (not at %s)\n' "$repo" "$dir"
    continue
  fi
  printf '\n===== %s =====\n' "$repo"

  # Compiler truth, not a source grep: package -> dir -> direct imports.
  listing=$(cd "$dir" && go list -f '{{.ImportPath}}	{{.Dir}}	{{join .Imports " "}}' ./... 2>/dev/null | grep -v '/vendor/')
  if [[ -z "$listing" ]]; then
    printf '  (go list produced nothing)\n'
    continue
  fi

  repo_total=0
  rows=""
  while IFS=$'\t' read -r pkg pdir imports; do
    [[ -n "${pdir:-}" ]] || continue
    for rule in "${RULES[@]}"; do
      cap="${rule%%:*}"; rest="${rule#*:}"
      imp="${rest%%:*}"; pat="${rest#*:}"
      # Require the import before counting anything.
      case " $imports " in *" $imp "*) ;; *) continue ;; esac

      n=$(find "$pdir" -maxdepth 1 -name '*.go' ! -name '*_test.go' -print0 2>/dev/null \
          | xargs -0 -r grep -hoE "$pat" 2>/dev/null | wc -l | tr -d ' ')
      n=${n:-0}
      if (( n > 0 )); then
        short="${pkg##*/offGridSoft/}"; short="${short#*/}"
        rows+=$(printf '%6d  %-42s %s\n' "$n" "$short" "$cap")$'\n'
        repo_total=$((repo_total + n))
        [[ -n "$TSV" ]] && printf '%s\t%s\t%s\t%d\n' "$repo" "$short" "$cap" "$n" >>"$TSV"
      fi
    done
  done <<<"$listing"

  printf '%s' "$rows" | sort -rn
  printf '%6d  %s\n' "$repo_total" "== $repo total =="
  grand=$((grand + repo_total))
done

printf '\n%6d  == ALL REPOS ==\n' "$grand"
[[ -n "$TSV" ]] && printf 'rows: %s\n' "$TSV"
exit 0
