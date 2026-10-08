# Aion 1.9 dungeon source inventory

Recorded 2026-10-05 for D0 of the
[dungeon correctness plan](DUNGEON_CORRECTNESS_PLAN.md).
The machine-readable [manifest](AION_19_DUNGEON_INVENTORY.json) records all
37 maps marked `instance="true"`, 21 candidate group-dungeon maps and six
priority dungeons in detail. It includes source hashes, portal attributes and
actual spawn locations, declared entry items, spawn coordinates/pools,
NPC templates, skill references, seeded drops and quest candidates.

**Evidence level: inspected source/data, not independently verified retail
1.9 behavior.** No gameplay code changed, no client session was recorded and
no live database or server was modified. Map declarations, levels up to 55,
later map names and SQL updates show that this catalog must not be accepted
as a pure 1.9 content specification. Version approval is pending for every
candidate. Later/unversioned and small quest instances are quarantined below.

This is the pre-correction source snapshot. The draft corrections are now
being validated in AionGo; catalog counts and hashes below describe the original
snapshot. Dungeon gameplay acceptance remains pending.

## Method and boundaries

Paths below are relative to AionGo unless explicitly identified otherwise.

1. Parse `java/AL-Game/data/static_data/world_maps.xml` and
   `portals/portal_templates.xml`; keep declared return `entrypoint` and arrival
   `exitpoint` distinct. A return point does not prove an entrance NPC exists.
2. Read **all** XML under `spawns/`, not only `spawns/Instances/`. Join on the
   spawn's `map` attribute, not its filename. This matters for NPCs and portals
   stored under `spawns/Npcs/` and for small quest instances.
3. Calculate declared pools and `min(pool, object-count)` totals, matching the
   loader's clamp in `go/game/data/world.go`. These are catalog counts, not
   runtime population: Dark Poeta holds six templates until progression;
   scripts can spawn further objects and missing templates can suppress spawns.
4. Join NPC templates in sorted file order, matching the loader's last-write
   behavior, and join NPC skill lists to skill IDs. Template names are server
   labels, not certified client/localization names. HERO rank identifies
   encounter candidates, including objects/chests; it does not prove boss status.
5. Join `docker/db/sql/3-droplist.sql` by mob ID. Count and retain seed rows;
   these are neither live-database drops nor approved probabilities. Quest
   drops/rewards come separately from `quest_data/quest_data.xml`.
6. Scan all Java quest handlers and XML quest scripts for map/spawn NPC IDs.
   Record matches as lexical candidates; inspect event branches before claiming
   dungeon objectives. For example, a quest can reference an entrance NPC or
   share a monster with an unrelated area. Explicitly inspect named entry quests
   even when their referenced NPC is absent from current dungeon spawns.
7. Record SHA-256 for parsed data and all scanned Java quest handlers. The
   manifest identifies its source snapshot; it does not attest server deployment.

No handler-tagged spawn groups were found on the 37 marked instance maps.
The general `spawnMap` handler exclusion is therefore not an identified missing
spawn cause for these maps in this snapshot. No missing NPC templates were found
for their non-gatherable spawn IDs. In the six priority maps, every referenced
NPC skill ID exists in the skill catalog; presence does not establish that its
scripted effects, targeting or phase timing work. No dungeon-specific door
catalog or reset/cooldown specification was established.

## Priority dungeon manifests

### Nochsana Training Camp — 300030000

- Admission: ordinary party, levels **25–28** in current XML. Elyos portal
  **700413** and Asmodian **700414** are actually spawned in map 400010000.
  Arrival is `(513, 668, 331)`; faction-specific return points are recorded in
  the manifest. No item cost or quest prerequisite is declared by these portals.
- Population: **20 groups / 133 clamped pool slots / 20 templates**, including
  NPC groups outside the Instances folder. General **256693**, level 27 HERO,
  has **18 skill declarations and 36 seed-drop rows**.
- Proposed ordinary route: eligible party enters its faction portal → reaches
  the General and any independently established required objectives → defeats
  him → receives eligible loot/quest credit → exits or recovers according to
  verified rules. Objective order, fortress gate/shield conditions, combat phases,
  lockout and ordinary exit are **unknown**, not assumed optional.
- Concrete data mismatch: `spawns/Npcs/300030000.xml` places portal **700565**
  at `(466.89075, 708.46313, 346.6602)`, static ID 14. Its template is the
  **Indratu Fortress** gate, and its portal destination is **310090000**.
  Establish the correct Nochsana exit from 1.9 evidence before replacing it.
