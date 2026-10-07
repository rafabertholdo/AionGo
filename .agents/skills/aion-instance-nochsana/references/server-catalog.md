# Nochsana 1.9 server and client catalog

Read this when inspecting or changing Nochsana code. Paths are relative to the AionGo repository root. The XML/SQL is the current repository's 1.9-targeted data; the installed client is 1.9.0.1. Counts below include the fortress gate added in the current worktree. A data declaration is not proof of scripted execution.

## Identity and source map

| Concern | Source |
| --- | --- |
| Map and zones | `java/AL-Game/data/static_data/world_maps.xml`, `zones/zones_300030000.xml`; map **300030000** |
| Entry | `portals/portal_templates.xml`, `spawns/Npcs/400010000.xml`; Elyos **700413**, Asmodian **700414** |
| Population | `spawns/Instances/300030000.xml` and `spawns/Npcs/300030000.xml`: **21 groups, 134 pool slots, 21 template IDs** in this worktree, including 17 combat mob IDs, gate, artifact, exit and 28 chests |
| NPC stats and skills | `npcs/npc_templates.xml`, `npc_skills/npc_skills.xml`, `skills/skill_templates.xml` |
| Quest and seed drops | `quest_data/quest_data.xml`, `quest_script_data/reshanta.xml`, `java/AL-Game/sql/drops/droplist.sql` |
| Go runtime | `go/game/services_portal.go`, `nochsana.go`, `combat.go`, `summon.go`, `skill.go`, `ai.go`, `instance.go`, `spawns.go`, `quest*.go` |
| Local client identities | Client `Data/npcs/npcs.pak` / `client_npcs.xml` and `Levels/idab1_minicastle/level.pak` / `mission_mission0.xml`. Inspect with `scripts/aion/dark-poeta-evidence.py`'s client reader or the underlying `go/scripts/extract-panel-assets.py` helpers. Never commit extracted archives or machine-specific paths. |

The client calls the map `IDAb1_MiniCastle` and names the General AI `MiBGuard_ChiefC`, gate AI `MiDoor`, artifact AI `NPC_AI_ProtectBuff`, and exit AI `ReturnToEntrance`. Matching a later reference AI name is a lead, not 1.9 mechanic proof.

The inspected client archives have SHA-256 `27e74d0e7aed77e936e585844eec0ae1d44775feca741be8939746e2917ee8b9` (`Data/npcs/npcs.pak`) and `ca553404c104185901d4d4491d6c85307fc08c98d83666b01c2af9f71d8d7b08` (`Levels/idab1_minicastle/level.pak`). Check these before treating a different local client's IDs or markers as the same evidence.

## Spawned combat catalog

`pool` is the number of current XML slots. Two IDs with the same display name are separate variants. Not all objects have a walking route in the server XML. Skill IDs below are distinct declarations after removing duplicate level-1 and level-matched entries.

| ID | NPC | Level/rank | Pool | Distinct skill IDs | Notable declared effects |
| --- | --- | --- | ---: | --- | --- |
| 256677/256678 | Drake | 25 normal | 6+6 | 16516, 16545 | damage, snare |
| 256679/256680 | Recruit | 25 normal | 14+5 | 17475, 17450, 17465 | physical-defense reduction, damage, snare |
| 256681/256682 | Recondo | 25 elite | 15+19 | 16662, 16531, 16616, 16524, 16405 | damage, defense/attack reductions, self defense |
| 256683/256684 | Picket | 25 elite | 6+5 | 16817, 16812, 17003, 16748, 16543; variant 256684 also 16846 | ranged attacks, snare, knockback-named attack |
| 256685/256686 | Runner | 25 elite | 8+9 | 16662, 16555, 17294, 17394, 16554, 16614, 17409 | critical/self buff, spin and defense reductions |
| 256687/256688 | Doc | 25 elite | 3+2 | 17398, 17486, 17362, 17008, 17380, 16728, 17361 | heal, mental cleanse, blind, stun, snare |
| 256689 | Guard | 26 elite | 1 | 17543, 17172, 17226 | fire spell, knockdown-named attack, damage over time |
| 256690 | Protector | 26 elite | 1 | 16632, 16723, 16919, 16592, 17699 | fire, stun, damage over time |
| 256691 | Teleporter | 26 elite | 1 | 17108, 17118, 17113 | ice snare, stun, water spell |
| 256692 | Aetheric Field Guard | 26 elite | 1 | 16630, 17486, 16595, 17487, 17008, 17547 | root, stun, blind, shield |
| 256693 | Nochsana General | 27 hero | 1 | 17303, 16796, 16519, 17290, 17305, 16736, 16684, 16704, 16531 | damage, defense debuffs, shield, knockdown-named attacks, stun, fear |

