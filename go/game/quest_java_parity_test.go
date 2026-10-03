package game

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// TestQuestJavaParity replays, against the Go handlers, every probe the Java harness
// (java/AL-Game/src/test/java/com/aionemu/gameserver/questEngine/QuestTraceDump.java) ran against the Java handlers:
// each quest in each reachable state, on each of its npcs, with a plain click and a sweep of dialog selects. A probe
// differs when the quest packets (dialog windows, quest updates, movies, system messages), the quest state after it,
// the inventory change or the experience gained are not the same. Generate the traces with
// scripts/quest-parity.sh, which then runs this test; one quest: scripts/quest-parity.sh 1001.
//
// The traces are not checked in, so without them the test skips.

const (
	parityTraces = "../.build/java_quest_traces.jsonl"
	parityReport = "../.build/quest_parity_report.tsv"
)

// parityOps are the packets compared byte for byte; the rest depend on the fixture (stats, items' object ids).
var parityOps = map[byte]string{smDialogWindow: "win", smQuestAccepted: "quest", smPlayMovie: "movie", smSystemMessage: "msg", smUseObject: "use"}

type parityQuest struct {
	Quest  int32    `json:"quest"`
	Race   string   `json:"race"`
	Class  string   `json:"class"`
	Gender string   `json:"gender"`
	Level  int      `json:"level"`
	NPCs   []int32  `json:"npcs"`
	States []string `json:"states"`
	// Reached is how many of States the dialogs reach from no quest; the rest are seeded (kills, items, timers).
	Reached int     `json:"reached"`
	Sweep   []int32 `json:"sweep"`
	probes  map[string]parityProbe
}

type parityProbe struct {
	NPC    int32            `json:"npc"`
	State  string           `json:"state"`
	Dialog int32            `json:"dialog"`
	After  string           `json:"after"`
	Out    []string         `json:"out"`
	Items  map[string]int64 `json:"items"`
	Exp    int64            `json:"exp"`
	Err    string           `json:"err"`
}

func (p parityProbe) key() string { return fmt.Sprintf("%s %d %d", p.State, p.NPC, p.Dialog) }

func loadParityTraces(t *testing.T) []*parityQuest {
	f, err := os.Open(parityTraces)
	if err != nil {
		t.Skipf("no Java quest traces (%v); run scripts/quest-parity.sh", err)
	}
	defer f.Close()
	var quests []*parityQuest
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<24)
	for sc.Scan() {
		line := sc.Bytes()
		if strings.HasPrefix(string(line), `{"quest"`) {
			q := &parityQuest{probes: map[string]parityProbe{}}
			if err := json.Unmarshal(line, q); err != nil {
				t.Fatal(err)
			}
			quests = append(quests, q)
			continue
		}
		var p parityProbe
		if err := json.Unmarshal(line, &p); err != nil {
			t.Fatal(err)
		}
		quests[len(quests)-1].probes[p.key()] = p
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return quests
}

func TestQuestJavaParity(t *testing.T) {
	quests := loadParityTraces(t)
	d := staticDataOrSkip(t)
	var compared, differing, unported int
	var report []string
	for _, q := range quests {
		script := d.QuestScripts[q.Quest]
		if script == nil {
			unported++
			continue
		}
		compared++
		t.Run(strconv.Itoa(int(q.Quest)), func(t *testing.T) {
			diffs := parityCheck(d, script, q)
			if len(diffs) == 0 {
				return
			}
			differing++
			for _, d := range diffs {
				report = append(report, fmt.Sprintf("%d\t%s", q.Quest, d))
			}
			for i, d := range diffs {
				if i == 10 {
					t.Errorf("... %d more differing probes", len(diffs)-i)
					break
				}
				t.Error(d)
			}
		})
	}
	if err := os.WriteFile(parityReport, []byte(strings.Join(report, "\n")+"\n"), 0o644); err != nil {
		t.Log(err)
	}
	t.Logf("every differing probe: go/.build/quest_parity_report.tsv")
	t.Logf("%d Java quests: %d compared, %d differ, %d not ported to Go; %d probes skipped where Java threw", len(quests), compared, differing, unported, parityJavaThrew)
}

func parityCheck(d *data.Data, script *data.QuestScript, q *parityQuest) []string {
	var diffs []string
	for i, state := range q.States {
		seeded := ""
		if i >= q.Reached {
			seeded = " (seeded state)"
		}
		for _, npcID := range q.NPCs {
			for _, dialog := range q.Sweep {
				want, ok := q.probes[fmt.Sprintf("%s %d %d", state, npcID, dialog)]
				if !ok {
					if dialog != -1 && slices.Contains(javaServiceDialogs, dialog) {
						continue // NpcController served it; nothing quest-specific to compare
					}
					want = parityDefault(q.Quest, state, npcID, dialog)
				}
				if want.Err != "" {
					// The Java handler threw (e.g. a reward index past the list): no client sees a defined answer,
					// and fixture gaps (no world, no spawns) show up here too.
					parityJavaThrew++
					continue
				}
				if parityDeviation(script, state, dialog) {
					continue
				}
				got := parityRun(d, script, q, state, npcID, dialog)
				if diff := parityDiff(d, want, got); diff != "" {
					diffs = append(diffs, fmt.Sprintf("state=%s%s npc=%d dialog=%d: %s", state, seeded, npcID, dialog, diff))
				}
			}
		}
	}
	return diffs
}

