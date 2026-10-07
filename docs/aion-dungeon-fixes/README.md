# Dungeon corrections: implementation handoff

**2026-10-05 — implementation and validation in progress.** The draft has
been applied to AionGo, preserving the original quest-event tests. See the
canonical [dungeon plan](../DUNGEON_CORRECTNESS_PLAN.md) and
[port status](../../go/PORTING.md). This document separates source corrections,
automated validation and outstanding 1.9 gameplay acceptance.

## Applied draft

The original draft added portal object validation and delayed-transfer
revalidation; external, race-compatible instance returns; the Nochsana exit;
the missing Asmodian Fire Temple entrance; hard Kromede quest 1470 credit;
Dark Poeta timer expiry, death deduplication, unique generator objectives,
active-run gathering credit; and the Huge Vine spawn.

The vine spawn does not implement Scar's escort. Neither source edits nor
passing automated tests alone establish dungeon gameplay acceptance.

## Dark Poeta reports and evidence

The player reports below are accepted reproduction targets. This table states
what was actually established and what still needs implementation.

| Report | Established evidence | Next correction / acceptance |
|---|---|---|
| Wounded Scar cannot be released | 1.9 Huge Vine **401111**, mesh `common/ska_preventplant`, requires **300 Essencetapping**, one harvest; client marker `(663.97913,1169.7542,143.60455)`. Template existed; the initial audit found no spawn. Scar **214871** has no server walker. 4.6 `IDLF1_Sca_NoAction` has ambush indices **7,14,24,25**, sanctuary-release continuation and Mad Scar replacement at **32**; `IDLF1_E_Path_SKA_50` has 42 points. | Vine spawn and gathering regression tests are applied. Establish release trigger ordering, derive 1.9 ground-safe escort points, implement paused ambush/resume, final Mad Scar replacement, death/reset and per-instance isolation. Test skill 299 rejection / 300 acceptance and duplicate harvest. Do not substitute a talk trigger from the alternate `IDLF1_Sca` AI without tracing. |
| Gates should receive one damage per hit | 1.9 barricades **700517,700556,700558** have attack cursors. 4.6 records declare **120 HP**, `physical_damage_trim=1`, `magical_damage_trim=1`; current Go damage path lacks those trims. | One-damage behavior is applied across normal attacks, skill damage and periodic damage; reported damage, aggro and HP must agree. Preserve misses/zero damage. Verify gate identities against the reported location before expanding scope. 120 HP is reference-version data, not independently established 1.9 health. |
| Marabata devices die too easily | Current 1.9 server templates give attack/property devices **5094 HP**, defense devices **7641 HP**, generic defense **85**. 1.9 identifies **700439–700447** as `ND2_WhHS1/2/3`. Extracted AI applies a wake-up skill, sends boss/control messages and spawns replacement switches; 4.6 HP/regen differ from this server. | Restore device/boss links, initial buffs, control replacement and reset before tuning toughness. Derive 1.9 health/regen/resistance; do not import a blanket 4.6 multiplier. |
| Trees and giant bugs do not patrol | Five 1.9 boss identities match reference world routes: **214864/215387** Ghost Elim and **214880/215388/215389** Spaller. Their server placements have no walker/rw. Reference paths contain **28/29** tree points and **17/16/13** bug points. | Validate path coordinates against 1.9 ground/collision, attach walkers, interrupt/resume around combat and return/reset. Random roaming is insufficient for authored patrols. |
| Hidden area after the mine entrance is inaccessible | 1.9 **700516** is `IDLF1_BombWall`, mesh `hugo_rock`, cursor none. Reference AI despawns it on `Q_IDLF1_BrownieBomb` in `on_spelled` **or** `on_see_spell`. Existing 1.9 skill **18130** has that name and is used by an item template. | Completed bomb casts now remove the nearby wall in the current run. Trace bomb acquisition/use, verify client collision/visual access and reset. Confirm this is the player's blocked side area. Do not turn it into an unrestricted clickable teleport. |
| Generator models and floating generator | 1.9 housings **214895–214897** use `SpaceDynamo`; cores **214898–214903** use `DynamoNucleus`. Three different housing AIs spawn cores along distinct paths. Go NPC info emits template ID twice, matching Java's packet layout. Client housing markers have Z **123.2962**; server housing Z is **120.63396 /121.82052 /124.2962**. | Audit actual spawned identity/visual state, housing/core phases and terrain contacts. Tertiary generator **214897** is a concrete floating-position suspect. Client marker height is not guaranteed ground height; no blanket Z replacement is drafted. |
| Observation-post entrance fragment/artifact looks wrong | Elyos **730186**, `LF3_Artifact_IDLF1In`, resolves to `NPC/Level_Object/artifact/artifact_light`; Asmodian **730185** uses the dark artifact. | Verify actual live NPC ID, static placement, packet appearance and client assets. If the report means another fragment object, identify it by location. No model substitution is established yet. |

