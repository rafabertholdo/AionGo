package main

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
)

// The quest-relevant client actions and what the server answered, in log order. The server packets that follow an
// action (up to the next action) are its effects; ids that differ between runs (object ids) are replaced by npc
// template ids, so a Java and a Go timeline of the same play-through compare directly.

// Step is one client action (or an NPC death) and the server effects that followed it.
type Step struct {
	Kind     string // click, select, movie, question, use, kill, loot
	NPC      int32  // npc template id, 0 when the log never showed this object's SM_NPC_INFO
	Object   int32
	Dialog   uint16
	Quest    int32
	Movie    uint16
	Question int32
	Answer   byte
	Item     int32 // loot: item template id
	Count    int32
	Key      string // what alignment pairs steps by
	Action   string // one-line rendering

	Effects  []string // rendered, consecutive repeats collapsed ("SM_UPDATE_ITEM x3")
	Windows  [][2]int32
	Quests   []QuestAct
	Movies   []uint16
	Messages []int32
}

// QuestAct is an SM_QUEST_ACCEPTED.
type QuestAct struct {
	Action byte
	ID     int32
	Status string
	Vars   int32
}

// questStatusNames is QuestStatus.value (game/worldpackets.go questStatus).
var questStatusNames = map[byte]string{0: "NONE", 3: "START", 4: "REWARD", 5: "COMPLETE", 6: "LOCKED"}

func statusName(b byte) string {
	if n, ok := questStatusNames[b]; ok {
		return n
	}
	return fmt.Sprintf("STATUS%d", b)
}

// TimelineOptions selects what a timeline shows.
type TimelineOptions struct {
	Only, Skip []string // effect opcodes (default: the quest set in effectOpcodes)
	Loot       bool     // also produce a step per CM_LOOT_ITEM (for -gen-test)
}

// effectOpcodes are the server packets rendered as effects by default.
var effectOpcodes = []string{
	"SM_DIALOG_WINDOW", "SM_QUEST_ACCEPTED", "SM_PLAY_MOVIE", "SM_SYSTEM_MESSAGE", "SM_NEARBY_QUESTS", "SM_QUEST_LIST",
	"SM_ADD_ITEMS", "SM_UPDATE_ITEM", "SM_DELETE_ITEM", "SM_STATUPDATE_EXP", "SM_LEVEL_UPDATE",
}

// emotionDie is EmotionType.DIE.
const emotionDie = 0x10

type lootEntry struct{ item, count int32 }

// Timeline extracts the steps of a log.
func Timeline(packets []Packet, options TimelineOptions) []Step {
	effects := set(effectOpcodes)
	if len(options.Only) > 0 {
		effects = set(options.Only)
	}
	skip := set(options.Skip)
	npcs := map[int32]int32{}        // object id -> npc template
	dead := map[int32]bool{}         // npc objects whose death was already a step
	loot := map[[2]int32]lootEntry{} // (target object, index) -> what the list offered
	var steps []Step
	var lastExp int64 = -1
	start := func(s Step) {
		if s.NPC == 0 && s.Object != 0 {
			s.NPC = npcs[s.Object]
		}
		steps = append(steps, s)
	}
	for _, p := range packets {
		b := p.Body
		if p.Client {
			switch p.Op {
			case "CM_SHOW_DIALOG":
				if len(b) >= 4 {
					start(Step{Kind: "click", Object: i32(b, 0)})
				}
			case "CM_DIALOG_SELECT":
				if len(b) >= 16 {
					start(Step{Kind: "select", Object: i32(b, 0), Dialog: binary.LittleEndian.Uint16(b[4:]), Quest: i32(b, 10)})
				}
			case "CM_PLAY_MOVIE_END":
				if len(b) >= 11 {
					start(Step{Kind: "movie", Movie: binary.LittleEndian.Uint16(b[9:])})
				}
			case "CM_QUESTION_RESPONSE":
				if len(b) >= 5 {
					start(Step{Kind: "question", Question: i32(b, 0), Answer: b[4]})
				}
			case "CM_USE_ITEM":
				if len(b) >= 4 {
					start(Step{Kind: "use", Item: i32(b, 0)})
				}
			case "CM_LOOT_ITEM":
				if options.Loot && len(b) >= 5 {
					e := loot[[2]int32{i32(b, 0), int32(b[4])}]
					start(Step{Kind: "loot", Object: i32(b, 0), Item: e.item, Count: e.count})
				}
			}
			continue
		}
		switch p.Op {
		case "SM_NPC_INFO":
			if len(b) >= 20 {
				npcs[i32(b, 12)] = i32(b, 16)
				delete(dead, i32(b, 12)) // a respawn re-sends the npc, possibly under the same object id
			}
		case "SM_ATTACK_STATUS", "SM_EMOTION":
			// A death is the die emotion (0x10) or the status update leaving the creature at 0% health, whichever comes
			// first (a quest handler runs between them).
			isDeath := p.Op == "SM_EMOTION" && len(b) >= 5 && b[4] == emotionDie || p.Op == "SM_ATTACK_STATUS" && len(b) >= 10 && b[9] == 0
			if isDeath && npcs[i32(b, 0)] != 0 && !dead[i32(b, 0)] {
				dead[i32(b, 0)] = true
				start(Step{Kind: "kill", Object: i32(b, 0)})
			}
		case "SM_LOOT_ITEMLIST":
			if len(b) >= 5 {
				target, n := i32(b, 0), int(b[4])
				for i, off := 0, 5; i < n && off+11 <= len(b); i, off = i+1, off+11 {
					key := [2]int32{target, int32(b[off])}
					if _, ok := loot[key]; !ok {
						loot[key] = lootEntry{i32(b, off+1), int32(binary.LittleEndian.Uint16(b[off+5:]))}
					}
				}
			}
		}
		if len(steps) == 0 || !effects[p.Op] || skip[p.Op] {
			continue
		}
		cur := &steps[len(steps)-1]
		text := ""
		switch p.Op {
		case "SM_DIALOG_WINDOW":
			if len(b) >= 10 {
				w := [2]int32{int32(binary.LittleEndian.Uint16(b[4:])), i32(b, 6)}
				cur.Windows = append(cur.Windows, w)
				text = fmt.Sprintf("window %d", w[0])
				if w[1] != 0 {
					text += fmt.Sprintf(" q%d", w[1])
				}
			}
		case "SM_QUEST_ACCEPTED":
			if len(b) >= 11 {
				q := QuestAct{Action: b[0], ID: i32(b, 1), Status: statusName(b[5]), Vars: i32(b, 7)}
				cur.Quests = append(cur.Quests, q)
				text = fmt.Sprintf("quest %d %s vars=%d (action %d)", q.ID, q.Status, q.Vars, q.Action)
			}
		case "SM_PLAY_MOVIE":
			if len(b) >= 11 {
				id := binary.LittleEndian.Uint16(b[9:])
				cur.Movies = append(cur.Movies, id)
				text = fmt.Sprintf("movie %d", id)
			}
		case "SM_SYSTEM_MESSAGE":
			if len(b) >= 10 {
				id := i32(b, 6)
				cur.Messages = append(cur.Messages, id)
				text = fmt.Sprintf("sysmsg %d", id)
			}
		case "SM_NEARBY_QUESTS":
			text = nearbyText(b)
		case "SM_QUEST_LIST":
			text = questListText(b)
		case "SM_STATUPDATE_EXP":
			if len(b) >= 8 {
				exp := int64(binary.LittleEndian.Uint64(b))
				if lastExp >= 0 && exp != lastExp {
					text = fmt.Sprintf("exp %+d", exp-lastExp)
				} else if lastExp < 0 {
					text = "exp (first update)"
				}
				lastExp = exp
			}
		default:
			text = p.Op
		}
		if text != "" {
			cur.Effects = append(cur.Effects, text)
		}
	}
	for i := range steps {
		steps[i].finish()
	}
	return steps
}

