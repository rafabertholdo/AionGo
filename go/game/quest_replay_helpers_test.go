package game

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"aionlightning/game/store"
)

// Runner for the tests `pktdiff -gen-test <questId> <java.log> <label>` writes (quest_<id>_replay_test.go): a recorded
// Java run's client actions replayed through the Go dialog framework, asserting what the Java server answered.

// replayQuest is an SM_QUEST_ACCEPTED (Action set) or a quest row of the starting state (Action unused).
type replayQuest struct {
	Action byte
	ID     int32
	Status string
	Vars   int32
}

// replayWant is what the Java server sent for one step, in order. Every list is compared, so a nil list asserts that
// the step sent none of that kind.
type replayWant struct {
	Windows  [][2]int32 // SM_DIALOG_WINDOW (dialog id, quest id)
	Quests   []replayQuest
	Movies   []uint16
	Messages []int32 // SM_SYSTEM_MESSAGE ids
}

// replayStep is one client action: Kind is click, select, movie, kill (an npc of that template dies) or give (quest
// items looted). Kill steps assert only the quest actions, since mob experience is outside the quest event.
type replayStep struct {
	Name        string
	Kind        string
	NPC         int32
	Dialog      uint16
	Quest       int32
	Movie       uint16
	Item, Count int32
	Want        replayWant
}

type replayFixture struct {
	*chainFixture
}

// newReplayFixture is the chain fixture (Wrathchild for the quest's race, prerequisites completed) with the given quest
// rows as the starting state.
func newReplayFixture(t *testing.T, questID int32, initial ...replayQuest) *replayFixture {
	f := &replayFixture{newChainFixture(t, questID)}
	for _, q := range initial {
		f.p.quests = append(f.p.quests, store.Quest{ID: q.ID, Status: q.Status, Vars: q.Vars})
	}
	return f
}

func (f *replayFixture) npc(template int32) *object {
	o := f.npcs[template]
	if o == nil {
		o = questCatalogNPC(f.s, f.p, template, 0x32000+int32(len(f.npcs)))
		f.npcs[template] = o
	}
	return o
}

// seen is what the step sent, decoded from the captured frames (opcode first, then the body).
func (f *replayFixture) seen(from int) replayWant {
	var got replayWant
	for _, frame := range f.packets.frames[from:] {
		switch frame[0] {
		case smDialogWindow:
			got.Windows = append(got.Windows, [2]int32{int32(binary.LittleEndian.Uint16(frame[5:])), int32(binary.LittleEndian.Uint32(frame[7:]))})
		case smQuestAccepted:
			got.Quests = append(got.Quests, replayQuest{Action: frame[1], ID: int32(binary.LittleEndian.Uint32(frame[2:])),
				Status: statusOf(frame[6]), Vars: int32(binary.LittleEndian.Uint32(frame[8:]))})
		case smPlayMovie:
			got.Movies = append(got.Movies, binary.LittleEndian.Uint16(frame[10:]))
		case smSystemMessage:
			got.Messages = append(got.Messages, int32(binary.LittleEndian.Uint32(frame[7:])))
		}
	}
	return got
}

func statusOf(b byte) string {
	for name, value := range questStatus {
		if value == b {
			return name
		}
	}
	return fmt.Sprintf("STATUS%d", b)
}

// run replays the steps and stops at the first one whose answer differs from Java's.
func (f *replayFixture) run(steps []replayStep) {
	f.t.Helper()
	for i, step := range steps {
		from := len(f.packets.frames)
		switch step.Kind {
		case "click":
			f.c.showDialog(dialogRequest(cmShowDialog, f.npc(step.NPC).id, 0, 0))
		case "select":
			f.c.dialogSelect(dialogRequest(cmDialogSelect, f.npc(step.NPC).id, step.Dialog, step.Quest))
		case "movie":
			f.c.playMovieEnd(movieEnded(step.Movie))
		case "kill":
			f.s.recordQuestKill(f.npc(step.NPC), f.p)
		case "give":
			if !f.s.addItem(f.p, step.Item, int64(step.Count)) {
				f.t.Fatalf("step %d %q: cannot grant item %d x%d", i+1, step.Name, step.Item, step.Count)
			}
			continue
		default:
			f.t.Fatalf("step %d %q: unknown kind %q", i+1, step.Name, step.Kind)
		}
		got := f.seen(from)
		want := step.Want
		if step.Kind == "kill" {
			got = replayWant{Quests: got.Quests}
		}
		if !reflect.DeepEqual(normalizeWant(got), normalizeWant(want)) {
			f.t.Fatalf("step %d %q diverges from the Java run\n  java: %+v\n  go:   %+v", i+1, step.Name, want, got)
		}
	}
}

// normalizeWant makes nil and empty lists equal.
func normalizeWant(w replayWant) replayWant {
	if len(w.Windows) == 0 {
		w.Windows = nil
	}
	if len(w.Quests) == 0 {
		w.Quests = nil
	}
	if len(w.Movies) == 0 {
		w.Movies = nil
	}
	if len(w.Messages) == 0 {
		w.Messages = nil
	}
	return w
}
