# Quest-system port handoff (2026-10-01)

For claim-safe handler work, use [QUEST_HANDLER_AGENT.md](QUEST_HANDLER_AGENT.md) and `scripts/quest-claim.py`; identical Java handlers may share one implementation after each ID is claimed.

The objective is to port the **entire** Aion Lightning 1.9 quest system from
Java to the tracked Go source under `Apps/AionServer`. Do not ask the user to
play every quest to establish coverage. Use the Java code and XML as the source
of behavior, add catalog-wide automated tests, then use the 1.9 client for
representative protocol and gameplay checks. Milestone 9 in [PORTING.md](PORTING.md)
remains in progress.

## Source and implementation map

The Java tree is at
`java`.

| Concern | Java source or data | Go source |
| --- | --- | --- |
| Quest metadata and rewards | `AL-Game/data/static_data/quest_data/quest_data.xml` | `game/data/quests.go` |
| XML handler declarations | `AL-Game/data/static_data/quest_script_data/*.xml` | `game/data/quest_scripts.go` |
| Handler registration | `AL-Game/src/main/java/com/aionemu/gameserver/questEngine/QuestEngine.java` | `game/data/quest_scripts.go`, `game/quest_dialog.go` |
| Shared template behavior | `AL-Game/src/main/java/com/aionemu/gameserver/questEngine/handlers/template/` | `game/quest_dialog.go`, `game/quest_workorder.go`, `game/quest_xml.go`, `game/quest_world.go`, `game/loot.go` |
| Eligibility and rewards | `AL-Game/src/main/java/com/aionemu/gameserver/services/QuestService.java` | `game/quest_dialog.go`, `game/store/player.go` |
| Custom handlers | `AL-Game/data/scripts/system/handlers/quest/` | `game/quest_custom.go`, quest-family handlers in `game/quest_*.go`, and registrations in `game/data/quest_scripts.go` |
| Quest packets and prologue | `AL-Game/src/main/java/com/aionemu/gameserver/network/aion/` | `game/quest.go`, `game/worldpackets.go` |

There are 3,726 metadata entries in `quest_data.xml` and 1,596 XML handler
declarations: 170 `report_to`, 316 `monster_hunt`, 613 `item_collecting`,
492 `work_order`, and five `xml_quest`. Exactly 330 custom Java quest handler
classes are present. These counts describe different sources and should not
be added together as a count of playable quests without mapping handler IDs.

## Implemented and checked

- The two race prologues, quest persistence and abandonment, nearby NPC
  markers, dialog transitions, report items, monster kills with packed quest
  variables, collection item turn-in, action-item use, and quest drops.
- All 1,099 report, hunt, and collection XML declarations are indexed and
  dispatched by their start/end/action/kill NPCs. The loader preserves Java's
  last-declaration-wins behavior when one hunt declares the same NPC twice
  (quest 2434) and avoids duplicate quest-drop registration.
- All 492 work-order declarations are indexed and dispatched. The handler
  checks the quest's crafting skill range, grants the declared components and
  temporary recipe, checks crafted turn-in items, and removes the temporary
  recipe, components, crafted items, and quest work items on completion or
  abandonment. The existing Go crafting flow can produce the items. The
  work-order catalog test runs every declaration through start and turn-in,
  with a focused skill-boundary and abandonment test. Its crafted goods are
  injected for the turn-in check, so a live crafting chain is still unverified.
- All five `xml_quest` declarations are dispatched through the ordered XML
  operation tree. Their event handler covers NPC dialogs, quest variables and
  statuses, item checks, and delayed object use. A catalog test completes all
  five paths, including the Ancient Cube's three-second object use.
