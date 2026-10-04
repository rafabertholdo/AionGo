# Java-to-Go port status

Updated 2026-10-04 by comparing the current Go source, Java handlers and XML
catalog. This is the single authoritative port status and remaining-work
roadmap, including quests. It replaces `docs/JAVA_GO_PORT_PLAN.md` and
`go/QUEST_PORTING.md`.

The target is the implemented Aion 1.9 behavior of the Java login, chat and
game servers, using the same static data and compatible database schemas.
Java 21 source contains known 2.0 packet leftovers; old 1.9 recordings and the
real client resolve protocol disagreements. Source coverage, automated parity,
client verification and deployment are separate facts. No overall completion
percentage or current running-image claim follows from this inventory.

## Implemented systems

| Area | Go implementation and coverage |
| --- | --- |
| Login | `login/`, `cmd/loginserver`: Blowfish/RSA, server list, play/reconnect, game registration, account authentication, kicks, access levels, bans and account time. Restarted game servers can replace stale registrations. |
| Chat | `chat/`, `cmd/chatserver`: Java chat behavior; messages are restricted to their own channel rather than all channels of the same kind. |
| Characters and world entry | Character creation/select/delete/restore, stat calculation, inventory, skills, quests, settings, titles, macros, bind point, weather and saving. Packet goldens include world entry and the corrected 1.9 player-info layout. |
| World and combat | Visibility, map channels, movement, walking/wandering NPCs, aggro/chase/home AI, attacks, NPC skills, death/respawn, experience/levels, regeneration, resurrection, PvP and duels. |
| Items and economy | Cube, equipment, drops, shops, warehouses/account warehouse, gathering/crafting, enchanting, manastones, godstones, stigmas, dye, remodelling, arms fusion, trade, private stores, mail and broker. |
| Skills | Casting, targets, cooldowns, interrupts, damage/healing, buffs/debuffs, control effects, shields, dispels, signets, auras, traps, servants and spirit-master summons. Saved effects and skill/item cooldowns persist across relogs. |
| Social | Friends, blocks, whispers, player search/LFG, six-player groups and loot roll/bid, 24-player alliances, subgroup moves, target brands, legions, emblem send/modify, history and kisks. |
| World services | NPC/flight teleporters, instance isolation, rifts, siege ownership/influence, zones, drowning/death levels, weather/game time, bind stones, postboxes, announcements, HTML welcome, optional simple class change, petitions, punishments and admin commands. |

These systems have implementations and automated checks of varying scope.
Their presence does not certify every Java branch or every client flow.
Login and chat also remain part of the final source review.

Recent completed features previously listed as missing include LFG, godstones,
group loot roll/bid, map channels, custom settings, saved effects/cooldowns,
`search`, `returnpoint`, `mpuseovertime`, `onetimeboostskillattack`,
`magiccounteratk`, `petorderuseultraskill`, and the 15-second disconnect delay.
Request implementations also include `CM_SHOW_BRAND`, `CM_ALLIANCE_GROUP_CHANGE`,
`CM_OPEN_STATICDOOR`, `CM_CLIENT_COMMAND_ROLL`, `CM_OBJECT_SEARCH`,
`CM_REPORT_PLAYER`, `CM_CLIENT_COMMAND_LOC` and `CM_DISCONNECT`.

## Quests: all source quest IDs are covered

There is no remaining backlog of unported Java quest IDs. The current source
inventory contains **1,923 distinct quest IDs**, covered as follows:

| Source/implementation | Count | Evidence |
| --- | ---: | --- |
| XML declarations | 1,596 | `java/AL-Game/data/static_data/quest_script_data/*.xml`, loaded by `game/data/quest_scripts.go`: 170 report-to, 316 monster hunts, 613 item collections, 492 work orders and five specialized XML quests. |
| Custom Go registrations | 325 | `QuestCustom` entries in `game/data/quest_scripts.go`; hand-written/shared handlers and translated Java handlers. |
| Race prologues | 2 | Quests 1000 and 2000 run through `game/quest.go:startPrologue`. |

