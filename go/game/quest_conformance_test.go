package game

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// Framework conformance for EVERY registered custom quest handler (see "Java conformance checklist" in
// QUEST_HANDLER_AGENT.md). The invariants are the ones the Java dialog framework guarantees whatever the handler
// does (NpcController.onDialogRequest / onDialogSelect, QuestEngine.onDialog):
//
//	click   a plain click (dialog -1) on any npc of a quest that has no state yet gives the main menu, window 10
//	echo    a select the handler does not know (1999) is answered by the window of the same number, with the quest
//	        id, or 0 when the client names no quest; and it changes nothing
//	movie   a page select (1000..9999) that plays a movie does not advance the quest (Java returns false there)
//	clickpage  a plain click in START or REWARD is never answered with a quest page: only the menu or, in REWARD,
//	        the reward window 5 (unless Java has a `case -1` for it)
//	finish  finishing a quest ends with the quest update then the main menu, the XP update before the XP message
//	accept  a startable quest only becomes started on 1002 (defaultQuestStartDialog) or 10000/10001
//	panic   no select, in any state, crashes the handler
//
// A failing rule is either a bug (listed in conformanceKnownFailing, tracked in QUEST_AUDIT.md, and removed from
// the list when fixed: TestQuestConformanceKnownFailingStillFail fails once it passes) or a documented Java
// deviation (conformanceExceptions, with the Java line). Both tables live in quest_conformance_known_test.go.

var conformanceRules = []string{"click", "clickpage", "echo", "movie", "accept", "finish", "panic"}

const conformanceUnknownDialog = 1999

// conformanceAcceptDialogs are the dialog ids that may start a quest.
var conformanceAcceptDialogs = []int32{1002, 10000, 10001}

// conformanceSweep is the dialog ids the movie and accept rules try on every npc in every state.
var conformanceSweep = func() []int32 {
	var out []int32
	add := func(from, to int32) {
		for d := from; d <= to; d++ {
			out = append(out, d)
		}
	}
	add(1, 60)
	add(1000, 1030)
	add(1350, 1360)
	add(1690, 1700)
	add(2370, 2380)
	add(2710, 2720)
	add(10000, 10005)
	return append(out, 10255)
}()

type conformanceFixture struct {
	s       *Server
	p       *player
	c       *conn
	packets *questPackets
	npcs    []*object
	initial []store.Quest
}

// conformanceNPCs is every npc the quest's handler is registered on.
func conformanceNPCs(d *data.Data, script *data.QuestScript) []int32 {
	ids := []int32{script.StartNPC, script.EndNPC, script.MiddleNPC, script.MiddleNPC2, script.MiddleNPC3, script.FinalNPC, script.LevelUpNPC}
	ids = append(ids, script.TalkNPCs...)
	for npc, list := range d.QuestCustomTalks {
		if slices.ContainsFunc(list, func(s *data.QuestScript) bool { return s.ID == script.ID }) {
			ids = append(ids, npc)
		}
	}
	slices.Sort(ids)
	return slices.Compact(slices.DeleteFunc(ids, func(id int32) bool { return id == 0 }))
}

// newConformanceFixture is a player who meets the quest's requirements, next to all its npcs, with the quest in
// the given status ("" for no state). It says whether the quest is startable for that player.
func newConformanceFixture(d *data.Data, script *data.QuestScript, status string, vars int32) (*conformanceFixture, bool) {
	template := d.Quests[script.ID]
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	if template != nil {
		if template.Race != "" {
			p.Race = template.Race
		}
		p.level = max(1, int(template.MinLevel))
		if len(template.ClassPermitted) != 0 {
			p.Class = template.ClassPermitted[0]
		}
		if template.GenderPermitted != "" {
			p.Gender = template.GenderPermitted
		}
		if template.CombineSkill != 0 {
			p.skills = append(p.skills, store.Skill{ID: template.CombineSkill, Level: template.CombineSkillPoint})
		}
		for _, id := range template.FinishedQuestConditions {
			p.quests = append(p.quests, store.Quest{ID: id, Status: "COMPLETE", CompleteCount: 1})
		}
	}
	p.seen = map[int32]*object{}
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: 182400001, Owner: p.ID}
	startable := s.canStartQuest(p, script)
	if status != "" {
		p.quests = append(p.quests, store.Quest{ID: script.ID, Status: status, Vars: vars})
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	f := &conformanceFixture{s: s, p: p, c: c, packets: packets, initial: slices.Clone(p.quests)}
	for i, id := range conformanceNPCs(d, script) {
		npc := questCatalogNPC(s, p, id, int32(0x40000+i))
		// The catalog npc has no stats to walk with; a task marks it as already walking (handlers that send an
		// npc off must not start a real ticker in a test).
		s.initNpc(npc)
		npc.watchers[p.ID] = p
		npc.move.task = newTask()
		f.npcs = append(f.npcs, npc)
	}
	return f, startable
}

