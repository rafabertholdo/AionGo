package game

import (
	"testing"

	"aionlightning/wire"
)

// TestFindGroup covers posting, listing, deleting and the team cleanup of Find Group posts, with the
// layouts the 1.9 client's SM_FIND_GROUP handler parses.
func TestFindGroup(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, _, _ := twoPlayers(t, s)
	annPackets, bobPackets := &questPackets{}, &questPackets{}
	ann.conn.tap, bob.conn.tap = annPackets.tap, bobPackets.tap
	send := func(p *player, build func(w *wire.Writer)) { p.conn.findGroup(packet(cmFindGroup, build)) }

	send(bob, func(w *wire.Writer) { w.C(2); w.D(bob.ID); w.S("need a healer"); w.C(0) })
	if bobPackets.last(smSystemMessage) == nil || bobPackets.last(smFindGroup) == nil {
		t.Fatal("posting did not answer with the message and the list")
	}
	send(ann, func(w *wire.Writer) { w.C(0) })
	r := wire.NewReader(annPackets.last(smFindGroup)[1:])
	action, total, count := r.C(), r.H(), r.H()
	r.D() // list id
	id, unk, groupType, message, name := r.D(), r.D(), r.C(), r.S(), r.S()
	size, lowest, highest := r.C(), r.C(), r.C()
	r.D()
	if r.Err != nil || action != 0 || total != 1 || count != 1 || id != bob.ID || unk != findGroupPlayerUnk ||
		groupType != 0 || message != "need a healer" || name != "Bob" || size != 1 || lowest != 10 || highest != 10 {
		t.Fatalf("unexpected recruit list: %x (read error %v)", annPackets.last(smFindGroup), r.Err)
	}

	send(ann, func(w *wire.Writer) { w.C(1); w.D(bob.ID); w.D(findGroupPlayerUnk) })
	if len(s.findGroups) != 1 {
		t.Fatal("Ann deleted Bob's post")
	}
	annPackets.frames = nil
	send(bob, func(w *wire.Writer) { w.C(1); w.D(bob.ID); w.D(findGroupPlayerUnk) })
	r = wire.NewReader(annPackets.last(smFindGroup)[1:])
	if action, id, unk := r.C(), r.D(), r.D(); r.Err != nil || action != 1 || id != bob.ID || unk != findGroupPlayerUnk || len(s.findGroups) != 0 {
		t.Fatalf("delete not broadcast: %x", annPackets.last(smFindGroup))
	}

	send(ann, func(w *wire.Writer) {
		w.C(6)
		w.D(ann.ID)
		w.S("cleric lf group")
		w.C(0)
		w.C(5)
		w.C(10)
	})
	send(bob, func(w *wire.Writer) { w.C(4) })
	r = wire.NewReader(bobPackets.last(smFindGroup)[1:])
	action, total, count = r.C(), r.H(), r.H()
	r.D()
	id, groupType, message, name = r.D(), r.C(), r.S(), r.S()
	class, level := r.C(), r.C()
	r.D()
	if r.Err != nil || action != 4 || total != 1 || count != 1 || id != ann.ID || message != "cleric lf group" ||
		name != "Ann" || class != byte(classIDs[ann.Class]) || level != 10 {
		t.Fatalf("unexpected apply list: %x (read error %v)", bobPackets.last(smFindGroup), r.Err)
	}

	// Joining a group drops the member's posts; the founder's post becomes the group's.
	send(bob, func(w *wire.Writer) { w.C(2); w.D(bob.ID); w.S("dungeon run"); w.C(0) })
	s.newGroup(bob)
	s.addToGroup(bob.group, ann)
	race, _ := raceGender(bob.Character)
	if s.findGroups[findGroupKey{race, findGroupApply, ann.ID}] != nil || s.findGroups[findGroupKey{race, findGroupRecruit, bob.ID}] != nil {
		t.Fatal("players' own posts stayed after grouping")
	}
	post := s.findGroups[findGroupKey{race, findGroupRecruit, bob.group.id}]
	if post == nil || post.message != "dungeon run" || len(post.members()) != 2 || post.unk() != 0 {
		t.Fatalf("group post = %+v", post)
	}
	g := bob.group
	s.leaveGroup(ann)
	if s.findGroups[findGroupKey{race, findGroupRecruit, g.id}] != nil {
		t.Fatal("the disbanded group's post stayed")
	}
}

// TestFindGroupChatWindow covers the Find Group window's questions about a listed player.
func TestFindGroupChatWindow(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, _, _ := twoPlayers(t, s)
	packets := &questPackets{}
	ann.conn.tap = packets.tap

	ann.conn.chatWindow(packet(cmChatWindow, func(w *wire.Writer) { w.S("bob"); w.D(0) }))
	r := wire.NewReader(packets.last(smChatWindow)[1:])
	if kind, name, group, class, level := r.C(), r.S(), r.D(), r.C(), r.C(); r.Err != nil || kind != chatWindowSolo ||
		name != "Bob" || group != 0 || class != byte(classIDs[bob.Class]) || level != 10 {
		t.Fatalf("solo chat window: %x", packets.last(smChatWindow))
	}

	s.newGroup(bob)
	s.addToGroup(bob.group, ann)
	ann.conn.chatWindow(packet(cmChatWindow, func(w *wire.Writer) { w.S("Bob"); w.D(0) }))
	r = wire.NewReader(packets.last(smChatWindow)[1:])
	if kind, name, group, leader := r.C(), r.S(), r.D(), r.S(); r.Err != nil || kind != chatWindowGroup ||
		name != "Bob" || group != bob.group.id || leader != "Bob" || len(r.B(12)) != 12 || r.Err != nil {
		t.Fatalf("group chat window: %x", packets.last(smChatWindow))
	}

	ann.conn.chatMemberInfo(packet(cmChatMemberInfo, func(w *wire.Writer) { w.S("bob") }))
	r = wire.NewReader(packets.last(smChatWindow)[1:])
	if kind, name, legion, level, class := r.C(), r.S(), r.S(), r.C(), r.C(); r.Err != nil || kind != chatWindowMemberInfo ||
		name != "Bob" || legion != "" || level != 10 || class != byte(classIDs[bob.Class]) {
		t.Fatalf("member info: %x", packets.last(smChatWindow))
	}
	packets.frames = nil
	ann.conn.chatWindow(packet(cmChatWindow, func(w *wire.Writer) { w.S("nobody"); w.D(0) }))
	if packets.last(smChatWindow) != nil {
		t.Fatal("answered for an offline player")
	}
}
