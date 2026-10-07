# Go review coverage and critical fixes

Started 2026-10-02 against the shared worktree. This ledger tracks implemented
fixes and reviewed scope separately from the Java-to-Go feature inventory.
A reviewed boundary does not certify its entire package.

## First critical batch: packet boundaries

Skills applied: Go routing, safety, security, error handling, code style and
testing; project constraints remain in `AGENTS.md`. Existing quest and panel
edits are preserved. No runtime stack changes or image publication are involved.

| Scope | Finding | Change | Status |
| --- | --- | --- | --- |
| `wire/wire.go`: Reader byte counts | Invalid large counts allocated the requested size, and later reads overwrote the first error | Reject counts outside one frame before allocation; preserve the first error and existing bounded short-field padding | Implemented; regression and fuzz checks |
| `wire/wire.go`: Frame/WriteFrame | Length cast silently wrapped above the 16-bit frame limit; writers discarded partial writes | Return an explicit frame-size error; provide checked whole-frame writing | Implemented; boundary and failure tests |
| `game/conn.go`: send | Failed/short writes left a rolling cipher stream in use | Reject oversized game packets before encryption; close on deadline/write failure under the write lock | Implemented; failure/boundary/concurrent cipher-order tests |
| `game/links.go`, `chat/server.go`, `login/gameserver.go`: plain sends | Framing errors and short writes were ignored | Use checked framing and close failed streams | Implemented; full protocol suites required |
| `login/client.go`, `client/login.go`, `client/game.go`: encrypted sends | Framed/padded lengths were unchecked and write failures ignored | Check padded/header sizes before encryption; use checked writes and close broken streams | Implemented; login boundary and existing handshake tests |
| `game/world.go`, `flight.go`, `summon.go`: client movement | NaN/infinite coordinates could reach world grids, zones and broadcasts | Validate current and destination coordinates before world access | Implemented; invalid-coordinate and truncated-packet tests |
| `login/gameserver.go`: registration ranges/account list | Unchecked signed counts could panic or allocate disproportionate memory; truncated names could reach state updates | Bound counts against remaining packet bytes; reject malformed lists before state changes | Implemented; malformed-count/name tests |
| `game/sniff.go`: handshake relay | Short key packets could panic; upstream failure left the relay waiting forever for a key | Validate headers/key size and close the key channel on upstream termination | Implemented; malformed-key termination tests |

`wire.Frame` now returns `([]byte, error)`. Repository callers use `WriteFrame`
or explicitly check the returned error. Valid packet bytes and required crypto
are preserved. Rejected game output closes the connection rather than trying
to continue a potentially desynchronized stream.

## Reproducible validation

- `go/scripts/check-go.sh`: sequential full tests, vet, seven command builds,
  formatting checks; logs stored under the ignored `.build/go-checks/` directory.
- `go/scripts/summarize_tests.py`: separates test-node and package counts,
  lists failed/skipped tests, and rejects empty or unfinished runs as success.
- `go/scripts/test_summarize_tests.py`: five tests for counts, optional DB skips,
  failed builds, interrupted runs, invalid JSON and empty logs.
- `AGENTS.md`: Go skill routing and protocol/toolchain constraints now explicit.

Baseline before this batch: 2,142 passing test nodes, zero failing nodes,
seven optional database skips. Those skips mean database integration was not
verified. Validation results for the completed batch are recorded below.

Completed batch validation:

- Full `go test -json -count=1 ./...`: **2,217 passing test nodes, zero failures,
  seven optional database skips**. Package results: ten passing packages and
  seven packages without tests. The seven skipped test nodes are listed in
  `.build/go-checks/summary.json`; no disposable database was configured.
- Focused `go test -race -json` across wire/login/game packet tests:
  **67 passing test nodes, zero failures or skips**; no races reported.
- Ten-second fuzz runs: `FuzzFrameRoundTrip` passed after 27,819 executions;
  `FuzzReader` passed after 759,539 executions. These are bounded fuzz runs,
  not an exhaustive input-space proof.
- Test-summary tooling: **five Python tests passed, zero failures or skips**.
- `go vet ./...`, all seven `cmd/...` builds, repository Go formatting,
  shell syntax and `git diff --check` passed.
