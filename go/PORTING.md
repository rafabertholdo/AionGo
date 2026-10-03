# Porting Aion Lightning 1.9 to Go

The goal is every server in Go, speaking the same protocols and using the same
database and `AL-Game/data`, so the 1.9 client and existing characters work
unchanged. Automated tests cover broad behavior; representative flows must
also be checked with the real client and old 1.9 server recordings.

## Done

- **Login server** (`login/`, `cmd/loginserver`) — all of AL-Login: client
  login with its Blowfish/RSA crypto, server list, play, reconnect; game server
  registration, account auth, kicks, access level, bans; account time.
- **Chat server** (`chat/`, `cmd/chatserver`) — all of AL-CServer, except that
  a message now reaches only its own channel, not every channel of its kind.
- The login server lets a restarted game server register again at once: a
  killed container never closes its old connection, which AL-Login would
  hold on to until TCP gave up.

## Game server (`game/`, `cmd/gameserver`)

The local development stack runs the Go game server; Java remains the behavior
reference until Go reaches parity.
Packets are checked byte for byte against AL-Game's: `cmd/gamesniff` relays a
client to either server and logs every packet decrypted, and
`game/testdata/java-enter-world.txt` is AL-Game entering the world with
Wrathchild, which `game/world_test.go` compares the Go packets with.

1. [x] Network: game crypto, opcodes, framing; login server link (register,
   account auth); chat server link.
2. [x] Character select: list (AL-Game's order: never played, then most
   recent), name check, create, delete, restore. Checked with the client.
3. [x] Entering the world: skills, quests, recipes, UI settings, inventory,
   stats (AL-Game's stat engine: class templates, item modifiers, sets,
   manastones, titles, mastery and other passive skills), bind point, macros,
   titles, channel, abyss rank, weather, chat token; moving; leaving the world
   and saving position, life stats, settings and the game clock. The client
   enters the world with Wrathchild on the Go server.
   Left for later milestones: god stones and enchantment in stats (7), item
   cooldowns and saved effects (8), sieges in SM_SIEGE_LOCATION_INFO (11),
   friends, blocks and mail (10). A new character's first login starts its
   race's intro quest and movie through the first quest handlers (9).
   Moving hasn't been tried in the client: synthetic keys and clicks don't
   reach the game once in the world (Wine raw input), so it needs real hands.
4. [x] World: known lists (visibility distance 95, all spawned players checked
   on each move, no map regions yet); seeing other players; moving (every
   movement type of CM_MOVE except flying and gliding state); normal and shout
   chat to those in range. Checked with the client: `cmd/aionbot` walks and talks
   as a second player and the client sees her.
   Emotions and player states (sit, weapon out, walk, loot) and target selection
   match the 1.9 server's (`game/testdata/play-session-1.9.txt`).
   Left: flying, group, alliance, legion and whisper chat (10), admin commands (11).
5. [~] Static data: NPC and gatherable templates, tribes and spawns loaded; every
   spawn group of the maps that aren't instances puts its pool at its first
   spots (50,739 npcs and 13,608 gatherables), and players see them within the
   visibility distance (map squares of 256 make the lookup cheap). SM_NPC_INFO
   and SM_GATHERABLE_INFO match the 1.9 server's byte for byte
   (`game/testdata/*-1.9.txt`, from the server the client was written for).
   Spawns take object ids from `firstObjectID` up, as AL-Game's take the first
   ids: with a low id (Gopherina, id 1, was created before the floor) the
   1.9 client shows an unnamed object where the player stands.
   Left: walking npcs (npc_walker.xml, random walks), respawns, static
   objects, instances, day and night spawns, the channels of twin maps.
