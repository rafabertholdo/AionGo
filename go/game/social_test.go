package game

import (
	"testing"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// twoPlayers puts Ann and Bob, both level 10, in the world.
func twoPlayers(t *testing.T, s *Server) (ann, bob *player, annTap, bobTap *tapped) {
	t.Helper()
	ann, annTap = fighter(t, s, 1000)
	bob, bobTap = fighter(t, s, 1005)
	ann.Name, bob.ID, bob.Name = "Ann", 0x20001, "Bob"
	ann.level, bob.level = 10, 10
	ann.appearance, bob.appearance = &store.Appearance{}, &store.Appearance{}
	s.spawn(ann)
	s.spawn(bob)
	return
}

func packet(op byte, build func(w *wire.Writer)) *wire.Reader {
	w := wire.Packet(op)
	build(w)
	return wire.NewReader(w.Data[1:])
}

// TestWhisper has Ann whisper to Bob, and Bob block her.
func TestWhisper(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	whisper := func(to string) {
		ann.conn.whisper(packet(cmChatMessageWhisper, func(w *wire.Writer) { w.S(to); w.S("hi") }))
	}
	whisper("bob")
	if bobTap.count(smMessage) != 1 {
		t.Fatalf("Bob got %d messages", bobTap.count(smMessage))
	}
	whisper("nobody")
	if annTap.count(smSystemMessage) != 1 {
		t.Errorf("Ann wasn't told nobody is there")
	}
	bob.conn.blockAdd(packet(cmBlockAdd, func(w *wire.Writer) { w.S("Ann"); w.S("rude") }))
	whisper("Bob")
	if bobTap.count(smMessage) != 1 || annTap.count(smSystemMessage) != 2 {
		t.Errorf("a blocked whisper got through")
	}
	if len(bob.blocks) != 1 || bobTap.count(smBlockList) != 1 {
		t.Errorf("blocks %v", bob.blocks)
	}
	bob.conn.blockDel(packet(cmBlockDel, func(w *wire.Writer) { w.S("ann") }))
	if len(bob.blocks) != 0 {
		t.Errorf("Ann is still blocked")
	}
}

// TestFriends has Ann ask Bob to be friends, and delete him.
func TestFriends(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	ann.conn.friendAdd(packet(cmFriendAdd, func(w *wire.Writer) { w.S("Bob") }))
	if bobTap.count(smQuestionWindow) != 1 {
		t.Fatalf("Bob wasn't asked")
	}
	bob.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(questionBuddyRequest); w.C(1) }))
	if len(ann.friends) != 1 || len(bob.friends) != 1 || annTap.count(smFriendList) != 1 || bobTap.count(smFriendList) != 1 {
		t.Fatalf("friends %d and %d", len(ann.friends), len(bob.friends))
	}
	bob.conn.friendStatus(packet(cmFriendStatus, func(w *wire.Writer) { w.C(1) }))
	if annTap.count(smFriendUpdate) != 1 || annTap.count(smFriendNotify) != 1 {
		t.Errorf("Ann wasn't told Bob is online")
	}
	ann.conn.friendDel(packet(cmFriendDel, func(w *wire.Writer) { w.S("bob") }))
	if len(ann.friends) != 0 || len(bob.friends) != 0 || bobTap.count(smFriendNotify) != 1 {
		t.Errorf("the friendship wasn't ended")
	}
}

// TestGroup has Ann invite Bob, chat with him, and leave: the group ends when one is left.
func TestGroup(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	ann.Race, bob.Race = "ELYOS", "ELYOS"
	ann.conn.inviteToGroup(packet(cmInviteToGroup, func(w *wire.Writer) { w.C(0); w.S("bob") }))
	if bobTap.count(smQuestionWindow) != 1 {
		t.Fatalf("Bob wasn't asked")
	}
	bob.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(questionGroupInvitation); w.C(1) }))
	if ann.group == nil || ann.group != bob.group || len(ann.group.members) != 2 || ann.group.leader != ann {
		t.Fatalf("no group")
	}
	if annTap.count(smGroupInfo) != 1 || bobTap.count(smGroupInfo) != 1 || bobTap.count(smGroupMemberInfo) != 1 || annTap.count(smGroupMemberInfo) != 1 {
		t.Errorf("wrong group packets: %v %v", annTap.counts, bobTap.counts)
	}
	ann.conn.chat(packet(cmChatMessagePublic, func(w *wire.Writer) { w.C(chatGroup); w.S("hi group") }))
	if bobTap.count(smMessage) != 1 {
		t.Errorf("Bob didn't hear the group")
	}
	bob.conn.playerStatusInfo(packet(cmPlayerStatusInfo, func(w *wire.Writer) { w.C(2); w.D(0) }))
	if ann.group != nil || bob.group != nil {
		t.Errorf("the group wasn't ended")
	}
}