- **215 custom Java handlers are completed and tested.** Coverage includes the original campaign quests, 1006 `Ascension`, 1007 `A Ceremony in Sanctum`, 1031 `The Manduri's Secret`, 1032 `A Ruler's Duty`, 1033 `Sataloca's Heart`, 1034 `Disappearing Aether`, 1035 `Refreshing the Springs`, 1036 `Kaidan Prisoner`, 1037 `Secrets of the Temple`, 1038 `The Shadow's Command`, 1039 `Something in the Water`, 1040 `Scouting the Scouts`, 1041 `A Dangerous Artifact`, 1042 `Keeper of the Kaidan Key`, 1043 `Balaur Conspiracy`, 2006–2007, item-started and NPC chains, level-up dispatch quests, crafting and reward conversations, the Poeta quest 1111 `Insomnia Medicine` and 1114 `The Nymph's Gown`, Elyos Verteron quests 1011–1023, 1130, 1149, 1156–1158, the Elyos quests 1097 `Sword of Transcendence` and 3060 `The Red Journal`, and Asmodian starter/campaign quests 2011–2018, 2123, 2136, 2200, and 2300. New ports also include 1197 `Krall Book`, 1220 `A Secret Delivery`, and 1300 `Orders from Telemachus`. Coverage includes the four talk-chain quests 4939, 4942, 4943, and 19004, plus the Sanctum talk chains 3934-3936, 3938, 3939, 3965-3967 (`game/quest_sanctum_chains.go`), and second-porter's 3093, 3200 (shares 4200's chain), 3319, 3326, 3914, 3930-3933 (`game/quest_sanctum_chains2.go`; `talkChain.startVar` for a start dialog that bumps the variable). The two prologues (1000/2000) and quest 3913 stay on their existing shared handlers; they are tracked as covered, not duplicated as custom handlers. The shared catalog keeps unsupported handlers out of NPC offer markers. `scripts/quest-claim.py status` reports 218 registered, zero actively claimed, and 109 free unported handlers. Ten registered drafts (1092, 1098, 1139, 1141, 1146, 1162, 1163, 1170, 1183, and 1192) remain pending focused tests; because they are already registered, the claim tool excludes them from its free unported count.
- Multi-handler family coverage includes 17 delayed item starts, 34 three-NPC conversations, 13 simple three-NPC chains, five two-report chains, five item-started two-report chains, eight level-up dispatch quests, three NPC conversations after item start, and six additional quest chains (4200, 4934–4938). The separate talk-chain group contains 4939, 4942, 4943, and 19004. Each family has catalog or behavior tests for its state transitions, rewards, and invalid or repeated events.
- `game/quest_catalog_test.go` runs each generic XML quest through synthetic start, progress, and completion. `game/data/data_test.go` verifies catalog counts and NPC indexes. Focused tests cover packets, eligibility, multi-counter hunts, drops, choices, titles, AP, cube size, quest families, and special level-up/item flows. `game/quest_workorder_test.go` covers all 492 work orders, and `game/quest_xml_test.go` covers the five specialized XML quests. The latest repository-wide run (`go/scripts/run-go.sh go test -json ./...`) passed 2,125 test nodes, skipped seven optional database tests, and had zero failures; `go/scripts/run-go.sh go vet ./...` passed. The focused Asmodian Altgard run covered quests 2011–2018, their startup progression, and `TestQuestConformance`, alongside prior coverage for 2123, 2136, 2200, and 2300. Known click-page findings remain in quests 1031, 1032, and 1034; quests 1057, 1042, 1197, 2123, 2016, and 2017 have source-backed Java exceptions. Quest 2018's object-use handler returns control to Java's action-item controller, so it intentionally emits no Go echo. Reward dialog 17 follows Java's `QuestService.questFinish`: it completes with fixed rewards but no selectable item. The manastone socket test now checks the weapon's hand-specific stat and passed ten consecutive focused runs.
- Quest 1114 `The Nymph's Gown` is registered as an item-started custom quest. Its diary acceptance, Namus and Asteros dialogs, Seirenia's timed clothes interaction, dress cleanup, and both reward records have focused automated coverage.
- Elyos Verteron quests 1011–1023 cover NPC and level-up starts, dialogs, collection, scripted attacks, item use, zone transitions, timed summons and escorts, death recovery, an instanced gate sequence, kill progress, and reward choices. Quests 1130 and 1156–1158 add Citadel entry, the village seal search, and the linked Gaphyrk attack/movie sequence with selectable rewards. Poeta quest 1001 now advances after the opening movie ends; quest 1097 ports the prerequisite-gated level-50 talk chain; quest 1149 adds Poppy's escort and rescue flow.
- Quest 1006 `Ascension` ports the journal and testimony item stages, timed trial spawns, minion and boss progression, the boss movie, class choice, death and world recovery, and the Elyos return teleport. Quest metadata now retains `nameId` so its failure message uses the correct localized client string.
- Quest 1007 `A Ceremony in Sanctum` starts at level 10 after Ascension, teleports to Sanctum, handles the prelate and deacon movie/dialog stages, and routes the reward to the four starting-class NPCs and matching reward groups.
- Quest 1031 `The Manduri's Secret` unlocks at level 19, counts six kills across four Manduri types, uses the paper glider, skips the escort as the Java handler does, and awards a selectable reward after Aurelius's report.
- Quest 1032 `A Ruler's Duty` unlocks at level 19, handles the Demro/Lodas/Kerubien sequence and delayed quest-item use in Putrid Mire, then awards a selectable reward after Demro's report.
- Quest 1033 `Sataloca's Heart` unlocks at level 20, gates ten Archon Drake kills behind Kimeia's movie, and completes through Diomedes with selectable or no-choice reward handling.
- Quest 1034 `Disappearing Aether` unlocks at level 21 after quest 1300, runs Lakaias's item route and delayed destroyed-artifact interaction, removes five collected aether items at turn-in, and offers the two selectable rewards through Valerius.
- Quest 1035 `Refreshing the Springs` unlocks at level 26 and ports the staged spring-guide route, three timed item interactions, two proof items and movie 31, plus the selectable reward turn-in.
- Quest 1036 `Kaidan Prisoner` unlocks at level 24, handles the prisoner rescue and movies, proof-item turn-in, key exchange, final report item, selectable reward, and title 10.
- Quest 1037 `Secrets of the Temple` unlocks at level 25, consumes the four temple collectibles, grants the ritual item, and ports the five alternative ritual-site objects, movie 33, and selectable rewards.
- Quest 1038 `The Shadow's Command` unlocks at level 29, adds both delayed object interactions and their movie/item effects, handles the three collectible turn-in, spawns Hippolytus at the Java coordinates, and awards selectable gear when he is killed.
- Quest 1039 `Something in the Water` unlocks at level 29 after quests 1035 and 1016, gives the empty bottle at Asclepius, converts it to the full bottle inside the Mystic Spring, accepts it at Jumentis, and counts the two three-kill scout branches before the fixed reward.
- Quest 1040 `Scouting the Scouts` unlocks at level 31 after quest 1036, ports the Asclepius and report route, three Vaegir kills, both scout-zone teleports, the scout leader kill and movie 36, and the Java switch fallthroughs before the fixed reward.
- Quest 1041 `A Dangerous Artifact` unlocks at level 33 after quest 1034, handles the Telemachus, engineer, Xenophon, Yuditio, and Laigas stages, spawns both temporary beacon exits, ports the delayed object interactions and proof cleanup, and completes with a selectable reward and title 11.
- Quest 1042 `Keeper of the Kaidan Key` unlocks at level 35 after quest 1040, opens through the movie and Kaidan kill/item routes, preserves Java's mismatched-stage dialog fallthrough, consumes the key at turn-in, and completes with its fixed reward.
- Quest 1043 `Balaur Conspiracy` gates level-up on all thirteen Java prerequisites, preserves the cascading NPC/dialog switch fallthrough, counts the level-three Balaur kill, teleports back to Eltnen for its reward, and grants the selected gear and title 12.
- Quest 1051 `The Ruins of Roah` gates its level-up start on quest 1500, preserves the start and second-NPC dialog fallthroughs, runs the tablet and stone-plate interactions, consumes the artifact and collectible, and grants selectable rewards.
- Quest 1052 `Root of the Rot` checks the Java level-up prerequisite, indexes its quest item IDs, consumes both three-item materials, and completes through its selectable reward dialogue.
- Quest 1053 `The Klaw Threat` handles its three-item collection, random temporary Queen Klaw spawn, no-respawn behavior, and selectable reward after the queen kill.
- Quest 1054 `The Power of Elim` checks the Java and XML prerequisites, grants and consumes both proof items, plays movie 187, and handles the 50-item turn-in plus selectable rewards.
- Quest 1055 `Eternal Rest` grants four idempotent offerings, consumes the collection at turn-in, runs the timed urn interaction, and completes through its selectable reward.
- Quest 1056 `Lepharist Poison Research` checks its level and prerequisites, requires exactly one poison sample before movie 101, consumes it, counts the target kill, and awards either selectable reward.
- Quest 1057 `Creating a Monster` handles the artifact and movie stages, advances on entering Heiron, counts three monster kills and the named boss, runs the final object interaction, and grants a selectable reward with title 24.
- Quest 1058 `Aether Insanity` starts after level 38 and quest 1500, preserves the end-NPC dialog fallthrough, checks and consumes both collections, and grants a selectable reward.
- Quest 1059 `The Archon of Storms` handles its geyser timer and movie transformation, then consumes the quest item after the Patema Geyser interaction for a fixed reward.
- Quest 1062 `Indratu Legion` handles its NPC and flight stages, counts ten legionnaire kills, spawns the named boss after a delay, and grants a selectable reward.
- Quest 1071 `Speaking Balaur` handles the paid phrase and cipher routes, consumes the phrase item to advance, and awards experience, Abyss Points, and title 30.
- Quest 1072 `Abyss Training` ports its seven NPC and movie stages, then grants its fixed item, experience, and Abyss Point rewards.
- Quest 1075 `New Wings` handles the flight, spawns both no-respawn Balaur at Java's instance coordinates, and preserves the final variable-three dialog fallthrough for its fixed reward.
- Quest 1076 `Fragment of Memory II` ports its NPC conversations, consumes the collection, uses the follow-up quest item and movie, removes all item copies at the report stage, and awards its fixed experience, kinah, and Abyss Points.
- Quest 1091 `A Request from Atropos` starts on entering Q1091, completes the report at Atropos, and locks follow-up quests 1092–1094.
- Quest 1194 `Reducing Tursin Strength` starts after quest 1193, initializes progress on entering Tursin Garrison, counts ten kills across two Krall types, and offers either selectable weapon reward.
- Quest 1197 `Krall Book` gives one book from the item NPC, starts after its three-second use animation, and consumes the book copies at Pernos before its fixed reward.
- Quest 1220 `A Secret Delivery` starts after quest 1219, advances through its report NPCs, and preserves Java’s middle-NPC switch fall-through before the fixed reward.
- Quest 1300 `Orders from Telemachus` starts on entering Eltnen Fortress or unlocks an existing locked state at level 19; its report locks quests 1031–1043, after which eligible level-up quests activate.
- Quest 2123 `The Imprisoned Gourmet` starts at Munin, preserves the three proof-item branches and Java's mismatched first-branch removal ID, handles the auxiliary NPC's three-second emotion/update, and finishes with Java's fixed reward.
- Quest 2136 `The Lost Axe` starts from item 182203130, runs the grave's three-second interaction and movie 59, spawns report NPC 790009 at Java's coordinates, removes all work-item copies at turn-in, and supports both Java report pages.
- Quest 2200 `Altgard Duties` starts on entering Altgard Fortress, advances at Vandarnt, and locks follow-up quests 2011–2022 on reward dialog 17.
- Quests 2011–2014 start the Altgard campaign: Fungus Among Us, Encroachers, Dangerous Crop, and Scout It Out. Coverage checks their level-up/prerequisite order, dialogues, kill counters, zone/object interactions, collections, and the 2013 crop-item removal before completion.
- Quests 2015–2018 continue that chain: Take the Initiative starts after 2014 and tracks three cap counters with a selectable reward; Fear This gates five kills behind its dialogue, collects three items, and accepts item 182203019 only while active; Observatory checks its level-12/prerequisite gate and collection; Impetusium starts at level 13 and covers its quest drop, delayed grave interaction, no-respawn boss spawn, and final collection. Generic cleanup of Java-declared work items on abandonment remains a cross-system gap.
- Quest 2300 `Morheim Commander's Call` starts on entering Morheim Ice Fortress or unlocks its existing LOCKED state at level 19, advances at Hegesias, and locks quests 2031–2042 at turn-in.
- A separate local-only image, `aiongo-game-quests1197-1220-1300:local`, was built on 2026-10-01 from this workspace. The running `al19-*` stack remains on its existing Go image, so quests 1197, 1220, and 1300 still need direct client confirmation. The first Elyos quest remains the only direct client-confirmed quest.
- The local-only image `docker.io/rafabertholdo/aiongo:1.9-game-go-local-ac3102019e50` was rebuilt on 2026-10-01 from the Asmodian Altgard 2011–2018 batch. Its `org.opencontainers.image.revision` label is `ac3102019e50cd55461a10941db321c4377448310a59a8bbf1ac9bc1943e4a64`; it was not published, and the running stack was not restarted. Quests 2011–2018 and the previously ported Asmodian quests have automated coverage but still need direct client confirmation.
- Start conditions: race when specified, level, permitted class and gender,
  completed prerequisites, and repeat count. Rewards: fixed and selected
  items, experience, kinah, titles, Abyss Points, and cube expansion. The
  database transaction in `store.Store.CompleteQuest` saves quest completion
  with title, AP, and inventory-size changes. Item, experience, and kinah
  writes still use their existing paths and are not part of that transaction.
