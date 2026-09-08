#!/bin/sh
set -eu
artifact_directory=/Users/d/code/primitive/.artifacts/release-v2026.1.19
goconst_admission_maximum=4
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
	if ! goconst_output=$(goconst -grouped -min-length 4 -min-occurrences 3 -ignore-tests -ignore '(^|/)(testdata|[._][^/]+)(/|$)' ./... 2>&1); then
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
validate_goconst_findings