6. [~] Combat: normal attacks (AL-Game's damage formulas, hit statuses, multiple
   hits), monsters that notice, chase, hit and lose interest, walk home and
   heal (the AI's events, states and desires ported as they are, at the same
   one second and half second beats), npcs that walk their routes or wander,
   death, corpses that decay, respawns, experience and levels, a player's
   life and mana coming back on their own and the client told of them within
   100 ms. Checked with the aionbot (`-fight`), which fights the nearest
   monster over the network, and `TestFightAMonster`.
   Aggro fix: AL-Game's SEE_PLAYER handler does nothing to a monster that is
   already ACTIVE and scheduled, so a wandering/walking monster (most spawns
   have `rw`) never noticed anyone; the port now looks again when a player
   comes into view and the monster has no AggressionDesire, and a fight isn't
   dropped because someone else comes into view. AttackDesire/MoveToTargetDesire
   `onClear` (weapon away, stop following) were empty. `aggro_test.go` covers
   approach, range, level+10, entering the world, wandering monsters, linked
   tribes, and going home and healing. The static data has no `AGGRESSIVE`
   npc_type: aggression is the tribe's (AggroIcon).
   Death costs experience (a third of it for good, the rest recoverable), and the
   player revives at the bind point or by skill, rebirth or a resurrection stone.
   Duels (request, confirmation, no death, result) and attacks between players of
   different races.
   Killing a player of the other race gives abyss points by the damage done (groups share),
   and the dead loses some; abyss ranks follow the points.
   Left: the abyss's other sources of points, the skills of the high ranks, the legions' shares, duels,
   summons, drops (7), spells and effects (8), group rewards (10).
