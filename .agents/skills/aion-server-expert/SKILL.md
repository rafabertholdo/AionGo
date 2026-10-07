---
name: aion-server-expert
description: Run, debug, port and play-test Aion 1.9 private servers (Aion Lightning login, chat and game servers, the Go port, Apple `container` images, ReRun's Aion tab, the 1.9 client under Wine). Use for any request about Aion servers, the Aion client on macOS, the Java-to-Go port, packet sniffing or diffing against AL-Game, the al19-* or aion-* containers, or "I can't see my character / can't walk / server is down".
---

# Aion Server Expert

Repository-local guidance adapted from `../the-one/.agents/skills/aion-server-expert`.
Run commands from the AionGo repository root unless a section says otherwise.
Read `AGENTS.md` and the maintained porting notes for the current task; historical
client observations below are diagnostic evidence, not current progress counts.
AionGo owns server code, data, schemas, images and the admin website. ReRun client
integration belongs to `../the-one`; its Git restrictions do not apply here.

## Decompiled 4.6 reference for 1.9 investigations

For requests to improve the Go 1.9 server using the locally analyzed 4.6 server,
read `docs/AION_46_REVERSE_ENGINEERING.md`. Local artifact locations stay in
private operational notes outside this repository. It records binary identity, function
lookup, analysis gaps, and the evidence/validation workflow. Only the main
server executable has been decompiled so far; other components need separate
analysis. Treat 4.6 behavior as a hypothesis until independently validated for
1.9. Preserve the 1.9 protocol and existing quest conformance workflow. Keep
local binary exports and machine-specific analysis notes outside this public
repository. If local evidence is unavailable, ask for the artifact locations rather than
inventing them.

## Where things live

| What | Path |
|------|------|
| Java 21 source tree (imported adaptation of Sinien/aion-lightning-2) | `java/` |
| Java modules | `AL-Login`, `AL-CServer`, `AL-Game` (config and `data/static_data` under it) |
| Standalone Go port and admin website (module `aionlightning`) | `go/`, progress in `go/PORTING.md` |
| Images, run script | `java/docker/` (`aion-servers.sh build\|push\|up\|down\|logs\|sql`) |
| Old morning stack (Docker Hub images `rafabertholdo/aion-{db,ls,cs,gs}:1.9`, amd64 via Rosetta) | `/Volumes/acasis/games/aion/servers/AionLightning1.9/aion-servers.sh` |
| Client (1.9.0.1) | `/Volumes/acasis/games/aion/clients/aion1.9` (`aion1.9-old` is the previous copy; installer at `clients/Aion1.9.0.1_FullInstaller.zip`) |
| ReRun's Aion support | `../the-one/Modules/ReRun/Sources/ReRunKit/{AionGame,AionServer,AionServerRole,AionTunnel,TCPRelay}.swift`, `../the-one/Apps/ReRun/Sources/{AionServerMode,ReRunModel,LibraryView}.swift` |
| ReRun data (Wine, prefix, logs) | `/Volumes/acasis/Games/ReRun` (Aion Wine log: `logs/aion-*.log`; GTA's Wine 10 root: `gta/`) |

## Images and repository checks

Read `image-manifest.json` for the image repository, release, platform and role
variants. Use `python3 scripts/images.py list` for the exact current tags; never
hard-code an old release. Build selected roles with
`python3 scripts/images.py build game-go` (omit roles for all images).
Published builds require committed source, a new release version and the OCI
source revision label. Never publish to the old Java 6 repositories or overwrite
any historical `:1.9` tag. Publishing is a separate task action; copying or
editing a skill does not authorize it.

For Go changes, format the changed files with `go/scripts/run-go.sh gofmt -w`
(paths passed to gofmt are relative to `go/`), then run sequentially:

```sh
go/scripts/run-go.sh go test -json ./...
go/scripts/run-go.sh go vet ./...
```

Report exact passing, failing and skipped test counts from the JSON output.
For Java changes, use Maven tests as documented in `README.md`; for image tooling
changes run `python3 -m unittest discover -s scripts`. Skill-only edits need
skill validation, not server suite execution.

ReRun's This Mac mode has a saved Go / Java 21 selector. Go is the default;
respect the selected implementation. ReRun code is in `../the-one`.
Go uses `au_server_gs`; Java 21 uses `au_server_gs_java`. ReRun initializes a
missing Java schema from the database image's `8-java-*.sql` scripts without
copying Go progress. Registry login may require the user to run
`container registry login docker.io` if no credential is stored.

## Stack, ports and names

- Containers: `al19-db` (MariaDB 11, volume `al19-db-data`, account `admin`/`admin`,
  historical development character Wrathchild), `al19-login` (2106 clients, 9014 game servers),
  `al19-chat` (9021 game server link, 10241 clients), the game server on 7777.
- Databases: `au_server_ls` (login: `account_data`, `gameservers` with id/mask/password)
  and `au_server_gs` (game). `gameservers` row 1 is `*`/`aion`; the Go login server
  reads the table only at start, so add a row then restart it.
- `al19-panel` (Go `cmd/panel`, `-p 127.0.0.1:8080:8080` → http://127.0.0.1:8080, only needs `AION_DB`; admin/admin is access level 3): account sign-up plus GM/admin web tools,
  signed in with a game account (access level 1+ GM, 3+ admin). Its options table `au_server_ls.server_options`
  is read by the Go login and game servers at start and wins over their env vars (`go/options/options.go`).
  Its `/character` page draws a character's equipment and cube with the client's own art.
- Client `.pak` files are ZIPs with scrambled signatures (`AF B4 FC FB` local header) whose entries have their
  first 32 bytes XORed with one of two tables; `.xml` inside are binary XML (first byte 0x80).
  `go/scripts/extract-panel-assets.py` reads both: item names (`L10N/1_enu/data/data.pak`
  `strings/client_strings.xml`), items and icons (`Data/Items/items.pak`), UI atlases (`Textures/ui/ui.pak`),
  skin rectangles (`Data/ui/ui.pak` `ui_preload.xml`) and window layouts (`UI_Game.xml`).
- Server environment (the Java `java/docker/entrypoint.sh` renders config templates;
  Go reads its environment directly): `AION_DB, AION_DB_USER, AION_DB_PASSWORD, AION_LS, AION_LS_PASSWORD,
  AION_CS, AION_GSID, SERVER_NAME, SERVER_CC, HOST_NAME`. `HOST_NAME` is the address
  the client is told to connect to for the game server.
- ReRun "Server: This Mac" runs the same `al19-*` containers itself, uses `HOST_NAME=127.0.0.1`
  and `TCPRelay`s 127.0.0.1:2106/7777/10241 to them. ReRun's Play **recreates the
  whole stack** if it doesn't find `al19-game`, so never leave an extra or renamed
  game container as the only one. Other server modes: Invite (friend's Cloudflare
  tunnels via `cloudflared access tcp`) and Address (type the login server's address).
- The game server registers its client port from `network.client.port` (7777) with the
  login server; two game servers need different ids, ports and a `gameservers` row.
  The client takes the first server on the list; the "last server" byte doesn't
  change that. Server names on the list come from the client, not the server.

## Apple `container` quirks (0.4.x)

- **Storage policy for this Mac:** Apple's container data root must stay at `~/Library/Application Support/com.apple.container` on the internal disk. Do not relocate container storage or image layers to `/Volumes/acasis`; that external volume is for source/data, not container runtime storage. When cleanup is requested, retain images needed for the running stack and active comparison/build workflows; inspect stopped instances and unused duplicate cache volumes before removing server images. Never remove `al19-db-data` or `aion-db-data`; they hold persistent game data. Before pruning, check `container list --all`, the running containers' mounts, and `df -h /`; do not delete a running container or a volume mounted by one. After cleanup, verify required images and any cache volumes used by the current helper remain available.
- The standalone `rafabertholdo/java:6` image is obsolete; the current source-built Java 21 game image is the manifest's `game-java21` role. `aion-gs:1.9` embeds the old Java 6 runtime and remains only as the captured 1.9-client reference. Its packets are not interchangeable with the Java 21 source tree, which still has known 2.0 leftovers. Keep `aion-gs:1.9` and the old `aion-{db,ls,cs}:1.9` images while `capture-1.9.sh` or old-reference comparisons are needed. The manifest's `login-java21` and `chat-java21` images are not required for the current Go-login/Go-chat plus Java-game stack.
- `-p 127.0.0.1:8080:8080` publishing works (verified 2026-10-01 with `al19-panel`); containers are
  also reachable directly at 192.168.64.x. ReRun still relays 127.0.0.1 with its own `TCPRelay`s.
- IPs change on every start; peer IPs baked into config go stale. For an
  authorized stack change, use the existing stack scripts to refresh peer addresses.
  Builds and quest verification must not recreate the live stack.
- The builder has historically reused stale worktree files. Use the isolated
  staging performed by `scripts/images.py`; use `--no-cache` for local quest builds.
  Diagnose a stale builder before changing runtime state. Build contexts belong
  on `/Volumes`, not `/tmp`. Never remove running game containers or mounted
  volumes as part of a build.
- `container delete a b c` fails as a whole if one name is missing: delete one by one.
- A volume attaches to one running container only (don't keep a golang container on
  `go-cache`); MySQL 5.7 volumes need `--ignore-db-dir=lost+found`.
- A killed container never sends FIN, so the login server keeps its old game-server
  connection; the Go login server lets a new registration replace it.
- Building/testing Go: no Go on the Mac. Use
  `go/scripts/run-go.sh go test ./...` (or `go vet` / `go build`).
  It mounts `java/AL-Game/data/static_data` and sets `AION_DATA`, so
  data-backed tests run. Build images with
  `python3 scripts/images.py build`; the source, Java static data, and
  optional panel assets are staged together in an isolated build context.

## Protocol facts (1.9)

- Game server framing: 2-byte little-endian size *including itself*. SM_KEY (0x64) is
  sent in the clear; everything after is rolling-XOR with the static 64-byte key,
  the key advancing by packet size. Server opcodes are encoded `(op+0xAE)^0xEE` then
  `0x50`, `~op`; client headers are op, 0x55, `~op`.
- Login: Blowfish + RSA; the client's checksum covers every word but the last (filler).
  Game server registration is CM_GS_AUTH (id, addresses, port, capacity, password);
  response 0 = ok, 1 = not authed.
- Auth response reasons (login): 1 system error, 6 no game server registered
  (CM_SERVER_LIST with an empty table), 7 already logged in, 8 server down,
  15 full, 16 GM only. An admin left "logged in" after a killed client returns 7.
- Enter world order (byte-identical to Java in the Go port): skill list, quest list,
  recipes, enter-world check, UI settings, inventory chunks, stats, cube, bind point,
  player id, macros, game time, titles, channel info, player spawn, emotions,
  influence, siege locations (54 entries), prices, abyss rank, welcome text, MP,
  nearby quests, fly time. CM_LEVEL_READY then triggers SM_PLAYER_INFO, state,
  weather, HP/MP (the 1.9 client ignores current HP before level load), abnormal
  state, quests. The empty siege list was the last real structural difference found.
- New object ids must start at `firstObjectID = 0x10000`: a low id (1) makes the client
  show an unnamed object where the player stands.
- Java `HashMap` iteration order matters for skill/recipe/macro lists (the port
  emulates it) and `Math.round` semantics for stats.
- The game server needs `au_server_gs.abyss_rank` migrated (`java/AL-Game/sql/Update/Rev 239 - abyss_update.sql`)
  on the old Docker Hub database, or enter-world fails and the client crashes in game.dll.
- Never drop in JD-GUI-decompiled classes from `AionLightning1.9/docker-gs/sources`:
  they mislabel enum switches and merge variables. Port by hand and check bytecode.
  Remaining 1.0.1 differences: `java/docker/1.0.1-differences.txt`.

## Java server policy

**Go remains the production default.** ReRun users can explicitly choose Java 21
under This Mac → Server software, or use Java for quest comparisons. Respect the
user's selected implementation; do not switch it back to Go automatically.
Java 21 remains a reference implementation with known 2.0 packet leftovers.

**CRITICAL: Go and Java MUST use different game databases.** Go uses `au_server_gs`, Java uses `au_server_gs_java` (`AION_GS_DB`). They cannot share the same database or quests completed on one server will be marked complete on the other, breaking quest debugging (the whole point of running both). The Go server leaves `AION_GS_DB` unset, so it uses the default `au_server_gs`. The Java server must have `AION_GS_DB=au_server_gs_java`.

For an authorized live quest comparison, use `java/docker/quest-debug.sh up` to run both servers together. To return to Go-only mode when requested, use `java/docker/quest-debug.sh down` and then `java/docker/go-stack.sh` (no argument; pass `login` to also recreate the login server). `go-stack.sh` kills the running Aion client, deletes `al19-game`, `al19-game-go` and `al19-game-java`, and starts `al19-game-go` behind the `al19-game` sniffer. To check which schema a game server really uses, query `information_schema.processlist` on `al19-db` (root password `aion`) for the server's container IP.

## Comparing the Go game server with Java

- `go/cmd/gamesniff` is a decrypting relay: run it as `al19-game` (`-target <ip>:7777`)
  in front of the real server (`al19-game-java` or `al19-game-go`); it logs every
  packet as `op=NAME size= hex=`. `AION_DEBUG=1` does the same inside the Go server.
- `go/game/testdata/java-enter-world.txt` is a captured Java session; `go/game/world_test.go`
  compares Go packets with it byte for byte (`TestEnterWorldPacketsMatchJava`).
- `go/cmd/aionbot` is a scripted player (login, enter world, move, say) for
  server-side tests without the real client: `aionbot -login <ls>:2106 -game <gs>:7777 -account bot -character Botty`.
  It proves the server chain works; it cannot show client-side problems.
- Stack swap scripts from the session: stop the client, delete containers one by one,
  start login (`AION_DB`), game (`AION_LS`, `AION_CS`, `HOST_NAME`), then the sniffer
  named `al19-game`, and let the user press Play so ReRun refreshes its relays.
- Read `go/PORTING.md` for current subsystem coverage and outstanding
  real-client verification; do not infer completion from an old session summary.

- **Java vs Go with the real client:** `java/docker/quest-debug.sh up|status|mark|log|bot|sync-java|snapshot|restore|reset-quest|down`
  runs BOTH game servers at once: list entry 1 = Go (id 1, `127.0.0.1:7777` via ReRun's relay and the `al19-game`
  sniffer), entry 2 = Java (id 2, the old `aion-gs:1.9` patched only for MariaDB 11 in `java/docker/old-gs/`, registered at
  its own sniffer `al19-game-java-sniff`'s container IP, dialed directly). `go/cmd/pktdiff` diffs the two
  sniffer logs per client request. **The two servers must use different game databases**: Go uses `au_server_gs`,
  Java uses `au_server_gs_java` (`AION_GS_DB`), otherwise a quest done on one is already done on the other and
  cannot be repeated. Both servers may share MariaDB (`al19-db`) and the login/chat servers (`au_server_ls`), just
  not the `au_server_gs` schema. `sync-java` re-copies the Go schema into the Java one (do it after creating a character
  on Go); `GS_DB=au_server_gs_java java/docker/quest-debug.sh status|snapshot|restore|reset-quest` acts on the Java copy.
  Runbook and worked example: `go/QUEST_DEBUG.md`.
- **A quest misbehaves in the client:** start at `go/QUEST_TRIAGE.md` (decision tree by symptom, top
  suspects with file/function names, how to capture a Java run from a Go-only stack). Mechanical steps:
  `python3 go/scripts/quest-java-table.py <id>` (Java handler as an event table with `L<line>`), `java/docker/quest-debug.sh prep <char> <id>`
  (same start state on Go and Java: snapshot, quest row, prerequisites, level, items, teleport to the giver),
  `java/docker/quest-debug.sh compare <labelJava> <labelGo> <id>` (`pktdiff -dialogs`: one verdict per client action, first
  divergence, plus the Java table), and `pktdiff -gen-test <id> <java.log> <label>` (a recorded Java run becomes
  `go/game/quest_<id>_replay_test.go`).

## The client under Wine (ReRun)

- Client folder: `bin32/aion.bin` (WinLicense protected, CryEngine, Wine's own D3D9,
  `WINEDLLOVERRIDES ;d3d9=b`). Launched from the client folder as
  `-ip:<server> -port:2106 -cc:1 -lang:enu -noweb -noauthgg`.
- ReRun only edits the folder's `system.cfg` (lines are byte-inverted, CRLF; sets
  `r_Width/r_Height/r_PseudoFullscreen` to the display's size on Play) and, in the Wine
  prefix, `UseConfinementCursorClipping=y` for aion.bin. Game files matched the
  installer's byte for byte, so the client was never corrupted.
- **Aion needs Wine 11** (ReRun's default engine): on Wine 10.0 the WinLicense
  protection raises `0x80000003` (breakpoint) and the client dies. GTA/GTA2 use Wine 10.0.
- Installer: `Aion1.9.0.1_FullInstaller.zip` holds an InstallShield package
  (`data1.cab`, `data1.hdr`, `data2.cab`). `unzip -j` those three, then
  `unshield -d out x data1.cab`; the game is in `out/DefaultComponent`. The old copy had
  one extra file, `bin32/d3dx9_38.dll` (2010, 40 KB), and launcher `.bat`s.
- Synthetic input: hover before clicking (`m:x-5,y-3 m:x,y dd du`); check the frontmost
  app before every key or click. UI keys work, but in the world synthetic W, clicks and
  mouse wheel never move the character or camera, so movement needs the user's real input.
  Client logs: `log.txt`, `Chat.log` in the client folder; Wine logs in `ReRun/logs`.
- Screenshot the client with `screencapture -x -l <windowid>` (window id via a
  CGWindowList script); read the image to see what is really on screen.
- The Esc menu (Quit/Logout/Options/Cancel) sends CM_MAY_QUIT and blocks movement.

## Debugging habits

1. Sniff both directions. "Nothing happened" is not a server bug until the opcode log
   shows the packet is absent; no `CM_MOVE` from the client means the client sent none.
2. Check which server the client really hit (login log "connection from", sniffer log).
3. Change one variable at a time (server, client folder, Wine engine, connection path).
4. `container logs al19-login | tail` shows Java login lines like `recived packet: CM_PLAY`;
   Java doesn't log the SM_PLAY_OK reply.
5. Use `rg` for text searches. If an output wrapper mangles packet logs or diffs,
   use the underlying tool directly.

## The invisible character (solved 2026-09-28)

The Aion Lightning source tree in `aion-lightning-1.9-src` is not purely 1.9: it comes from a
tree with 2.0 packet changes. `SM_PLAYER_INFO` wrote an extra `writeC(0x00) // new 2.0 Packet`
before the race byte. The 1.9 client then misread the whole packet, so the character was
invisible and could not move (no `CM_MOVE` was ever sent), while HUD, radar and NPCs worked.
The old Docker Hub image (`AionLightning1.9`, decompiled from the 1.0.1 jars) sent the right
layout; a capture of it made the difference obvious. The Go port had copied the extra byte
(byte-identical to the wrong Java), so it was fixed there too, with
`TestPlayerInfoMatchesClient19` and `go/game/testdata/player-info-1.9.txt`.

**Lesson:** "byte-identical to AL-Game" only proves parity with the source tree, not with the
1.9 client. Ground truth is a capture of the morning stack (old amd64 images), taken with a
sniffer: start `aion-sniff` (gamesniff) first, predict the next container IPs (they climb
monotonically; sniffer = next, game server = next+1), then start `aion-gs` with
`HOST_NAME=<sniffer ip>` and the client in ReRun's Address mode at `aion-ls`. Compare
op by op with `go/game/testdata/java-enter-world.txt`. Other 2.0-isms may remain in
AL-Game's other packets; every packet with no capture from the morning stack is unverified.

## Reference recordings and helper scripts

- `go/game/testdata/play-session-1.9.txt` is a real play session on the morning (old amd64) stack:
  login, world entry, walking, sitting, emotes, targeting, auto-attacks, spells, mob AI attacking
  back, kills, exp, level up, loot, item moves, quest dialogs (`time C|S OP size hex`). The old
  server is the only truth for the 1.9 client; AL-Game's source has 2.0-isms. Golden files from it:
  `player-info-1.9.txt`, `npc-info-1.9.txt`, `gatherable-info-1.9.txt`.
- `java/docker/capture-1.9.sh` starts that old stack with `aion-sniff` in front (the container addresses
  climb monotonically and wrap at .254, so it probes for the next ones and retries); the user plays
  in ReRun's Address mode and `container logs aion-sniff` is the recording.
- `java/docker/go-stack.sh` restarts the Go game server behind the sniffer;
  `go/scripts/run-go.sh` runs Go commands from the tracked module.

## Quest porting and current status

Read `go/PORTING.md` for authoritative subsystem and quest status, and
`go/QUEST_AUDIT.md` for outstanding conformance findings. Do not
copy dated handler counts into this skill. The claim inventory scans custom
registrations and local claim markers; a free ID may already use an XML or
prologue handler. Check the roadmap before treating it as unported:

```sh
python3 go/scripts/quest-claim.py status
python3 go/scripts/quest-claim.py next <agent-name> <count>
```

Before implementing or auditing a handler, read `go/QUEST_HANDLER_AGENT.md`,
including its Java conformance checklist. Some older examples in those docs
still use `Apps/AionServer` or `docker/` paths: here the module is `go/`, Java
handlers/data are under `java/AL-Game/`, and stack scripts are in `java/docker/`.
Claim each ID atomically before editing it; port every Java event and branch,
including false returns and switch fall-through. Reuse the shared dialog,
start and reward helpers; register a handler only when all its events work.

Use bounded integration batches. Run each handler's focused tests as it is
implemented, integrate shared registrations and dispatch, format the batch,
then run combined focused/conformance checks and the full Go suite and vet
once. Run all Go commands sequentially because the runtime can attach shared
cache volumes to only one container at a time. Build one local game image for
the completed batch. If code changes after full checks, rerun the suite and
vet and rebuild before handoff. Mark claims done after integration and tests.

If the image tool rejects a dirty shared worktree, preserve that worktree and
build locally using the `game-go` manifest role to derive the tag, with a
SHA-256 of the worktree source inputs in its local tag and revision label:

```sh
container build --no-cache --platform linux/arm64 --progress plain --cpus 2 \
  --memory 4G -f go/Dockerfile --target game -t <manifest-derived-local-tag> \
  --label org.opencontainers.image.revision=<worktree-content-hash> .
```

Never publish this local verification image or restart the server stack for
quest verification. Use `go/QUEST_TRIAGE.md` for client symptoms and
`go/QUEST_DEBUG.md` for an authorized live comparison. Go and Java must use
separate game schemas even when they share login/chat and MariaDB.