- Rewards: ten spawned templates have seed drops; no matching quest-drop
  metadata or XML quest-script match was found. A lexical custom-quest match
  is not evidence of Nochsana credit. General-specific eligibility and rewards
  remain unverified.

### Dark Poeta — 300040000

- Admission: ordinary party, **50–55** in XML. Actual external entrance spawns:
  **730185** in Beluslan/220040000 (Asmodians), **730186** in Heiron/210040000
  (Elyos), **730215** in Abyss/400010000 (no race restriction declared).
  Arrival is `(1225.2749, 419.43173, 140.18988)`.
- Declared costs: faction entrances list item **186000048 ×20** and
  **186000050 ×1**; Abyss entrance lists **186000048 ×5** and **186000050 ×1**.
  Go `data.Portal` does not decode `portalitem`; the inspected Go admission
  path has no consumption. Java exposes `getPortalItem()` but its inspected
  controller does not call it either. This is an observed data/implementation
  gap in both references, **not yet an approved 1.9 consumption rule**.
- Population: **145 groups / 625 catalog pool slots / 143 templates**.
  `darkPoetaHeld` suppresses Anuhart and five rank-boss templates on initial
  Go spawn. Catalog counts include those deferred groups.
- Implemented route: preparation → start hook → score/kill/gather events →
  generators **214895/214896/214897** → Anuhart **214904** → rank boss
  **215280/215281/215282/215283/215284** → room/exit gate. These transitions
  come from current Go, with later-reference comments; verify each against 1.9.
- Encounter candidates include Atmach, three Marabata, Noah, Spallers,
  Telepathy Controller, protection devices, generators and rank bosses. Anuhart
  has **9 skill declarations / 22 seed-drop rows**, Tahabata **10 / 37**.
  Adds, device dependencies, room access, phase scripts and wipe resets remain
  unknown. Skill declarations do not encode the complete encounter.
- Door/timer gaps: any static-door request reaches the start hook; generator
  progression is a count, gathering is not state-guarded, and no scheduled
  expiry transition exists in `darkpoeta.go`. Record reproductions and independent
  expected behavior before fixing these source audit targets.
- Portal **730211** has a same-map room destination and is spawned in multiple
  places, including an external Abyss placement. Go's exit workaround uses
  the gate's proximity to the boss-room coordinates. Verify every placement and
  that fallback return selection chooses an appropriate external entrance.
- Rewards: **110** spawned templates have seed drops; **13** quest metadata
  entries have matching drop NPCs. IDs and rewards are retained in the manifest.
  XML report-to **4920** matches an NPC, not an encounter objective. Officer
  bonus, exact score/time boundaries, loot and one-time completion need 1.9 evidence.

### Fire Temple — 320100000

- Admission: portal **730069**, ordinary party, **30–38** in XML. Return points
  exist for Eltnen/210020000 and Morheim/220020000. NPC **730069** is spawned
  only in Eltnen; the faction-specific Asmodian entrance **730047** is spawned
  in Morheim but has no portal template. The earlier same-ID search missed
  this entrance. Arrival `(148.256, 460.857, 141.713)`. The 1.9 client declares
  minimum level **30 Elyos / 27 Asmodians**; the XML maximum 38 needs validation.
- Population: **29 groups / 215 slots / 29 templates**. Spawned HERO
  **214621**, Vile Judge Kromede, has **5 skill declarations / 23 seed-drop rows**.
- Proposed route: establish the faction-specific entrance and any access quest →
  eligible party enters → navigates to the correct Kromede variant and other
  required encounters → obtains credit/loot → leaves. Alternate boss selection,
  keys, wipe and lockout rules are unknown; the ordinary exit correction is
  recorded below.
- Exit correction (2026-10-07): **730048**, `DF2_DragonDoor_Out`, is now
  spawned beside the arrival point at `(148.854584, 465.935089, 142.0)`,
  heading 0, static ID 647. The 1.9 client declares `ReturnToEntrance` AI;
  the 4.6 `iddf2_dflame/world.xml` territory
  `DF2_DFlame_FObj_DF2_DragonDoor_Out` supplies this placement. Go uses the
  existing three-second interaction and faction-specific instance return to
  Eltnen or Morheim, with delayed distance/object/run revalidation. The added
  regression covers spawn visibility from arrival, both returns, and moving
  away during use. Placement and interaction still need real 1.9 client QA;
  the historical population snapshot above predates this additional spawn.