7. [~] Items: drops (droplist, at AL-Game's rate), the corpse's loot window and
   taking loot, the cube (adding, stacking, moving, removing, kinah),
   equipping and unequipping with AL-Game's checks (level, race, class, the skill
   for the weapon or armor, hands), soul binding through question windows, and the
   client told through SM_ADD_ITEMS, SM_UPDATE_ITEM and SM_UPDATE_PLAYER_APPEARANCE.
   SM_ADD_ITEMS and SM_UPDATE_ITEM match the 1.9 server's byte for byte.
   Using items works for the skill actions (potions, food, scrolls: the skill is
   cast, the item used up, the use delay kept and sent) and skill books.
   Shops: the buy and sell dialogs (2 and 3) of npcs with a trade list, buying and
   selling at AL-Game's prices (`shop.go` answers those two dialogs before the quests'
   `dialogSelect`, which handles the rest). Item cooldowns, self resurrection stones.
   Warehouse (npc dialog 20, moving, splitting, merging, swapping, deleting, kinah
   between cube and warehouse), expanding cube and warehouse, soul healing by dialog.
   The warehouse packets follow AL-Game's and aren't checked against a 1.9 recording.
   Gathering (materials, the skill and its experience, the plant used up and back after its
   respawn time), crafting (recipes learned by items or with the skill, components, critical
   products, skill and player experience), dye scrolls, and the masters who teach the next level of a crafting skill.
   Enchanting (stones with AL-Game's rates, the stats each level adds) and socketing manastones.
   Godstone socketing charges the Java service fee, replaces the existing socket and consumes one stone atomically;
   sockets persist across item reloads and appear in inventory/equipment packets. Combat procs remain unimplemented.
   Account warehouse (`accountwh.go`: item and kinah rows in location 2 owned by the account id, 17 places, no expansion,
   shared by the characters of an account, sent with the warehouse dialog; AL-Game has no restrictions on what goes in).
   Left: the other item actions (supplements for enchants), crafting stations (static objects),
   the legion warehouse, abyss shops, enchanting, stigmas, mail, the broker.
8. [~] Skills engine: the skill templates read as they are in the XML; casting
   (properties, conditions, cast time, cooldowns, interrupts by movement and
   damage), targets (one, area, friends, enemies), effects: damage (magic
   and physical, front and back modifiers), healing (hp, mp, fp, dp), stat
   buffs and debuffs, damage and healing over time, poison and bleeding,
   stun, root, sleep, paralyze, fear, silence, blind, snare, slow, bind,
   shields, taunts, dispelling buffs; effect icons within 100 ms as AL-Game.
   SM_CASTSPELL and SM_CASTSPELL_END match the 1.9 server's byte for byte.
   Also transform, hide, always block/dodge/parry/resist, drains, delayed damage,
   provoking, stagger, stumble, open aerial, spin, pulled, dash, move behind,
   resurrect and rebirth, dispelling physical and mental debuffs, HP/MP switch.
   Monsters use the skills of `npc_skills.xml` (SkillUseDesire).
   Signets (carve and burst), auras, traps and servants, and the summons of spirit masters (the
   panel, the modes, attacking on command, skills, the client-driven movement, letting go).
   Stigma stones (shards, the skills they teach while worn, shown as stigma skills).
   Left: search, skill launchers, the toy pets. Skills are learned on level up (and at 10, the crafting
   skill switches), matching the 1.9 server's SM_SKILL_LIST.
9. [~] Quests: the two prologues, four shared Java XML templates
   (`report_to`, `monster_hunt`, `item_collecting`, `work_order`), and the five
   specialized XML quests are ported. The Go server dispatches all 1,596 XML
   declarations, with indexed NPC markers, dialogs, hunt counters, collection and
   quest drops. Work orders grant temporary components and recipes, check
   crafting skill ranges, and clean up on completion or abandonment. Eligibility
   covers race, level, class, gender, prerequisites, and repeat count. Completion handles fixed and selectable
   items, experience, kinah, titles, Abyss Points, and cube expansion. Quest
   state and persistent special rewards are saved together. The automated
   catalog tests start and complete all 1,596 XML quests; these are
   synthetic tests, not a client or live database parity check.
   The user confirmed the first Elyos quest at Elpas in the 1.9 client.
   215 custom Java handlers are now completed and tested, including the first Elyos
   and Asmodian campaign chains, Poeta's The Nymph's Gown and The Kerub Threat,
   Elyos quests 1011–1023, Summons to the Citadel (1130), Missing Poppy (1149),
   Stolen Village Seal (1156), Gaphyrk's Love (1157), Village Seal Found (1158),
   Sword of Transcendence (1097), The Red Journal (3060), the Asmodian startup
   and campaign quests 2011–2018, 2123, 2136, 2200, and 2300, Reducing Tursin Strength
   (1194), Krall Book (1197), A Secret Delivery (1220), and Orders from Telemachus (1300).
   **Still open:** the current `scripts/quest-claim.py status` reports zero active claims,
   109 free unported handlers, and 218 registered in Go. Ten registered drafts still
   need focused tests and are excluded from the free count. Group progress, generic
   quest work-item cleanup on abandonment, reward edge cases,
   and broader client verification remain. See [QUEST_PORTING.md](QUEST_PORTING.md)
   for the source map, exact tests, gaps, and next steps. Do not mark milestone
   9 complete based on the catalog test alone.
10. [~] Social: whispers (level 10, blocked players), friends (asking through the
   question window, notes, online status and login/logout notices to friends in
   the world), blocks with reasons; stored in `friends` and `blocks`.
   Groups of six (invitation, leaving, kicking, leader, group chat, member info kept
   up to date, shared experience and loot rules, kinah sharing). The 1.9 client hasn't
   seen the group packets yet: they follow AL-Game's, which has 2.0 leftovers elsewhere,
   so check them against a recorded session.
   Trading between players (offers of items and kinah, lock, confirm) and private stores
   (the sign, selling by price, buying through the seller's dialog).
   Mail (sending with AL-Game's commission, reading, taking the item or the kinah, deleting;
   the mailbox is loaded five seconds after entering the world).
   Legions (founding, invitations, ranks, leaving, kicking, brigade general, announcements, nicknames,
   permissions, chat, levels, disbanding; stored in `legions` and `legion_members`).
   The broker (registering, browsing by kind and sorting, buying, cancelling, settling the account, expiry).
   Small requests: titles, macros, taking off effects, looking at a player, searching for players, a group's loot rules.
   NPC map search (`CM_OBJECT_SEARCH`): first spawn in static file order, including other maps;
   Java-compatible `SM_SHOW_NPC_ON_MAP`, with capture/client verification still open.
   Alliances of 24 (invitation merging groups, leaving, banning, captain and vice captains,
   alliance chat, member info; no loot rules or readiness checks; unverified against 1.9).
   Group/alliance target brands (`CM_SHOW_BRAND`), including clearing: any member can
   broadcast Java-compatible `SM_SHOW_BRAND`; capture/client verification remains open.
   Legion emblems (buying, sending; uploaded emblem images aren't).
   Legion history (the tabs). Kisks (the item, binding by use mask, resurrections, destruction).
   Left: the legion warehouse and express mail (AL-Game leaves both off).
11. [~] Regular npc teleporters (the map, the price, the teleport delay), and admin
   commands (`//add`, `//kinah`, `//goto`, `//moveto`, `//heal`, `//kill`, `//rez`, `//set level|exp`,
   `//morph`, `//announce`) for accounts with an access level.
   Flying and gliding (fly time running out in the air and coming back on the ground).
   Instances (a portal npc, with its use time and checks of race, level, title and group; an
   instance of the map with its npcs for the player or group, that goes when it has been empty
   a minute; logging in inside one) — players see only those of their own instance.
   Petitions and punishments (petition.go, punish.go, store/punish.go): CM_PETITION files or cancels a player's single
   open petition (petitions table, queue in memory, SM_PETITION with place and wait, resent on login and when the queue
   moves; game masters online are told); `//petition [id [delete | reply <text>]]` lists, reads, deletes or answers by mail
   (`sendLetter`, so the admin pays the commission and must share the race, as in AL-Game). `//gag|ungag`, `//sprison|rprison`
   (player_punishments: sentence in ms, counted only while online, saved on logout, restored on login into the prison map
   510010000; chat, attack, skills, items, equipment and invites refused there), `//kick`, and `//ban|unban|banip|unbanip`
   which send CM_BAN to the login server. Left, as in AL-Game: gags aren't saved; a prison sentence of 0 minutes has no timer.
   Differences: cancelling with no open petition does nothing (AL-Game files an empty one); `//sprison` on a dead or
   still-loading player doesn't move him (teleportTo needs a spawned, living player); per-command access levels
   (administration.properties) aren't checked, any game master may use them; the ban result isn't reported back to the admin.
   Left: instance-specific scripts (doors, bosses, keys), the twin maps' channels, abyss, the
   rest of the commands.
   Rifts (services_rift.go): every 100 minutes one of each region's seven rifts opens at random in Eltnen, Heiron, Morheim
   and Beluslan (master and its slave in the other race's map, 26 minutes); the master asks the question to go through,
   teleports to the slave, counts entries and closes both when full; SM_RIFT_STATUS on sight, SM_RIFT_ANNOUNCE on open and on
   level ready. Left: the other channels of the maps; the level range and race are only shown to the client, as in AL-Game;
   AL-Game's rifts never close on their own (backwards despawn test), these do after 26 minutes; the 1.9 packet layout
   of SM_RIFT_ANNOUNCE is the source's own guess, unchecked against a capture.
   Sieges (services_siege.go): AL-Game's SiegeService is only ownership, not battles. Each fortress, artifact and boss raid
   has a race, legion, vulnerable and next state (artifacts always vulnerable), owners kept in siege_locations;
   `//siege capture|set|list|help` (smart prefix matching) changes them, saves, and broadcasts SM_SIEGE_LOCATION_INFO
   (change) and SM_INFLUENCE_RATIO (fortress 10, artifact 1, boss raids 0) to everyone in the world. Left, as in AL-Game:
   the siege timer (always 0), the fights, converting a captured fortress's spawns; a 0 total influence sends zeros where AL-Game sends NaN.
   Zones, weather and game time (zones.go, data/zones.go): zones/zones_*.xml (polygon, top/bottom, priority, fly,
   breath, links) load with world_maps.xml's death and water levels. A player's zone is found on entering a map and
   after a same-map teleport (ZONE_REFRESH) and follows moves through the linked zones (ZONE_UPDATE); CM_EMOTION's fly
   is refused with STR_FLYING_FORBIDDEN_HERE where the zone doesn't say fly (no zone allows it); below the death level a
   player dies, and below the water level less 1.6 heights it drowns (a tenth of max HP every 2 s) unless the zone is
   breathable. Weather is redrawn every 2 h by a 2-minute check and its map's players told (before, only at login);
   SM_GAME_TIME goes to everyone every 3 minutes. Left, as in AL-Game: it sends no zone-name packet, and day/night
   events have no caller; zones are checked on each move, not in a 4 s batch; AL-Game keeps a stale zone on a refresh
   that finds none and never looks it up on login (so its flying check waits for a teleport), here it does;
   the quest engine's onEnterZone isn't called (the quest files belong to another change: hook it in `updateZone`);

## To check against the 1.9 client

Everything below was ported from AL-Game's source and tested with unit tests (and the SQL
against the real database: `AION_TEST_DB=<db host> go test ./game/store`), but no recorded
1.9 session has these packets, and the source has 2.0 leftovers. Record a session with
`docker/capture-1.9.sh` doing each of these on the Java stack, save it under
`game/testdata/`, and compare the way `TestCastPacketsMatchClient19` does:

- godstone socketing/replacement, the service fee and weapon glow after equipping/relogging;
- groups (invite, accept, leave, kick, leader, loot rules), group chat;
  group loot rolls and quality settings are implemented with protected winner
  reservations and atomic grants; bid distribution remains unported;
- friends, blocks, whispers, `/who` (CM_PLAYER_SEARCH), looking at a player;
  LFG status 9, LFG-only search and result status 2 are implemented with packet
  regression tests; real-client confirmation remains pending;
- trading, private stores, mail (send, read, take), the broker (register, list, buy, settle);
- warehouse (open, put in, take out, expand), cube expansion, soul healing;
- npc teleporters, portals into a dungeon (SM_CHANNEL_INFO with the instance);
- flying, gliding and landing; duels; summons of a spirit master;
- gathering and crafting (SM_GATHER_UPDATE, SM_CRAFT_UPDATE), enchanting, stigma stones, dye;
- founding and running a legion.

Item remodelling (CM_ITEM_REMODEL: skin and dye taken from another item, or removed with a pattern reshaper, 1000 kinah, level 20) and arms fusion (CM_FUSION_WEAPONS/CM_BREAK_WEAPONS, `services_remodel.go`); as in AL-Game a fused weapon's manastones are not transferred, its stats are not added, and the fusion fee is checked but never charged. 

Small controllers and services (`services_small.go`; hooks on CM_SHOW_DIALOG and CM_DIALOG_SELECT that sort after the quests'):
bind stones (npc type RESURRECT, `bind_points.xml`: the question, the price, SM_SET_BIND_POINT, the level-update broadcast, AL-Game's
race and territory refusals unless `Config.CrossFactionBinding`, no binding in the prison map), postboxes (dialog 18), the item
`extract` action (item 165000001: five seconds, then the item and one extractor become 1-3 enchantment stones), HTMLService
(SM_QUESTIONNAIRE in chunks; `HTML/welcome.xhtml` shown on entering the world when `Config.HTMLWelcome`), AnnouncementService (the
`announcements` table, one repeating task per row, by faction and chat type; loaded once at start, no reload command) and
ClassChangeService (`Config.SimpleSecondClass`, off as in AL-Game's custom.properties: the level 9 dialog on login and on level up, the
choice sets the class, recomputes stats and skills, completes quests 1006/1007 or 2008/2009; the class is saved with the character).
The three flags are the env vars `AION_SIMPLE_2NDCLASS`, `AION_HTML_WELCOME`, `AION_CROSS_FACTION_BINDING` of cmd/gameserver.
Differences: AL-Game takes a class choice from anyone at any time (and always completes the quests), here only a level 9 player of the
matching first class may choose, and the quests' dialog handler sees the packet when it isn't one; the bind stone doesn't leave a kisk.
Not ported: StaticObjectController (empty in AL-Game; static objects, `spawns/StaticObjects`, send the client nothing, and no code looks
them up), ActionitemController (USEITEM npcs: its dialog is the quest engine's, with the quest work items).

Not ported yet: siege battles (below), the legion warehouse.
Flight teleporters (flight.go): the earlier note was wrong, npc_teleporter.xml has 58 FLIGHT teleporters with a `teleportid` per destination, but
the path itself is in the client (AL-Game has no path data either). CM_TELEPORT_SELECT on a FLIGHT teleporter charges the same 0.8 x price,
sets the flying bit (FLIGHT_TELEPORT shares FLYING's), clears ACTIVE, and broadcasts SM_EMOTION START_FLYTELEPORT(path id); SM_PLAYER_INFO
adds path id and distance while it lasts; CM_FLIGHT_TELEPORT moves the player to the client's position and keeps the distance;
LAND_FLYTELEPORT lands (state back to ACTIVE, zone refreshed). Left, as in AL-Game: nothing checks the client's positions against the
path, and nothing lands a player who disconnects mid-flight (the state isn't saved).
Pets: AL-Game 1.9 has no pet system (no PetService, no SM_PET*/CM_PET* packets, no adopt action; `pet_skills` static data is unused).
`ToyPetSpawnAction` is the kisk, ported in `services_kisk.go`. Nothing to port.

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
