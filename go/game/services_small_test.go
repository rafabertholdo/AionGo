package game

import (
	"testing"
	"unicode/utf16"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// setNpc makes the object an npc of a type, with the npc id.
func setNpc(s *Server, o *object, kind string, id int32) {
	npc := *s.data.Npcs[210133]
	npc.Type, npc.ID = kind, id
	o.npc = &npc
}

func showDialog(p *player, o *object) {
	handlers[cmShowDialog](p.conn, packet(cmShowDialog, func(w *wire.Writer) { w.D(o.id) }))
}

func answer(p *player, code int32, yes byte) {
	p.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(code); w.C(yes) }))
}

func TestPostboxAndBindStone(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	frames := &questPackets{}
	p.conn.tap = frames.tap
	s.spawn(p)
	postbox := monster(t, s, 1005)
	s.visMu.Lock()
	s.updateNpcKnown(postbox)
	s.visMu.Unlock()
	setNpc(s, postbox, "POSTBOX", 210133)
	showDialog(p, postbox)
	if got := frames.last(smDialogWindow); got == nil || wire.NewReader(got[1:]).D() != postbox.id {
		t.Fatalf("the postbox opened no window: %v", got)
	}

	// Akarios Village (bind 3, 110 kinah) is in Elyos land: the player pays and binds.
	stone := postbox
	setNpc(s, stone, "RESURRECT", 700013)
	showDialog(p, stone)
	if frames.last(smQuestionWindow) == nil {
		t.Fatal("a bind stone asked nothing")
	}
	answer(p, questionBindToLocation, 1)
	if p.BindPoint != 0 || frames.last(smSystemMessage) == nil {
		t.Fatalf("bound with no kinah: %d", p.BindPoint)
	}
	p.kinah.Count = 1000
	showDialog(p, stone)
	answer(p, questionBindToLocation, 1)
	if p.BindPoint != 3 || p.kinah.Count != 890 || frames.last(smSetBindPoint) == nil || frames.last(smLevelUpdate) == nil {
		t.Fatalf("bind point %d, kinah %d", p.BindPoint, p.kinah.Count)
	}

	// Again: it is registered already, and nothing is asked.
	frames.frames = nil
	showDialog(p, stone)
	if frames.last(smQuestionWindow) != nil || frames.last(smSystemMessage) == nil {
		t.Fatal("bound twice")
	}

	// An Asmodian can't bind in Elyos land (cross faction binding is off).
	p.Race, p.BindPoint = "ASMODIANS", 0
	frames.frames = nil
	showDialog(p, stone)
	if frames.last(smQuestionWindow) != nil || frames.last(smMessage) == nil {
		t.Fatal("an Asmodian bound in Elyos land")
	}
	s.config.CrossFactionBinding = true
	showDialog(p, stone)
	if frames.last(smQuestionWindow) == nil {
		t.Fatal("cross faction binding didn't ask")
	}
}

func TestBindStonesLoaded(t *testing.T) {
	d := staticDataOrSkip(t)
	if b := d.BindStones[700012]; b.ID != 1 || b.Price != 1540 {
		t.Fatalf("Sanctum's stone: %+v", b)
	}
	if d.WorldMaps[110010000].WorldType != "ELYSEA" {
		t.Fatalf("Sanctum is %q", d.WorldMaps[110010000].WorldType)
	}
}

func TestClassChange(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	frames := &questPackets{}
	p.conn.tap = frames.tap
	s.spawn(p)
	p.level = 9
	choose := func(dialog uint16) {
		handlers[cmDialogSelect](p.conn, packet(cmDialogSelect, func(w *wire.Writer) { w.D(0); w.H(dialog); w.H(0); w.H(0); w.D(1006) }))
	}

	// Off, as in AL-Game by default: nothing is offered and the choice does nothing.
	s.visMu.Lock()
	s.classChangeDialog(p)
	s.visMu.Unlock()
	choose(3058)
	if frames.last(smDialogWindow) != nil || p.Class != "MAGE" {
		t.Fatalf("class change is off: %s", p.Class)
	}

	s.config.SimpleSecondClass = true
	s.visMu.Lock()
	s.classChangeDialog(p)
	s.visMu.Unlock()
	r := wire.NewReader(frames.last(smDialogWindow)[1:])
	if object, dialog, quest := r.D(), r.H(), r.D(); object != 0 || dialog != 3057 || quest != 1006 {
		t.Fatalf("Elyos mage dialog: %d %d %d", object, dialog, quest)
	}
	choose(2376) // a gladiator: not for a mage
	if p.Class != "MAGE" {
		t.Fatalf("a mage became %s", p.Class)
	}
	choose(3058) // a sorcerer
	if p.Class != "SORCERER" {
		t.Fatalf("class %s", p.Class)
	}
	for _, id := range []int32{1006, 1007} {
		if q := p.quest(id); q == nil || q.Status != "COMPLETE" {
			t.Fatalf("quest %d: %+v", id, q)
		}
	}
	choose(3143) // a spirit master: not for a sorcerer any more
	if p.Class != "SORCERER" {
		t.Fatalf("class changed twice: %s", p.Class)
	}
	p.Class, p.Race, p.level = "PRIEST", "ASMODIANS", 10
	s.visMu.Lock()
	s.classChangeDialog(p)
	s.visMu.Unlock()
	r = wire.NewReader(frames.last(smDialogWindow)[1:])
	if _, dialog, quest := r.D(), r.H(), r.D(); dialog != 4080 || quest != 2008 {
		t.Fatalf("Asmodian priest dialog: %d %d", dialog, quest)
	}
}

