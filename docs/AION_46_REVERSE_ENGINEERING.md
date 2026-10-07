# Aion 4.6 decompiler handoff for AionGo 1.9

Recorded: 2026-10-05. Purpose: use recovered 4.6 server behavior to investigate
and improve the Go 1.9 implementation without changing its target client version.
No Go changes or gameplay comparisons have been performed as part of this analysis.

## Start here

1. Read `AGENTS.md`, its canonical
   `.agents/skills/aion-server-expert/SKILL.md`, and `go/PORTING.md`.
2. Choose one concrete suspected defect or missing behavior in the Go server.
3. Find the corresponding recovered function, inspect its callers and data,
   and record an evidence-backed hypothesis.
4. Establish whether that behavior applies to 1.9, then add a meaningful Go
   regression test before implementing the correction.

Use `go/QUEST_TRIAGE.md`, `go/QUEST_DEBUG.md`, and
`go/QUEST_HANDLER_AGENT.md` for quest work. Follow AionGo's Go topic skills
before changing code. The current Java source has known 2.0 protocol leftovers;
use historical Java 6 stack captures and real 1.9 client behavior to resolve
protocol disputes. Passing Java parity alone does not establish 1.9 correctness.

## Local inputs and outputs

Local workspace locations are supplied by the operator. The placeholders below
keep filesystem and operational details outside this public repository.

| Item | Location |
| --- | --- |
| Go/Java repository | `<repository-root>` |
| Original downloaded archive | `<archive-path>` |
| Extracted 4.6 server | `<reference-root>` |
| Analysis workspace | `<analysis-root>` |
| Saved Ghidra project | `aion4.6-analysis/projects/Aion46.gpr` |
| Main executable pseudocode | `aion4.6-analysis/exports/Server64/pseudocode.c` |
| Function address/name/result index | `aion4.6-analysis/exports/Server64/functions.tsv` |
| Address-to-byte-offset index | `aion4.6-analysis/exports/Server64/function-offsets.json` |
| Final counts | `aion4.6-analysis/exports/Server64/summary.txt` |
| Binary SHA-256/section inventory | `aion4.6-analysis/binary-inventory.json` |
| Export scripts | `aion4.6-analysis/scripts/` |
| Analysis/export logs | `aion4.6-analysis/{headless,ghidra-analysis,ghidra-export,retry}.log` |

The archive extracted successfully with 7-Zip: 11,434 files, 376 directories,
18,207,487,077 uncompressed bytes. It has 47 trailing bytes after the archive
end; extraction reported no data errors. The extracted tree and all 21 nested
archives were checked: no C/C++ source files or build projects were found.
The original archive was retained. Server binaries were not executed or modified.

## Verified analysis state

Ghidra **12.1.4**, OpenJDK **21**, Windows language/compiler
`x86:LE:64:default:windows`. Both tools were installed through Homebrew.
The initial analysis completed successfully in approximately 201 seconds.

Analyzed executable: `_SERVER/MainServer/Server64.exe`, 31,890,432 bytes.
SHA-256:

```text
22a7d1bad4dc0b0e3758f3f246e52f7bd13162fe62b318575a72df9c20c67b58
```

The final export contains **31,786 recovered functions**, **4 unresolved
functions**, and **34 skipped thunk functions**. Pseudocode size:
49,287,447 bytes. The initial export used a 15-second per-function timeout;
a second pass allowed 90 seconds and recovered one additional function.
All exported function addresses were checked against the index, the saved
project exists, and the original binary hashes still match the inventory.

| Unresolved function address | Final limitation |
| --- | --- |
| `1401aabb0` | Still timed out on the longer retry |
| `140263330` | Still timed out on the longer retry |
| `14053d260` | Instruction-flow limit; also warned about an unreadable address |
| `1406aea30` | Instruction-flow limit; also warned about an unreadable address |

These are analysis gaps, not demonstrated server defects. Disassembly remains
available in Ghidra. Check boundaries, callers, exception handling and indirect
branches before changing analysis settings or drawing conclusions from them.

Only `Server64.exe` has been imported, analyzed and exported. Other components
have binary and string inventories, but **have not been decompiled**:

