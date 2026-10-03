# Remaining Java-to-Go port: draft inventory and plan

Audit date: 2026-10-03 (quests and the missing-feature list below rechecked
against the source; the rest from 2026-10-02). This is a planning baseline, not
an exhaustive method-by-method parity certification.

Implementation started with the critical Go packet-boundary batch. Track its
fixes, verification and remaining review scope in
[GO_REVIEW_LEDGER.md](GO_REVIEW_LEDGER.md). Quest work remains a parallel stream.

## Scope and completion criteria

Target the implemented behavior of this repository's Java servers that is
relevant to Aion 1.9. Use old 1.9 server captures and the real client to resolve
protocol differences: Java 21 source contains known 2.0 leftovers.

Track each feature as missing, partial, implemented but unverified, verified,
intentionally different, or unavailable in Java. A packet constant, handler
registration, or Java class name alone is not evidence of implemented behavior.
Do not derive a global completion percentage from file or class counts.

For each backlog item record Java source/methods, Go implementation, missing
branches and dependencies, persistence requirements, focused checks, and client
verification. Close it only when these are accounted for.

## Current evidence

Login and chat are reported complete in `go/PORTING.md`; they still need to be
included in the final source audit. Most basic game systems already have Go
implementations. Quests and incomplete edges of those systems remain.

### Quests

Every Java quest handler is ported. `TestQuestJavaParity` runs the Java
handlers in process (`QuestTraceDump`) over each quest's reachable states, npcs
and a dialog sweep and replays the same probes on Go (`go/scripts/quest-parity.sh`):
1,922 of the 1,923 handlers match on every probe, and the prologues 1000/2000 run
on `startPrologue`. Deliberate differences are listed in `parityDeviation`
(monster hunts need their kills; 1146 and same-npc report_to quests stay
completable). 109 handlers were translated with `go/scripts/quest-java-port.py
--events` (`game/quest_java_dialogs.go`, dispatched by `game/quest_java_events.go`).

Still open for quests:

- Non-dialog events (kills, attacks, item use, zones, world entry, deaths, movie
  ends, timers, spawns, teleports) are not covered by the parity probes. Extend
  `QuestTraceDump` to those events, and confirm the translated handlers in the
  client.
- `go/QUEST_AUDIT.md` rows still marked open need reclassifying against the
  parity result.
- Cross-system items from `go/QUEST_PORTING.md` (party kill credit, generic
  work-item cleanup on abandonment) still need checking against current code.

### Missing Java features (verified 2026-10-03)

Each row is implemented in AL-Game 1.9. Go gaps and completed fixes are tracked below.

| Area | Java source | Go state | Notes |
| --- | --- | --- | --- |
| **LFG / find group (reported broken in client testing)** | `CM_PLAYER_STATUS_INFO` status 9 calls `Player.setLookingForGroup(playerObjId == 2)`; `CM_PLAYER_SEARCH` filters `lfgOnly` on `isLookingForGroup()`; `SM_PLAYER_SEARCH` writes status 2 for LFG players | `game/group.go:playerStatusInfo` handles status 9 with a session-only flag; `game/misc.go:playerSearch` filters on that flag and writes result status 2 | Implemented 2026-10-03. Packet tests cover toggle on/off, normal and LFG-only searches, offline exclusion and truncated toggles. Real-client confirmation remains pending |
| Godstones | `CM_GODSTONE_SOCKET`, `ItemStoneListDAO` godstone rows, godstone procs | Socketing implemented in `game/godstone.go`: NPC range, unequipped weapon, godstone metadata, service fee and atomic persistence; inventory and appearance packets carry the socket | Combat procs remain missing. Client/capture confirmation remains pending |
| Group loot roll/bid | `CM_GROUP_LOOT` -> `DropService.handleRoll/handleBid` | Mode 2 rolls implemented in `game/group_loot.go`: quality settings, eligible-member prompts, roll/pass messages, tie handling and atomic winner grants | Mode 3 bids remain unported. Roll capture/client confirmation remains pending |
| Map channels | `CM_CHANGE_CHANNEL` -> `TeleportService.changeChannel` | No handler | Needs channel isolation of npcs/players |
| Custom settings | `CM_CUSTOM_SETTINGS` (display/deny flags, `SM_CUSTOM_SETTINGS` broadcast) | No handler; deny/display flags are loaded and used but not saved | Save kinds 2/3 with the logout save |
| Saved effects | `PlayerEffectsDAO` | Active effects are not persisted | Lost on relog; check expiry and cooldowns |
| Skill effects | `search` (10 XML uses), `returnpoint` (1), `mpuseovertime` (1), `onetimeboostskillattack` (6), `magiccounteratk` (6), `petorderuseultraskill` (25) | No Go handler | `skilllauncher` (27) is a stub in Java too: new functionality, not a port |
| Legion emblems | `CM_LEGION_UPLOAD_EMBLEM`, `CM_LEGION_SEND_EMBLEM` | Upload requests ignored (`game/legion.go`) | |
| Logout delay | `AionConnection.onDisconnect` delays logout 15 s outside an orderly shutdown | Immediate logout | Lifecycle/combat difference |

