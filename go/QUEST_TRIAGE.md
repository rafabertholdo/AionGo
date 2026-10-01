# Quest misbehaves in the client: triage

Goal: from "quest X does something wrong" to a Go fix and a regression test with as little manual work as
possible. Background: [QUEST_DEBUG.md](QUEST_DEBUG.md) (the two-server rig), [QUEST_HANDLER_AGENT.md](QUEST_HANDLER_AGENT.md)
(Java conformance rules), [QUEST_AUDIT.md](QUEST_AUDIT.md) (rule-by-rule audit of ported handlers).
`QD` below is `java/docker/quest-debug.sh`; run everything else from
`Apps/AionServer` (Go via `scripts/run-go.sh go ...`, one container at a time).

## 0. Always first (2 minutes, no client needed)

```sh
python3 scripts/quest-java-table.py <id>          # what the Java handler does: npcs, dialog ids, guards, actions, L<line> numbers
scripts/run-go.sh go test ./game -run 'TestQuestConformance|Quest<id>'   # framework rules + the quest's own tests
```

Read the table against the Go handler (`game/data/quest_scripts.go` says which Go function serves the quest: custom
handler, template kind, talk chain, level-up start). Most bugs are visible here: a Java `return false` ported as an
answer, a `break` that falls through, a var guard that differs. If the table already shows the difference, fix it,
add the row to the quest's table test, done. Only go to the client when it does not.

## 1. Decision tree

| Symptom in the client | Look first | Then |
|---|---|---|
| **No marker** on the giver | `nearbyQuests` / `canStartQuest` (`game/quest_dialog.go`): race, `MinLevel-2`, class, gender, prerequisites, an existing row that is not NONE/COMPLETE. Level-up quests: is there a `START/0` row (`RESET_TO=START`)? Table: `starts: ON LEVEL-UP` vs `by npc` | Registration in `game/data/quest_scripts.go` (`QuestStarts[npc]`). Level-up quests: `levelUpStartQuests` (`game/quest_levelup_start.go`) sets `LevelUpStart` scripts to START when `level >= MinLevel`. Log check: `SM_NEARBY_QUESTS` in the diff |
| **Click shows a quest page instead of the menu / wrong first window** | Java answers a plain click with the main menu (window 10) unless the handler has `case -1`. `conn.showDialog` -> `questTalk` -> handler with dialog `-1` | The Go handler answers `-1` where Java does not (table: `dialog==-1` rows). Fix the handler, not the framework |
| **Dialog stuck / a page never advances** | `dialogSelect` echoes any select the handler did not answer as `SM_DIALOG_WINDOW(dialogId, questId)`; `questTalk`/`answered` decide "answered" from what was sent (`dialogReplies`) | A handler sent something for a select Java returns `false` on (double window), or returned without sending for a Java `true` (`c.dialogSilent()`), or sent a movie and Java `return false` (`c.dialogNotHandled()`). Table rows `return false` = echo |
| **Wrong page** | The var/status guard: `questVar(q.Vars, n)`, status START vs REWARD, which npc | Table row with the same dialog id: compare guards. Chain quests: `talkChainDialog` / `talkChainClick` (`game/quest_talk_chain.go`) |
| **Quest does not start / accept does nothing** | Accept dialogs: 1002 (`defaultQuestStartDialog` -> `c.startQuest`), 10000/10001 (custom var change) | `customQuestStart`/`customQuestStartN` (`game/quest_custom.go`); `startQuest` returns silently when `canStartQuest` or the level check fails; item-started quests: `itemStartedQuestDialog` |
| **Does not advance after kill / item / zone / movie** | Registration: `QuestKills[npc]`, item use, `playMovieEnd` (`game/quest.go`), zone entry | `recordQuestKill` (`game/quest_world.go`) dispatches by script id; a new handler needs its line there. Var counters: `questVar` packs five 6-bit counters |
| **No reward / wrong reward / wrong XP or items** | Reward id 8-17 = selectable index, 17 = none. `finishQuestReward` (`game/quest_dialog.go`): `questRewardsFit`, class rewards, `template.Rewards` from `quest_data.xml` | Packet order follows Java `QuestService.questFinish`: kinah, **XP (`SM_STATUPDATE_EXP` or level-up first, then the gained-XP `SM_SYSTEM_MESSAGE`)**, title, AP, expansion, `SM_QUEST_ACCEPTED(COMPLETE)`, nearby quests, level-up quests, THEN window 10. Use `quest-debug.sh compare`: an `exp`/`sysmsg` order difference is real |
| **Quest completes twice / repeats / vanishes** | `CompleteCount` vs `MaxRepeatCount` in `canStartQuest`; `SaveQuest` before mutating `p.quests` | `restore` a snapshot; do not hand-set status in a handler (`c.startQuest` / `c.finishQuestReward` only) |

