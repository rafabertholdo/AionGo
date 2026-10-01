# Quest debugging: real client against the Java server, then the Go server

Goal: play one quest in the real 1.9 client against a **Java game server**, then
against the **Go game server**, and see exactly which packets differ
(`cmd/pktdiff`). Only the game server exists in Java for this; the login server,
chat server and MariaDB (`al19-db`) stay the Go/al19 ones.

Everything is driven by `docker/quest-debug.sh` (in
`java/docker/`). Read
`.claude/skills/aion-server-expert/SKILL.md` for the stack background.

## Which Java server is the reference

| Image | Built from | Use |
|-------|------------|-----|
| `rafabertholdo/aion-gs:1.9` (**default, `JAVA_GS=old`**) | the real 1.0.1 jars, decompiled once, Java 6, amd64 under Rosetta | The reference the 1.9 client accepts (it sends the right `SM_PLAYER_INFO`). |
| `rafabertholdo/aion-lightning-game:1.9` (`JAVA_GS=source`) | `AL-Game` source tree, Java 21, arm64 | Faster, but the tree has 2.0 packet leftovers ("The invisible character" in the skill). Only proves parity with the source, not with the client. |

Use the old image; use `source` only to see where the source tree differs from it
(`JAVA_GS=source quest-debug.sh up`; the choice is remembered in
`docker/.quest-debug/java-image`).

The old image needs two database-dialect fixes to run on `al19-db` (MariaDB 11); they are in
`docker/old-gs/start.sh`, and change no game code or packets: `useSSL=false` (Java 6 cannot do
MariaDB 11's TLS) and `MySQL5DAOUtils` accepting major version >= 5 (the DAO scripts refuse
anything but exactly 5, and MariaDB reports 11). `al19-db` already has `abyss_rank`.

## How it is wired

Both game servers run and are registered with the login server at the same time. The client's
server list has **two entries**, each reaching its own backend:

```
entry 1 = id 1 = GO    127.0.0.1:7777
   client -> ReRun relay 127.0.0.1:7777 -> al19-game (sniffer) -> al19-game-go
entry 2 = id 2 = JAVA  <ip of al19-game-java-sniff>:7777
   client -> (direct, no relay) al19-game-java-sniff (sniffer) -> al19-game-java
```

- ReRun (This Mac) relays only `127.0.0.1:2106/7777/10241`, so only the Go entry can use a
  loopback address. The Mac can also reach the containers directly on `192.168.64.x`, so the Java
  server registers **its own sniffer's container IP** (`HOST_NAME=<sniffer ip>`, port 7777) as
  its client-facing address and the client dials it directly.
- `au_server_ls.gameservers` has rows 1 and 2 (mask `*`, password `aion`); the Go login server
  needs no code change for this (it lists every row, validates the session keys per game server,
  and `CM_ACCOUNT_AUTH`/`PLAY_OK` work for id 2). It loads the table once at start, so
  `quest-debug.sh` recreates `al19-login` when the loaded count differs from the table, which also
  drops the registrations: the script then recreates both game servers (their `AION_LS` changed).
- IPs change on every (re)create, so the script computes them: sniffer 2 is created first (with a
  placeholder target), the Java server is started with `HOST_NAME=<sniffer 2 ip>` and
  `AION_GSID=2`, then sniffer 2 is retargeted at the Java server's IP through the sniffer's
  control port (`:7778`, `target host:port`, `mark label`, `status`; needs the rebuilt gamesniff
  image, the script recreates an older sniffer). `al19-game` keeps its name and IP unless it has to be
  recreated, and is only retargeted when `al19-game-go` gets a new IP.
- Telling the entries apart in the client: the login server sorts the list by id, so **the first
  entry is Go (id 1) and the second is Java (id 2)**. The 1.9 client's list names come from its own
  files, not from the server, so both may carry the same/no name; use the order, and
  `quest-debug.sh status`. The client highlights the last server played.