[`dark-poeta-evidence.json`](dark-poeta-evidence.json) records local client
identities/markers, selected reference AI/world/routes/stats and SHA-256 source
hashes. [`client-evidence.json`](client-evidence.json) contains the earlier
Fire Temple, Nochsana and Steel Rake observations. Source keys identify logical inputs; local archive and workspace paths are
kept outside this repository. Reference 4.6 numeric
values and editor Z coordinates remain explicitly version-qualified.

Regenerate Dark Poeta evidence from this repository with local inputs:

```sh
python3 scripts/aion/dark-poeta-evidence.py \
  --client "<client-root>" \
  --reference "<reference-map-root>" \
  --output docs/aion-dungeon-fixes/dark-poeta-evidence.json
python3 -m unittest discover -s scripts/aion/tests -v
```

## Nochsana follow-up (2026-10-06)

The 1.9 client NPC catalog identifies fortress gate **256694** as
`Mini_Castle_Door_Dr` and artifact **700437** as
`IDAB1_MiniCastle_Artifact_Nohsana`. The client camp mission places its door
marker at `(346.237, 356.81699, 379.36295)`, matching the later reference
world's door position. The gate now spawns there in each run, accepts damage,
and stays dead for that instance. The artifact now performs its three-second
use and applies the declared skill **1872** shield to nearby players in the
same run. The existing abyss exit **700438** returns each faction to its
outside entrance.

All 17 spawned combat templates from **256677–256693** have skill lists; their
52 distinct skill IDs resolve to templates, and every declared effect type has
a Go handler. This confirms catalog coverage, not encounter timing. The 1.9
client identifies the General's AI as `MiBGuard_ChiefC`; the later reference
has health-triggered skills and battle timers, and the gate's `MiDoor` AI
spawns two reinforcements on death. Those scripts have not been independently
validated for 1.9, so the Go server still uses generic mob skill selection and
does not spawn gate-death reinforcements. General phases, linked aggro, patrols,
gate collision, quest **3702/4702** credit, loot eligibility, wipe recovery and
full client completion require a 1.9 gameplay comparison.

The focused Nochsana checks passed. The whole Go suite passed with **4,636
passed, 0 failed, 16 skipped** test nodes; vet passed. The Java Maven reactor
passed **5 tests, 0 failures, 0 errors, 0 skipped**. These checks do not
establish in-client collision or scripted encounter parity.

## Remaining original scope

Steel Rake still needs an approved conditional population/event manifest;
675 client markers are insufficient to spawn every variant unconditionally.
Portal item requirements still need atomic admission/rollback semantics.
Adma drop questions need 1.9 loot evidence. Dredgion still requires admission,
teams, objectives, scores, ending and reward implementation. Dungeon gate
progression, wipe/reset, reconnect/group changes and lifecycle tests remain.

The original draft and private-path evidence were migrated from the preparation
repository. Existing quest-event tests were retained; their replacement in the
draft was not adopted. The evidence extractor now records logical source names
and runs from this repository. The website remains in its owning repository
and follows the canonical documents here.

## Additional source corrections

- Barricades **700517/700556/700558** cap positive normal, skill and periodic
  damage at one per hit. Avoidance and shields preserve zero damage. Multi-hit
  attacks remain separate at the HP/aggro hook; packet damage and HP agree.