- An intermediate focused race run had three failing test nodes (the login
  size-boundary parent and its two cases): the fixture set a deadline after
  intentional pipe closure. Deadline setup was moved before the send; the
  subsequent focused race and full suite passed. No production race was found
  in that run.

Logs and machine-readable summaries are under `.build/go-checks/`.
No new real-client capture, database integration, deployment, image publication
or live-stack restart was performed. Packet goldens and existing handshake/game
tests ran in the full suite. This completes the first packet-boundary batch,
not the entire critical pass or full codebase audit.

## Remaining critical scope

1. Add pinned correctness lint and vulnerability tooling, CI, and an isolated
   database fixture job. Retain the Apple-host validation workaround until a
   replacement toolchain has been qualified.
2. Audit every request parser for `Reader.Err` before mutation. This batch checks
   movement and two login count paths; it does not validate every game, login
   or chat request. Assess remaining finite coordinate trust boundaries too.
3. Review persistence errors, transactions and in-memory/client commit order
   for inventory, rewards, trades, mail, broker and options loading.
4. Review server lifecycle, shared-state ownership, shutdown saves, reconnect
   loops and all timer/goroutine cancellation. Check slow-peer backpressure
   and lock-held network I/O separately from the cipher-order regression.
5. Review panel authorization, mutation protection, proxy/cookie policy,
   request limits and authentication rate limits.

Next persistence slice, confirmed by source inspection in this session:
`game/inventory.go` mutates item/kinah counts and sends updates even when
`saveItem` logs an update failure; `game/mail.go` moves/splits an attachment
before `InsertLetter` can fail; `game/broker.go` charges the registration fee
and moves the item through separate writes before registration completes.
These paths need explicit failure behavior and transaction/failure-injection
coverage. This batch does not change their shared quest/inventory interfaces.

## Full codebase review coverage

| Area | Coverage so far | Remaining |
| --- | --- | --- |
| wire | Reader fields, framing and checked writing | Broader consumer error propagation |
| crypt | Existing protocol tests; game cipher integration exercised | Independent full crypto/usage review |
| login | Two count parsers and output framing | Remaining requests, persistence, authentication, lifecycle and configuration |
| chat | Plain output framing | Parsers, authentication, channel/state ownership and lifecycle |
| client | Output framing and header/padding limits | Parsing, read-channel backpressure and lifecycle |
| game | Movement/output boundaries, relay key handling | Other handlers, effects, AI, inventory, social systems, world state and lifecycle |
| game/data | Not reviewed in this batch | Entire package |
| game/store | Not reviewed in this batch | Entire package with disposable database failure tests |
| options | Prior plan's sampled concerns only | Load failures, iteration errors and contexts |
| commands | Not reviewed in this batch | Entire package |
| cmd | Existing tests and command builds | Each command's configuration, wiring, shutdown and panel paths |
| Go tooling | Check runner and JSON summary added | CI, pinned scanners and fixture setup |

Quest porting/correctness proceeds alongside this ledger. Coordinate shared
quest/store changes and continue to serialize container-backed Go commands.

## Kinah persistence slice

`game/inventory.go` now writes a copied kinah item before changing the live
player balance or sending `SM_UPDATE_ITEM`. `increaseKinah` reports persistence
failure, and `addItem` propagates that result for kinah grants. Failed decreases
leave the balance and client state unchanged. The failure-injection regression
test covers both directions and the `addItem` grant path.

Validation: focused test passed; full `go test -json -count=1 ./...` reported
2,267 passing test nodes, zero failures, and seven optional database tests
skipped because `AION_TEST_DB` was not configured. `go vet ./...`, gofmt check,
and `git diff --check` passed. Trade, mail, broker and non-kinah item writes
remain open; this slice does not make those multi-item flows atomic.

## Java-to-Go dice roll request

Java source: `java/AL-Game/src/main/java/com/aionemu/gameserver/network/aion/clientpackets/CM_CLIENT_COMMAND_ROLL.java`
(`readImpl`, `runImpl`). Go registers `CM_CLIENT_COMMAND_ROLL` in
`game/client_command.go`, rolls inclusively from 1 through a positive maximum,
sends system message 1400126 to the player, and broadcasts 1400127 to players
in the player's known list. Nonpositive maxima produce 1, matching Java's
minimum result at zero and avoiding invalid random bounds; `MaxInt32` is safe.
Truncated requests are ignored before sending. Nonpositive values are
malformed; Go returns 1 for them (the Java zero case also returns 1).