- Explicit quest evidence: Java **1355**, The Fire Temple Key, handles item
  **182201400** and NPC **203933**; no corresponding quest check is present
  in the generic portal admission. Quest **1470**, Hannet's Vengeance,
  registers kills of **212846**, Kromede the Corrupt; that template exists but
  has **no static spawn anywhere in the inspected spawn tree**. The Fire Temple
  catalog instead spawns **214621**. This is a concrete catalog/quest-ID mismatch;
  do not alias IDs or swap the boss without version-specific evidence.
  The 4.6 spawn territory `DF2_C5Dg_F_FireSanctuaryQueenBoss_37_1` resolves
  it: group G1 (Kromede the Corrupt 212846, default) at select_prob 900 and
  G2 (Vile Judge Kromede 214621) at 100, same spot. Go rolls that 90/10 when
  the run spawns (`go/game/firetemple.go`).
- Mob AI: every spawned template has the same AI name in the 1.9 client and
  4.6 data, and 27 of 29 have identical skill lists. Go runs Kromede's
  `ND2_Sum_B` (knockback opening, trap 280501 bursts every 35 s / 25 s below
  half HP, last stand below 20%) and the `ND2_AnN` obscuras' once-a-life flee
  below half HP. Other mobs use generic rotations approximated by random casts.
- Three quest metadata entries have matching drop NPCs; XML monster-hunt **2374**
  also matches current dungeon NPCs. The generated manifest contains their IDs.
  Quest/loot existence does not establish that the boss variant selection works.

### Aetherogenetics Lab — 310050000

- Mob AI: all 37 spawned templates have the same AI name in the 1.9 client and
  4.6 data. 4.6 added skills to most Lepharist lists, so their pattern skill
  slots do not map onto 1.9 lists; Go runs only the patterns' structural
  actions (`go/game/aetherlab.go`): Lepharists flee their target for 5 s once
  below 35% HP and call allies within 10 m; scholars (NLehpar_WeA) summon
  pretor 280262 once between 36% and 70%; snipers (XLehpar_ReB_S1) drop
  ice-snare trap 280466 and flee 4 s; Perfected Pretor 212205 leaves Perfected
  Mudthorn 212206 for 600 s; RM-78c 212211 leaves burst creature 280790.
  The seven named mobs (Perfected Pretor, Pretor Key Keeper, Keykeeper, Key
  Eater, Head Chef Pamsey, RM-108c, RM-78c) run their full 4.6 timer patterns,
  skill slots matched by name to their 1.9 lists; the three 4.6-only skills
  (RM-108c/RM-78c idle poison stance, RM-78c self buff) are skipped.

### Adma Stronghold — 320130000

- Admission: spawned portal **730165** in Brusthonin/220050000, Asmodians,
  ordinary party, **46–55** in XML; arrival `(450.66864, 199.70248, 167.84444)`.
- Population: **60 groups / 503 slots / 60 templates**. Princess Karemiwen
  **214695** has **6 skills / 0 seed-drop rows**; Lord Lannok **214696** has
  **10 / 1**. Zero seed rows is a reward audit question, not proof of no retail loot.
- Proposed route: eligible Asmodian party enters → resolves verified key/gate
  dependencies and encounters → Lannok completion → credit/loot → verified exit.
  Key ownership, door graph, boss prerequisites, timed events, wipe/reset and
  ordinary exit are unknown. No internal portal template matched its spawn IDs.
- Quest **2094**, The Secret of Adma Stronghold, registers **214700** and
  requires completed quests **2092/2093/2054** to start via its level-up path.
  Its later spawn action is explicitly in **220050000**, outside the dungeon.
  These are quest conditions, **not proven universal dungeon entry conditions**.
- XML monster-hunt **4093** targets Lannok. Seven matching quest-drop entries
  and 38 templates with seed drops are recorded; validate party eligibility and
  objective completion independently.

### Steel Rake — 300100000

- **No static spawn groups, no portal-template entrance, no internal portal
  template and no in-map encounter population** were found across the entire
  loaded spawn tree. A map declaration does not provide a functioning dungeon.
- Java quests **3200**, Price of Goodwill, and **4200**, A Suspicious Call,
  explicitly create/register a player instance and teleport into a cell at
  `(403.55, 508.11, 885.77)`. Go `quest_4200.go` and translated dialogs also
  create this instance. Quest handlers reference Haorunerk **798332** and his
  bag **700522**, with bag-use coordinates in comments; neither NPC has a
  static spawn anywhere in the inspected tree.
