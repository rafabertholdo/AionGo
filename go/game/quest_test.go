package game

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"aionlightning/game/store"
	"aionlightning/wire"
)

type recordedQuests struct {
	saved   []store.Quest
	rewards []store.QuestCompletion
	err     error
}

func TestQuestAcceptedMatchesClient19(t *testing.T) {
	// The 1.9 server's play-session capture: step 1 of quest 1102, and the
	// beginning of quest 1103 after talking to the NPC.
	for _, tc := range []struct {
		action byte
		quest  store.Quest
		want   string
	}{
		{2, store.Quest{ID: 1102, Status: "START", Vars: 1}, "024e0400000300010000000000"},
		{1, store.Quest{ID: 1103, Status: "START"}, "014f0400000300000000000000"},
	} {
		got := hex.EncodeToString(questAccepted(tc.action, tc.quest).Data[1:])
		if got != tc.want {
			t.Errorf("quest %d packet = %s, want %s", tc.quest.ID, got, tc.want)
		}
	}
}

func (r *recordedQuests) SaveQuest(_ int32, quest store.Quest) error {
	if r.err != nil {
		return r.err
	}
	r.saved = append(r.saved, quest)
	return nil
}

func (r *recordedQuests) CompleteQuest(_ int32, quest store.Quest, reward store.QuestCompletion) error {
	if r.err != nil {
		return r.err
	}
	r.saved = append(r.saved, quest)
	r.rewards = append(r.rewards, reward)
	return nil
}

type questPackets struct{ frames [][]byte }

func (r *questPackets) tap(w *wire.Writer) {
	r.frames = append(r.frames, bytes.Clone(w.Data))
}

func (r *questPackets) last(opcode byte) []byte {
	for index := len(r.frames) - 1; index >= 0; index-- {
		if r.frames[index][0] == opcode {
			return r.frames[index]
		}
	}
	return nil
}

func movieEnded(id uint16) *wire.Reader {
	w := wire.Packet(cmPlayMovieEnd)
	w.C(1)
	w.D(0)
	w.D(0)
	w.H(id)
	w.D(0)
	return wire.NewReader(w.Data[1:])
}

func TestPrologueStartsAndFinishes(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		race      string
		questID   int32
		movieID   uint16
		startData []byte
		movieData []byte
	}{
		{"ELYOS", 1000, 1, []byte{0x78, 1, 0xe8, 3, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0}, []byte{0x87, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0}},
		{"ASMODIANS", 2000, 2, []byte{0x78, 1, 0xd0, 7, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0}, []byte{0x87, 1, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0}},
	} {
		t.Run(tc.race, func(t *testing.T) {
			s := testServer(d)
			saver := &recordedQuests{}
			s.quests = saver
			p := wrathchild(s)
			p.Race = tc.race
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			originalExp := p.Exp

			c.startPrologue()
			if len(saver.saved) != 1 || len(p.quests) != 1 || p.quests[0].Status != "START" {
				t.Fatalf("quest did not start and persist: saved=%+v player=%+v", saver.saved, p.quests)
			}
			if got := packets.last(smQuestAccepted); !bytes.Equal(got, tc.startData) {
				t.Errorf("start packet = %x, want %x", got, tc.startData)
			}
			if got := packets.last(smPlayMovie); !bytes.Equal(got, tc.movieData) {
				t.Errorf("movie packet = %x, want %x", got, tc.movieData)
			}
			if len(packets.frames) != 3 || packets.frames[1][0] != smNearbyQuests {
				t.Errorf("start packet sequence = %x", packets.frames)
			}

			c.playMovieEnd(movieEnded(tc.movieID + 1))
			if len(saver.saved) != 1 {
				t.Fatal("an unrelated movie finished the quest")
			}
			c.playMovieEnd(movieEnded(tc.movieID))
			if len(saver.saved) != 2 || p.quests[0].Status != "COMPLETE" || p.quests[0].CompleteCount != 1 {
				t.Fatalf("quest did not finish and persist: saved=%+v player=%+v", saver.saved, p.quests)
			}
			if p.Exp != originalExp+d.Quests[tc.questID].Rewards[0].Experience {
				t.Errorf("experience = %d, want %d", p.Exp, originalExp+d.Quests[tc.questID].Rewards[0].Experience)
			}
			wantComplete := bytes.Clone(tc.startData)
			wantComplete[1], wantComplete[6] = 2, 5
			if got := packets.last(smQuestAccepted); !bytes.Equal(got, wantComplete) {
				t.Errorf("complete packet = %x, want %x", got, wantComplete)
			}
			frameCount := len(packets.frames)
			c.playMovieEnd(movieEnded(tc.movieID))
			c.startPrologue()
			if len(saver.saved) != 2 || len(packets.frames) != frameCount {
				t.Error("completed quest was rewarded or replayed again")
			}
		})
	}
}