func i32(b []byte, off int) int32 { return int32(binary.LittleEndian.Uint32(b[off:])) }

func (s *Step) finish() {
	who := fmt.Sprintf("npc %d", s.NPC)
	if s.NPC == 0 {
		who = "npc ?"
	}
	switch s.Kind {
	case "click":
		s.Key, s.Action = "click "+who, "click "+who
	case "select":
		s.Key = fmt.Sprintf("select %d q%d %s", s.Dialog, s.Quest, who)
		s.Action = fmt.Sprintf("select dialog %d quest %d on %s", s.Dialog, s.Quest, who)
	case "movie":
		s.Key, s.Action = fmt.Sprintf("movie %d", s.Movie), fmt.Sprintf("movie %d ended", s.Movie)
	case "question":
		s.Key = fmt.Sprintf("question %d %d", s.Question, s.Answer)
		s.Action = fmt.Sprintf("question %d answered %d", s.Question, s.Answer)
	case "use":
		s.Key, s.Action = "use item", fmt.Sprintf("use item uid=%#x", s.Item)
	case "kill":
		s.Key, s.Action = "kill "+who, "kill "+who
	case "loot":
		s.Key, s.Action = "loot", fmt.Sprintf("loot item %d x%d", s.Item, s.Count)
	}
	s.Effects = collapse(s.Effects)
}

// collapse folds consecutive equal effects into "effect xN".
func collapse(in []string) []string {
	var out []string
	for i := 0; i < len(in); {
		j := i
		for j < len(in) && in[j] == in[i] {
			j++
		}
		if j-i > 1 {
			out = append(out, fmt.Sprintf("%s x%d", in[i], j-i))
		} else {
			out = append(out, in[i])
		}
		i = j
	}
	return out
}

func nearbyText(b []byte) string {
	if len(b) < 4 {
		return "SM_NEARBY_QUESTS"
	}
	n := int(i32(b, 0))
	var ids []string
	for i, off := 0, 4; i < n && off+4 <= len(b); i, off = i+1, off+4 {
		ids = append(ids, fmt.Sprintf("%d/%d", binary.LittleEndian.Uint16(b[off:]), binary.LittleEndian.Uint16(b[off+2:])))
	}
	slices.Sort(ids)
	return "nearby [" + strings.Join(ids, " ") + "]"
}

// startedQuests decodes SM_QUEST_LIST's in-progress quests.
func startedQuests(b []byte) (started []QuestAct, ok bool) {
	if len(b) < 3 {
		return nil, false
	}
	off := 2 + 5*int(binary.LittleEndian.Uint16(b))
	if off >= len(b) {
		return nil, false
	}
	n := int(b[off])
	off++
	if off+4*n+6*n > len(b) {
		return nil, false
	}
	for i := 0; i < n; i++ {
		started = append(started, QuestAct{ID: int32(binary.LittleEndian.Uint16(b[off+4*i:]))})
	}
	off += 4 * n
	for i := 0; i < n; i++ {
		started[i].Status, started[i].Vars = statusName(b[off]), i32(b, off+1)
		off += 6
	}
	return started, true
}

func questListText(b []byte) string {
	started, ok := startedQuests(b)
	if !ok {
		return "SM_QUEST_LIST"
	}
	var parts []string
	for _, q := range started {
		parts = append(parts, fmt.Sprintf("%d:%s:%d", q.ID, q.Status, q.Vars))
	}
	return "questList started [" + strings.Join(parts, " ") + "]"
}
