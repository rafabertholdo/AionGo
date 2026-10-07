# Aion 1.9 dungeon correctness plan

Created 2026-10-05. Task status is maintained in
[go/PORTING.md](../go/PORTING.md#dungeon-correctness-roadmap).
This document defines scope, investigation order and acceptance criteria.
Applied source corrections and executed automated checks are recorded in the
[implementation handoff](aion-dungeon-fixes/README.md). Dungeon gameplay
acceptance and independent 1.9 client verification remain pending.

## What correctness means

A dungeon is verified when ordinary eligible players can enter the correct
shared run, progress through its encounters and gates, wipe and recover,
reconnect according to the 1.9 rules, receive the correct credit and loot,
and leave without leaking state into another run. NPC presence or a matching
Java dialog is insufficient. Classify each finding as a Go regression, a
missing reference behavior, an intentional difference, or an unresolved
1.9 content question before implementing it.

Recovered 4.6 main-server code is useful for identifying state transitions,
validation order, event ownership, timer cancellation and script boundaries.
It is not an authority for 1.9 content, numeric constants or packet bytes.
The NPC server and script libraries have not yet been decompiled. Boss phases,
movement and encounter scripts may require those components; first inventory
the relevant data and exported interfaces, then analyze bounded functions.
Maintain binary hashes and function addresses in evidence notes. Keep local
artifact paths and operational details outside this public repository.

## Initial source observations

- `game/instance.go` creates per-player/group instances, restores registrations
  on login, and destroys runs using a periodic checker. The comment describes
  an empty-minute lifetime, but actual conditions also depend on online
  registrations and group membership. Establish intended semantics before
  changing them.
- `game/staticdoor.go` broadcasts a supplied door ID and invokes Dark Poeta's
  start hook. This handler has no door identity, distance, key or encounter-state
  validation. Determine which checks belong to each 1.9 door type.
- `game/spawns.go:spawnMap` skips handler-tagged spawn groups and suppresses
  respawn for instance NPCs. Inventory the skipped groups and the scripts that
  are meant to activate them; these rules are not proof that every dungeon
  spawn or respawn is correct.
- `game/darkpoeta.go` already implements preparation, scoring, generators,
  Anuhart, rank bosses and an exit workaround. Its comments explicitly cite a
  later reference, accept any door as the start trigger and omit an officer
  score bonus. Generator progression is a counter rather than a set of unique
  objectives. Time is calculated on demand; there is no scheduled run-expiry
  transition here, and gathering increments without a running-state guard.
  These are audit targets, not independently demonstrated 1.9 defects.
- Existing instance, static-door and Dark Poeta tests cover useful happy paths.
  Dark Poeta rank tests cover selected thresholds, but not every exact time
  boundary. No dedicated Dredgion encounter implementation was found in the
  inspected Go file/identifier searches; audit dispatch and data before declaring
  its mechanics absent.

## Investigation and implementation sequence

| ID / priority | Scope and existing entry points | Required acceptance evidence |
| --- | --- | --- |
| D0 / first | Build the 1.9 dungeon inventory from world maps, portals, spawn groups, NPC skills, drops, quests and Java handlers. Separate later-version data and content with no reference implementation. | Per-dungeon manifest lists entrances, requirements, encounters, objectives, gates, rewards and exit/recovery behavior, with a source or explicit unknown for each. Select one normal-player route for each dungeon; inventory handler-tagged spawns and missing assets. |
| D1 / P0 | Shared admission and lifecycle: `instance.go`, `services_portal.go`, `data/portals.go`, `group.go`, login/teleport and task cleanup. Use recovered dynamic-world registration/retention/restriction functions as investigation leads. | Two parties in the same map never share NPCs, doors, scores, credit or loot. Cover joining/leaving/disbanding, leader changes, disconnect during loading, full-party logout/re-entry, expired runs and destruction with casts/tasks pending. Establish 1.9 entry-item, quest, capacity and lockout rules before implementing them; denied entry must not consume items or create orphan runs. GM solo admission must not prove normal-party correctness. |
| D2 / P0 | Door and objective state: `staticdoor.go`, `spawns.go`, quest action items and shared event dispatch. Investigate recovered checked toggle, unchecked script toggle and auto-close separately. | Valid doors resolve within the player's run; invalid or remote requests do not change progression. Cover keys/prerequisites, duplicate clicks, prerequisite death events, late entrants/reconnect state, timed closure and cleanup. A locked gate must not advance the encounter or timer. Validate collision and client state as well as animation. |
| D3 / P0 | Dark Poeta vertical slice: `darkpoeta.go`, its hooks and tests. | Independently establish preparation/start door, expiry, scoring, gathering, unique generator objectives, one-time Anuhart/rank-boss spawning and room/exit access. Test duplicate/reordered deaths, all score/time boundaries, expiry without player actions, wipe/reconnect and isolation between simultaneous runs. Verify score packet opcode/layout and timer units with a 1.9 capture. Leave officer bonuses unresolved until version-specific evidence exists. |
| D4 / P1 | Shared encounter execution: `ai.go`, `ai_scene.go`, `npc.go`, `npcworld.go`, `data/npcskills.go`, `skill.go`, `effectctl.go`. Analyze NPC/script binaries as needed. | A first verified boss covers aggro/target selection, scripted skills, phase/health triggers, adds, leash/reset, wipe recovery and cancellation on death/destruction. Replay boundary/duplicate events and confirm summons/players from other runs cannot participate. Trace pathing/line-of-sight limitations explicitly; a skill list alone does not establish a working encounter. |
| D5 / P1 | Apply the verified shared mechanisms to Nochsana, Fire Temple, then Adma Stronghold and Steel Rake. | For each approved 1.9 manifest: normal-party entry-to-exit route, required objectives/keys, conditional bosses/spawns, wipe/reset, quest credit and loot. Resolve spawn-catalog gaps before claiming content complete; Steel Rake's map declaration does not establish populated encounters. Add encounter helpers only where behavior actually repeats. |
| D6 / P1 | Rewards and persistence: `loot.go`, `group_loot.go`, quest kill/event dispatch, store code and instance admission state. | Correct eligible party members receive credit; disconnected, out-of-range and other-run players follow independently established 1.9 rules. Duplicate kills/completion cannot award twice. Cover full inventory, loot rolls, corpse lifetime, relog and crash/restart policy with disposable database fixtures. Do not persist complete live instances merely because 4.6 has save/load functions. |
| D7 / P2 | Dredgion, after the PvE slice and objective framework. Investigate recovered base Dredgion death, entry, scoring and end hooks; exclude later variants. | Audit current queue/admission first. Establish 1.9 team assignment, schedule, objectives/Surkana, PvP/PvE score changes, captain/timeout endings, reconnect and reward eligibility. Test simultaneous matches and one-time results; capture 1.9 score/end packets. Keep 4.6 ratings and later Dredgion variants outside scope unless independently applicable. |

Recommended execution: D0 → D1 → D2 → D3; use Dark Poeta to prove the
shared lifecycle and objective corrections. Then D4 → D5, with D6 before any
dungeon is declared verified. D7 follows that foundation. No time estimate is
assigned before the manifest and missing-evidence inventory are complete.

Nochsana is the smaller second validation route. Fire Temple checks gated
entry and boss/spawn rules; Adma and Steel Rake broaden objective/key/script
coverage. This ordering is a scope proposal, not a claim that all mechanics
in those dungeons are present in the current reference data.

## Evidence and verification gates

For each correction record: concrete symptom; current Go/Java/data behavior;
4.6 binary identity and function/caller addresses; observed instructions;
inferred meanings; independent 1.9 evidence; expected regression result; and
remaining unknowns. Do not derive test expectations solely from current Go,
Java 21 or 4.6. Historical Java 6 captures establish client behavior but may
also lack retail mechanics; unresolved gameplay requires additional 1.9 evidence.

Use deterministic event/timer tests where needed, existing fixtures and replay
helpers, and two simultaneous runs for shared-state checks. Run focused tests,
then the full Go suite, vet and formatting checks sequentially under repository
instructions. Report exact passed/failed/skipped counts, including subtests.
Use disposable databases for persistence tests. Real-client checks must record
version, ordinary admission, progression, wipe/re-entry, completion and packets;
a bot cannot verify doors, collision or visual score/timer behavior.

Track source implementation, automated checks, client verification and
remaining uncertainty separately. A task is complete only when its applicable
acceptance evidence is attached. Investigation does not require deploying
images or restarting the live server.

## Implementation evidence, 2026-10-05

The [source inventory](AION_19_DUNGEON_INVENTORY.md), its original hashed
snapshot and sanitized client/reference evidence now belong to this repository.
The current source adds run expiry, death deduplication, unique generator
objectives and state-guarded gathering. Portal requests revalidate object
identity, proximity, life, world and instance after their delay. Instance
cleanup immediately stops NPC AI, movement, casts, effects and talk timers.

Dark Poeta barricades 700517/700556/700558 cap each positive damage hit to one;
misses and absorbed hits remain zero. Multi-hit attacks retain separate HP
loss, and skill results, periodic damage, HP and aggro use the same cap.
Item bomb spell 18130 removes nearby 700516 walls only within its seven-metre
area and current run after successful cast completion. Huge Vine requires
skill 300 and rejects a repeated use of its depleted identity.

Scar escort release ordering, ambush continuation, ground-safe movement,
Marabata boss/device protection and reset, authored boss patrols, generator
housing/core phases and terrain contacts, entrance artifact identity, bomb
acquisition and client collision access remain unresolved. The reference
height/health/timer constants and a matching client AI name do not by themselves
approve 1.9 execution. These gaps keep D0/D1/D3 in progress; D2/D4–D7 retain
planned status. No dungeon is marked complete.