func TestPrologueResumeAndSaveFailure(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c

	saver.err = errors.New("database unavailable")
	c.startPrologue()
	if len(p.quests) != 0 || len(packets.frames) != 0 {
		t.Fatal("quest started in memory after its save failed")
	}
	saver.err = nil
	c.startPrologue()
	packets.frames = nil
	c.startPrologue()
	if len(saver.saved) != 1 || len(packets.frames) != 1 || packets.frames[0][0] != smPlayMovie {
		t.Errorf("unfinished movie did not resume cleanly: saved=%+v packets=%x", saver.saved, packets.frames)
	}

	saver.err = errors.New("database unavailable")
	originalExp := p.Exp
	packets.frames = nil
	c.playMovieEnd(movieEnded(1))
	if p.quests[0].Status != "START" || p.Exp != originalExp || len(packets.frames) != 0 {
		t.Error("failed completion changed the quest or rewarded the player")
	}
}

func TestDeleteQuestRespectsCannotGiveUpAndPersists(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.quests = []store.Quest{{ID: 1000, Status: "START"}, {ID: 85, Status: "START", Vars: 3}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	request := func(id uint16) *wire.Reader {
		w := wire.Packet(cmDeleteQuest)
		w.H(id)
		return wire.NewReader(w.Data[1:])
	}

	c.deleteQuest(request(1000))
	if len(saver.saved) != 0 || len(packets.frames) != 0 || p.quests[0].Status != "START" {
		t.Fatal("cannot_giveup quest was deleted")
	}
	saver.err = errors.New("database unavailable")
	c.deleteQuest(request(85))
	if p.quests[1].Status != "START" || len(packets.frames) != 0 {
		t.Fatal("quest changed after its save failed")
	}
	saver.err = nil
	c.deleteQuest(request(85))
	if len(saver.saved) != 1 || saver.saved[0].Status != "NONE" || p.quests[1].Status != "NONE" {
		t.Fatalf("quest was not abandoned and persisted: saved=%+v player=%+v", saver.saved, p.quests[1])
	}
	if got := packets.last(smQuestAccepted); !bytes.Equal(got, []byte{0x78, 3, 85, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Errorf("delete packet = %x", got)
	}
	if len(packets.frames) != 2 || packets.frames[1][0] != smNearbyQuests {
		t.Errorf("abandon packet sequence = %x", packets.frames)
	}
}

func dialogRequest(opcode byte, objectID int32, dialogID uint16, questID int32) *wire.Reader {
	w := wire.Packet(opcode)
	w.D(objectID)
	if opcode == cmDialogSelect {
		w.H(dialogID)
		w.H(1)
		w.H(10)
		w.D(questID)
	}
	return wire.NewReader(w.Data[1:])
}

func TestFirstReportQuestsFromMarkerToReward(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		race           string
		questID        int32
		startNPC       int32
		endNPC         int32
		wantMarker     []byte
		wantRemaining  []byte
		wantExperience int64
		wantKinah      int64
	}{
		{"ELYOS", 1101, 203049, 203057, []byte{smNearbyQuests, 1, 0, 0, 0, 0x4d, 4, 0, 0}, []byte{smNearbyQuests, 0, 0, 0, 0}, 50, 30},
		{"ASMODIANS", 2101, 203500, 203504, []byte{smNearbyQuests, 2, 0, 0, 0, 0x35, 8, 0, 0, 0x36, 8, 0, 0}, []byte{smNearbyQuests, 1, 0, 0, 0, 0x36, 8, 0, 0}, 130, 20},
	} {
		t.Run(tc.race, func(t *testing.T) {
			s := testServer(d)
			saver := &recordedQuests{}
			s.quests = saver
			p := wrathchild(s)
			p.Race = tc.race
			p.seen = map[int32]*object{}
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: 182400001, Owner: p.ID}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			start := &object{id: 0x30001, worldID: p.WorldID, x: p.X + 2, y: p.Y, z: p.Z, npc: d.Npcs[tc.startNPC], watchers: map[int32]*player{p.ID: p}}
			end := &object{id: 0x30002, worldID: p.WorldID, x: p.X + 3, y: p.Y, z: p.Z, npc: d.Npcs[tc.endNPC]}
			if start.npc == nil || end.npc == nil {
				t.Fatal("the report quest NPCs are absent from static data")
			}
			p.seen[start.id] = start
			p.seen[end.id] = end
			s.byID[start.id] = start
			s.byID[end.id] = end
			if got := s.nearbyQuests(p).Data; !bytes.Equal(got, tc.wantMarker) {
				t.Fatalf("nearby quest marker = %x, want %x", got, tc.wantMarker)
			}
			c.showDialog(dialogRequest(cmShowDialog, start.id, 0, 0))
			if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(start.id, 10, 0).Data) || packets.last(smLookatobject) == nil {
				t.Fatalf("right-click did not open NPC dialog: %x", packets.frames)
			}
			selectDialog := func(o *object, dialog uint16) {
				c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, tc.questID))
			}
			selectDialog(start, 25)
			if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(start.id, 1011, tc.questID).Data) {
				t.Errorf("quest offer = %x", got)
			}
			selectDialog(start, 1007)
			if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(start.id, 4, tc.questID).Data) {
				t.Errorf("quest detail = %x", got)
			}
			selectDialog(start, 1002)
			if q := p.quest(tc.questID); q == nil || q.Status != "START" || len(saver.saved) != 1 {
				t.Fatalf("quest was not started and persisted: %+v, %+v", q, saver.saved)
			}
			if got := s.nearbyQuests(p).Data; !bytes.Equal(got, tc.wantRemaining) {
				t.Errorf("quest marker remained after accepting: %x", got)
			}
			selectDialog(end, 25)
			if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(end.id, 2375, tc.questID).Data) {
				t.Errorf("turn-in dialog = %x", got)
			}
			selectDialog(end, 1009)
			if q := p.quest(tc.questID); q.Status != "REWARD" || q.Vars != 1 || len(saver.saved) != 2 {
				t.Fatalf("quest did not enter reward state: %+v, %+v", q, saver.saved)
			}
			selectDialog(end, 17)
			if q := p.quest(tc.questID); q.Status != "COMPLETE" || q.CompleteCount != 1 || len(saver.saved) != 3 {
				t.Fatalf("quest did not complete: %+v, %+v", q, saver.saved)
			}
			if p.Exp != 206+tc.wantExperience || p.kinah.Count != tc.wantKinah {
				t.Errorf("reward exp=%d kinah=%d, want %d and %d", p.Exp, p.kinah.Count, 206+tc.wantExperience, tc.wantKinah)
			}
			selectDialog(end, 17)
			if len(saver.saved) != 3 || p.kinah.Count != tc.wantKinah {
				t.Error("quest reward could be claimed twice")
			}
		})
	}
}

func TestReportQuestRejectsUnseenOrDistantNPC(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	o := &object{id: 0x30001, worldID: p.WorldID, x: p.X + 11, y: p.Y, z: p.Z, npc: d.Npcs[203049]}
	c.dialogSelect(dialogRequest(cmDialogSelect, o.id, 1002, 1101))
	p.seen[o.id] = o
	c.showDialog(dialogRequest(cmShowDialog, o.id, 0, 0))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(o.id, 10, 0).Data) {
		t.Fatalf("right-click on a known NPC should open its dialog: %x", got)
	}
	packets.frames = nil
	c.dialogSelect(dialogRequest(cmDialogSelect, o.id, 1002, 1101))
	if p.quest(1101) != nil || len(packets.frames) != 0 {
		t.Fatal("quest was accepted through an unseen or distant NPC")
	}
}
