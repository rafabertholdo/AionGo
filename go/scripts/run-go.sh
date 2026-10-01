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

# Pin the validation toolchain and serialize compilation: Go 1.25.14 crashed
# inside the Go tool on this Apple container host during large test builds.
exec container run --rm --arch arm64 --memory 4G --cpus 1 \
    -v "$module_dir:/repo" \
    -v "$data_dir:/data/static_data" \
    -w /repo -e AION_DATA=/data/static_data -e GOMAXPROCS=1 -e GODEBUG=asyncpreemptoff=1 \
    golang:1.25.1 "/usr/local/go/bin/$go_command" "$@"