- `game/quest_catalog_test.go` runs each of the 1,099 generic XML quests
  through synthetic start, progress, and completion. `game/data/data_test.go`
  verifies catalog counts and NPC indexes. Focused tests cover packets,
  eligibility, multi-counter hunts, drops, choices, titles, AP, and cube size.
  `game/quest_workorder_test.go` covers all 492 work orders, and
  `game/quest_xml_test.go` covers the five specialized XML quests. The latest
  complete run passed 2,125 test nodes with zero failures and seven optional
  database tests skipped (`AION_TEST_DB` unset); `go vet ./...` passed.
- The running `al19-*` stack was last restarted on 2026-09-29 with the game image
  containing 161 registered custom handlers. `al19-game-go` registered with
  login and chat. It remains on that image; neither local quest image was deployed.
  The restart script closes an existing Aion client; launch through ReRun again
  after a deliberate stack restart.
  The user confirmed Elpas's first quest in the 1.9 client before the catalog
  expansion. The other quests have automated coverage, not individual client
  confirmation.

## Remaining work

### Batch cadence

To improve throughput, claim a bounded group of related handlers and integrate
them as one batch. For each quest, record the Java event/branch table and add a
focused test while the behavior is in view. Reuse family helpers and catalog
fixtures; avoid repeating source-reading and environment setup for each quest.
Format once after integration, then run one combined focused-test and
conformance pass, one repository-wide test suite, one vet pass, and one local
game image build for the batch. Serialize Go commands because the shared cache
volumes can attach to only one container at a time. If full checks uncover a
code fix, rerun the suite and vet before handoff. The contributor and
integrator steps are in [QUEST_HANDLER_AGENT.md](QUEST_HANDLER_AGENT.md).