| Binary under `_SERVER/` | Candidate investigation area, to confirm from call/data evidence |
| --- | --- |
| `NPCServer/NPCSvr64.exe` | NPC behavior and simulation |
| `MainServer/ScriptDLL64.dll` | Script/gameplay logic |
| `NPCServer/ScriptDLL64.dll` | Script library used by NPC server; compare hashes before assuming it is the same DLL |
| `CacheServer/CacheD64.exe` | Persistence/cache behavior |

The inventoried `ScriptDLL64.dll` is the MainServer copy. `exports/` also
contains ASCII strings, source-path references, and RTTI names for the principal
binaries. Server64 has 134 embedded source paths and 853 RTTI names; these
help locate code but are not recovered source files. The `Server64_1.exe-bak`
backup has high-entropy sections and appears packed/protected; the analyzed
`Server64.exe` has readable normal code sections. Do not substitute the backup
without separately identifying and analyzing it.

## Find and extract relevant functions

Avoid reading the entire 47 MiB pseudocode file into an agent context. Search
for meaningful strings, then extract a small function and its relevant callers.
Start with strings such as `AbyssLevelGroup::Load`, class names, XML attributes,
error messages, or known stat names. Actual names must be discovered from the
inventories; do not assume every subsystem has a readable string.

```sh
analysis_root='<analysis-root>'
rg -n -i 'damage|critical|quest|AbyssLevelGroup' \
  "$analysis_root/exports/Server64.exe.strings.txt" \
  "$analysis_root/exports/Server64.exe.rtti-names.txt"
rg -n -i 'AbyssLevelGroup::Load' \
  "$analysis_root/exports/Server64/pseudocode.c"
```

Every exported function begins with `/* Address: <hex> */`; a recovered retry
has `; recovered on retry` in that marker. The offset index covers both.
The index uses byte offsets, so open the pseudocode in binary mode:

```sh
python3 - <<'PYCODE'
from pathlib import Path
import json
root = Path('<analysis-root>/exports/Server64')
address = '140001100'  # Known exported example; replace with the desired address.
entry = json.loads((root / 'function-offsets.json').read_text())[address]
with (root / 'pseudocode.c').open('rb') as stream:
    stream.seek(entry['offset'])
    print(stream.read(entry['length']).decode('utf-8'))
PYCODE
```

In Ghidra, open `projects/Aion46.gpr`, select `Server64.exe`, and navigate to
an address from the index. Follow references to strings and functions; inspect
assembly for arithmetic widths, signedness, casts, rounding and comparisons.
Generated names such as `FUN_140...`, `DAT_...`, and `undefined8` are placeholders.
A raw call with a pointer offset is not proof of a particular struct field.
RTTI can help identify types but does not restore all original layouts.

Use the workspace README for export commands. Re-exporting replaces the bulk
pseudocode and function index; regenerate `function-offsets.json` afterward,
rerun failed-function retries, and update counts before relying on old offsets.
Do not run headless commands while the same project is open in the GUI.

## Apply findings to the 1.9 Go implementation

The retail/PTS label comes from the distribution thread; its precise build
provenance has not been independently established. Treat recovered behavior
as evidence for this specific binary hash. No formulas or packet layouts have
yet been validated against 1.9.

Version compatibility is a separate question from successful decompilation.
4.6 added and changed content, stats, item systems, class behavior and protocol.
Never transplant 4.6 opcodes, packet widths/order, map/NPC/item IDs, database
schemas, skill tables or quest state machines into the 1.9 implementation
without independent 1.9 evidence. Even familiar formulas may have changed.

For a candidate correction:

1. Read the existing Go helper and tests, then the corresponding Java code and
   1.9 static data. Check the maintained porting/audit notes for known findings.
2. Locate the 4.6 function using multiple clues. Follow callers and helpers to
   determine inputs, units, integer widths, overflow, clamps, ordering,
   randomness, and side effects. Confirm ambiguous pseudocode in assembly.
3. Write an evidence note with the binary hash and function addresses.
   Distinguish observed instructions from inferred meanings and unknowns.