Focused tests cover both messages, valid range, nonpositive and maximum integer
bounds, and truncated input. Full validation: **2,269 passing test nodes, zero
failures, seven optional database skips**; `go vet ./...` passed. No real-client
or packet-capture verification was performed, so this remains implemented but
unverified for 1.9 runtime parity.

## Java-to-Go static door request

Java source: `java/AL-Game/src/main/java/com/aionemu/gameserver/network/aion/clientpackets/CM_OPEN_STATICDOOR.java`
(`readImpl`, `runImpl`) reads a door object ID and broadcasts `SM_EMOTION`.
Go registers the request in `game/staticdoor.go`, ignores truncated input, and
broadcasts the matching switch-door packet to the player and their known list.
The existing Java packet format is preserved: door ID, emotion type 29, state
9, and trailing zero `D`. This ports only the broadcast request; door state and
access mechanics remain outside this behavior.

Focused packet and truncated-input tests passed. Full `go test -json ./...`
reported **2,271 passing test nodes, zero failures, and seven optional database
skips**. `go vet ./...`, gofmt, and `git diff --check` passed. No client or
capture verification was performed.

## Java-to-Go NPC map search request

Skills applied: Aion server reference, Go routing, safety, testing and code style.
Reviewed `game/object_search.go`, spawn loading and indexing in
`game/data/{data,world}.go`, and the add/delete/reload paths in
`game/admin_world.go`. Java references are `CM_OBJECT_SEARCH.runImpl`,
`SpawnsData.getFirstSpawnByNpcId`, and `SM_SHOW_NPC_ON_MAP.writeImpl`.

Go registers `CM_OBJECT_SEARCH` for in-game clients, reads the template ID,
and sends one marker for the first nonempty spawn group in static file order.
The marker writes the NPC ID, world ID twice, and three float coordinates.
Unknown templates and truncated requests produce no reply. Selection includes
other worlds and groups with zero pools or special handlers, matching Java's
lookup behavior rather than the set of active NPC objects.

The new NPC index retains cross-map insertion order and shares spawn groups
with the world index. Admin addition, deletion and data reload update both;
the admin test fixture owns both indexes to avoid mutating shared static data.
Tests cover the response bytes, dispatch registration, group/spot ordering,
missing templates, all short request lengths, absent players, loader reload,
and admin addition/deletion. Capture and real-client verification remain open.

Validation: focused run **17 passing test nodes, zero failures or skips**;
final full `go test -json -count=1 ./...` **2,274 passing test nodes, zero
failures, seven optional database skips** (ten passing packages, seven without
tests). `go vet ./...`, repository gofmt check, all `cmd/...` builds and
`git diff --check` passed. Logs and the list of skipped tests are in
`.build/object-search/`. No test database, client capture or live-stack change
was involved.

## Java-to-Go group and alliance target brands

Skills applied: Aion server reference, Go routing, safety, testing and code style.
Java references: `network/aion/clientpackets/CM_SHOW_BRAND.readImpl/runImpl`,
`services/GroupService.showBrand`, `services/AllianceService.showBrand`, and
`network/aion/serverpackets/SM_SHOW_BRAND.writeImpl`, under
`java/AL-Game/src/main/java/com/aionemu/gameserver/`.

`game/brand.go` registers the in-game request and reads both signed IDs before
entering player state. Any group/alliance member may broadcast; Java performs
no leader, brand-range or target-resolution check. Both membership branches
are independent, matching Java. All group members and connected alliance
members receive `H(1), D(brand), D(target)`. The packet builder also replaces
the identical existing zero/zero alliance-join reset. There is no persisted
brand state in Java or this port; clearing IDs pass through unchanged.

Reviewed the group/alliance membership lists, the `withPlayer` visibility lock,
and logout's removal from both rosters. No inventory, reward, quest, or database
write is involved, so the remaining persistence findings do not block this
bounded request port. Broader lifecycle and lock-held network I/O reviews
remain open.

Focused checks: **seven passing test nodes, zero failures or skips**. Coverage
includes literal packet bytes, member rather than leader authority, group and
alliance recipient isolation, both membership branches, an offline alliance
entry, solo and absent players, clearing, signed ID extremes, registration and
all eight truncated request lengths. Capture/client verification remains open.