- **Separate game databases.** Go uses `au_server_gs`, Java uses `au_server_gs_java` (same MariaDB
  `al19-db`, same login `au_server_ls`), so a quest done on one server is still open on the other. The Java
  schema is a copy of the Go one, made when it is missing; `quest-debug.sh sync-java` re-copies it (stops the
  Java server; run `up` after) and gives both servers the same characters and quest state. Do that after
  creating a test character on Go. To inspect or reset the Java side: `GS_DB=au_server_gs_java
  quest-debug.sh status|snapshot|restore|reset-quest ...` (default is the Go schema).
  Log out of one server before entering the other (same account: auth error 7 otherwise).

## Commands

```
quest-debug.sh up                       idempotent: db, chat, login (2 rows), Go + Java servers, 2 sniffers
quest-debug.sh status [char]            which entry is Go/Java, IPs, registrations, sniffer state, rows
quest-debug.sh mark <label>             a labelled marker in BOTH sniffer logs (msg=mark label=<label>)
quest-debug.sh log java|go [label]      save that sniffer's log (from the last marker <label>) and print the path
quest-debug.sh bot go|java [flags]      aionbot through that entry's real path; dumps the server list
quest-debug.sh snapshot <char> [tag]    save the character's rows (players, inventory, every player_id table)
quest-debug.sh restore  <char> [tag]    put them back in one transaction (character logged out of both)
quest-debug.sh reset-quest <char> <questId> [itemId...]   delete the quest row and those items.
  Quests that START ON LEVEL-UP (campaign chains 1001-1005, 2001-2005, and any handler whose Java has onLvlUpEvent
  adding the quest) must be reset with `RESET_TO=START quest-debug.sh reset-quest ...` (row START/0): with no
  row the giver shows no marker, because nothing offers the quest.
quest-debug.sh prep <char> <questId>    one command to the quest's start state on BOTH schemas that exist (Go au_server_gs, Java
                                        au_server_gs_java): snapshot first (<char>-prep-<id>-go|java), quest row (START/0 when the Java
                                        handler starts on level-up, else deleted), prerequisites (transitive) COMPLETE, exp >= the min
                                        level's, leftover quest items removed, teleported 1.5 m from the start npc (spawn from
                                        spawns/Npcs). Needs only al19-db; says what to run when it is down. Log out first.
quest-debug.sh compare <labelJava> <labelGo> [questId] [pktdiff flags]
                                        `pktdiff -dialogs` on the labelled slices (refreshed from the sniffers when they run, else the newest
                                        saved logs), then the Java handler table. Extra flags go to pktdiff, e.g. -last-session.
quest-debug.sh down                     remove Java server + its sniffer + list entry 2: back to Go only
```

Triage entry point when a quest misbehaves: [QUEST_TRIAGE.md](QUEST_TRIAGE.md).

State lives in `docker/.quest-debug/{logs,snapshots,bin,java-image}`. The script never removes
`al19-db` or its volume, and only touches `al19-game`, `al19-game-go`, `al19-game-java`,
`al19-game-java-sniff` (plus `al19-login` when its `gameservers` rows and loaded rows differ).
`up` prints which entry is which and the instruction to press Play.

## Runbook

**Prerequisites:** Apple `container` running; images `aion-lightning-{db,login,chat,game-go,gamesniff}:1.9`
and `aion-gs:1.9` present (`container image ls`); the client installed and ReRun's Aion tab set
to "Server: This Mac"; `al19-db` up.

1. **Bring up.** `docker/quest-debug.sh up`, then **press Play once in ReRun** (it refreshes its
   relays to the new login/sniffer/chat IPs). Not again unless `up` says it recreated the login
   server or `al19-game`. The client's list shows two entries: 1 = Go, 2 = Java.
2. **Test character.** `quest-debug.sh prep <char> <questId>` puts the character in the quest's start state on both servers
   (snapshotting first). Use one that is not your main one, or snapshot first:
   `quest-debug.sh snapshot Wrathchild base`. Account `admin`/`admin` owns Wrathchild and Helay.
3. **Java run.** `quest-debug.sh sync-java && quest-debug.sh up` (same starting state on both; or `GS_DB=au_server_gs_java quest-debug.sh restore <char> base`), `quest-debug.sh mark java-quest42-start`,
   log in, choose **entry 2 (Java)**, pick the character, do the quest steps once, slowly (talk,
   dialog options, accept, objective, hand in; write the sequence down). Quit to character select and
   log out with the game's Quit (not by killing the client). `quest-debug.sh mark java-quest42-end`,
   `quest-debug.sh log java java-quest42-start` prints the file, `quest-debug.sh status <char>` shows the
   resulting quest row and items.