The Java custom-handler tree has **330 files representing 329 distinct IDs**.
Of those IDs, 325 have custom registrations; 1139 is covered by the XML monster
hunt, 3913 by the XML report-to handler, and 1000/2000 by the prologue path.
Do not add Java file counts to the XML count without deduplicating IDs.
Quest metadata also contains entries without executable handlers; metadata
counts are not playable-quest coverage counts.

`python3 go/scripts/quest-claim.py status` currently prints 329 handlers,
325 registered, zero claimed and two free (1139 and 2000). This tool only scans
custom registrations and local claim markers, so **its free count is not a
missing-quest count**. Both entries already have implementations. Local claim
state can change this output without changing source coverage.

### Implementation and evidence

- Shared XML behavior is in `game/quest_dialog.go`, `quest_templates.go`,
  `quest_workorder.go`, `quest_xml.go`, `quest_world.go` and `loot.go`.
- Custom behavior is in `game/quest_*.go`. Generated Java translations are in
  `game/quest_java_dialogs.go`, produced by `go/scripts/quest-java-port.py`.
  `game/quest_java_events.go` dispatches translated kills, attacks, item use,
  zones, movies, world entry, deaths and quest finishes. Level-up and other
  shared hooks also participate; these events are implemented, not an unported
  handler backlog.
- `TestQuestJavaParity` replays `QuestTraceDump` dialog probes, comparing quest
  packets, state, inventory and experience. The local 2026-10-03 trace artifact
  contains all 1,923 IDs, and its report contains no differing-probe rows.
  This artifact was inspected, not regenerated during this documentation update.
  The test skips IDs absent from `Data.QuestScripts` (the separate prologue
  path), probes where Java throws, and explicit `parityDeviation` cases. An
  empty report therefore does not mean every event was verified.
- Catalog tests cover all XML families using synthetic fixtures. Work-order
  turn-in tests inject crafted goods; they do not prove a live crafting chain.
  Focused custom-handler and event tests provide additional evidence.
- The first Elyos quest was previously confirmed by the user in the 1.9
  client. Broader individual quest/client confirmation is not recorded here.

### Remaining quest correctness and verification

1. Extend parity probes beyond dialogs to kills, attacks, item use, zones,
   world entry, deaths, movies, timers, spawns and teleports. Inspect event
   branches and shared helper side effects against Java.
2. Audit party kill credit, generic work-item cleanup on completion/abandonment,
   special rewards, and atomic or idempotent item/experience/kinah persistence.
   Treat these as cross-system parity/reliability checks, not missing quest IDs.
3. Reconcile the open findings in [QUEST_AUDIT.md](QUEST_AUDIT.md) with Java
   behavior and parity evidence. A matching dialog probe does not automatically
   close every conformance finding.
4. Check representative 1.9 client flows and disposable-database persistence:
   dialog pages, NPC availability, quest drops, timers and rewards. Do not ask
   the user to play every quest as a substitute for automated coverage.

Use [QUEST_HANDLER_AGENT.md](QUEST_HANDLER_AGENT.md) for bounded implementation
and verification batches, [QUEST_TRIAGE.md](QUEST_TRIAGE.md) for symptoms, and
[QUEST_DEBUG.md](QUEST_DEBUG.md) for Java/Go comparisons. These documents describe
workflow and findings; maintain coverage/status only in this roadmap.

## Confirmed missing Java behavior

| Behavior | Java evidence | Current Go gap |
| --- | --- | --- |
| Fall damage | `CM_MOVE` calls `StatFunctions.calculateFallDamage` when enabled, for grounded active players meeting the movement/distance conditions. | `game/world.go:move` explicitly notes fall damage is unported. |
| Enchantment-stone supplements | `services/EnchantService.java:enchantItem` adjusts success chance and consumes the applicable supplement quantity. | `game/enchant.go:startEnchanting` passes supplements only to manastone socketing; `enchantItem` has no supplement argument or handling. Manastone supplements already work. |