- Proposed ordinary route: use the appropriate faction quest's verified dialogue
  and prerequisites → cell arrival → Haorunerk/bag interaction → scroll use →
  quest return. This route is **blocked by missing static population evidence**;
  inspect all dynamic quest hooks before deciding a spawn fix is necessary.
  The group dungeon's normal entrance, deck progression, bosses, keys, rewards
  and exit are unknown and separate from the solo entry quest.
- Do not populate Steel Rake from 4.6 IDs/coordinates as a shortcut. Obtain
  version-approved 1.9 spawns, encounter scripts, drop/quest relationships and
  map/door identities before implementation.

### Dredgion — 300110000

- **No portal-template entrance**. Admission may require queue/registration
  behavior; a map enum, admin teleport or direct instance creation does not
  establish ordinary matchmaking. No dedicated Dredgion Go identifier/map-ID
  implementation was found in the bounded production-Go search.
- Population: **37 groups / 138 slots / 37 templates**. Captain Adhati
  **214823** has **4 skill declarations / 0 seed-drop rows**; First Mate
  Aznaya **215086** has **7 / 16**, Auditor Nirshaka **215390** has **7 / 5**.
  Spawned attackable objects include cabin power Surkana **700498** and
  starboard cabin door **700504**. Full skills, coordinates and seed rows are
  retained in the manifest.
- Proposed ordinary route: establish 1.9 queue/schedule/admission → assign the
  two teams to one match → objectives and PvP/PvE scoring → captain or timeout
  result → eligible rewards and exit. Every transition beyond static population
  is currently an evidence gap; do not label a GM visit a verified route.
- Four matching quest-drop entries and XML hunt **3714** are recorded.
  No scoring, timed match end, matchmaking, reward or lockout specification was
  established. Do not infer result rewards from the Captain's zero seed drops.
- The 4.6 base `World_IDDre` functions recorded in the reverse-engineering handoff
  are useful objective/death/end leads; later variants and ratings stay excluded.

## Findings to carry into D1–D7

| Finding | Classification now | Next verification |
| --- | --- | --- |
| Dark Poeta portal-item declarations are unenforced in inspected Go and Java admission | Shared reference limitation / 1.9 rule unresolved | Establish required items, timing and re-entry consumption, then regression tests for atomic admission. |
| Nochsana contains a gate configured for Indratu | Data inconsistency, observable destination not client-tested | Capture the intended exit and inspect gate/static-ID identity before correcting shared data. |
| Fire Temple Asmodian entrance 730047 is spawned but lacks a portal template | Admission data gap | Draft template prepared using existing return/arrival; runtime verification pending. |
| Fire Temple kill quest 1470 targets unspawned 212846 rather than spawned 214621 | Data/quest relationship gap | Establish boss variants and kill-credit rule; do not broad-match by name. |
| Steel Rake has empty static population despite quest entry code | Missing catalog evidence; dynamic-path audit pending | Trace dynamic spawns and obtain approved 1.9 population before scheduling boss work. |
| Dredgion has population without demonstrated ordinary admission/results | Missing gameplay evidence / implementation audit pending | Trace request handlers/queue and base 4.6 event leads, preserving 1.9 protocol. |
| Adma Princess and Dredgion Captain have no seed drop rows | Reward catalog question | Inspect seed history and version-approved rewards; live data was not queried. |
| Fortress maps Right Wing and Kysis have population but no portal-template entrance | Additional admission inventory gap | Audit fortress ownership access and alternate entry handlers. |

First implementation candidate after D1 evidence collection: shared admission
validation, especially delayed portal use, correct run binding, and any approved
entry costs. Dark Poeta remains the first end-to-end verification target.
Steel Rake should not be treated as merely a boss-AI task with ready static data.

## Remaining version and mechanics evidence

For **every** candidate dungeon, still establish: retail 1.9 inclusion and
faction access; ordinary entry prerequisites and lockout; encounter/door graph;
phase/add and leash/wipe rules; completion/reward eligibility; ordinary exit;
and reconnect/restart policy. The manifest gives a source or explicit unknown
for each category. No independent dungeon recording was identified in the
bounded name search of existing Go text fixtures; absence from that search does
not prove that no capture exists elsewhere.

The 4.6 main-server functions guide lifecycle/door/event investigation. Full
boss correctness still needs NPC-server/script-DLL analysis or other credible
1.9 evidence. Retain Go 1.9 packet/layout conventions, and distinguish Java 21
parity from client compatibility and retail mechanics.

## Full marked-instance catalog