4. Compare with an independent 1.9 reference: existing captured packets,
   historical Java 6 behavior, or a controlled real-client observation. State
   version-dependent differences explicitly. If evidence conflicts or is
   insufficient, record a hypothesis rather than applying a speculative fix.
5. Add a regression test that demonstrates the Go issue with independently
   justified expected results. Include meaningful boundaries and negative
   cases; do not construct an expected value by copying the implementation.
6. Fix the shared helper all callers use. Preserve 1.9 packet order, rounding,
   crypto and nil/empty semantics. Reuse existing helpers and manual wiring.
7. Run focused tests, then the full suite and vet sequentially. Where observable
   behavior changes, validate with the 1.9 client or the applicable recording.
   Report exact test counts and remaining uncertainty.

Useful Go entry points, relative to AionGo:

| Investigation | Existing code/tests to inspect first |
| --- | --- |
| Combat formulas | `go/game/combat.go`, `go/game/combat_test.go` |
| Character stats | `go/game/stats.go`, `go/game/lifestats.go`, `go/game/data/stats.go` |
| Skills and effects | `go/game/skill.go`, `go/game/skilleffects.go`, `go/game/effectctl.go`, related tests |
| Inventory | `go/game/inventory.go`, `go/game/inventory_test.go`, persistence tests |
| Quest dialogs | `go/game/quest_dialog.go`, `go/QUEST_HANDLER_AGENT.md`, conformance/parity tests |

Combat/stat/effect calculations are candidate investigation areas, not verified
bugs or guarantees of version compatibility. Quest and NPC behavior may require
analyzing NPCSvr64 or ScriptDLL first; a main-server export alone is insufficient.

For code changes, run from AionGo's root (substitute changed Go paths):

```sh
go/scripts/run-go.sh gofmt -w game/<changed-file>.go
go/scripts/run-go.sh go test -json ./game -run '<focused-test>'
go/scripts/run-go.sh go test -json ./...
go/scripts/run-go.sh go vet ./...
```

The helper runs in `go/`. Follow repository instructions for fixtures and
local builds; a source investigation does not authorize restarting the live
stack or publishing images. Keep Go/Java comparison databases separate.

## Evidence note template

```text
Subsystem / concrete symptom:
Go files and existing tests:
Binary SHA-256 / component:
Function addresses and important caller/callee addresses:
Observed assembly/pseudocode behavior:
Inferred names, units, types and assumptions:
4.6-versus-1.9 differences or unknowns:
Independent 1.9 reference and reproduction:
Proposed shared-helper correction:
Regression cases and expected results:
Validation commands, pass/fail counts and client evidence:
Remaining uncertainty / next action:
```

Keep binary artifacts, full pseudocode exports and local logs in the analysis
workspace. Put concise, non-sensitive findings and the resulting original Go
changes in AionGo. Update this handoff's analysis state when additional binaries
are analyzed; do not claim an investigation or gameplay fix is complete merely
because decompilation succeeded.

## Dungeon correctness investigation, 2026-10-05

The scoped plan is in `docs/DUNGEON_CORRECTNESS_PLAN.md`; task status
remains canonical in `go/PORTING.md`. All eight outcomes are planned.
Only source inspection and bounded pseudocode inspection were performed; no
new binaries were analyzed, no gameplay was tested and no server code changed.

The following addresses refer to the Server64 SHA-256 recorded above. Labels
come from embedded diagnostic strings, not fully recovered symbols. Observations
below are from pseudocode; caller/assembly confirmation is still required.