Final validation: full `go test -json -count=1 ./...` reported **2,281 passing
test nodes, zero failures, seven optional database skips** (ten passing
packages, seven without tests). No disposable database fixture was configured.
`go vet ./...`, all seven command builds, repository gofmt check and
`git diff --check` passed. Logs and skipped-test names are in `.build/brands/`.
No quest code, running stack, deployment or published image was changed.

## Java-to-Go alliance subgroup reassignment

Skills applied: Aion server reference, Go routing, safety, testing and code style.
Java sources under `java/AL-Game/src/main/java/com/aionemu/gameserver/`:
`CM_ALLIANCE_GROUP_CHANGE.readImpl/runImpl`,
`AllianceService.handleGroupChange/broadcastAllianceMemberInfo`,
`PlayerAlliance.hasAuthority/swapPlayers/setAllianceGroupFor/getOpenAllianceGroup`,
`PlayerAllianceEvent`, `SM_ALLIANCE_MEMBER_INFO` and `SM_PLAYER_ID`.

`game/alliance_group.go` registers the in-game request, reads all three signed
IDs before accessing player state, and retains Java's captain/vice-captain
authority and rejection messages. Group zero swaps two members; a group ID
moves one member. Each changed subject produces event-13 member info followed
by player-ID info for every connected alliance member, including the subject.
Swaps broadcast the first subject before the second, after both assignments
change. Existing packet builders are reused.

`game/alliance.go` now retains subgroup assignments independently of roster
order. Leaving removes only the departing assignment; joining fills the first
group with fewer than six members. Roster order and captain/vice status survive
moves and swaps. Source inspection covered formation, addition, removal,
member updates, authority and dispatch. The request uses the existing serialized
player/world dispatch; no new goroutines or database writes are introduced.
Open inventory transaction findings therefore do not block this slice.

Intentional defensive differences: unknown member IDs, groups outside the four
advertised IDs (1000–1003), and moves into another full subgroup are ignored
before mutation. Java assumes valid input and can dereference absent members,
create arbitrary group IDs, or overfill a subgroup. Valid same-group moves and
self-swaps retain Java's broadcasts. This port retains Go's existing immediate
alliance departure on logout; Java's delayed offline-member retention remains
a separate lifecycle gap. No new alliance persistence is claimed.

Focused coverage includes authority, registration, packet order/recipients,
ID packet bytes, event/group fields, swaps between full subgroups, capacity,
stable assignments across movement/leave/join, missing/foreign members, invalid
groups, all twelve truncated lengths, absent players and disconnected recipients.
Real-client/capture verification remains open. The existing group-restricted
kisk check also consumes `alliance.slot`, so it follows subgroup reassignment.

Validation: focused checks **21 passing test nodes, zero failures or skips**;
full `go test -json -count=1 ./...` **2,301 passing test nodes, zero failures,
seven optional database skips** (ten passing packages, seven without tests).
No disposable database fixture was configured. Logs and skipped-test names
are under `.build/alliance-groups/`. `go vet ./...`, all seven command builds,
repository gofmt check and `git diff --check` passed. No quest code, running
stack, deployment or published image was changed.

## Java-to-Go small client requests (2026-10-03)

Skills applied: Aion server reference, Go routing, safety, testing and code style.
Java sources under `java/AL-Game/src/main/java/com/aionemu/gameserver/`:
`CM_CLIENT_COMMAND_LOC.readImpl/runImpl`, `SM_SYSTEM_MESSAGE.CURRENT_LOCATION`,
`CM_REPORT_PLAYER.readImpl/runImpl`, `CM_DISCONNECT.readImpl/runImpl`,
`CM_SHOW_MAP.readImpl/runImpl`, `CM_QUESTIONNAIRE.readImpl/runImpl`,
`network/factories/AionPacketHandlerFactory`, and
`network/aion/AionConnection.onDisconnect`. Socket close semantics also reference
`java/AL-Commons/.../network/AConnection.close(boolean)`.

`game/client_command.go` registers `/loc` and privately sends message 230038
with the authoritative world and coordinates. Float parameters keep Java's
fractional digit for integral values and scientific exponent style, including
negative zero and the smallest float. `game/client_requests.go` reads the unknown
report byte and terminated UTF-16 name before emitting structured reporter/target
fields. Java does not resolve the target, restrict report names, or reply.
Structured fields preserve that scope and escape embedded newlines.