Java source paths in this table are under
`java/AL-Game/src/main/java/com/aionemu/gameserver/`.
This is a confirmed list, not an exhaustive method-by-method certification.

## Remaining source audit

- Channel interactions with rifts, broadcasts and world-scoped scans.
- Handler-managed spawns and static/action objects; crafting station checks;
  instance-specific doors, keys and boss behavior. Establish which branches
  Java actually implements before adding dungeon functionality.
- Abyss shop currency, prices, conditions and rank restrictions; legion
  contribution/reward edges; alliance readiness/loot and group reward edges.
- Java DAO/configuration coverage, command branches, and reconnect/restart
  persistence. The Go command catalog lists the Java admin command set, so an
  old note saying “rest of the commands” is not proof of missing commands.
- Packet safety, authorization, concurrency, lifecycle and transaction review
  across all Go packages. Follow
  [GO_SKILLS_APPLICATION_PLAN.md](../docs/GO_SKILLS_APPLICATION_PLAN.md) and
  record fixes and actual checks in
  [GO_REVIEW_LEDGER.md](../docs/GO_REVIEW_LEDGER.md).

Implement confirmed gaps in bounded batches. Audit existing systems before
scheduling their reimplementation. Keep port coverage and Go review evidence
separate, and update this document when either establishes a new port gap.

## Implemented behavior awaiting broader verification

Compare old 1.9 captures and representative client flows for:

- Groups, loot roll/bid, alliance subgroup changes/brands, friends, blocks,
  whispers, LFG/search and player inspection.
- Trade, private stores, mail, broker, warehouses, cube expansion and soul healing.
- Teleporters, channels, instances, static doors, flight/gliding, duels and summons.
- Gathering, crafting, enchanting, godstone socketing/procs, stigmas, dye and legions.
- Settings, saved effects and cooldowns across relogs/restarts, plus disconnect
  cleanup and persistence. Java flags report/disconnect opcodes as uncertain.

Use existing captures before recording duplicates. Byte parity with Java 21
alone does not prove compatibility with the 1.9 client.

Existing databases may need the item-cooldown schema update shipped in the
source: `ALTER TABLE item_cooldowns MODIFY use_delay INT UNSIGNED NOT NULL`.
Delays can reach 43,200,000 ms. This documentation update does not apply migrations.

## Java limitations and intentional differences

- Siege battles/timers and captured-fortress spawn conversion are absent from
  Java; both servers implement ownership/influence. Working battles are new work.
- Legion warehouse/express mail are disabled or unavailable in the reference.
  A general pet system is absent; `ToyPetSpawnAction` is the implemented kisk.
- `skilllauncher` is a Java stub. Legion emblem upload has no observable Java
  implementation; send/modify are ported. Static objects send no client packets.
- `CM_SHOW_MAP`, `CM_QUESTIONNAIRE` and the trivial group-response path match
  inert Java behavior; empty handlers do not by themselves indicate port gaps.
- Go intentionally restricts chat to its channel, expires rifts after 26 minutes,
  and restores saved effect duration without Java's relog extension. Defensive
  request validation and the simple-class-change eligibility checks also differ.

Validate disabled paths before deciding to expand their scope. Day/night event
scheduling and broader dungeon mechanics likewise need source-backed scope.

## Development checks

Run from the repository root, sequentially on this host:

```sh
go/scripts/run-go.sh go test -json ./...
go/scripts/run-go.sh go vet ./...
go/scripts/run-go.sh gofmt -l .
go/scripts/run-go.sh go build ./cmd/...
```

