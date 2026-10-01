# Handoff (2026-09-29): where the Aion Go port stands, and what to pick up

Read this first in a new session, then PORTING.md, QUEST_HANDLER_AGENT.md (Java conformance checklist),
QUEST_DEBUG.md and the aion-server-expert skill (`.claude/skills/aion-server-expert/SKILL.md`).
Rules of the repo: never run git commands; don't modify External/; run gofmt/vet/tests after changes.
The user works in the 1.9 client through ReRun (Aion tab, Server: This Mac). Work is split by feature into
fresh subagents (keeps the main context small).

## What is done
- Everything AL-Game 1.9 implements outside quests is ported (see PORTING.md "Not ported yet": only siege
  battles and the legion warehouse, both absent/off in AL-Game; express mail and pets do not exist in 1.9).
  This session added: alliances, legion emblems and history, kisks, rifts, sieges (ownership/influence),
  item remodel and arms fusion, zones/weather/game time, petitions and punishments, bind stones, postboxes,
  class change, announcements, account warehouse, flight teleporters, gathering fix (starter skills from
  craft_skill_tree.xml), mob proximity aggro fix (ai.go), quest dialog framework fix (main menu, echo of
  unhandled selects, see QUEST_HANDLER_AGENT.md), ~60 more quest handlers (164 of 329 custom handlers registered).
- Debug environment (Java vs Go with sniffers) exists: `docker/quest-debug.sh` in
  `java/docker`, `cmd/pktdiff`, QUEST_DEBUG.md.
  It is currently TORN DOWN on purpose (user asked): the stack is ReRun's plain one (al19-db, al19-login,
  al19-chat, al19-game = Go game image). `quest-debug.sh up` brings back Java (id 2, own DB
  `au_server_gs_java`) plus sniffers; `quest-debug.sh down` returns to the plain stack (ReRun requires the
  container `al19-game` with env HOST_NAME=127.0.0.1). Caveats and the workflow are in QUEST_DEBUG.md.

## Agents that were running when quota ended (results may be missing; check the tree)
1. Audit batch 1 (fix dialog-framework violations in ported quests): it claimed 12 audit ids with
   `scripts/quest-claim.py audit-next audit-agent-1 12`. Run `scripts/quest-claim.py audit-status` and read
   QUEST_AUDIT.md: rows still "open" are not fixed. Release stale claims (`.quest-claims/audit-<id>` dirs) whose
   quests are still in `game/quest_conformance_known_test.go`'s known-failing list, then re-claim.
2. Quest-investigation tooling agent: was to add `scripts/quest-java-table.py <questId>`, `pktdiff -dialogs` and
   `-gen-test`, `quest-debug.sh prep|compare`, and QUEST_TRIAGE.md. Check which of those exist; finish the rest.

## Next steps, in priority order
1. Finish the quest audit: 46 registered handlers (49 violations, mostly "plain click answered with a quest page
   instead of the main menu") are listed in QUEST_AUDIT.md. Fix by zone with audit agents (claims via
   `scripts/quest-claim.py audit-next <name> 12`), verifying each against its Java handler (`case -1` in
   the Java class is a legitimate exception; cite the line in the exceptions table). Similar quests (same
   helper/shape) share the same bug, so fix helpers once.
2. Keep porting the remaining custom handlers (165 free: `scripts/quest-claim.py status`; claims via
   `scripts/quest-claim.py next <name> N [--from-end]`). Another agent works on the same list from the low ids
   (they use the claim tool: `quest_1016_agent` etc.). Every new handler must follow the Java conformance checklist
   and pass `TestQuestConformance`.
3. Use the Java-vs-Go loop (QUEST_DEBUG.md) on quests the user reports as wrong in the client; the recorded
   Kerub Threat run (logs in docker/.quest-debug/logs) is the worked example. Character to use: Wrathchild.
4. Record 1.9 sessions for the unverified packets listed at the end of PORTING.md (needs the user playing).
5. Known problems: `login/login_test.go:367` doesn't compile (undefined `id`); several quest tests fail or race
   (TestForestOutlawLevelUpNineKillRouteAndReward, TestForestOutlawFiveKillRoute,
   TestBelbuasTreasureOfferAndBarrelReward, TestAllGenericQuestScriptsComplete, TestGrainSackUseOpensQuestLoot,
   TestAllSpecializedXMLQuestsComplete, timer-based -race failures); DelicateMandrake panics on nil countItems
   (skip it or fix). Switching servers inside one client session hangs the client (a 4-byte first packet;
   restart the client between servers). Restarting the client is needed after every game-container rebuild.

## How to run things
- Go (no Go on the Mac): `scripts/run-go.sh go test ./...` or the scratchpad helpers gob.sh/gobc.sh/gobd.sh
  (containers share cache volumes: ONE Go container at a time per volume; "storage device attachment is invalid"
  = volume busy, retry; kill leaked ones with `container kill <id>; container delete --force <id>`).
  Skip list for a green run: `-skip 'GrainSack|SpecializedXMLQuests|AshesToAshes|DelicateMandrake'`.
- Rebuild and deploy the Go game server: `scripts/build-images.sh game`, then recreate `al19-game` with
  `container run --detach --name al19-game -m 3G -e AION_DB=<db ip> -e AION_LS=<login ip> -e AION_CS=<chat ip>
  -e HOST_NAME=127.0.0.1 docker.io/rafabertholdo/aion-lightning-game-go:1.9` (delete the old one first; kills the
  client relay: user presses Play again). `docker/go-stack.sh` is the older sniffer flavour of this.
- Data: MariaDB `al19-db` (root/aion; game DB `au_server_gs`, login DB `au_server_ls`).

## Test character state (set 2026-09-29 for the user's own testing)
Wrathchild (player id 129946, Elyos): class set to SORCERER, exp set to the start of level 49 (853743989) so the
user can run `//set level 50` in game (admin account) to trigger the real level-up path and learn the skills;
equipped Miragent cloth set (110100917 torso, 113100829 legs, 112100779 shoulders, 111100821 gloves, 114100857
boots) and the Miragent-tier devanion book 100600595. Backup of the earlier state:
`docker/.quest-debug/snapshots/Wrathchild-before-lvl50.sql` (in the quest-debug dir of the Java source tree).

## Ascension (class change) note
The level-9 class change in Java is the quest pair 1006/1007 (Elyos) and 2008/2009 (Asmodian), started by their
onLvlUpEvent; 1007 completion sets the second class. In Go, 1006/2008 (the Ascension handlers) are NOT ported yet (still in
`scripts/quest-claim.py list`); only `services_small.go` `classChangeDialog` exists (SimpleSecondClass, off by default).
So when testing levelling from a low level, ascension will not happen by itself. Wrathchild was given SORCERER directly
in the database and quests 1006 and 1007 were marked COMPLETE so her state matches a real ascension. Port 1006/1007/2008/2009
(and check the level-up hooks in `lifestats.go` levelUp / `quest_levelup_start.go`) before relying on normal levelling.