The disconnect handler rejects truncated requests, ignores nonzero bytes, and
closes on zero without a final packet. It reuses `conn.close` and leaves logout
and saves to the read loop's deferred `disconnected`; it does not introduce a
second save or new goroutine. Go's immediate logout versus Java's conditional
15-second delayed logout remains a broader lifecycle gap. Pending inventory
transactions and lifecycle audits remain open; these handlers do not mutate
items, balances, quests or shared rosters. End-to-end disconnect persistence is
not certified by the socket-close unit test.

Map-open and questionnaire handlers match Java's inert actions; questionnaire
reads D/H/H/H/H without acting on them. `CM_CUSTOM_SETTINGS` is a confirmed
follow-up: Go loads display/deny kinds 2/3, but `saveSettings` currently saves
only kinds 0/1. Port its H/H request and broadcast together with logout saves and
disposable-database reconnect checks. Source inspection covered these existing
loads and deny/display consumers, without changing their persistence behavior.

Coverage includes request state/registration, literal location packet bytes,
private delivery, coordinate strings, absent players, report Unicode/empty names
and log escaping, every truncated report length, disconnect values and socket
closure, and inert requests with truncated payloads. Java comments mark report
opcode 0x32 and disconnect opcode 0xED as uncertain; capture/client verification
remains open for the new requests.

An initial focused run reported 46 passing nodes and one failing location
packet test because its literal expected message ID was mistyped. The golden
was corrected; the next focused run passed all 47 nodes with zero failures or
skips. A further smallest-float case was then added before final full checks.
Validation logs are under `.build/client-requests/`.

Final validation: full `go test -json -count=1 ./...` reported **2,349 passing
test nodes, zero failures and seven optional database skips** (ten passing
packages and seven without tests). No disposable database fixture was configured;
the skipped-test names and machine-readable summary are under
`.build/client-requests/summary.json`. `go vet ./...`, all seven command builds,
repository gofmt check and `git diff --check` passed. The container system service
was started because it was initially unavailable; no game stack was restarted,
no quest source changed, and no image was built, deployed or published.

## Godstone socketing (2026-10-03)

Ported `CM_GODSTONE_SOCKET` and `ItemService.socketGodstone` in
`game/godstone.go`. Requests parse all three object IDs before any side effect,
check Java's strict 15-unit XY range in the same map/instance, require an
unequipped owned weapon and a stone with XML godstone metadata, and charge
Java's 100,000-kinah base service price through the existing price modifiers.
Malformed, absent, invalid, dead-player and insufficient-resource requests do
not spend resources. Invalid armor targets and missing stones are rejected
instead of reproducing Java's unchecked target or null dereference.

`store.SocketGodstone` commits the fee, one stone consumption and replacement
of category-1/slot-0 together. It locks the weapon and guards resource writes
against ownership, location, equipment and expected-count changes. Memory and
success packets change after commit. The new SQL accepts a context with a
five-second deadline; connection lifetime cancellation remains part of the
broader lifecycle review. No schema change or new dependency is required.

Inventory/warehouse/mail and broker item loads retain the godstone ID. Item,
character selection, world entry, appearance and broker weapon packets write
it in the existing Java field without changing packet lengths. Manastone
failure deletes only category-0 sockets, preserving godstones. Combat procs
remain outside this socketing change; old-server capture and real-client
confirmation of socketing, replacement and weapon appearance remain open.

Focused validation passed 45 test nodes with zero failures or skips, including
13 database test nodes against a disposable MariaDB fixture. Persistence checks
cover stacked and final-stone consumption, replacement, item reloads, stale
resource/ownership/equipment guards, and rollback after an injected socket
insert failure. That failure also verifies godstone preservation when removing
manastones. The fixture was removed after testing. Logs are under
`.build/godstone/`; they are development artifacts, not a published release.

Final repository-wide `go test -json ./...` passed **4,490 test nodes with zero
failures and 12 database skips**. Nine pre-existing database tests lacked their
optional fixtures; the three godstone persistence tests lacked a fixture in the
standard runner, but all their 13 nodes passed in the dedicated disposable-DB
run above. All 35 godstone game test nodes passed in the full suite, including
three additional checks for Java's horizontal range, a literal appearance
packet and refusal to mutate without atomic persistence. `go vet ./...`, the
repository-wide gofmt check and `git diff --check` passed.
The existing LFG worktree changes were retained and included in the full suite.
No game stack was restarted and no image was deployed or published.

