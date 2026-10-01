#!/bin/zsh
# Compatibility wrapper for the tracked Go module in Apps/AionServer.
module_dir=${0:A:h:h:h}/go
if [[ "$1" == go || "$1" == gofmt ]]; then
    exec "$module_dir/scripts/run-go.sh" "$@"
fi
# Preserve the old single-string shell command interface.
data_dir=${AION_DATA_PATH:-$module_dir/../java/AL-Game/data/static_data}
exec container run --rm --arch arm64 -v go-cache:/go -v go-build-cache:/root/.cache/go-build -v "$module_dir:/repo" -v "$data_dir:/data/static_data" -w /repo -e AION_DATA=/data/static_data golang:1.25 sh -c "$*"