1. **Custom Java handlers (zero actively claimed and 109 free unported handlers, according to the current claim-tool status).**
   Use `python3 scripts/quest-claim.py status` to see current work, then atomically claim IDs with `python3 scripts/quest-claim.py next <agent-name> <count>` before porting. Java handlers use NPC talk, kills, item use, movies, zones, timers, groups, and other events; XML templates cannot stand in for them. Keep unsupported quests out of nearby offer markers until every required event is wired and tested. After registration and passing tests, mark each claimed ID done with `python3 scripts/quest-claim.py done <id>`; release abandoned IDs with `release <id>`.

   The Java source scan counts 330 handler classes across 14 zone folders; the claim script indexes numbered filenames. `quest-claim.py status` is the source for current registered, claimed, and free counts.
   Source method counts give an initial event map: 328 `onDialogEvent`, 121
   `onLvlUpEvent`, 65 `onItemUseEvent`, 60 `onKillEvent`, 22
   `onEnterZoneEvent`, 10 `onEnterWorldEvent`, nine `onMovieEndEvent`, six
   `onDieEvent`, five `onAttackEvent`, two `onQuestFinishEvent`, and one
   `onQuestTimerEndEvent`. Many classes implement several events. Inventory
   each handler's quest ID and required operations before registering markers.
   Zone entry, LOCKED transitions, quest item use, object interactions, level-up starts, and shared multi-NPC conversations have verified implementations. The Asmodian Altgard 2011–2018 campaign batch is complete. Continue with the next coherent startup/campaign family from the claim tool; do not reassign active claims.