- Successful item **164000096** / spell **18130** casts remove nearby mine wall
  **700516**, within seven metres and the same instance. Cancelled casts and
  duplicate completion cannot remove another wall. Natural bomb acquisition,
  the exact reported side area and client collision access remain unverified.
- Huge Vine's gathering path rejects depleted/stale identities and cross-run
  use. Regression coverage exercises skill 299 rejection, 300 acceptance,
  concurrent duplicate use and a stale request after its one harvest.
- NPC despawn now immediately cancels AI, walking, casts, effects and talk
  tasks, including dead NPCs. Removal happens before cleanup so losing the last
  observer cannot restart an AI task during instance destruction.

## Remaining Dark Poeta acceptance

Scar escort is not implemented: the matching 1.9 AI name and vine marker do
not yet establish the release event ordering or ground-safe escort segments.
The reference includes paused ambushes and sanctuary release; a talk shortcut
would select a different AI. Boss patrol routes and generator positions need
1.9 collision/ground validation. The client terrain archive is available but
its heightmap alone does not establish walkable surfaces on structures.

Marabata controls need boss/device messaging, initial protection, replacements
and reset. The current server skills 18553–18555 all contain zero-value
reflector effects and the generic reflector handler does not apply protection;
this is a concrete source gap, not evidence for a guessed health multiplier.
Generator core phases and entrance artifact appearance also remain unverified.
No model, blanket Z, health or regeneration substitution was made.

## Validation

- Final uncached whole Go suite (`go test -json -count=1 ./...`):
  **4,631 passed, 0 failed, 16 skipped** test nodes, including subtests;
  **11 passing packages, 0 failing packages, 6 packages without tests**.
  Every skipped node requires database fixtures; no database integration
  coverage is claimed.
- Final focused race checks repeated five times: **170 passed, 0 failed,
  0 skipped**, with no race reports. This includes Dark Poeta, portal transfer,
  Nochsana exit, hard Kromede and the corrected bottle-use timer fixture.
- Expanded focused dungeon run: **39 passed, 0 failed, 0 skipped**.
  Focused multi-hit/cleanup/exit/vine checks: **16 passed, 0 failed, 0 skipped**.
- Python image and dungeon evidence/data tests: **26 passed, 0 failed,
  0 skipped**. Go script tooling tests: **14 passed, 0 failed, 0 skipped**.
- Java Maven reactor: **5 passed, 0 failed, 0 errors, 0 skipped**.
- Website build/tests: **8 passed, 0 failed, 0 skipped**; roadmap sync
  **16 products / 10 source files**. Source hashes are refreshed after this handoff.
- Final `go vet ./...`, `go build ./cmd/...` and repository-wide
  `gofmt -l .`: **passed**.
- Local-only ARM64 game image: **built and revision label verified**,
  `aiongo-game-local:0.1.15-dungeons-f76a90a2ddf9`. The label identifies the worktree content
  as `worktree-sha256:f76a90a2ddf9b50cf99cb26f04e8aa69068a1d9a46beb02318a18cd81f335cd8`.
  Initial image attempts used the Apple builder's stale context and could not
  see new helper files; resetting only its disposable build container fixed
  the context, and the unchanged canonical Dockerfile then built successfully.
- Development failures were corrected and retained here as evidence:
  the initial Kromede fixture used the wrong quest setup; deterministic exit
  checks exposed surviving NPC AI tasks; the draft expiry fixture accessed
  run state without synchronization (**32 pass / 1 fail** on its first race run).
  An intermediate full suite had **4,630 pass / 1 fail / 16 skip** when existing
  quest 1039's bottle test asserted before its timer completed. That fixture
  now uses `synctest`; its first five-repeat race attempt (**169 pass / 1 fail**)
  also exposed an unlocked animation assertion, corrected before the final
  five-repeat race and whole-suite passing runs above. No production quest 1039
  behavior changed.
- One early whole-suite attempt was interrupted for a final lock correction;
  it is not counted as a passing run.
- Actual 1.9 client gameplay/collision/appearance and disposable database
  admission/reward checks: **not run**. No deployment or image publication.

Historical draft-only checks are not acceptance of this implementation.