// TestGroupReward has a group kill a monster: both members get experience, and the loot is Bob's by round robin.
func TestGroupReward(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	o := monster(t, s, 1002)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	ann.level, bob.level = 1, 1 // near the monster's level, so that it gives something
	s.newGroup(ann)
	s.addToGroup(ann.group, bob)
	o.aggro[ann.ID] = &aggroInfo{damage: 10}
	s.updateNpcKnown(o)
	s.reward(o)
	if annTap.count(smStatupdateExp) == 0 || bobTap.count(smStatupdateExp) == 0 {
		t.Errorf("exp: Ann %d, Bob %d", annTap.count(smStatupdateExp), bobTap.count(smStatupdateExp))
	}
	if o.loot == nil || len(o.loot.allowed) != 1 {
		t.Errorf("loot %+v", o.loot)
	}
}

// TestDuel has Ann challenge Bob, who accepts, and win: Bob isn't killed, and they can't hurt each other afterwards.
func TestDuel(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	ann.conn.duelRequest(packet(cmDuelRequest, func(w *wire.Writer) { w.D(bob.ID) }))
	if bobTap.count(smQuestionWindow) != 1 || annTap.count(smQuestionWindow) != 1 {
		t.Fatalf("the questions weren't asked: %v %v", annTap.counts, bobTap.counts)
	}
	bob.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(questionDuelAccept); w.C(1) }))
	if annTap.count(smDuel) != 1 || bobTap.count(smDuel) != 1 || s.duels[ann.ID] != bob.ID {
		t.Fatalf("the duel didn't start: %v", s.duels)
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	bob.life.HP = 5
	for range 50 {
		if len(s.duels) == 0 {
			break
		}
		ann.lastAttack = time.Time{}
		s.playerAttack(ann, bob)
	}
	if bob.dead || bob.life.HP != 1 || len(s.duels) != 0 || annTap.count(smDuel) != 2 {
		t.Errorf("dead %v, hp %d, duels %v, %d duel packets", bob.dead, bob.life.HP, s.duels, annTap.count(smDuel))
	}
	// Same race, no duel: no attack.
	hp := bob.life.HP
	ann.lastAttack = time.Time{}
	s.playerAttack(ann, bob)
	if bob.life.HP != hp {
		t.Errorf("Ann hurt Bob outside a duel")
	}
	// Another race is an enemy.
	bob.Race = "ASMODIANS"
	ann.lastAttack = time.Time{}
	s.playerAttack(ann, bob)
	if annTap.count(smAttack) == 0 {
		t.Errorf("Ann couldn't attack an Asmodian")
	}
}

// TestExchange has Ann trade some of her potions and kinah for Bob's potions.
func TestExchange(t *testing.T) {
	d := staticDataOrSkip(t)
	const potion = 160000001 // tradeable, stacks
	if it := d.Items[potion]; it == nil || it.Mask&itemTradeable == 0 {
		t.Skip("no tradeable stackable item")
	}
	s := testServer(d)
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	s.visMu.Lock()
	s.addItem(ann, potion, 10)
	s.addItem(bob, potion, 4)
	ann.kinah.Count = 1000
	s.visMu.Unlock()
	ann.conn.exchangeRequest(packet(cmExchangeRequest, func(w *wire.Writer) { w.D(bob.ID) }))
	bob.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(questionExchange); w.C(1) }))
	if annTap.count(smExchangeRequest) != 1 || bobTap.count(smExchangeRequest) != 1 || !ann.trading {
		t.Fatalf("the trade didn't begin: %v %v", annTap.counts, bobTap.counts)
	}
	ann.conn.exchangeAddItem(packet(cmExchangeAddItem, func(w *wire.Writer) { w.D(ann.cube[0].UniqueID); w.D(3) }))
	ann.conn.exchangeAddKinah(packet(cmExchangeAddKinah, func(w *wire.Writer) { w.D(400); w.D(0) }))
	bob.conn.exchangeAddItem(packet(cmExchangeAddItem, func(w *wire.Writer) { w.D(bob.cube[0].UniqueID); w.D(4) }))
	if annTap.count(smExchangeAddItem) != 2 || bobTap.count(smExchangeAddItem) != 2 || bobTap.count(smExchangeAddKinah) != 1 {
		t.Fatalf("offers weren't shown: %v %v", annTap.counts, bobTap.counts)
	}
	ann.conn.exchangeLock(nil)
	bob.conn.exchangeLock(nil)
	ann.conn.exchangeOK(nil)
	bob.conn.exchangeOK(nil)
	if ann.trading || bob.trading || len(s.exchanges) != 0 {
		t.Fatalf("the trade didn't end")
	}
	count := func(p *player) (n int64) {
		for _, item := range p.cube {
			n += item.Count
		}
		return
	}
	if count(ann) != 7+4 || count(bob) != 3 || ann.kinah.Count != 600 || bob.kinah.Count != 400 {
		t.Errorf("Ann has %d and %d kinah, Bob %d and %d", count(ann), ann.kinah.Count, count(bob), bob.kinah.Count)
	}
}

