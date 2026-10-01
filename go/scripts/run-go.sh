#!/bin/zsh
# Run a Go command against the tracked source with the original static data mounted.
set -eu

if (( $# == 0 )); then
    print -u2 'usage: scripts/run-go.sh go <build|test|vet|...> [arguments]'
    exit 2
fi
go_command=$1
shift
case "$go_command" in
    go|gofmt) ;;
    *) print -u2 'run-go.sh accepts go or gofmt'; exit 2 ;;
esac

module_dir=$(cd "${0:A:h}/.." && pwd)
data_dir=${AION_DATA_PATH:-$module_dir/../java/AL-Game/data/static_data}
if [[ ! -d "$data_dir" ]]; then
    print -u2 "Aion static data not found: $data_dir (set AION_DATA_PATH)"
    exit 1
fi

exec container run --rm --arch arm64 --memory 4G --cpus 4 \
    -v go-cache:/go \
    -v go-build-cache:/root/.cache/go-build \
    -v "$module_dir:/repo" \
    -v "$data_dir:/data/static_data" \
    -w /repo -e AION_DATA=/data/static_data \
    golang:1.25 "/usr/local/go/bin/$go_command" "$@"
