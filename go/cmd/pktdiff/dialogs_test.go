package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func le32(v int32) []byte        { return binary.LittleEndian.AppendUint32(nil, uint32(v)) }
func le16(v uint16) []byte       { return binary.LittleEndian.AppendUint16(nil, v) }
func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func client(op string, body []byte) Packet { return Packet{Client: true, Op: op, Body: body} }
func server(op string, body []byte) Packet { return Packet{Op: op, Body: body} }

func npcInfo(object, template int32) Packet {
	return server("SM_NPC_INFO", cat(make([]byte, 12), le32(object), le32(template)))
}
func click(object int32) Packet { return client("CM_SHOW_DIALOG", le32(object)) }
func selectDialog(object int32, dialog uint16, quest int32) Packet {
	return client("CM_DIALOG_SELECT", cat(le32(object), le16(dialog), le16(1), le16(10), le32(quest), le16(0)))
}
func dlgWindow(object int32, dialog uint16, quest int32) Packet {
	return server("SM_DIALOG_WINDOW", cat(le32(object), le16(dialog), le32(quest), le16(0)))
}
func questAccepted(action byte, quest int32, status byte, vars int32) Packet {
	return server("SM_QUEST_ACCEPTED", cat([]byte{action}, le32(quest), []byte{status, 0}, le32(vars), le16(0)))
}
func death(object int32) Packet {
	return server("SM_ATTACK_STATUS", cat(le32(object), le32(5), []byte{5, 0}, le16(0), le16(0xa6)))
}

func dialogsReport(t *testing.T, java, goLog []Packet) (string, bool) {
	t.Helper()
	var out bytes.Buffer
	found := Dialogs(&out, java, goLog, Options{})
	return out.String(), found
}