func TestHTMLChunks(t *testing.T) {
	html := make([]rune, 2*htmlChunk+5)
	for i := range html {
		html[i] = 'é'
	}
	packets := htmlPackets(7, string(html))
	if len(packets) != 3 {
		t.Fatalf("%d packets", len(packets))
	}
	total := 0
	for i, w := range packets {
		r := wire.NewReader(w.Data[1:])
		id, chunk, count, size := r.D(), r.C(), r.C(), r.H()
		text := r.S()
		if id != 7 || int(chunk) != i || count != 3 || int(size) != len(utf16.Encode([]rune(text)))*2 {
			t.Errorf("packet %d: %d %d %d %d", i, id, chunk, count, size)
		}
		total += len([]rune(text))
	}
	if total != len(html) {
		t.Errorf("%d characters of %d", total, len(html))
	}
	if got := htmlPackets(1, ""); len(got) != 1 {
		t.Errorf("an empty page is %d packets", len(got))
	}
	if d := staticDataOrSkip(t); d.Welcome == "" {
		t.Error("welcome.xhtml wasn't loaded")
	}
}

func TestAutoAnnouncements(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	bob.Race = "ASMODIANS"
	ann.Race = "ELYOS"
	s.spawn(ann)
	s.spawn(bob)
	annTap.counts, bobTap.counts = map[byte]int{}, map[byte]int{}
	s.sendAutoAnnouncement(store.AutoAnnouncement{Text: "hello", Faction: "ELYOS", Type: "YELLOW", Delay: 60})
	if annTap.count(smMessage) != 1 || bobTap.count(smMessage) != 0 {
		t.Fatalf("elyos only: %v %v", annTap.counts, bobTap.counts)
	}
	s.sendAutoAnnouncement(store.AutoAnnouncement{Text: "all", Faction: "ALL", Type: "SHOUT", Delay: 60})
	if annTap.count(smMessage) != 2 || bobTap.count(smMessage) != 1 {
		t.Fatalf("all: %v %v", annTap.counts, bobTap.counts)
	}
	if got := announcementType("NORMAL"); got != chatPeriodNotice || announcementType("x") != chatSystemNoticeAuto || announcementType("ORANGE") != chatGroupLeader {
		t.Errorf("chat types %x", got)
	}
	tasks := s.startAnnouncements([]store.AutoAnnouncement{{Text: "a", Delay: 3600}, {Text: "b", Delay: 0}})
	if len(tasks) != 1 {
		t.Errorf("%d tasks", len(tasks))
	}
	for _, task := range tasks {
		task.cancel()
	}
}

func TestBreakItem(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	extract := &store.Item{UniqueID: 0x40001, ItemID: 165000001, Count: 2, Owner: p.ID}
	target := &store.Item{UniqueID: 0x40002, ItemID: 100600034, Count: 1, Owner: p.ID}
	p.cube = []*store.Item{extract, target}
	if !s.canAct(p, &data.Node{Name: "extract"}, target) || s.canAct(p, &data.Node{Name: "extract"}, nil) {
		t.Fatal("extract needs a target")
	}
	level := d.Items[target.ItemID].Level
	s.breakItem(p, target, extract)
	if itemIn(p.cube, target.UniqueID) != nil || extract.Count != 1 {
		t.Fatalf("the item stays or the extractor wasn't used: %+v", p.cube)
	}
	var stones int64
	for _, item := range p.cube {
		if item.ItemID >= enchantStoneBase+level && item.ItemID <= enchantStoneBase+level+20 {
			stones += item.Count
		}
	}
	if stones < 1 || stones > 3 {
		t.Fatalf("%d enchantment stones: %+v", stones, p.cube)
	}
}