4. **Nothing to reset** for the Go run: it has its own database. To repeat a run on either side use
   `restore`/`reset-quest` (with `GS_DB=au_server_gs_java` for Java).
5. **Go run.** `quest-debug.sh mark go-quest42-start`, log in, choose **entry 1 (Go)**, do the **same**
   actions in the same order, `mark go-quest42-end`, then `quest-debug.sh log go go-quest42-start`.
6. **Diff the packets** (see "Running pktdiff"):

```sh
go run ./cmd/pktdiff -quest -norm id,time,coord java.log go.log
go run ./cmd/pktdiff -from CM_SHOW_DIALOG -skip SM_MOVE,SM_NPC_INFO,SM_EMOTION,SM_GATHERABLE_INFO,SM_DELETE java.log go.log
```

7. **Compare the code and fix Go** ("Reading the output" below): Java handler/service for the differing
   opcode versus the Go handler; add a golden test in `game/testdata/`; rebuild the Go game server image
   and rerun steps 4-5 for it (after an image rebuild: `container delete --force al19-game-go`, then
   `quest-debug.sh up`, which recreates it, retargets `al19-game` and re-registers id 1).
8. **Tear down.** `quest-debug.sh down` (removes Java + its sniffer + list entry 2), press Play.
   `docker/go-stack.sh` is the older Go-only restart and **deletes `al19-game-java`**; do not mix it with this.

