#!/usr/bin/env sh

set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"
normalized_repository_root=$(printf '%s' "$repository_root" | tr '\\' '/')

artifact_directory=${1:-".artifacts/gates/local"}
mkdir -p "$artifact_directory"
artifact_directory=$(CDPATH= cd -- "$artifact_directory" && pwd)
go_binary_directory="$artifact_directory/bin"
mkdir -p "$go_binary_directory"
PATH="$go_binary_directory:$PATH"
export PATH
GOBIN="$go_binary_directory"
export GOBIN

result_file="$artifact_directory/gate-results.tsv"
platform=$(go env GOOS)/$(go env GOARCH)
workflow_run=${GITHUB_RUN_ID:-NOT_APPLICABLE}
workflow_attempt=${GITHUB_RUN_ATTEMPT:-NOT_APPLICABLE}
gate_failure_status=0
goconst_admission_maximum=18
benchmark_duration=30s
fuzz_duration=30s
fuzz_minimize_duration=30s

printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
	"gate" "command" "platform" "duration_seconds" "exit_status" "log" "bytes" "sha256" \
	>"$result_file"
printf '%s\n' "$workflow_run" >"$artifact_directory/workflow-run.log"
printf '%s\n' "$workflow_attempt" >"$artifact_directory/workflow-attempt.log"

file_sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		hash_line=$(sha256sum "$1")
	elif command -v shasum >/dev/null 2>&1; then
		hash_line=$(shasum -a 256 "$1")
	else
		printf '%s\n' "no SHA-256 tool found" >&2
		return 1
	fi
	hash_value=${hash_line%% *}
	case "$hash_value" in
		????????????????????????????????????????????????????????????????)
			case "$hash_value" in
				*[!0-9A-Fa-f]*)
					printf '%s\n' "invalid SHA-256 output" >&2
					return 1
					;;
				*)
					printf '%s' "$hash_value"
					;;
			esac
			;;
		*)
			printf '%s\n' "invalid SHA-256 output" >&2
			return 1
			;;
	esac
}

file_bytes() {
	wc -c <"$1" | tr -d ' '
}

clock_now_seconds() {
	date +%s
}

append_result() {
	result_name=$1
	result_command=$2
	result_duration=$3
	result_status=$4
	result_log=$5
	result_bytes=$(file_bytes "$result_log")
	result_hash=$(file_sha256 "$result_log")
	printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
		"$result_name" "$result_command" "$platform" "$result_duration" \
		"$result_status" "$(basename "$result_log")" "$result_bytes" "$result_hash" \
		>>"$result_file"
}

run_gate() {
	gate_name=$1
	shift
	gate_log="$artifact_directory/$gate_name.log"
	gate_command="$*"
	gate_started=$(clock_now_seconds)
	if "$@" >"$gate_log" 2>&1; then
		gate_status=0
	else
		gate_status=$?
	fi
	gate_finished=$(clock_now_seconds)
	gate_duration=$((gate_finished - gate_started))
	append_result \
		"$gate_name" "$gate_command" "$gate_duration" "$gate_status" "$gate_log"
	if test "$gate_status" -eq 0; then
		printf '%s\n' "PASS $gate_name"
		return
	fi
	printf '%s\n' "FAIL $gate_name"
	cat "$gate_log"
	if test "$gate_failure_status" -eq 0; then
		gate_failure_status=$gate_status
	fi
}

run_empty_output_gate() {
	gate_name=$1
	shift
	gate_log="$artifact_directory/$gate_name.log"
	gate_command="$*"
	gate_started=$(clock_now_seconds)
	if "$@" >"$gate_log" 2>&1; then
		gate_status=0
	else
		gate_status=$?
	fi
	if test "$gate_status" -eq 0 && test -s "$gate_log"; then
		gate_status=1
	fi
	gate_finished=$(clock_now_seconds)
	gate_duration=$((gate_finished - gate_started))
	append_result \
		"$gate_name" "$gate_command" "$gate_duration" "$gate_status" "$gate_log"
	if test "$gate_status" -eq 0; then
		printf '%s\n' "PASS $gate_name"
		return
	fi
	printf '%s\n' "FAIL $gate_name"
	cat "$gate_log"
	if test "$gate_failure_status" -eq 0; then
		gate_failure_status=$gate_status
	fi
}