The wrapper mounts Java static data and sets `AION_DATA`. Database integration
requires disposable fixtures; absent optional fixtures must be reported as skips.
Report exact passed/failed/skipped test-node counts, including subtests, for
new verification runs. Run `go/scripts/quest-parity.sh` to regenerate Java
traces and execute the parity comparison; traces are local artifacts and the
parity test skips when they are absent.

For quest code changes, use focused checks during implementation, then one full
suite/vet pass and one local game image for the completed batch as described in
[QUEST_HANDLER_AGENT.md](QUEST_HANDLER_AGENT.md). Documentation-only updates
need source-count and link checks, not a server image or server test run.

Image names/releases come from `image-manifest.json`. Port verification does
not authorize deployment or stack restarts. Keep Go and Java game databases
separate and preserve persistent database volumes. The runtime's actual deployed
revision must be inspected when relevant; dated handoffs cannot establish it.

## How to test gathering

Code: `game/craft.go` (GatherableController, GatheringTask, AbstractCraftTask, SkillList.addSkillXp), tests in
`game/craft_test.go` (`go test ./game -run 'Gather|Legacy|StartingGath'` through `scripts/run-go.sh`).

Needs: the skill of the plant. A new character starts with Collection (30001, level 1; the entry is in
`skill_tree/craft_skill_tree.xml`, which the Go loader didn't read until now, so characters made before that fix had no
gathering skill and the client greyed every plant out as unavailable: they get it on entering the world now).
At level 10 Collection turns into Essencetapping (30002, same level) and the character learns Extract Aether (30003) and
Morph Substances (40009). Plants say which skill and level they need: Aria/Azpha (level 1) need 30001, so they stop
being gatherable once the character is level 10; herbs and shells use 30002, vortexes 30003. A missing skill or too low a
level makes the server ignore CM_GATHER silently (AL-Game has a TODO for the message; the client greys the plant out).

Where (radius of the 1.9 view is 50, the spots are the first `pool` ones of `data/static_data/spawns/Gather/<map>.xml`,
all with respawn 230 s and 3 harvests):

| Race | Map | Plant | Skill | Coordinates (x, y, z) |
|------|-----|-------|-------|-----------------------|
| Elyos (start Poeta 1212.9, 1044.9, 140.8) | 210010000 | Aria (400601) | Collection 1 | 1198.3, 1066.8, 137.3 (26 m from the start); 1189.9, 1060.2, 137.0; 1138.4, 1011.1, 131.4 |
| Asmodian (start Ishalgen 571.0, 2787.3, 299.9) | 220010000 | Azpha (400651) | Collection 1 | 577.5, 2817.3, 303.6 (31 m from the start); 627.7, 2785.7, 294.6; 555.2, 2859.5, 303.4 |

Do: select the plant (click it), use gather (double-click or the Collection action). Expected: the gathering bar
(SM_GATHER_UPDATE), a progress update every 2.5 s, then "You have gathered Aria" (1330078) plus
"You have gained experience" (1330082) and the item in the cube; after 3 harvests the plant disappears and comes back 230 s
later. Moving cancels: "You have stopped gathering" (1330080) and it counts as a try, as in AL-Game. A full
cube gives "Your inventory is full" (1330081) before it starts. Dying, teleporting or logging out ends it too. Skill points:
each success gives 0.008 * (level + 100)^2 + 60 skill points (141 at level 1) and the skill rises when they pass
0.15 * (level + 30)^2 (one or two plants at level 1); it stops at 99, 199, 299, 399, 450 and 499 (Collection at 49), where
"you can't gain production experience" (1390221) shows until a master teaches the next stage.

Admin help (GM account, chat): `//moveto 210010000 1198 1066 137` (or `220010000 577 2817 303`) puts you next to a plant;
`//goto poeta` / `//goto ishalgen` go to the towns; `//set level 10` and `//set exp <n>` change the level (to test the
level 10 switch); `//add <item id> <count>` fills the cube (until it is full, to see the full-cube message).
