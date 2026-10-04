# Aion 1.9 server in Go

This is the tracked Go port of Aion Lightning's login, chat and game servers.
Subsystem and quest coverage, known gaps and verification work are maintained
in [PORTING.md](PORTING.md). The original Java
source and its 55 MB of game data remain at
`java`; this module uses
the original `AL-Game/data/static_data` without copying it into the repository.

The Go module requires Go 1.25. On this Mac, Go runs in Apple's `container`:

```sh
go/scripts/run-go.sh go test ./...
go/scripts/run-go.sh go vet ./...
go/scripts/run-go.sh go build ./cmd/...
```

`run-go.sh` mounts the static data and sets `AION_DATA`, so the data-backed tests
run instead of skipping. Set `AION_DATA_PATH` to use a different copy of the
original data.

Run the baseline checks sequentially with `go/scripts/check-go.sh`. It saves
test JSON, exact test-node counts (including subtests), optional database skips,
vet/build output and formatting results under `.build/go-checks/` at the
repository root. Set `AION_CHECK_DIR` to choose another output directory.
The wrapper uses the same pinned Go toolchain as `run-go.sh` and builds all
seven commands. Optional database tests remain skipped when their fixture is
unavailable; they are not counted as passing. Lint/vulnerability scans and a
disposable database integration job are tracked separately in
[the review ledger](../docs/GO_REVIEW_LEDGER.md).

Build and publish images from the repository root with
`python3 scripts/images.py build` and `python3 scripts/images.py push`.
`image-manifest.json` pins the client version, server release, platform, and
Go/Java role tags. The compatibility `go/scripts/build-images.sh` builds the
five Go roles only. See [the repository README](../README.md) for image names.

## Panel (website)

`cmd/panel` is the server's website on port 8080: a public account sign-up, and
GM (access level 1+: characters, account locks, IP bans) and admin tools
(level 3+: access levels, server options, game server rows) for players who
sign in with their game account. It needs only the database:

GMs can search a character (`/character?name=`) and see its equipment and
cube drawn like the client's Profile window, with item tooltips. That art and
item data come from the 1.9 client itself: run
`python3 go/scripts/extract-panel-assets.py` once (Pillow needed)
to write `/Volumes/acasis/games/aion/servers/panel-assets`, which
`go/scripts/build-images.sh panel` copies into the image. It is game content, so it stays
out of the repository.

```sh
go/scripts/build-images.sh panel
container run --detach --name al19-panel -p 127.0.0.1:8080:8080 -e AION_DB=<al19-db ip> docker.io/rafabertholdo/aiongo:1.9-panel-go-v0.1.0
```

Server options live in `au_server_ls.server_options` (catalog in
`options/options.go`); the login and game servers read them at start, and a
stored value wins over the environment variable of the same name. To make the
first admin: `UPDATE au_server_ls.account_data SET access_level = 3 WHERE name = '<account>'`.