func (f *conformanceFixture) reset() {
	f.p.quests = slices.Clone(f.initial)
	f.p.cube = nil
	f.packets.frames = nil
	// Despawn handlers remove NPCs from the player's known list. Restore the
	// fixture's visible NPCs before the next independent dialog probe.
	f.p.seen = make(map[int32]*object, len(f.npcs))
	for _, npc := range f.npcs {
		f.p.seen[npc.id] = npc
		npc.watchers[f.p.ID] = f.p
	}
}

func (f *conformanceFixture) state(id int32) string {
	if q := f.p.quest(id); q != nil {
		return fmt.Sprintf("%s/%d", q.Status, q.Vars)
	}
	return "none"
}

// try runs do and turns a panic into a message.
func try(do func()) (panicked string) {
	defer func() {
		if r := recover(); r != nil {
			panicked = fmt.Sprint(r)
			// The first frame of the game package under the panic says where it happened.
			for _, line := range strings.Split(string(debug.Stack()), "\n") {
				if i := strings.Index(line, "/game/"); i >= 0 && !strings.Contains(line, "quest_conformance_test.go") && !strings.Contains(line, "/runtime/") {
					panicked += " @ " + strings.Fields(line[i+1:])[0]
					break
				}
			}
		}
	}()
	do()
	return ""
}

// conformanceViolations is rule -> examples, for one quest.
type conformanceViolations map[string][]string

func (v conformanceViolations) add(rule, format string, args ...any) {
	if len(v[rule]) < 3 {
		v[rule] = append(v[rule], fmt.Sprintf(format, args...))
	}
}

func checkConformance(d *data.Data, script *data.QuestScript) conformanceViolations {
	v := conformanceViolations{}
	id := script.ID
	states := []store.Quest{{Status: ""}, {Status: "REWARD"}}
	for vars := int32(0); vars < 8; vars++ {
		states = append(states, store.Quest{Status: "START", Vars: vars})
	}
	for _, st := range states {
		status := st.Status
		f, startable := newConformanceFixture(d, script, status, st.Vars)
		if status == "" && startable {
			conformanceStartable++
		}
		for _, npc := range f.npcs {
			at := fmt.Sprintf("state=%s/%d npc=%d", conformanceStatus(status), st.Vars, npc.npc.ID)
			run := func(do func()) bool {
				f.reset()
				if msg := try(do); msg != "" {
					v.add("panic", "%s: %s", at, msg)
					return false
				}
				return true
			}
			sel := func(dialog int32, quest int32) func() {
				return func() { f.c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, uint16(dialog), quest)) }
			}
			if run(func() { f.c.showDialog(dialogRequest(cmShowDialog, npc.id, 0, 0)) }) {
				got := f.packets.windows(0)
				menu := [][2]int32{{10, 0}}
				if status == "" && (!slices.Equal(got, menu) || f.state(id) != "none") {
					v.add("click", "%s: windows %v state %s, want the main menu", at, got, f.state(id))
				} else if status != "" && slices.ContainsFunc(got, func(w [2]int32) bool { return w != menu[0] && (status != "REWARD" || w != [2]int32{5, id}) }) {
					v.add("clickpage", "%s: a plain click was answered with %v", at, got)
				}
			}
			for _, quest := range []int32{id, 0} {
				if run(sel(conformanceUnknownDialog, quest)) {
					if got := f.packets.windows(0); !slices.Equal(got, [][2]int32{{conformanceUnknownDialog, quest}}) || f.state(id) != conformanceState(f.initial, id) {
						v.add("echo", "%s quest=%d: windows %v state %s, want the echo", at, quest, got, f.state(id))
					}
				}
			}
			for _, dialog := range conformanceSweep {
				if !run(sel(dialog, id)) {
					continue
				}
				if f.packets.last(smPlayMovie) != nil {
					conformanceExercised["movie selects"]++
				}
				changed := f.state(id) != conformanceState(f.initial, id)
				if changed && dialog >= 1000 && dialog < 10000 && f.packets.last(smPlayMovie) != nil {
					v.add("movie", "%s d=%d: a movie page moved the quest to %s", at, dialog, f.state(id))
				}
				if q := f.p.quest(id); status == "REWARD" && q != nil && q.Status == "COMPLETE" {
					conformanceExercised["finishes"]++
					conformanceCheckFinish(v, f, at, dialog)
				}
				if status == "" && startable && changed {
					conformanceExercised["accepts"]++
				}
				if status == "" && startable && changed && !slices.Contains(conformanceAcceptDialogs, dialog) {
					v.add("accept", "%s d=%d: the quest became %s", at, dialog, f.state(id))
				}
			}
		}
	}
	return v
}

