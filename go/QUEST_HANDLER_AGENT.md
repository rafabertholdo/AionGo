# Claim-safe Java quest handler porting

## Java conformance checklist (MANDATORY, read before porting or auditing any handler)

A systemic dialog bug once shipped in dozens of ported quests because each port looked right in isolation and only a
Java-vs-Go packet comparison with the real client showed it. These rules are verified against the Java source
(`AL-Game/src/main/java/com/aionemu/gameserver`, paths below relative to it). The Go framework in
`game/quest_dialog.go` implements them once; a handler must only port its own `onDialogEvent` faithfully.

**Framework rules (do not re-implement in a handler):**

1. Plain click (`network/aion/clientpackets/CM_SHOW_DIALOG`): npc `setTarget(player)`, broadcast `SM_LOOKATOBJECT`, then
   `NpcController.onDialogRequest` (l.169): `QuestEngine.onDialog(QuestEnv(npc, player, questId 0, dialogId -1))`; if no
   handler returned true -> `SM_DIALOG_WINDOW(npcObjId, 10)` (main menu, quest id 0). The client then picks the quest
   itself and sends `CM_DIALOG_SELECT` 25 with the quest id. So a handler answers `-1` ONLY where Java has `case -1:`
   or calls `defaultQuestEndDialog` (REWARD -> window 5). Never answer a click with a page (1011, 1352, 1693, 2375).
2. `CM_DIALOG_SELECT` (`CM_DIALOG_SELECT`, `NpcController.onDialogSelect` l.196): only within 10 m of the npc; first
   `QuestEngine.onDialog(QuestEnv(npc, player, questId, dialogId))`; if it returns true, stop; else the service switch
   (ids 2-7, 20, 27, 29-31, 35-42, 47, 50, 52, 53, 60, 61 are served by the npc, never echoed); else the `default`
   branch answers **`SM_DIALOG_WINDOW(npcObjId, dialogId, questId)`** (`questId > 0`, else without a quest id): the
   window with the same number. This echo is how the client pages 1012 -> 1013 -> accept. A page the Java handler does
   not `case` needs NO Go code, and a Go handler must not send it (or the echo is doubled/skipped).
3. `QuestEngine.onDialog` (`questEngine/QuestEngine.java` l.134): `questId != 0` -> only that quest's handler; if it
   returns false there is NO fallback to other handlers. `questId == 0` -> the npc's on-talk handlers in registration
   order, first true wins. (Go: `conn.questTalk`.)
4. `onDialogEvent` returning **true** = the handler answered; **false** = fall to the echo. Every `case`, every
   `break`, every switch fall-through and every implicit `return false` at the end of the method counts; a `case` that
   `break`s out of the inner switch and falls into the next NPC's `case` (no `return`) is real Java behaviour (e.g.
   `_1914DispatchtoVerteron` falls from npc 203726 into 203097). Movies do not count as answers: Java
   `sendPacket(SM_PLAY_MOVIE)` followed by `return false` means movie THEN echoed page.

**Java quest API -> Go (1.9 has only these; `sendQuestStartDialog`, `sendQuestEndDialog`, `defaultQuestNoAction`,
`playQuestMovie`, `giveQuestItem`, `removeQuestItem`, `changeQuestStep` do NOT exist in this tree, so a handler that
"uses" them was written from a newer server and must be re-derived from the raw calls):**