func TestDialogsVerdictsAndFirstDivergence(t *testing.T) {
	// Object ids differ between runs; the npc template is what steps are keyed by.
	java := []Packet{npcInfo(0x6d8, 203071), click(0x6d8), dlgWindow(0x6d8, 10, 0),
		selectDialog(0x6d8, 25, 1001), dlgWindow(0x6d8, 1011, 1001),
		selectDialog(0x6d8, 10000, 1001), questAccepted(2, 1001, 3, 1), dlgWindow(0x6d8, 10, 0)}
	goLog := []Packet{npcInfo(0x501, 203071), click(0x501), dlgWindow(0x501, 10, 0),
		selectDialog(0x501, 25, 1001), dlgWindow(0x501, 1012, 1001),
		selectDialog(0x501, 10000, 1001), dlgWindow(0x501, 10, 0)}
	out, found := dialogsReport(t, java, goLog)
	if !found {
		t.Fatalf("expected a divergence\n%s", out)
	}
	for _, want := range []string{
		"  1 same     click npc 203071",
		"  2 differs  select dialog 25 quest 1001 on npc 203071",
		"  3 missing  select dialog 10000 quest 1001 on npc 203071",
		"! window 1011 q1001",
		"-- steps: 1 same, 1 differs, 1 missing in Go, 0 extra in Go",
		"-- first divergence: step 2 (differs)",
		"java: window 1011 q1001",
		"go:   window 1012 q1001",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestDialogsExtraStepAndIdenticalRuns(t *testing.T) {
	base := []Packet{npcInfo(1, 203071), click(1), dlgWindow(1, 10, 0)}
	if out, found := dialogsReport(t, base, base); found || !strings.Contains(out, "no divergence") {
		t.Fatalf("identical runs: found=%v\n%s", found, out)
	}
	more := append(append([]Packet{}, base...), client("CM_PLAY_MOVIE_END", cat([]byte{0}, le32(0), le32(0), le16(15), le32(0))))
	out, found := dialogsReport(t, base, more)
	if !found || !strings.Contains(out, "extra    movie 15 ended") || !strings.Contains(out, "(Java never did this action)") {
		t.Fatalf("found=%v\n%s", found, out)
	}
}

func TestDialogsKillIsAnNpcDeathAndSkipFilters(t *testing.T) {
	log := []Packet{npcInfo(7, 210670), death(7), death(7), questAccepted(2, 1001, 3, 2),
		server("SM_UPDATE_ITEM", []byte{1}), server("SM_UPDATE_ITEM", []byte{2})}
	steps := Timeline(log, TimelineOptions{})
	if len(steps) != 1 || steps[0].Action != "kill npc 210670" {
		t.Fatalf("steps = %+v", steps)
	}
	if got := strings.Join(steps[0].Effects, ";"); got != "quest 1001 START vars=2 (action 2);SM_UPDATE_ITEM x2" {
		t.Fatalf("effects = %q", got)
	}
	if got := Timeline(log, TimelineOptions{Skip: []string{"SM_UPDATE_ITEM"}})[0].Effects; len(got) != 1 {
		t.Fatalf("-skip left %v", got)
	}
}

func TestDeathByDieEmotionComesBeforeTheQuestUpdate(t *testing.T) {
	log := []Packet{npcInfo(7, 210670), server("SM_EMOTION", cat(le32(7), []byte{0x10})), questAccepted(2, 1001, 3, 2), death(7)}
	steps := Timeline(log, TimelineOptions{})
	if len(steps) != 1 || len(steps[0].Quests) != 1 {
		t.Fatalf("steps = %+v", steps)
	}
}

func TestVerdict(t *testing.T) {
	for _, tc := range []struct {
		java, goServer []string
		want           string
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, verdictSame},
		{[]string{"a", "b"}, []string{"a"}, verdictMissing},
		{[]string{"a"}, []string{"a", "b"}, verdictExtra},
		{[]string{"a", "b"}, []string{"b", "a"}, verdictDiffers},
		{[]string{"a"}, []string{"b"}, verdictDiffers},
	} {
		if got := verdict(tc.java, tc.goServer); got != tc.want {
			t.Errorf("verdict(%v, %v) = %s, want %s", tc.java, tc.goServer, got, tc.want)
		}
	}
}

func questList(quest int32, status byte, vars int32) Packet {
	return server("SM_QUEST_LIST", cat(le16(0), []byte{1}, le16(uint16(quest)), le16(0), []byte{status}, le32(vars), []byte{0}))
}

func TestGenTestWritesAReplayOfTheLastSession(t *testing.T) {
	old := []Packet{client("CM_ENTER_WORLD", nil), questList(1001, 6, 0), npcInfo(1, 1), click(1)}
	session := []Packet{client("CM_ENTER_WORLD", nil), questList(1001, 3, 0), npcInfo(9, 203071), npcInfo(8, 210670),
		click(9), dlgWindow(9, 10, 0),
		selectDialog(9, 1012, 1001), server("SM_PLAY_MOVIE", cat([]byte{0}, le32(0), le32(0), le16(15), le32(0))), dlgWindow(9, 1012, 1001),
		client("CM_PLAY_MOVIE_END", cat([]byte{0}, le32(0), le32(0), le16(15), le32(0))),
		death(8), questAccepted(2, 1001, 3, 2),
		server("SM_LOOT_ITEMLIST", cat(le32(8), []byte{1, 1}, le32(182200001), le16(2), le32(0))),
		client("CM_LOOT_ITEM", cat(le32(8), []byte{1})),
		client("CM_USE_ITEM", cat(le32(5), []byte{0}))}
	var out bytes.Buffer
	if err := GenTest(&out, 1001, append(old, session...), "java.log ready", []int32{182200001}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"func TestQuest1001JavaReplay(t *testing.T) {",
		`newReplayFixture(t, 1001, replayQuest{ID: 1001, Status: "START", Vars: 0})`,
		`Kind: "click", NPC: 203071, Want: replayWant{Windows: [][2]int32{{10, 0}}}`,
		`Kind: "select", NPC: 203071, Dialog: 1012, Quest: 1001, Want: replayWant{Windows: [][2]int32{{1012, 1001}}, Movies: []uint16{15}}`,
		`Kind: "movie", Movie: 15`,
		`Kind: "kill", NPC: 210670, Want: replayWant{Quests: []replayQuest{{Action: 2, ID: 1001, Status: "START", Vars: 2}}}`,
		`Kind: "give", Item: 182200001, Count: 2`,
		"// TODO not replayable: use item",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "LOCKED") {
		t.Errorf("used the first session's quest list:\n%s", got)
	}
	var none bytes.Buffer
	if err := GenTest(&none, 1001, []Packet{client("CM_MOVE", nil)}, "x", nil); err == nil {
		t.Error("a log without quest actions should be an error")
	}
}