Not missing (Java has no implementation either): siege battles and timers
(Java keeps ownership and influence only, as Go does), the legion warehouse
(disabled in Java's config), pets (AL-Game 1.9 has none; the toy pet is the
kisk). Handler-managed spawn groups are RIFT (ported in `services_rift.go`) and
STATIC (static objects send the client nothing).

### Confirmed request/dispatch gaps

The initial inventory found the following request gaps; all rows below are now
implemented and await client verification. The requests still missing are in
the table above.

| Java request | Missing behavior |
| --- | --- |
| `CM_SHOW_BRAND` | Implemented in `go/game/brand.go`: Java-compatible broadcasts to group/alliance members, including clearing. Packet, recipient and malformed-input tests added; capture/client verification remains open. |
| `CM_ALLIANCE_GROUP_CHANGE` | Implemented in `go/game/alliance_group.go`: captain/vice-captain moves and swaps, stable subgroup assignments, member-info/ID broadcasts. Automated authority, capacity, ordering and malformed-input checks added; capture/client verification remains open. |
| `CM_OPEN_STATICDOOR` | Implemented in `go/game/staticdoor.go` as Java's door-emotion broadcast. Automated packet and malformed-input tests pass; client verification remains open. Broader door mechanics need separate scope. |
| `CM_CLIENT_COMMAND_ROLL` | Implemented in `go/game/client_command.go`; Java-compatible player and nearby-player messages. Automated tests cover bounds and truncated input; capture/client verification remains open. |
| `CM_OBJECT_SEARCH` | Implemented in `go/game/object_search.go`: first spawn in file order, Java map-marker packet, no reply for missing NPCs. Packet, lookup order, deletion/reload and truncated-input tests added; capture/client verification remains open. |
| `CM_REPORT_PLAYER` | Implemented in `go/game/client_requests.go`: structured player-report audit, no reply or target lookup, complete parsing before logging. Java flags opcode `0x32` as uncertain; capture/client verification remains open. |
| `CM_CLIENT_COMMAND_LOC` | Implemented in `go/game/client_command.go`: private current-location system message with Java-style float strings. Literal packet and coordinate formatting tests added; capture/client verification remains open. |
| `CM_DISCONNECT` | Implemented in `go/game/client_requests.go`: zero closes the socket without a final packet; the existing read-loop cleanup owns logout. Nonzero and truncated requests do nothing. Java flags opcode `0xED` as uncertain; capture/client and end-to-end persistence verification remain open. |

`go/game/group_loot.go` handles mode-2 `CM_GROUP_LOOT` replies and protects
active rolls and uncollected winner reservations. Group settings now retain
autodistribution and the seven quality rules. Java uses the per-quality rule
to choose a roll; the global autodistribution field is retained for the UI.
Mode-3 bid handling remains unported.

`go/game/legion.go` registers upload emblem/info requests as empty handlers and
explicitly documents that uploaded emblem images are unported. Other legion
features already exist.

`CM_SHOW_MAP` and `CM_QUESTIONNAIRE` are now registered in
`go/game/client_requests.go` with Java's inert behavior. The questionnaire reads
one D and four H fields but performs no action; the map-open request is empty.
They do not represent missing gameplay systems.