| Java (1.9) | Effect | Go |
|---|---|---|
| `sendQuestDialog(player, objId, d)` (`QuestHandler` l.87) | `SM_DIALOG_WINDOW(objId, d, questId)`, returns true | `c.send(dialogWindow(o.id, d, id))` (any packet sent counts as answered) |
| `defaultQuestStartDialog(env)` (l.93) | 1007 -> window 4; 1003 -> window 1004; 1002 -> `QuestService.startQuest` then window 1003 (returns **false** if start fails, so 1002 gets echoed); other ids false | `c.customQuestStart` / `c.startQuest` |
| `defaultQuestEndDialog(env)` (l.114) | ids 8-17 -> `questFinish` then window 10, true; 1009 and -1 -> window 5 only if status REWARD; else false | `c.finishQuest`, `c.readyQuestReward`, window 5 by hand |
| `QuestService.startQuest` (`services/QuestService` l.248) | checks race/level-2/class/gender/prerequisites/minlevel first; sends `SM_QUEST_ACCEPTED(id, START, 0)`, creates state, then `updateNearbyQuests` | `c.startQuest` (never set START by hand) |
| `qs.setQuestVarById(i, v)` | changes memory only, tells nobody | `setQuestVar` + persist |
| `updateQuestStatus(player, qs)` (l.66) | `SM_QUEST_ACCEPTED(id, status, vars)`; on COMPLETE also nearby quests | `c.send(questAccepted(2, q))` after `SaveQuest` |
| `qs.setStatus(REWARD/START)` | memory only; pair it with `updateQuestStatus` exactly where Java does | same |
| `QuestService.questFinish` (l.69) | only if `ItemService.addItems` fits (if not, nothing happens but it STILL returns true, so window 10 follows); then kinah, **XP (SM_STATUPDATE_EXP or level-up first, THEN the gained-XP message)**, title, AP, inventory expansion, status COMPLETE, count+1, `SM_QUEST_ACCEPTED`, nearby quests, level-up quests; the caller then sends window 10 | `c.finishQuestReward` (never hand-roll rewards) |
| `PacketSendUtility.sendPacket(SM_PLAY_MOVIE(0, n))` | cutscene; NOT an answer | `c.send(playMovie(n))`; a Java `return false` after it = do nothing else |
| `ItemService.addItems/addItem`, `removeItemFromInventoryByItemId` (all), `decreaseItemCountByItemId(n)` | grant / remove quest items | `c.s.addItem`, `c.s.removeItemsByID`, `c.s.countItems`; guard capacity the way Java does |
| `return true` / `return false` | answered / echo | Go return values are ignored: only packets sent count. Use `c.dialogSilent()` for a true that sends nothing, `c.dialogNotHandled()` for a false after sending something (not a movie), `c.dialogResult(ok)` |

**Typical dialog ids (verify per quest, never assume):** 25 = quest picked from the main menu; 1011 = first page;
1012/1013/1352/1353/1354 = follow-up pages (usually echo only); 1002 = accept via `defaultQuestStartDialog`;
10000/10001 = custom accept (var change + window 10); 33 = hand over items; 1009 = ready for reward (window 5);
8-17 = reward choice (`questFinish`); 10002 = second-stage accept.

**Mistakes the conformance test catches (`game/quest_conformance_test.go`):** a click that is not the main menu; a
click answered with a quest page without a Java `case -1`; an unknown select not echoed with the quest id (or 0);
a movie page that also advances the quest or accepts (movie-then-accept shortcut); a quest started on any dialog other
than 1002/10000/10001; a finish whose packets are not `SM_QUEST_ACCEPTED`, then window 10, with XP update before the
XP message; a handler that panics.

**How to port a handler (in this order):**

1. Read the Java class end to end. In `onDialogEvent`, list EVERY `case` and EVERY `return`, including the implicit
   `return false` at the bottom, the `break`s that fall through to another branch, and which npc / status / var each
   sits under. Read `register()` for the npcs and the events (`addOnTalkEvent`, `addOnKillEvent`, `addQuestLvlUp`...).
2. Write the switch table FIRST as a table-driven Go test (one row per Java branch: npc, status, var, dialog id ->
   expected windows (dialog id, quest id) / actions / state after / items). Include the rows Java answers with
   `false` (expect the echo of the same-numbered window) and the plain click (`-1`: main menu unless Java has `case -1`).
   Use `quest_1001_test.go` (recorded Java windows) as the model.
3. Implement the Go handler to satisfy the table, using the helpers above; send only what Java sends, in Java's order.
4. Run the handler's focused test after implementing it. The integrating agent
   runs `TestQuestConformance` once against the fully registered batch. Your
   quest must produce no violation. If you are integrating the batch yourself,
   include conformance in that batch check. If you believe Java itself breaks a
   rule, add it to `conformanceExceptions` in
   `game/quest_conformance_known_test.go` with the Java file:line. Do NOT add
   your quest to `conformanceKnownFailing` (that list is for the old audit
   findings and only shrinks; see [QUEST_AUDIT.md](QUEST_AUDIT.md)).
   Before hand-porting, try `scripts/quest-java-port.py --check <id>`: it translates the Java onDialogEvent (and an
   unlocking onLvlUpEvent) line for line into `game/quest_java_dialogs.go` (`scripts/quest-java-port.py <id>` writes
   it), and says which Java call it cannot map when it cannot. Translated handlers take precedence over hand ports in
   `questDialog`; keep only their other events (kills, items, zones) hand-ported.
5. Run `scripts/quest-parity.sh <id>` (Java handler vs Go handler over every state, npc and dialog; see
   [QUEST_TRIAGE.md](QUEST_TRIAGE.md) section 0). A non-seeded difference is a bug unless you can name the Java line
   that makes it unreachable.