Scripted check without the client: `quest-debug.sh bot java` and `bot go` (log in, list, enter, walk,
quit). Each prints two `server list` lines (`entry=0 id=1 address=127.0.0.1`, `entry=1 id=2
address=<sniffer 2 ip>`) and `playing on id=N`. `bot go` overrides the address with `al19-game`'s IP
(127.0.0.1 is ReRun's relay, unreachable from a container); `bot java` dials the address the list gave.
Space the runs a few seconds apart (same account: error 7 while the last session is still closing).

### Accelerators (read these first)

- `scripts/quest-java-table.py <id>`: the Java handler as an event table (register ids, then per event method rows
  `L<javaLine> guards -> action`, `return true|false`, `break`, `FALLTHROUGH`, template quests from `quest_script_data`,
  quest_data facts). Heuristic parser: verify a row against its `L<n>`. `--json` / `--shell` give the facts to scripts.
- `pktdiff -dialogs [-label L] [-last-session] java.log go.log`: client-action timeline. Steps are clicks
  (`CM_SHOW_DIALOG`), selects (`CM_DIALOG_SELECT`: dialog id, quest id, npc template), movie ends, question responses,
  item uses and NPC deaths that had a quest effect; effects are the server's `SM_DIALOG_WINDOW`, `SM_QUEST_ACCEPTED`,
  `SM_PLAY_MOVIE`, `SM_SYSTEM_MESSAGE`, `SM_NEARBY_QUESTS`, `SM_QUEST_LIST`, item packets, `exp +N` (deltas, so runs
  from different exp compare), narrowed with `-ops`/`-skip`. Object ids never appear (steps are keyed by npc
  template). Verdict per step: `same`, `differs` (other packets or other order), `missing` (Java sent more, or Go never
  did the action), `extra`; then the counts and the first divergence. Exit 1 on any divergence.
- `pktdiff -gen-test <id> [-items a,b] [-o file] <java.log> [label]`: writes `game/quest_<id>_replay_test.go`, the Java
  run's last session replayed against the Go handlers (details in QUEST_TRIAGE.md section 4).

### Running pktdiff

From `Apps/AionServer`, with a scratch script like `run-go.sh` that also mounts the logs
(`-v <dir>:/logs`), or copy the two logs into the module and:

```sh
scripts/run-go.sh go run ./cmd/pktdiff -quest java.log go.log
scripts/run-go.sh go test ./cmd/pktdiff
```

Flags: `-from OP` / `-until OP` (window each log by client opcode, e.g. `CM_SHOW_DIALOG`),
`-ops A,B` (only these server opcodes), `-quest` (adds the quest opcode set: dialog, quest list/accepted,
system message, nearby quests, movie, emotion, move, message, item add/update/delete, exp, level,
target, loot), `-skip A,B` (drop noisy opcodes), `-norm id,time,coord,text|none` (fields to mask;
default `id,time`), `-all` (also print requests without differences). Exit status 1 = differences found.
Input is gamesniff's `msg=server|client op=NAME size=N hex=...` or the testdata form
(`server SM_KEY 7 hex`, optional time prefix), so `game/testdata/*.txt` recordings work too.

How it aligns: each client request starts a *turn* (server packets before the first request form
one too). Turns of the two logs are paired by client opcode (longest common subsequence). Within a
paired turn, the i-th packet of an opcode on one side meets the i-th on the other; extras are
`only Java` / `only Go`, unequal bytes are `differs` with the byte runs (`@offset..end java=.. go=..`),
a field name where the offset is known (`fields.go`), and the UTF-16 text if any. If both sides send
the same packets in another order, `order differs` lists both orders. The masker learns object ids
from spawn-type packets as the log advances and blanks them everywhere afterwards; add opcodes to
`fields.go` when a diff shows bytes worth naming or masking.

### Reading the output, and turning it into a Go fix

- `only Java: SM_X` in the turn of `CM_Y`: the Go handler for `CM_Y` misses a packet. Find the Java
  class: `AL-Game/src/main/java/com/aionemu/gameserver/network/aion/clientpackets/CM_Y.java`, follow it to the
  service (`QuestService`, `DialogService`, `ItemService`, ...) and find who sends `SM_X`
  (`network/aion/serverpackets/SM_X.java` for the layout). The quest scripts are under
  `data/scripts/system/handlers/quest/<zone>/`. Then fix the Go handler (`game/quest_dialog.go`,
  `game/quest_custom.go`, the quest's own `game/quest_<id>.go`, `game/data/quest_scripts.go` for registration).
- `only Go: SM_X`: Go sends something Java does not. Usually a missing guard (status/var check) or an extra
  broadcast.
- `differs SM_X @off`: compare the two hex runs against the Java `writeD/writeC/writeH` order in
  `SM_X.java`; a dialog id or variable that differs means the wrong branch ran (check the quest var in
  `status <char>` after each run). Remember the **old** Java is the truth for the client, and the
  AL-Game source may write 2.0 fields the 1.9 client does not know: when they disagree, follow the
  capture (see the skill's "invisible character" section) and add a golden test in `game/testdata/`.
- `order differs`: the packet order matters to the client only occasionally; check `SM_QUEST_LIST` /
  `SM_DIALOG_WINDOW` first.
- Noise that is not a bug: random bytes (session keys in `SM_CHARACTER_LIST`, chat init key),
  regeneration ticks (`SM_STATUPDATE_HP/MP`), NPC wandering (`SM_MOVE`, `SM_NPC_INFO`), positions and
  HP values when the two runs started from different character state (hence `restore`).

## Worked example: aionbot session, old Java vs Go

Same bot against each backend through the sniffer (`aionbot -login <ls>:2106 -game <sniffer>:7777
-account bot -character Botty -stay 15s -say hello`), logs from `quest-debug.sh log`, then
`pktdiff -norm id,time,coord -skip SM_MOVE,SM_NPC_INFO,SM_EMOTION,SM_GATHERABLE_INFO,SM_DELETE,SM_ATTACK_STATUS`
(39 requests on each side):

```
== CM_ENTER_WORLD (java #4, go #4)
  only Go:   SM_FLY_TIME size=11 3c0000003c000000
  only Go:   SM_MESSAGE size=109 1900000000000000570065006c0063006f006d006500200074006f0020005300...
  only Go:   SM_NEARBY_QUESTS size=7 00000000
  differs:   SM_PLAYER_SPAWN #0 size java=29 go=29
      @25..25 java=00 go=4a
  differs:   SM_SKILL_LIST #0 size java=107 go=107
      @2..3 java=5d05 go=4000
      ...
== CM_LEVEL_READY (java #5, go #5)
  only Java: SM_MESSAGE size=129 1900000000000000570065006c0063006f006d006500200074006f0020005300...
  only Java: SM_MESSAGE size=189 19000000000000005400680069007300200073006f0066007400770061007200...
  only Java: SM_MESSAGE size=111 190000000000000041006e0064002000720065006d0065006d00620065007200...
  only Go:   SM_NEARBY_QUESTS size=11 010000004d040000
-- opcodes that differ: opcode, java count, go count, same-count packets with different bytes
   SM_MESSAGE                   java=5 go=3 bytesDiffer=0
   SM_NEARBY_QUESTS             java=1 go=6 bytesDiffer=0
   SM_SKILL_LIST                java=1 go=1 bytesDiffer=1
   SM_STATS_INFO                java=1 go=1 bytesDiffer=1
   ...
```

What it found:

1. The old Java server sends **three** welcome `SM_MESSAGE` lines (129, 189 and 111 bytes, "Welcome to Siel,
   powered by ...", "This software is under GPL...") **after** `CM_LEVEL_READY`; Go sends one different
   line (109 bytes) during enter-world. A text/timing difference, not a layout one.
2. Go sends `SM_FLY_TIME`, `SM_NEARBY_QUESTS` and a `SM_STATUPDATE_MP` before `CM_LEVEL_READY`; Java sends
   them after it (`SM_FLY_TIME` shows up in a later turn). Go also sends `SM_NEARBY_QUESTS` 6 times versus 1.
3. `SM_SKILL_LIST` has the same length but different bytes at every entry, which looks like a different
   entry order (Java `HashMap` order in the old jar; check before assuming). `SM_STATS_INFO`, `SM_PLAYER_INFO`
   and `SM_PLAYER_SPAWN` differ only in HP/MP and position bytes because the second run started from the
   state the first left: restore a snapshot between runs to remove that noise. `SM_CHARACTER_LIST` (session
   id) and `SM_CHAT_INIT` (key) are random per run.

Every other enter-world packet was byte-identical after masking, and both backends accepted the same login
chain (`al19-login` sees game server id 1 from either).

## Pitfalls

- **Press Play after `up` recreated something.** ReRun's relays (127.0.0.1:2106/7777/10241) resolve
  the containers' IPs when Play is pressed; a recreated login, chat or `al19-game` leaves them
  pointing at dead addresses (symptom: server selection and nothing happens). The Java entry needs no
  relay, but its address is fixed by the sniffer-2 IP at registration: if `al19-game-java-sniff` is
  recreated, `up` recreates the Java server too (about a minute).
- **The sniffer must be named `al19-game`.** ReRun's Play recreates the whole stack when it does not find
  it, which throws away the Java server too. Do not rename it.
- **Container IPs climb** (192.168.64.x, wrapping at .254) and change on every start; the script reads
  them at run time and passes them as env. Do not hard-code them.
- **`container` volumes attach to one container at a time**; Go runs need their own cache volumes and
  fail with "storage device attachment is invalid" when another agent holds them. Leaked `golang`
  containers: `container kill` then `container delete --force`.
- **Docker Hub images are amd64** (`aion-gs`), run under Rosetta: slower boot and `-m 3G`.
  `aion-lightning-*` images are arm64. `JAVA_GS=source` (Java 21 build of AL-Game) also works as id 2.
- **Killing the client** leaves the account "logged in" (auth error 7) until the login server notices
  the disconnect; Quit from the game menu, or recreate `al19-login` (`quest-debug.sh up` after
  `container delete --force al19-login`).
- **Each server writes the character back on logout.** `restore` while a session is live is overwritten
  when it ends.
- **`reset-quest` removes only what you name.** Quest rewards (items, exp, skills, kinah) are separate:
  use `snapshot`/`restore` for a full reset, or pass the reward item ids. `restore` covers `players`,
  `inventory` and every table with a `player_id` column, not `item_stones`, mail, friends or legions.
- **Marker logs.** The sniffers log from container start; `log <backend> <label>` cuts from the last
  `mark <label>` so the two runs are comparable and earlier sessions do not pollute the diff.
- The 1.9 client under Wine needs real user input for movement; synthetic clicks do not move the
  character (see the skill). A scripted scenario with `aionbot` proves the server side only.