## Group loot rolls (2026-10-03)

Ported mode-2 `CM_GROUP_LOOT`, `SM_GROUP_LOOT`, and Java's quality-based roll
flow from `DropService`. Group settings retain the global autodistribution
field and all seven per-quality rules, and group info broadcasts those values.
Java selects the loot operation using the item's quality rule; the global
field does not override it. Default quality rules remain byte-identical.
Group leaders retain authority to change settings.

Nearby members at the kill are recorded separately from round-robin/leader
corpse-opening rights. A roll snapshots online members of that original group,
accepts one fully parsed response per participant, generates inclusive 1–100
scores on the server, and preserves Java's first-response winner on a tie.
Pending rolls block other item takes from that corpse. All passes return the
item to ordinary looting; a full inventory or failed persistence keeps a
winner reservation for retry. The looter alone may close the corpse. Responses
for a wrong group, corpse, item, distribution, choice, participant or duplicate
response have no effect.

`Store.ReceiveLoot` commits every stack increase and new row together, with
expected-count/ownership/location/equipment guards for existing stacks. The
game plans capacity before any mutation, uses a bounded context, releases newly
allocated IDs after failure, and updates inventory and removes the drop only
after commit. This closes the persistence error boundary for automatic roll
awards; the older ordinary-loot `addItem` path remains separate review scope.
No schema change or dependency is required.

The roll uses existing world locking and owns no new timers or goroutines.
Leaving/disconnecting members are removed from pending responses; a departed
winner cannot receive an item, and an uncollected reservation is released when
its winner leaves. Corpse removal discards roll state. These bounded cleanup
rules intentionally avoid Java's stale-member wait and duplicate-response
weaknesses. Bid mode remains unported. Capture and real-client confirmation of
roll windows, messages, settings and winner collection remain open.

Focused validation under the race detector passed **85 test nodes with zero
failures or skips**: 66 group-roll nodes, 17 loot-receipt nodes (including the
disposable MariaDB transaction cases), and two existing group/reward nodes.
The disposable database used the repository schema and was removed afterward.

Repository-wide `go test -json ./...` passed **4,565 test nodes with zero
failures and 13 optional database skips**. The rolled-loot transaction test
requires its disposable fixture in the standard runner; all eight of its
nodes passed in the focused run above. `go vet ./...`, repository-wide gofmt
and `git diff --check` passed. Existing LFG and godstone worktree changes were
preserved and included in these checks. No server stack was restarted and no
image was deployed or published.

## Saved effects, skill effects, godstone procs and loot bids (2026-10-03)

Saved effects: `store.Effects`/`SaveEffects` read and replace a player's
`player_effects` rows in one transaction. At logout the icon effects with a
minute or more left are saved with their skill's cooldown, then the other
cooldowns with a minute or more left, as `PlayerEffectsDAO.storePlayerEffects`.
The effects' timers stop with the discarded player instead of firing on it
later. At login the cooldowns that are not over return (with
`SM_SKILL_COOLDOWN` after the skill list) and the effects resume for the time
left. `current_time` is stored as the template duration minus the time left,
so restoring is exact; Java stores the time run and lengthens a restored
effect on every relog. `CM_LEVEL_READY` sends the player's real
`SM_ABNORMAL_STATE`, which is byte-identical to the old empty packet without
effects. Item cooldowns (`ItemCooldownsDAO`) remain unsaved.

Skill effects: `search` sets the player's see state, now written in
`SM_PLAYER_STATE`; `returnpoint` teleports to the exit of the item's named
return portal; `mpuseovertime` drains a share of max MP every checktime and
ends when MP runs short; `onetimeboostskillattack` and `magiccounteratk` use a
new SKILLUSE observer (`effectController.usingSkill`, notified from
`skill.use` before casting, as `Skill.useSkill`); `petorderuseultraskill`
sends `SM_SUMMON_USESKILL` with the summon's skill from `pet_skills.xml`.