## 2. Reproduce against the real Java server (only when 0 did not explain it)

```sh
$QD status                 # Go-only stack running? then:
$QD up                     # adds the Java game server + 2 sniffers (about a minute); press Play ONCE in ReRun
$QD sync-java              # only if the test character was created on Go after Java's schema was copied; then $QD up again
$QD prep <char> <id>       # SAME start state on both schemas: snapshot, quest row, prerequisites, level, items, teleport next to the giver
$QD mark j<id>             # (marks both logs)
# client: entry 2 (JAVA): play the quest slowly, Quit properly at the end
$QD log java j<id>
$QD mark g<id>             # no reset needed: Go has its own schema, prep already put it in the same start state
# client: entry 1 (GO): the SAME actions in the same order
$QD log go g<id>
$QD compare j<id> g<id> <id> -last-session    # timeline with a verdict per step + first divergence + the Java handler table
$QD down                   # back to the Go-only stack; press Play
```

To repeat a run, `prep` again (it snapshots first; `restore <char> prep-<id>-go|java` undoes it). `prep` needs only `al19-db` (`container start al19-db` if `status` says it is down). `compare` works from saved logs
when the containers are gone (it says how to bring the stack up when a log is missing). Log out of a server before
entering the other, and never `prep` a character that is logged in.

## 3. Read the diff

`compare` prints one line per client action: `same | differs | missing | extra`, with the Java and Go effects side by
side for the steps that differ, and the first divergence. Then:

- `differs` on `click`: the framework menu rule (row "Click shows a quest page" above).
- `differs` on `select` with `window A | window B`: wrong branch, table row for that dialog id.
- `missing quest ... vars=N`: the Go handler did not advance (guard, registration, or `SaveQuest` failed).
- `exp` / `sysmsg` swapped, `SM_UPDATE_ITEM` counts: reward order or item grant path.
- A step `missing in Go` means the Go run never did that action: usually an earlier divergence stopped the quest;
  fix the first divergence and rerun.
- Bytes inside one packet (dialog ids equal but the packet differs): `pktdiff -quest -norm id,time,coord` on the same two
  logs (see QUEST_DEBUG.md "Running pktdiff").

## 4. Turn the Java run into the regression test (one command)

```sh
go run ./cmd/pktdiff -gen-test <id> [-items <questItemId,...>] <java.log> [label]
go test ./game -run TestQuest<id>JavaReplay
```

Writes `game/quest_<id>_replay_test.go`: the last session of the log, every click / select / movie end / quest kill
replayed through the real dialog framework (`newReplayFixture` in `game/quest_replay_helpers_test.go`), asserting
the windows, quest actions, movies and system messages Java sent. The test fails at the first step Go answers
differently, naming the step. Fix the handler until it passes, keep the file. Looted quest items are only granted
when listed in `-items` (the header names what the run looted). Steps the generator cannot replay (question responses,
item use) are `// TODO` comments with the Java effects; handwrite those in the quest's own test.