6. Where a real 1.9 client is available, compare packets with `docker/quest-debug.sh` (see below).

**Auditing already ported quests:** `scripts/quest-claim.py audit-next <who> [n]` claims registered handlers that have
open rows in QUEST_AUDIT.md (claims live in `.quest-claims/audit-<id>`, separate from port claims). Fix the handler
against its Java class, remove its `<id>/<rule>` entries from `conformanceKnownFailing` (the test
`TestQuestConformanceKnownFailingStillFail` fails until you do), mark the row `fixed`, then `audit-done <id>`.
The generic rules only find what they can see; when auditing, still diff the whole `onDialogEvent` against the Go
handler branch by branch.

Use this brief for a fresh agent context. The overall objective and current counts
are in [QUEST_PORTING.md](QUEST_PORTING.md). Take only Java handler IDs claimed to your agent name. Identical handlers may share one Go implementation after each source ID has been individually claimed. The Java source is
under `java/AL-Game`.

## Claim your handler first (several agents port quests at once)

Never pick a handler by eye. From `Apps/AionServer`:

```sh
scripts/quest-claim.py next <your-name>            # claims and prints the lowest free id
scripts/quest-claim.py next <your-name> 1 --from-end   # the highest instead (the second porter uses this)
scripts/quest-claim.py claim <your-name> <id>      # a specific one; exit 1 if someone has it
scripts/quest-claim.py status                      # who holds what
scripts/quest-claim.py release <id>                # if you give up
scripts/quest-claim.py done <id>                   # after it is registered and its tests pass
```

The claim is an atomic `mkdir .quest-claims/<id>`, so two agents can't get the same
handler. Handlers registered in `game/data/quest_scripts.go` count as ported and can't be
claimed. Port only ids you hold; if `claim` fails, take another.

## Inspect before coding

1. Read that handler in `data/scripts/system/handlers/quest/<zone>/`, its quest
   entry in `data/static_data/quest_data/quest_data.xml`, and any prerequisite
   handlers it calls or starts. Inspect the corresponding Go files:
   `game/quest_custom.go`, `game/quest_dialog.go`, `game/quest_world.go`,
   `game/data/quest_scripts.go`, and `game/quest_custom_test.go`.
2. Make an event table: start source, NPCs and objects, dialog IDs and pages,
   state/variable guards and changes, item grants/removal/drops, kills, level,
   zone, world entry, movie, timer, finish, prerequisites, and reward record.
   Read Java helpers (`QuestService`, `QuestEngine`, `QuestHandler`) when a call's
   side effects are unclear. Check whether the 1.9 client protocol needs a
   packet the Go server can already send.
3. If an event mechanism is missing, report that dependency explicitly. Do
   not register a marker or silently omit an event to make the quest appear
   implemented.

## Implement

1. Add the handler to a **new quest-specific Go file** in `game/`, with one
   dispatch function or method. Reuse the shared helpers and use storage
   updates before mutating in-memory quest state. Keep checks for quest
   status, variable, NPC/object and dialog ID precise. Guard item capacity and
   duplicate grants. Follow Java's reward branch and cleanup behavior.
2. Add a **new quest-specific test file** covering acceptance, each event
   branch, completion, invalid or repeated event, and item/reward handling.
   For a blocked mechanism, write the exact blocker in the handoff instead of
   treating a partial test as completion.
3. The integrating agent owns shared registration and dispatch edits:
   `game/data/quest_scripts.go`, `game/quest_custom.go`, shared event wiring,
   `QUEST_PORTING.md`, and `PORTING.md`. Provide the registration facts (NPC,
   object, item and kill IDs) and function name in your handoff. If your test
   requires registration before it runs, say so; integration will follow.

## Verify and hand off

- Format changed Go files with `scripts/run-go.sh gofmt -w ...` from
  `Apps/AionServer`. Apple `container` attaches Go cache volumes to one
  container at a time: coordinate Go commands with the integrating agent and
  run them sequentially.
- Run the focused test for each handler you implement. Do not run the
  repository-wide suite, vet, or image build once per quest.
- Return Java source path, implemented events and behavior, registration facts,
  test names/results, any blockers, and the files changed. Do not run Git
  commands or modify `External/`.

## Batch cadence for throughput

1. Claim a bounded batch with `scripts/quest-claim.py next <agent-name> <count>`.
   Group related or straightforward handlers where useful, but only claim work
   that can be finished and tested in the batch. Every claimed handler still
   needs its own Java source review, branch table, and focused test coverage.