The portal column counts templates arriving in the map, including internal
portals. Pool totals include deferred spawns. “Group candidate” is scope for
this audit, not independent approval of 1.9 content. The 16 quarantined maps
include quest instances and unversioned content; they are not all later dungeons.

| Map | Source name | Scope | Groups / pool slots | Arrival portal templates |
| --- | --- | --- | ---: | ---: |
| 300030000 | NochsanaTrainingCamp | Group candidate | 20 / 133 | 2 |
| 300040000 | DarkPoeta | Group candidate | 145 / 625 | 4 |
| 300050000 | AstreiaChamber | Group candidate | 13 / 48 | 1 |
| 300060000 | SulfurTreeNest | Group candidate | 9 / 95 | 1 |
| 300070000 | ChamberOfRoah | Group candidate | 8 / 75 | 1 |
| 300080000 | LeftWingChamber | Group candidate | 9 / 100 | 1 |
| 300090000 | RightWingChamber | Group candidate | 11 / 236 | 0 |
| 300100000 | SteelRake | Group candidate | 0 / 0 | 0 |
| 300110000 | Dredgion | Group candidate | 37 / 138 | 0 |
| 300120000 | KysisChamber | Group candidate | 59 / 199 | 0 |
| 300130000 | MirenChamber | Group candidate | 66 / 206 | 1 |
| 300140000 | KrotanChamber | Group candidate | 60 / 208 | 1 |
| 300150000 | IDTemple_Up | Quarantined | 0 / 0 | 0 |
| 300160000 | IDTemple_Low | Quarantined | 0 / 0 | 0 |
| 300170000 | IDCatacombs | Quarantined | 0 / 0 | 0 |
| 300190000 | IDElim | Quarantined | 0 / 0 | 0 |
| 300200000 | IDNovice | Quarantined | 0 / 0 | 0 |
| 300210000 | IDDreadgion_02 | Quarantined | 0 / 0 | 0 |
| 300220000 | IDAbRe_Core | Quarantined | 0 / 0 | 0 |
| 310010000 | IDAbProL1 | Quarantined | 19 / 49 | 1 |
| 310020000 | IDAbProL2 | Quarantined | 1 / 1 | 0 |
| 310030000 | IDAbGateL1 | Quarantined | 5 / 10 | 0 |
| 310040000 | IDAbGateL2 | Quarantined | 1 / 1 | 0 |
| 310050000 | IDLF3Lp | Group candidate | 37 / 78 | 1 |
| 310060000 | IDLF1B | Quarantined | 3 / 3 | 0 |
| 310070000 | IDLF1B_Stigma | Quarantined | 2 / 2 | 1 |
| 310090000 | IDLF3_Castle_indratoo | Group candidate | 43 / 268 | 1 |
| 310100000 | IDLF3_Castle_Lehpar | Group candidate | 30 / 182 | 1 |
| 310110000 | TheobomosLab | Group candidate | 36 / 337 | 1 |
| 320010000 | IDAbProD1 | Quarantined | 19 / 47 | 1 |
| 320020000 | IDAbProD2 | Quarantined | 1 / 1 | 0 |
| 320050000 | IDDF2Flying | Group candidate | 23 / 251 | 3 |
| 320070000 | IDSpace | Quarantined | 1 / 1 | 1 |
| 320080000 | IDDF3_Dragon | Group candidate | 97 / 345 | 1 |
| 320100000 | IDDF2_Dflame | Group candidate | 29 / 215 | 1 |
| 320110000 | IDDF3_LP | Group candidate | 39 / 153 | 1 |
| 320130000 | AdmaStronghold | Group candidate | 60 / 503 | 1 |

The JSON gives portal requirements/locations, encounter candidates, quest
matches and explicit route unknowns for every row. For additional group
dungeons, start at a listed external spawned portal if one exists, verify
objectives/encounters in the manifest, and establish completion and return;
this is a proposed test route, not an assertion of working mechanics.

### Follow-up: player-reported Dark Poeta issues

The October 5 audit records Huge Vine/Scar, one-damage barricades, Marabata
controls, tree/Spaller patrols, mine access, generator placement/models, and
the Heiron entrance artifact in [the correction handoff](aion-dungeon-fixes/README.md).
Independent 1.9 client evidence confirms Huge Vine **401111**, Essencetapping
**300**, and a missing spawn. Local 4.6 AI/world data supplies escort, patrol,
core-spawn and bomb-wall leads; its quantitative values remain version-qualified.
The source JSON above remains the original inventory snapshot. The corrections are applied in AionGo and automated validation is recorded
in the handoff. No dungeon gameplay completion status has changed.