Godstone procs: every attack a player makes (auto-attack or a damage effect)
gives each godstone on a weapon in hand Java's `Rnd(prob-left, prob) >
Rnd(0, 1000)` chance to use its skill on the player's target. Weapons in the
off set do not proc.

Loot bids: mode-3 `CM_GROUP_LOOT` uses the roll flow. A bid more than the
bidder's kinah passes, bids are not announced, and the first of the highest
bids wins. `Store.ReceiveBidLoot` commits the item, the winner's payment and
each other member's share (bid / (members asked - 1)) in one transaction,
each kinah row guarded by its expected count. A winner who can no longer pay
at award time leaves the item free to all.

Validation: `go test -json ./...` passed **4,577 test nodes with zero failures
and 15 optional database skips** (13 existing, plus the new effects and bid
store tests). Those two and the other store transaction tests passed against a
disposable MariaDB with the repository schema (7 store tests and their
subtests, zero failures), which was removed afterward. `go vet ./...` and gofmt are clean. Client and
capture confirmation of all four remain open. No image was built or deployed.

## Warehouse item packet compatibility (2026-10-05)

Reviewed `game/storage.go`, the shared item writers in `game/worldpackets.go`,
warehouse packet tests, and the original 1.9 server's warehouse and inventory
packet bytecode. Applied `golang-how-to`, `golang-troubleshooting`,
`golang-safety` and `golang-testing`.

Confirmed a client freeze risk: warehouse load/add packets reused the inventory
header, omitting the byte after the template ID, and all warehouse item paths
included inventory-only detail suffixes. Warehouse stigmas incorrectly used
the inventory stigma block. Kinah load/update packets also require an FFFF slot,
whereas warehouse additions use FF00. Dedicated warehouse headers now use the
shared detail writer with the correct mode and kinah slot. This changes packet
serialization without changing stored items or adding dependencies.

`TestWarehouseItemPackets19` checks load/add/update bytes for ordinary items,
weapons, armor, stigmas and kinah in regular/account storage, including two
consecutive items after an expansion. All five cases failed against the
original source and passed after the fix. Expected details were transcribed
from the original server bytecode; they are not a real-client capture.

Validation: `go test -json ./...` passed 4,599 test nodes (517 top-level tests),
zero failures and 16 optional database skips. `go vet ./...`, repository-wide
`gofmt -l .`, `go build ./cmd/...` and `git diff --check` passed. No disposable
DB fixture was supplied, and no database mutation was needed for this fix.
Real-client warehouse verification remains open.

## Dungeon corrections: scoped lifecycle and event review, 2026-10-05

Skills applied: Go routing, troubleshooting, testing, code style and concurrency.
This reviews the touched dungeon boundaries, not the whole game package.

| Scope | Confirmed finding | Correction / acceptance boundary |
| --- | --- | --- |
| `game/services_portal.go`, `data/portals.go` | Seen objects were accepted without life/world/instance/distance checks; delayed use did not revalidate; internal portals could be selected as returns | Check current object and transfer participants; return through an external race-compatible portal. Admission item transactions and ordinary party lifecycle still need separate evidence. |
| `game/darkpoeta.go`, `npc.go` | Run expiry depended on later events; duplicate deaths and generator events could advance progress; preparing/ended gathers scored | Schedule expiry, cancel it on ending/destruction, credit deaths once per life, track unique generator templates and guard gathering by active time. 1.9 scoring capture and full gameplay remain pending. |
| `game/ai.go`, `instance.go`, `npc.go` | Despawn only changed AI state; dead guard skipped cleanup, and observer removal could restart AI | Remove observers before immediate task/cast/movement/effect cancellation. Deterministic tests catch tasks surviving instance destruction. Server-wide shutdown remains outside this review. |
| `game/combat.go`, `skilleffects.go`, `darkpoeta_objects.go` | Barricades used ordinary damage and aggregated multi-hit damage | Cap positive damage per hit after avoidance/shields; keep multiple hits separate for HP/aggro. Existing 1.9 health is retained. |
| `game/skill.go`, `darkpoeta_objects.go` | Self/area bomb spell did not react with the untargetable mine wall | Observe completed item casts in the same run/area. Natural bomb acquisition and client collision remain unverified. |
| `game/craft.go` | A depleted vine identity could be reused through a stale gathering request | Reject depleted/stale/cross-run Huge Vine use; exercise skill 299/300 and duplicates. Scar escort release remains unresolved. |

Executed validation and remaining content gaps are recorded in
[the dungeon handoff](aion-dungeon-fixes/README.md). No dungeon or broader
critical-pass outcome is certified complete by this review.
