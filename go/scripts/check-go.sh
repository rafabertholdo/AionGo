#!/bin/zsh
# Repository checks run sequentially through the pinned Apple-container runner.
set -eu

script_dir=${0:A:h}
repo_dir=${script_dir:h:h}
check_dir=${AION_CHECK_DIR:-$repo_dir/.build/go-checks}
mkdir -p "$check_dir"
runner="$script_dir/run-go.sh"

print 'Running Go tests (including static data); results will be saved to:' "$check_dir"
if "$runner" go test -json -count=1 ./... > "$check_dir/tests.jsonl" 2> "$check_dir/tests.stderr"; then
    python3 "$script_dir/summarize_tests.py" "$check_dir/tests.jsonl" > "$check_dir/summary.json"
else
    cat "$check_dir/tests.stderr" >&2
    python3 "$script_dir/summarize_tests.py" "$check_dir/tests.jsonl" || true
    exit 1
fi
cat "$check_dir/summary.json"

print 'Running go vet'
"$runner" go vet ./... > "$check_dir/vet.log" 2>&1 || { cat "$check_dir/vet.log" >&2; exit 1; }
print 'Building all Go commands'
"$runner" go build ./cmd/... > "$check_dir/build.log" 2>&1 || { cat "$check_dir/build.log" >&2; exit 1; }
print 'Checking Go formatting'
"$runner" gofmt -l . > "$check_dir/format.log" 2> "$check_dir/format.stderr"
if [[ -s "$check_dir/format.log" ]]; then
    cat "$check_dir/format.log" >&2
    exit 1
fi
print 'Go tests, vet, command builds and formatting passed.'