2. **Cross-system parity.** Add group/party kill credit, generic quest work-item cleanup
   on finish and abandon, special reward cases, and
   transactional or idempotent handling for item/experience/kinah rewards.
   Check the Java implementation for each behavior instead of assuming that
   metadata alone is sufficient.
3. **Verification.** Extend the catalog test with scenarios for each new
   handler kind and custom handler family. Verify representative packets and
   flows against the 1.9 client and the old 1.9 server capture. A synthetic
   completion test proves Go dispatch logic, not NPC spawn availability,
   correct client dialog pages, drop rates, or live MariaDB persistence.

## Commands and local runtime

Run from the repository root, sequentially: Apple `container` cannot attach
the shared Go cache volumes to concurrent Go containers.

```sh
go/scripts/run-go.sh go test ./... -count=1
go/scripts/run-go.sh go vet ./...
go/scripts/build-images.sh game
java/docker/go-stack.sh login
container logs al19-game-go
```

`run-go.sh` mounts the original `AL-Game/data/static_data` and sets `AION_DATA`.
The Go image uses that same static data. `go-stack.sh login` recreates the
login, Go game, and sniffer containers and kills the Aion client; ReRun's Play
refreshes its relays. Never run Git commands in this repository, per the root
`AGENTS.md`, and do not modify `External/`.