`CM_CUSTOM_SETTINGS` remains unported: Java reads display/deny H fields and
broadcasts `SM_CUSTOM_SETTINGS`. Go already loads setting kinds 2/3 and uses
deny flags in social requests and display flags in player info, but
`game/world.go:saveSettings` saves only UI/shortcuts (kinds 0/1). Implement the
request together with logout saves and disposable-database reconnect checks.
Disconnect retains Go's existing immediate logout cleanup. Java's
`AionConnection.onDisconnect` delays logout by 15 seconds outside an orderly
shutdown; that combat/logout lifecycle difference remains a separate audit item.
Some empty Go handlers match trivial Java behavior: for example Java's
`CM_GROUP_RESPONSE` only logs values. Empty handlers need classification,
not automatic implementation.

### Skills and persistence

Comparing Java's `skillengine/effect/Effects.java` XML names against production
Go source identifies the following missing-name candidates used in skill data:

| XML effect | Occurrences in skills XML |
| --- | ---: |
| `search` | 10 |
| `returnpoint` | 1 |
| `mpuseovertime` | 1 |
| `onetimeboostskillattack` | 6 |
| `magiccounteratk` | 6 |
| `petorderuseultraskill` | 25 |
| `skilllauncher` | 27 |

These counts describe XML nodes, not unique skills or guaranteed reachable
gameplay. Confirm the registration and lifecycle requirements for each effect.
Search, return and returnpoint have real Java behavior. SkillLauncherEffect's
Java apply method is a TODO stub, so a working launcher is new functionality,
not a port of implemented Java behavior. `buf` and `backdash` also lack Go
literals but had no occurrences in the scanned skills XML.

Saved active effects need a persistence audit: Java has PlayerEffectsDAO;
no equivalent player-effects persistence was found in this initial Go scan.
Check expiry, cooldowns, passive effects, summons, death and reconnect behavior
as lifecycle concerns, not merely packet implementations.

### World, items and social systems requiring deeper audit

- Map channels and their NPC/player isolation, rifts and instance interactions.
- Handler-managed spawn groups: `spawnMap` skips groups with a nonempty Handler.
  Map each Java handler path to the intended Go spawning behavior.
- Static objects, action-item controllers, crafting station checks, and
  instance-specific doors, keys and boss behavior. Check which behaviors Java
  actually implements before committing to new dungeon scripting.
- Godstone combat triggers and persistence; enchantment supplements; Abyss
  shop conditions, currency, prices and rank restrictions.
- Legion contribution/reward edges and warehouse behavior/configuration.
- Alliance loot/readiness behavior and group reward edges beyond the confirmed
  request gaps above.
- Java DAO/configuration coverage, reconnect/restart persistence, and remaining
  administrative command branches. Go's current command catalog already lists
  the Java admin command set, so the older "rest of the commands" note is not
  evidence that those commands are wholly missing.

Enchantment, stigmas, summons, flight teleporters, walking NPCs, respawns,
instances, mail and broker implementations already exist. Audit their missing
edges; do not schedule their complete reimplementation from stale "Left" lists.
Manastone supplements also have Go logic; enchantment-stone supplements require
separate comparison.

### Implemented but awaiting 1.9 verification

The maintained port notes call for captures/client checks of groups, social
requests, trading, private stores, mail, broker, warehouses, teleports,
instances, flight, duels, summons, gathering, crafting, enchanting, stigmas,
dye and legion flows. These are validation backlog items, not wholly unported
systems. Check available captures before recording duplicates.

### Java limitations and optional new functionality

Keep a separate expansion backlog for siege battles/timers and fortress spawn
conversion (Java SiegeService provides ownership rather than battles), working
skill launchers, disabled legion warehouses/express mail, and inactive day/night
event scheduling. Validate disabled paths before deciding parity scope.
AL-Game 1.9 has no general pet system; the toy-pet action is the existing kisk.

## Critical Go work before broader porting

The Go port predates installation of the Go skills. Existing implementations
therefore need a systematic review as well as a missing-feature inventory.
Use [GO_SKILLS_APPLICATION_PLAN.md](GO_SKILLS_APPLICATION_PLAN.md) for the
concrete reliability batches and project-specific interpretation of the skills.
Its sampled findings are review targets, not proof of defects in every caller.

Start with a bounded critical pass:

1. Establish reproducible test/vet/build results and correctness-oriented lint,
   vulnerability and focused race checks, with explicit fixture availability.