| Function/address | Observed evidence | Use for 1.9 investigation / limitation |
| --- | --- | --- |
| `DoorFieldObject::ToggleDoor`, `14034e540` | Checks object/user values and helper/virtual-call results before invoking `14034e850`; conditionally schedules an auto-closer and invokes a world callback. | Separate player-authorized operation from scripted state changes. Field meanings, distance/key semantics and timer units remain unconfirmed. |
| `DoorFieldObject::ToggleDoorNoCheck`, `14034e850` | Diagnostic string identifies the unchecked operation; branches to `14034ea30` or `14034ebf0` under object-state conditions with locking. | Trace open/close helpers and replication; do not copy pointer offsets or assume matching 1.9 door IDs. |
| `DoorAutoCloser::TimerExpired`, `14034e360` | A non-null stored pointer causes a call to `140804560` with a stored value. | Trace close/world lookup and instance lifetime; this alone does not establish 1.9 auto-close policy. |
| `Group::SetInsideInstance`, `1403c7980` | Stores a flag value of 1 and the supplied 32-bit argument in separate fields. | Lead for group binding, not proof of member/reconnect/retention rules. |
| `User::InvokeEndDungeonScript`, `140700850` | Looks up callbacks by supplied identifier and iterates callable entries using a user interface wrapper. | Locate script registration and dungeon completion/reward boundary; rewards themselves were not recovered here. |
| `World_IDDre::OnNPCDie`, `140843970` | Branches on NPC/template values, writes objective state, invokes update/end helpers and sets named `SWITCH_*_DESTROYED` / `TELEPORT_*_DESTROYED` flags. Includes a branch resolving another object from a killer-related identifier. | Strong Dredgion objective-event lead. Ownership/pet attribution is an inference requiring type/caller confirmation; none of its NPC IDs, score constants or later rules are approved for 1.9. |
| `World_IDDre::OnUserEnter`, `1408450b0` | Updates a looked-up entry under a lock for some entry modes, then delegates to `1407d4f70`. | Trace admission/rejoin synchronization; team/race and entry-mode meanings remain unconfirmed. |

Additional string-only leads: `DynamicWorld::StartBindingTimer`,
`DynamicWorld::BindingTimerExpired`, `WorldDb::GetInstanceLiveTimeAtNoUser_Party`,
`WorldDb::GetInstanceValidityTime`, `WorldDb::LoadInstanceRequisit`,
`WorldDb::LoadInstanceCooltime`, `DynamicWorldManager::SaveInstance` and
`World::InvokeEndDungeonScript_AllUser`. No behavior or addresses have yet
been established for these leads. Arenas and `World_IDDre2/3/4` are later-content
candidates and are excluded from the initial 1.9 scope.

Highest-value next analysis: trace checked door helpers and dynamic-world
lifetime/admission callers, then inventory NPC/script DLL exports and dungeon
assets for boss mechanics. Compare the two script DLL hashes before sharing an
analysis. Keep extracted exports in the existing analysis workspace.

## D0 source inventory, 2026-10-05

See [AION_19_DUNGEON_INVENTORY.md](AION_19_DUNGEON_INVENTORY.md) and its
[hashed JSON manifest](AION_19_DUNGEON_INVENTORY.json). The audit covers all
37 marked instance maps, with six priority dungeon manifests and 21 group
dungeon candidates. Independent 1.9 provenance and gameplay remain unresolved.
The source inventory records empty Steel Rake static population, missing
Dredgion portal admission, unenforced Dark Poeta XML item declarations and
concrete Nochsana/Fire Temple data relationships requiring verification.
Canonical AionGo task status was not changed: that sibling is read-only in
this session. No new decompilation or gameplay code change was performed.

## Dark Poeta authored AI follow-up, 2026-10-05

The extracted `_AION_LIVE_MAP/map.450319/XML/NpcAIPatterns.xml` and
`Worlds/idlf1/world.xml` supply plain authored NPC AI and routes in addition
to the native executable leads. These files are UTF-16 XML; parse them with
an XML reader rather than assuming UTF-8 text. The local 1.9 client resolves
Scar, both Ghost Elim, three Spaller bosses, generator housing/core variants,
Marabata controls and the bomb wall to matching symbolic AI names. This is
structural evidence, not proof that 4.6 health, timers or ground positions
are correct for 1.9.

See [the implementation handoff](aion-dungeon-fixes/README.md) and
[hashed evidence](aion-dungeon-fixes/dark-poeta-evidence.json). The 1.9 client
independently establishes Huge Vine 401111 and Essencetapping 300. Authored AI
establishes the Scar escort/ambush/final transformation lead, control-unit
messages, generator core spawns and the bomb-wall reaction. The original preparation session lacked write and container access, so its
draft patch had no Go or gameplay validation at that time. The
current implementation and validation are recorded in the correction handoff.