// javaServiceDialogs are the ids NpcController.onDialogSelect serves itself when no quest handler answers.
var javaServiceDialogs = []int32{2, 3, 4, 5, 6, 7, 20, 27, 29, 30, 31, 35, 36, 37, 38, 39, 40, 41, 42, 47, 50, 52, 53, 60, 61}

const parityNPC = 0x40000

var parityJavaThrew int

// parityDefault is what the Java harness leaves out: the framework's answer and nothing else.
func parityDefault(quest int32, state string, npc, dialog int32) parityProbe {
	w := dialogWindow(parityNPC, uint16(dialog), quest)
	if dialog == -1 {
		w = dialogWindow(parityNPC, 10, 0)
	}
	return parityProbe{NPC: npc, State: state, Dialog: dialog, After: state,
		Out: []string{fmt.Sprintf("%02x SM_DIALOG_WINDOW %x", w.Data[0], w.Data[1:])}}
}

// parityRun is one probe on the Go side: a fresh player in state, next to one npc with the Java harness' object id.
func parityRun(d *data.Data, script *data.QuestScript, q *parityQuest, state string, npcID, dialog int32) (got parityProbe) {
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.Gender, p.level = q.Race, q.Class, q.Gender, q.Level
	p.Exp = d.ExpStart(q.Level) // PlayerCommonData.setExp(getStartExpForLevel(level)) in the Java harness
	if template := d.Quests[q.Quest]; template != nil {
		for _, id := range template.FinishedQuestConditions {
			p.quests = append(p.quests, store.Quest{ID: id, Status: "COMPLETE", CompleteCount: 1})
		}
	}
	if state != "none" {
		status, vars, _ := strings.Cut(state, "/")
		v, _ := strconv.Atoi(vars)
		p.quests = append(p.quests, store.Quest{ID: q.Quest, Status: status, Vars: int32(v)})
	}
	p.seen = map[int32]*object{}
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: 182400001, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	npc := questCatalogNPC(s, p, npcID, parityNPC)
	if t := d.Npcs[npcID]; t != nil {
		npc.npc = t // the npc type decides the click (USEITEM: ActionitemController)
	}
	s.initNpc(npc)
	npc.watchers[p.ID] = p
	npc.move.task = newTask()
	p.targetID = npc.id // the client selects the npc before talking to it, as in the Java harness

	before, exp := parityInventory(p), p.Exp
	got = parityProbe{NPC: npcID, State: state, Dialog: dialog}
	got.Err = try(func() {
		if dialog == -1 {
			c.showDialog(dialogRequest(cmShowDialog, npc.id, 0, 0))
		} else {
			c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, uint16(dialog), q.Quest))
		}
	})
	for _, f := range packets.frames {
		if name, ok := parityOps[f[0]]; ok {
			got.Out = append(got.Out, fmt.Sprintf("%02x %s %x", f[0], name, f[1:]))
		}
	}
	got.After = "none"
	if st := p.quest(q.Quest); st != nil {
		got.After = fmt.Sprintf("%s/%d", st.Status, st.Vars)
	}
	got.Items = map[string]int64{}
	after := parityInventory(p)
	for id := range maps.Keys(after) {
		before[id] += 0
	}
	for id, n := range before {
		if delta := after[id] - n; delta != 0 {
			got.Items[strconv.Itoa(int(id))] = delta
		}
	}
	got.Exp = p.Exp - exp
	return got
}

func parityInventory(p *player) map[int32]int64 {
	m := map[int32]int64{}
	if p.kinah != nil {
		m[p.kinah.ItemID] = p.kinah.Count
	}
	for _, it := range p.cube {
		if it != nil {
			m[it.ItemID] += it.Count
		}
	}
	return m
}