2. Check packet parsing before mutation, frame length boundaries, invalid
   coordinates, encrypted write ordering and failed/slow connections.
3. Check data integrity: error propagation, transaction boundaries and rollback
   for items, kinah, rewards, trade, mail and broker operations. Define when
   in-memory changes and client success responses may occur.
4. Check ownership and lifecycle of shared state, goroutines, timers, reconnect
   loops and shutdown saves; fix confirmed race, deadlock and leak risks.
5. Check panel authentication, authorization, mutation protection, request
   limits and trusted proxy handling.

Before broadening the non-quest port, resolve confirmed critical defects or
record a bounded follow-up with evidence explaining why it does not block the
next work. Installing skills alone does not validate existing code.

Review the entire Go codebase afterward, package by package. Record reviewed
files/functions, applicable skills, findings, severity, fixes and verification
in a coverage ledger. Include login, chat, game, stores, wire, data loaders,
options, panel, commands and tooling; do not limit the review to new Go code or
to Java features that have not yet been ported.

Prioritize correctness, data integrity and security, then maintainability and
measured performance. Apply relevant guidance with the repository's constraints;
do not adopt every library suggestion or mechanically rewrite working code for
style. Preserve Java behavior and 1.9 packet goldens while fixing Go defects.

## Parallel work and proposed sequence

Quest porting and quest correctness are a continuing workstream alongside the
reliability review and other feature work. The sequence below orders the
non-quest dependencies; it does not require all quests to finish before skills,
items, social or world work starts.

Coordinate changes to shared quest helpers, dispatch, registration, stores and
reward persistence. A handler whose required event/system is missing waits for
that dependency before registration. Keep review and porting batches small
enough to integrate without broad simultaneous edits to shared files. Parallel
development still uses sequential Go commands on this host.

0. **LFG tool implemented**: status 9, the LFG-only filter and result status
   now match the Java reference. Confirm the completed fix in the 1.9 client.
1. **Finish the inventory.** Map Java packets, services, controllers, AI,
   quest events, skill effects, item actions, data loaders, DAOs, configuration,
   and admin command branches to Go. Inspect no-op registrations and reconcile
   stale port notes. Produce a source-linked checklist with explicit status.
2. **Secure progression, in parallel.** Every quest handler is ported and matches
   Java's dialogs; extend the parity harness to non-dialog events, confirm the
   translated handlers in the client, and resolve party credit, item cleanup and
   reward persistence dependencies.
3. **Complete skill/item behavior.** Port the live missing effects, godstone
   socketing/procs, relevant supplements and saved effects; check class coverage
   and restart/reconnect behavior.
4. **Complete cooperative play.** Implement loot roll/bid, brands, alliance
   subgroup changes and other source-backed alliance gaps; finish emblem uploads.
5. **Complete world dependencies.** Add channels, handler-managed spawns and
   implemented Java static/action-object behavior; integrate dependent quests.
6. **Smaller requests and configuration.** Complete source-backed smaller
   requests and DAO/configuration gaps alongside the systems they affect.
7. **Close verification.** Compare representative flows against old 1.9
   captures, verify client behavior and MariaDB persistence, and re-audit the
   checklist. Decide optional expansion work separately.

Client verification should also happen during each relevant phase, so protocol
mistakes are found before dependent work accumulates. No duration estimate or
overall completion percentage is justified until phase 1 is complete.
The full Go review continues alongside these phases after the critical pass.
Close both ledgers: Java behavior coverage and Go code review coverage.

## Verification cadence

Follow `go/QUEST_HANDLER_AGENT.md`: claim a bounded quest batch, implement each
handler with focused checks, integrate shared dispatch/registrations, then
format and run the full suite and vet sequentially. For Go changes use
`go/scripts/run-go.sh go test -json ./...`,
`go/scripts/run-go.sh go vet ./...`, and gofmt; report exact pass/fail/skip counts.
Build one local game image per completed quest batch. Rerun full checks if code
changes afterward. Preserve the worktree and use the documented hash-labelled
local build if the clean-tree guard rejects shared edits.

Do not deploy, publish or restart the server stack for port verification. Keep
Go and Java game schemas separate and preserve database volumes. This draft
was prepared through source inspection and read-only inventory commands; no
server tests were run and no runtime state was changed.