// TestPrivateStore has Ann open a store, and Bob buy from it.
func TestPrivateStore(t *testing.T) {
	d := staticDataOrSkip(t)
	const potion = 160000001
	if it := d.Items[potion]; it == nil || it.Mask&itemTradeable == 0 {
		t.Skip("no tradeable item")
	}
	s := testServer(d)
	ann, bob, _, bobTap := twoPlayers(t, s)
	s.visMu.Lock()
	s.addItem(ann, potion, 10)
	bob.kinah.Count = 1000
	s.visMu.Unlock()
	ann.conn.privateStore(packet(cmPrivateStore, func(w *wire.Writer) {
		w.H(1)
		w.D(ann.cube[0].UniqueID)
		w.D(potion)
		w.H(6)
		w.D(50)
	}))
	ann.conn.privateStoreName(packet(cmPrivateStoreName, func(w *wire.Writer) { w.S("cheap") }))
	if ann.store == nil || ann.state&statePrivateShop != statePrivateShop || bobTap.count(smPrivateStoreName) != 1 {
		t.Fatalf("no store: %v", bobTap.counts)
	}
	bob.conn.serviceDialog(ann.ID, dialogBuy)
	if bobTap.count(smPrivateStore) != 1 {
		t.Fatalf("Bob wasn't shown the store")
	}
	bob.conn.buyItem(packet(cmBuyItem, func(w *wire.Writer) {
		w.D(ann.ID)
		w.H(shopSellToPlayers)
		w.H(1)
		w.D(0) // the first thing it sells
		w.D(4)
		w.D(0)
	}))
	if bob.kinah.Count != 800 || ann.kinah.Count != 200 || len(bob.cube) != 1 || bob.cube[0].Count != 4 || ann.cube[0].Count != 6 {
		t.Errorf("Bob has %d kinah, Ann %d", bob.kinah.Count, ann.kinah.Count)
	}
}

// TestPvpKill has Ann kill an Asmodian: she gets abyss points and he loses some.
func TestPvpKill(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, _ := twoPlayers(t, s)
	bob.Race = "ASMODIANS"
	s.visMu.Lock()
	defer s.visMu.Unlock()
	bob.abyss.AP, bob.abyss.Rank = 500, 1
	s.playerHit(bob, ann, 0, statusRegular, 100000)
	if !bob.dead || ann.abyss.AP <= 0 || ann.abyss.AllKill != 1 || annTap.count(smAbyssRank) == 0 {
		t.Fatalf("dead %v, Ann has %d AP and %d kills", bob.dead, ann.abyss.AP, ann.abyss.AllKill)
	}
	if bob.abyss.AP >= 500 {
		t.Errorf("Bob still has %d AP", bob.abyss.AP)
	}
	// Killing one's own race costs nothing.
	cal, _ := fighter(t, s, 1010)
	cal.ID, cal.Name, cal.appearance = 0x20005, "Cal", ann.appearance
	s.spawnLocked(cal)
	ap := ann.abyss.AP
	s.playerHit(cal, ann, 0, statusRegular, 100000)
	if ann.abyss.AP != ap {
		t.Errorf("a kill of the same race gave abyss points")
	}
}

// TestAlliance has Ann invite Bob to an alliance, chat in it, make Bob its captain, and leave: the alliance ends with two members.
func TestAlliance(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	ann.Race, bob.Race = "ELYOS", "ELYOS"
	ann.conn.inviteToGroup(packet(cmInviteToGroup, func(w *wire.Writer) { w.C(10); w.S("bob") }))
	if bobTap.count(smQuestionWindow) != 1 {
		t.Fatalf("Bob wasn't asked")
	}
	bob.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(questionAllianceInvite); w.C(1) }))
	if ann.alliance == nil || ann.alliance != bob.alliance || len(ann.alliance.members) != 2 || ann.alliance.captain != ann {
		t.Fatalf("no alliance")
	}
	if annTap.count(smAllianceInfo) != 1 || bobTap.count(smAllianceInfo) != 1 || bobTap.count(smAllianceMemberInfo) != 1 || annTap.count(smAllianceMemberInfo) != 1 {
		t.Errorf("wrong alliance packets: %v %v", annTap.counts, bobTap.counts)
	}
	ann.conn.chat(packet(cmChatMessagePublic, func(w *wire.Writer) { w.C(chatAlliance); w.S("hi alliance") }))
	if bobTap.count(smMessage) != 1 {
		t.Errorf("Bob didn't hear the alliance")
	}
	ann.conn.playerStatusInfo(packet(cmPlayerStatusInfo, func(w *wire.Writer) { w.C(15); w.D(bob.ID) }))
	if ann.alliance.captain != bob {
		t.Errorf("Bob isn't the captain")
	}
	ann.conn.playerStatusInfo(packet(cmPlayerStatusInfo, func(w *wire.Writer) { w.C(12); w.D(0) }))
	if ann.alliance != nil || bob.alliance != nil {
		t.Errorf("the alliance wasn't ended")
	}
}