// parityDiff compares the Java probe with the Go one; "" when they agree.
// Updates of quests that have no Go handler yet (another quest's level-up start) are left out.
func parityDiff(d *data.Data, java, gop parityProbe) string {
	var jout []string
	for _, o := range java.Out {
		op, _ := strconv.ParseUint(o[:2], 16, 8)
		if _, ok := parityOps[byte(op)]; !ok {
			continue
		}
		o = parityDescribe(o)
		if id := parityQuestID(o); id > 0 && d.QuestScripts[int32(id)] == nil {
			continue
		}
		jout = append(jout, o)
	}
	var gout []string
	for _, o := range gop.Out {
		o = parityDescribe(o)
		if id := parityQuestID(o); id > 0 && d.QuestScripts[int32(id)] == nil {
			continue
		}
		gout = append(gout, o)
	}
	paritySortQuestRuns(jout)
	paritySortQuestRuns(gout)
	var parts []string
	if !slices.Equal(jout, gout) {
		parts = append(parts, fmt.Sprintf("packets java %v go %v", jout, gout))
	}
	if java.After != gop.After {
		parts = append(parts, fmt.Sprintf("state java %s go %s", java.After, gop.After))
	}
	if !maps.Equal(java.Items, gop.Items) && (len(java.Items) != 0 || len(gop.Items) != 0) {
		parts = append(parts, fmt.Sprintf("items java %v go %v", java.Items, gop.Items))
	}
	if java.Exp != gop.Exp {
		parts = append(parts, fmt.Sprintf("exp java %+d go %+d", java.Exp, gop.Exp))
	}
	if gop.Err != "" {
		parts = append(parts, "go panicked: "+gop.Err)
	}
	return strings.Join(parts, "; ")
}

// parityDescribe turns "<op> <name> <hex>" into a short readable form, keeping the bytes it does not decode.
func parityDescribe(o string) string {
	f := strings.Fields(o)
	op, _ := strconv.ParseUint(f[0], 16, 8)
	b, _ := hex.DecodeString(f[len(f)-1])
	le16 := func(i int) int { return int(binary.LittleEndian.Uint16(b[i:])) }
	le32 := func(i int) int { return int(int32(binary.LittleEndian.Uint32(b[i:]))) }
	switch byte(op) {
	case smDialogWindow:
		if len(b) >= 10 {
			return fmt.Sprintf("win(%d,q%d)", le16(4), le32(6))
		}
	case smQuestAccepted:
		if len(b) >= 11 {
			for name, v := range questStatus {
				if v == b[5] {
					return fmt.Sprintf("quest(a%d,q%d,%s/%d)", b[0], le32(1), name, le32(7))
				}
			}
			return fmt.Sprintf("quest(a%d,q%d,%d/%d)", b[0], le32(1), b[5], le32(7))
		}
	case smPlayMovie:
		if len(b) >= 11 {
			return fmt.Sprintf("movie(%d,type%d)", le16(9), b[0])
		}
	case smUseObject:
		if len(b) >= 13 {
			return fmt.Sprintf("use(%d,%d)", le32(8), b[12])
		}
	case smSystemMessage:
		if len(b) >= 10 {
			return fmt.Sprintf("msg(%d:%x)", le32(6), b[10:])
		}
	}
	return f[1] + ":" + f[len(f)-1]
}

// parityDeviation is where Go deliberately differs from Java:
//   - a monster hunt is not handed in (25 turn-in page, 1009 reward) before every kill count is reached; Java lets it
//     through with any lower count;
//   - 1146 (Delicate Mandrake) can be finished: Java's handler returns false for 1009 and 8-17 in REWARD, so the
//     quest could never be completed there;
//   - a report_to quest whose start npc is also its end npc can be handed in (Java's start branch swallows it).
func parityDeviation(script *data.QuestScript, state string, dialog int32) bool {
	status, vars, _ := strings.Cut(state, "/")
	v, _ := strconv.Atoi(vars)
	if script.ID == delicateMandrakeQuestID && status == "REWARD" && (dialog == 1009 || dialog >= 8 && dialog <= 17) {
		return true
	}
	if script.Kind == data.QuestReportTo && script.StartNPC == script.EndNPC && status != "none" {
		return true // a report_to quest whose start npc is its end npc can be handed in (Java never reaches its end branch)
	}
	return script.Kind == data.QuestMonsterHunt && status == "START" && (dialog == 25 || dialog == 1009) &&
		!monsterHuntComplete(&store.Quest{Vars: int32(v)}, script)
}

// paritySortQuestRuns orders each run of consecutive quest updates by quest id (stable, so one quest's updates keep
// their order): QuestEngine.onLvlUp runs the level-up handlers in script load order, which is not part of the port.
func paritySortQuestRuns(out []string) {
	questID := parityQuestID
	for i := 0; i < len(out); {
		j := i
		for j < len(out) && questID(out[j]) >= 0 {
			j++
		}
		if j > i {
			slices.SortStableFunc(out[i:j], func(a, b string) int { return questID(a) - questID(b) })
			i = j
		} else {
			i++
		}
	}
}

// parityQuestID is the quest id of a described quest update, -1 for other packets.
func parityQuestID(o string) int {
	if !strings.HasPrefix(o, "quest(") {
		return -1
	}
	_, rest, _ := strings.Cut(o, ",q")
	id, _, _ := strings.Cut(rest, ",")
	n, _ := strconv.Atoi(id)
	return n
}