discover_go_targets() {
	target_prefix=$1
	while IFS= read -r package_path; do
		if ! target_output=$(go test -run '^$' -list "^$target_prefix" "$package_path" 2>&1); then
			printf '%s\n' "$target_output" >&2
			return 1
		fi
		printf '%s\n' "$target_output" | while IFS= read -r target_name; do
			case "$target_name" in
				"$target_prefix"*)
					printf '%s\t%s\n' "$package_path" "$target_name"
					;;
			esac
		done
	done <"$package_list"
}

validate_target_inventory() {
	inventory=$1
	minimum=$2
	target_kind=$3
	actual=$(awk 'END { print NR + 0 }' "$inventory")
	if test "$actual" -lt "$minimum"; then
		printf '%s target inventory = %s, want at least %s\n' \
			"$target_kind" "$actual" "$minimum" >&2
		return 1
	fi
	printf '%s target inventory = %s, ratcheted minimum = %s\n' \
		"$target_kind" "$actual" "$minimum"
}

run_benchmarks() {
	benchmark_inventory=$1
	while IFS="$(printf '\t')" read -r package_path target_name; do
		package_name=${package_path##*/}
		evidence_name="benchmark-$package_name-$target_name"
		cpu_profile="$artifact_directory/$evidence_name.cpu.pprof"
		memory_profile="$artifact_directory/$evidence_name.memory.pprof"
		run_gate "$evidence_name" \
			go test -run '^$' -bench "^$target_name$" -benchmem \
			-benchtime="$benchmark_duration" -count=1 -p=1 -parallel=1 \
			-cpuprofile="$cpu_profile" -memprofile="$memory_profile" \
			-outputdir="$artifact_directory" \
			"$package_path"
		write_benchmark_report "$evidence_name" "$package_path" "$target_name" \
			"$cpu_profile" "$memory_profile"
	done <"$benchmark_inventory"
}

write_benchmark_report() {
	evidence_name=$1
	package_path=$2
	target_name=$3
	cpu_profile=$4
	memory_profile=$5
	benchmark_log="$artifact_directory/$evidence_name.log"
	report="$artifact_directory/$evidence_name.md"
	{
		printf '%s\n\n' "# $target_name"
		printf '%s\n' "- Package: \`$package_path\`"
		printf '%s\n' "- Source revision: \`$source_revision\`"
		printf '%s\n' "- Source tree: \`$source_tree_state\`"
		printf '%s\n' "- Toolchain: \`$toolchain_identity\`"
		printf '%s\n' "- Machine: \`$machine_identity\`"
		printf '%s\n' "- Duration: \`$benchmark_duration\`"
		printf '%s\n' "- Command: \`go test -run '^$' -bench '^$target_name$' -benchmem -benchtime=$benchmark_duration -count=1 -p=1 -parallel=1 -cpuprofile=$(basename "$cpu_profile") -memprofile=$(basename "$memory_profile") $package_path\`"
		write_profile_report_line "CPU profile" "$cpu_profile"
		write_profile_report_line "Memory profile" "$memory_profile"
		printf '\n%s\n\n' '```text'
		cat "$benchmark_log"
		printf '%s\n' '```'
	} >"$report"
}

write_profile_report_line() {
	label=$1
	profile=$2
	if test ! -f "$profile"; then
		printf '%s\n' "- $label: unavailable"
		return
	fi
	printf '%s\n' "- $label: \`$(basename "$profile")\` ($(file_bytes "$profile") bytes, SHA-256 \`$(file_sha256 "$profile")\`)"
}

run_deadcode() {
	deadcode -test ./... </dev/null
}

run_fuzz_targets() {
	fuzz_inventory=$1
	while read -r package_path target_name; do
		package_name=${package_path##*/}
		run_gate "fuzz-$package_name-$target_name" \
			go test -run '^$' -fuzz "^$target_name$" \
			-fuzztime="$fuzz_duration" -fuzzminimizetime="$fuzz_minimize_duration" \
			-count=1 -parallel=1 "$package_path"
	done <"$fuzz_inventory"
}

validate_goconst_findings() {
	admissions="scripts/goconst_admissions.tsv"
	admitted="$artifact_directory/goconst-admitted.log"
	observed="$artifact_directory/goconst-observed.log"
	if ! awk -F '\t' '
		NF != 2 || $1 == "" || $2 == "" { exit 1 }
		END { if (NR > maximum) exit 1 }
	' maximum="$goconst_admission_maximum" "$admissions"; then
		printf '%s\n' "goconst admissions are malformed or exceed the ratcheted maximum" >&2
		return 1
	fi
	awk -F '\t' '{ print $1 }' "$admissions" | sort >"$admitted"
	if ! goconst_output=$(goconst -grouped ./... 2>&1); then
		printf '%s\n' "$goconst_output" >&2
		return 1
	fi
	printf '%s\n' "$goconst_output"
	printf '%s\n' "$goconst_output" |
		sed -n 's/.*other occurrence(s) of "\([^"]*\)" found in:.*/\1/p' |
		sort -u >"$observed"
	if ! diff -u "$admitted" "$observed"; then
		printf '%s\n' "goconst findings differ from the reasoned exact admission set" >&2
		return 1
	fi
}


source_revision=$(git rev-parse HEAD)
if test -n "$(git status --porcelain)"; then
	source_tree_state=dirty
else
	source_tree_state=clean
fi
toolchain_identity=$(go version)
machine_identity=$(uname -a)

run_gate go-version go version
run_gate go-environment go env GOOS GOARCH GOVERSION
run_gate go-workspace go env GOWORK
run_gate git-revision git rev-parse HEAD
run_gate git-status git status --short --branch
run_gate install-tools bash scripts/install-tools.sh
run_gate module-tidy go mod tidy -diff
run_gate workflow actionlint .github/workflows/ci.yml
run_empty_output_gate format gofmt -l .

package_list="$artifact_directory/packages.log"
package_stderr="$artifact_directory/packages.stderr.log"
package_started=$(clock_now_seconds)
if go list ./... >"$package_list" 2>"$package_stderr"; then
	package_status=0
else
	package_status=$?
fi
package_finished=$(clock_now_seconds)
package_duration=$((package_finished - package_started))
append_result \
	"package-list-stdout" "go list ./..." "$package_duration" "$package_status" \
	"$package_list"
append_result \
	"package-list-stderr" "go list ./..." "$package_duration" "$package_status" \
	"$package_stderr"
if test "$package_status" -ne 0; then
	printf '%s\n' "FAIL package-list"
	cat "$package_stderr"
	exit "$package_status"
fi
if ! test -s "$package_list"; then
	not_applicable_log="$artifact_directory/package-gates.log"
	printf '%s\n' \
		"NOT_APPLICABLE package gates: the module discovered no Go packages to inspect" \
		>"$not_applicable_log"
	append_result \
		"package-gates" "NOT_APPLICABLE" 0 0 "$not_applicable_log"
	printf '%s\n' "PASS package-gates (not applicable: no packages)"
	exit 0
fi

run_gate install-package-tools bash scripts/install-package-tools.sh
run_gate build go build ./...
run_empty_output_gate go-fix go fix -diff ./...
run_gate vet go vet ./...
run_gate staticcheck staticcheck ./...
run_gate errcheck errcheck ./...
run_gate nilaway nilaway ./...
run_gate witness-lint witness-lint ./...
run_gate complexity gocyclo -over 10 --ignore '_test.go' .
run_gate constants validate_goconst_findings
run_gate field-alignment fieldalignment ./...
run_gate security gosec -quiet ./...
run_gate vulnerabilities govulncheck ./...
run_empty_output_gate dead-code run_deadcode
run_gate test go test -count=1 ./...
run_gate test-race-shuffle go test -race -shuffle=on -count=2 ./...
run_gate fuzz-inventory discover_go_targets Fuzz
run_gate fuzz-inventory-ratchet validate_target_inventory \
	"$artifact_directory/fuzz-inventory.log" 30 fuzz
run_fuzz_targets "$artifact_directory/fuzz-inventory.log"
run_gate benchmark-inventory discover_go_targets Benchmark
run_gate benchmark-inventory-ratchet validate_target_inventory \
	"$artifact_directory/benchmark-inventory.log" 51 benchmark
run_benchmarks "$artifact_directory/benchmark-inventory.log"

if test "$gate_failure_status" -ne 0; then
	printf '%s\n' "FAIL canonical gate; see $result_file"
	exit "$gate_failure_status"
fi
printf '%s\n' "PASS canonical gate"