2. Contributors implement quest-specific files and run their focused tests.
   The integrating agent collects the handoffs, applies shared registration
   and dispatch edits once, updates the porting notes once, and formats all
   changed Go files together.
3. Run one combined focused-test command for the batch, including all new quest
   tests and `TestQuestConformance`. For example, the recent batch used this
   from the AionGo repository root:
   `go/scripts/run-go.sh go test ./game -run 'Test(KrallBook|SecretDelivery|OrdersFromTelemachus|QuestConformance)'`.
   Replace the quest-name prefixes with those in the current batch. Fix any
   failures before the full checks.
4. After the batch is integrated, run
   `go/scripts/run-go.sh go test -json ./...` and
   `go/scripts/run-go.sh go vet ./...` once each, sequentially. Report exact
   pass, skip, and failure counts. Then build one local game image with
   `go/scripts/build-images.sh game`.
   If that script's clean-tree guard rejects the shared worktree, preserve all
   edits and build a local-only image directly with `container build
   --no-cache --platform linux/arm64 --progress plain --cpus 2 --memory 4G -f
   go/Dockerfile --target game -t <manifest-derived-local-tag> .`. Derive the
   tag from `image-manifest.json` and a SHA-256 of the local source inputs; use
   that snapshot hash as the local revision label. Do not publish the local
   image or restart the live stack. `--no-cache` prevents Apple's persistent
   builder from reusing a stale worktree context.
5. If a code fix is needed after the full checks, rerun the repository-wide
   suite and vet before handoff, then rebuild the local image from the final
   source. Mark claims done only after integration and tests pass.

An individually coded quest is counted as ported only after all its required
events are wired, it is registered, and its tests pass. A client marker by
itself does not establish parity.

## Fast investigation of a misbehaving quest

[QUEST_TRIAGE.md](QUEST_TRIAGE.md): decision tree by symptom, the tools to run first (`scripts/quest-java-table.py <id>`
prints the Java handler as an event table with line numbers; use it as step 1 of "How to port a handler" too),
`quest-debug.sh prep|compare`, `pktdiff -dialogs` and `pktdiff -gen-test`, which turns a recorded Java run into
`game/quest_<id>_replay_test.go` in one command.

## Compare against the real Java server

To see which packets a quest emits in Java versus Go with the real 1.9 client, follow
[QUEST_DEBUG.md](QUEST_DEBUG.md): `docker/quest-debug.sh` runs the old Java game server
(`aion-gs:1.9`) or the Go one behind a decrypting sniffer, and `go run ./cmd/pktdiff -quest
java.log go.log` prints the differences per client request.

## Dialog framework (mirrors NpcController.onDialogRequest/onDialogSelect)

Recorded with the real 1.9 client on the Java server (quest 1001); implemented once in `game/quest_dialog.go`, so a
handler only has to port its own `onDialogEvent`:

- **CM_SHOW_DIALOG** turns the npc to the player (SM_LOOKATOBJECT), then calls the npc's quest handlers with dialog id
  `-1`. A handler answers `-1` only where Java has `case -1` / `defaultQuestEndDialog` (REWARD -> window 5, or its own
  page). If none answers, the client gets the **main menu, SM_DIALOG_WINDOW 10**, and picks the quest itself: it sends
  CM_DIALOG_SELECT 25 with that quest's id. Never answer a plain click with a quest page (1011, 1352, 2375) yourself.
- **CM_DIALOG_SELECT** (`questTalk`): the named quest's handler (or every handler of the npc when the quest id is 0);
  if it does not answer, the server sends **SM_DIALOG_WINDOW(dialogId, questId)**, the window of the same number. That
  is how the client pages 1012 -> 1013 -> accept 10000: a page such as 1012/1013/1353/1354 must NOT be handled, and a
  select the handler does not know needs no code. The ids `NpcController` serves itself (2-7, 20, 27, 29-31, 35-42, 47,
  50, 52, 53, 60, 61) are never echoed.
- "Answered" is Java's `return true`: the handler sent something (`conn.dialogReplies`, which ignores periodic packets
  and SM_PLAY_MOVIE). A Java handler that plays a movie and returns false therefore gets its page echoed after the
  movie for free. Use `c.dialogNotHandled()` for a Java `return false` after sending anything else,
  `c.dialogSilent()` for a `return true` that sends nothing, and `c.dialogResult(ok)` when the Go handler returns
  Java's boolean.
