# AionGo

Aion 1.9 servers: the Go port, Java 21 reference implementation, MariaDB schema,
static game data, and the account/GM/admin website developed for ReRun.

| Directory | Contents |
| --- | --- |
| `go/` | Go login, chat, game, packet tools, admin panel, tests, and porting notes |
| `java/` | Aion Lightning source adapted to Java 21, including game data |
| `docker/db/` | MariaDB 11 configuration and first-start schemas |
| `scripts/` | Versioned image build/push tooling and its tests |

The Go stack is the production default. Java 21 is retained for comparison and
quest debugging. Their game databases are separate: `au_server_gs` for Go and
`au_server_gs_java` for Java. Both use `au_server_ls` for login accounts.
Java still has known 2.0 packet leftovers; the old Java 6 stack is the captured
1.9 protocol reference. See [go/PORTING.md](go/PORTING.md) and
[go/QUEST_TRIAGE.md](go/QUEST_TRIAGE.md).

## Checks

On this Mac, Go and Maven run through Apple's `container`. The Go validation
helper pins Go 1.25.1 and uses one CPU after Go 1.25.14 crashed during large
test builds on this host:

```sh
go/scripts/run-go.sh go test -json ./...
go/scripts/run-go.sh go vet ./...
go/scripts/run-go.sh gofmt -l .
container run --rm --arch arm64 -v m2-cache:/root/.m2 -v "$PWD/java:/src" -w /src \
  maven:3.9-eclipse-temurin-21 mvn -B test
python3 -m unittest discover -s go/scripts
python3 -m unittest discover -s scripts
```

Java unit tests now run by default. Empty historical JUnit scaffolds and a
database sample requiring a developer's private `df` account were removed;
the inventory fixture now explicitly sets the capacity it exercises.

## Docker Hub releases

All new images live under `docker.io/rafabertholdo/aiongo`. Tags combine client
version, role, implementation, and server release. The initial release includes:

| Role | Tag |
| --- | --- |
| Database | `1.9-db-mariadb11-v0.1.0` |
| Go login / chat / game | `1.9-login-go-v0.1.0`, `1.9-chat-go-v0.1.0`, `1.9-game-go-v0.1.0` |
| Java 21 login / chat / game | `1.9-login-java21-v0.1.0`, `1.9-chat-java21-v0.1.0`, `1.9-game-java21-v0.1.0` |
| Admin website | `1.9-panel-go-v0.1.0` |
| Packet relay | `1.9-gamesniff-go-v0.1.0` |

Each image also receives a source tag ending in `-sha-<12-character Git SHA>`
and OCI source/revision/version labels. `image-manifest.json` is the source of
truth. Built references and digests are recorded under `releases/`; see
[docs/ROLLOUT.md](docs/ROLLOUT.md) for the initial rollout. Increment its release for subsequent publication; do not overwrite an
existing release tag. There is no `latest` tag, and the legacy Java 6 repositories
and their `:1.9` tags remain untouched. Initial images are Linux arm64 for Apple
Silicon Macs.

```sh
python3 scripts/images.py list
python3 scripts/images.py build          # all nine roles, or append selected roles
container registry login docker.io
python3 scripts/images.py push
# Resume an existing build after later documentation/test commits:
python3 scripts/images.py push --revision <built-source-commit>
```

Commit source before building so its image revision label identifies published
code. Build contexts are staged under `.build` on the external source volume;
the builder alone is reset to avoid stale source. Container runtime storage
stays on the internal disk. Builds never stop the running game stack or remove
database volumes.

## Running and the admin website

```sh
java/docker/aion-servers.sh up          # Go stack, same names/volume as ReRun
AION_IMPLEMENTATION=java21 java/docker/aion-servers.sh up
java/docker/aion-servers.sh logs game
```

Use one stack at a time with this launcher. For side-by-side quest comparison,
use `java/docker/quest-debug.sh`; it preserves separate Go and Java game schemas.

Fresh database volumes initialize both game schemas. MariaDB initialization
does not migrate existing volumes. If an existing volume lacks the Java schema,
use the quest-debug runbook's `sync-java` preparation before Java comparisons.
Never delete `al19-db-data` or `aion-db-data` to update an image.

The Go website is embedded in `go/cmd/panel` and listens on port 8080. Set
`AION_DB` to the database address, then run the panel image with
`-p 127.0.0.1:8080:8080`. It supports account sign-up, GM tools, admin settings,
and character equipment/inventory views. Client artwork is an optional local
build input under ignored `.build/panel-assets`; extract it with
`python3 go/scripts/extract-panel-assets.py <client-folder>`, or override
`AION_PANEL_ASSETS_PATH` when building. No game client or extracted art is
committed to this repository.

## Source provenance

The Go port and admin website were migrated from `the-one/Apps/AionServer`.
The Java tree was imported from the local Java 21 adaptation of
[Sinien/aion-lightning-2](https://github.com/Sinien/aion-lightning-2), including
the configuration, DAO, scripting, and 1.9 packet fixes used in ReRun.
Preserve the upstream GPL notices; the license text is in [LICENSE](LICENSE).