// conformanceCheckFinish: QuestService.questFinish sends the experience update before the "gained XP" message, the
// finished quest, and then the main menu as the only window.
func conformanceCheckFinish(v conformanceViolations, f *conformanceFixture, at string, dialog int32) {
	var accepted, window, expUpdate, expMessage = -1, -1, -1, -1
	for i, frame := range f.packets.frames {
		switch frame[0] {
		case smQuestAccepted:
			accepted = i
		case smDialogWindow:
			window = i
		case smStatupdateExp, smStatsInfo, smLevelUpdate: // a level-up replaces the XP update
			if expUpdate < 0 {
				expUpdate = i
			}
		case smSystemMessage:
			if binary.LittleEndian.Uint32(frame[7:]) == msgExp && expMessage < 0 {
				expMessage = i
			}
		}
	}
	if got := f.packets.windows(0); !slices.Equal(got, [][2]int32{{10, 0}}) || accepted < 0 || accepted > window {
		v.add("finish", "%s d=%d: windows %v, quest update at frame %d, window at %d; want the quest update then the main menu", at, dialog, got, accepted, window)
	} else if expMessage >= 0 && (expUpdate < 0 || expUpdate > expMessage) {
		v.add("finish", "%s d=%d: the gained-XP message (frame %d) came before the XP update (frame %d)", at, dialog, expMessage, expUpdate)
	}
}

func conformanceStatus(status string) string {
	if status == "" {
		return "none"
	}
	return status
}

// conformanceState is the quest's state in a quest list, formatted as conformanceFixture.state.
func conformanceState(quests []store.Quest, id int32) string {
	for _, q := range quests {
		if q.ID == id {
			return fmt.Sprintf("%s/%d", q.Status, q.Vars)
		}
	}
	return "none"
}

var (
	conformanceOnce      sync.Once
	conformanceResults   map[int32]conformanceViolations
	conformanceChecked   int
	conformanceStartable int
	conformanceExercised = map[string]int{}
)

// conformanceRun checks every registered custom handler (and every talk chain) once per test binary.
func conformanceRun(t *testing.T) map[int32]conformanceViolations {
	d := staticDataOrSkip(t)
	conformanceOnce.Do(func() {
		conformanceResults = map[int32]conformanceViolations{}
		for id, script := range d.QuestScripts {
			if script.Kind == data.QuestCustom || talkChains[id] != nil {
				conformanceChecked++
				conformanceResults[id] = checkConformance(d, script)
			}
		}
		if path := os.Getenv("QUEST_CONFORMANCE_DUMP"); path != "" {
			var lines []string
			for id, v := range conformanceResults {
				for rule, examples := range v {
					for _, e := range examples {
						lines = append(lines, fmt.Sprintf("%d\t%s\t%s", id, rule, e))
					}
				}
			}
			sort.Strings(lines)
			_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		}
	})
	return conformanceResults
}

func TestQuestConformance(t *testing.T) {
	results := conformanceRun(t)
	if conformanceChecked < 100 {
		t.Fatalf("only %d custom handlers checked", conformanceChecked)
	}
	t.Logf("%d registered handlers checked, %d known-failing rules, %d startable; exercised %v", conformanceChecked, len(conformanceKnownFailing), conformanceStartable, conformanceExercised)
	ids := make([]int32, 0, len(results))
	for id := range results {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		for _, rule := range conformanceRules {
			examples := results[id][rule]
			key := fmt.Sprintf("%d/%s", id, rule)
			if len(examples) == 0 || conformanceKnownFailing[key] || conformanceExceptions[key].Reason != "" {
				continue
			}
			t.Errorf("quest %d breaks the %q rule (fix the handler; if Java really does this, add it to conformanceExceptions with the Java line): %s", id, rule, examples[0])
		}
	}
}

// TestQuestConformanceKnownFailingStillFail keeps the known-failing list honest: a quest that now passes must be
// taken off it (and its row in QUEST_AUDIT.md closed), and an exception that no longer triggers must go too.
func TestQuestConformanceKnownFailingStillFail(t *testing.T) {
	results := conformanceRun(t)
	check := func(kind, key string) {
		var id int32
		var rule string
		fmt.Sscanf(strings.Replace(key, "/", " ", 1), "%d %s", &id, &rule)
		v, ok := results[id]
		if !ok {
			t.Errorf("%s %s: quest %d is not a registered custom handler any more", kind, key, id)
		} else if len(v[rule]) == 0 {
			t.Errorf("%s %s now passes: remove it from quest_conformance_known_test.go and close its QUEST_AUDIT.md row", kind, key)
		}
	}
	for key := range conformanceKnownFailing {
		check("known-failing", key)
	}
	for key := range conformanceExceptions {
		check("exception", key)
	}
}