Distinct skills across these 17 combat IDs: **52**. The generic Go AI selects a random declared skill while attacking; it does not reproduce a scripted 1.9 encounter timeline. The presence of an effect handler proves neither its targeting nor numeric outcome.

## Objects, spells, and progression checks

| Object | Local declaration and expected test |
| --- | --- |
| Gate **256694** | Level 27 HERO, template max HP **74,152**, `USEITEM`, `DRAGON_CASTLE_DOOR`. Client marker and current spawn at `(346.237,356.81699,379.36295)`. Client cursor says `action`; check whether the client sends attack, siege summon cast, or both. Current Go combat special-cases gate damage in this map; current summon logic recognizes siege weapons in the same run. Destroying it does **not** yet establish reinforcement spawning, collision clearing, or quest credit. |
| Artifact **700437** | At `(312.7441,596.1464,373.5324)`, client mesh `artifact_Dragon_Nohsana`. NPC skill list declares **1872 Shield of Compassion** (3,000-point shield, 60 seconds, area 25 m, maxcount 6). Current Go timed click selects nearby players. Verify full-party target cap and reuse; the current selection starts with the clicker and may count six *additional* players, which needs a focused six-person check. |
| Quest siege items | Elyos **182202179** casts **9953**, spawning **201054** with attack skill **18008**. Asmodian **182205676** casts **9954**, spawning **201055** with attack skill **17814**. The server's ordinary summon-stats catalog lacks these NPC rows; the current Go worktree has a template-stats fallback, but a real item-use and gate-target packet comparison remains outstanding. The 1.9 XML omits the door bonus: the 4.6 `skill_base` gives **18008/17814** `reserved10=19900` for `_Race_PC_Light_Castle_door`, `_Race_PC_Dark_Castle_door` and `_Race_Dragon_Castle_Door`. Go adds it to the blow (`siegeDoorBonus`), about 20.7k or 27% of the gate per hit, because npc physical damage ignores the skill value. |
| Exit **700438** | At `(466.89075,708.46313,346.6602)`, client `IDAB1_MiniCastle_exit`. The Go handler returns to the appropriate Lower Abyss entrance. Verify on client; the original server spawn had an Indratu portal ID at this position. |
| Chests | Current `spawns/Npcs/300030000.xml` has **28 × 700419**, while quest-drop metadata and client NPC identity point to Nochsana weapons chest **700415**. Both use an old jewelbox mesh, so visual similarity is not enough to resolve object identity. Capture click/loot packets before a bulk replacement. |

General **256693** has nine distinct skill IDs. The local XML includes fear **16704** (declared 3-second effect), defense-reducing hits **16796/16531**, knockdown-named **17305/16736**, stun **16684**, and **17290** with a 10-second shield declaration whose `percent=true` and large value require skill-engine validation. The 2009 guide reports fear below about 30%; do not substitute the 4.6 `MiBGuard_ChiefC` thresholds as 1.9 truth. The later AI has timer/health branches and the later `MiDoor` spawns two adds; period guides corroborate a gate response in broad terms, not exact timings or IDs.

## Current acceptance gaps

- No 1.9 packet/client proof of portal attunement, lockout, exact artifact target cap, door collision, siege attack cursor behavior, two gate reinforcements, authored patrols, linked boss-room aggro, General phase timing, wipe/reset, or repeat entry.
- Java `quest_script_data/reshanta.xml` marks **3702/4702** TODO. A preset can place START rows and siege items in a character's inventory, but that does not implement stage progression or reward credit. The weapons-cache object mismatch can block **3704/4704**.
- Seed drops and generic AI operate, but eligibility, party credit, and all named-elite scripts require an observed 1.9 run. Keep independent tests for two runs and both factions; avoid treating a GM solo run as normal group admission.
