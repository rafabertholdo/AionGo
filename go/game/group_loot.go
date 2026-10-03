package game

import (
	"context"
	"errors"
	"math"
	"slices"
	"strconv"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() { handlers[cmGroupLoot] = (*conn).groupLoot }

const (
	msgRollMe         = 1390162
	msgRollOther      = 1390163
	msgRollPassMe     = 1390164
	msgRollPassOther  = 1390165
	msgRollWonMe      = 1390180
	msgRollWonOther   = 1390181
	msgLootOtherOwner = 1390220
	msgPayAccountMe   = 1390185
	msgPayAccountOthr = 1390186
	msgPayDistribute  = 1390187
)

// Loot distributions of the group's quality rules that ask the members first.
const (
	distributionRoll = 2
	distributionBid  = 3
)

func (g *group) lootQualityRules() [7]int32 {
	if g.qualityRules != nil {
		return *g.qualityRules
	}
	return [7]int32{0, 2, 2, 2, 2, 2, 0}
}

func (g *group) qualityDistribution(quality string) int32 {
	for index, name := range []string{"COMMON", "RARE", "LEGEND", "UNIQUE", "EPIC", "MYTHIC"} {
		if name == quality {
			return g.lootQualityRules()[index]
		}
	}
	return 0
}

// lootRoll snapshots participants at corpse registration and accepts one response per participant.
// All accesses use visMu, including departure and corpse removal; no extra timers or workers are owned here.
type lootRoll struct {
	distribution byte // distributionRoll or distributionBid
	group        *group
	item         *dropItem
	participants []*player
	pending      map[*player]bool
	replies      []lootReply
	winner       *player
	best         int64 // the winning roll or bid
}

type lootReply struct {
	player *player
	value  int64 // the roll, or the kinah bid; 0 passes
}

// groupLoot is CM_GROUP_LOOT. Luck comes from the server; a roll request only chooses roll or pass.
// A bid more than the bidder's kinah passes, as DropService.handleBid.
func (c *conn) groupLoot(r *wire.Reader) {
	groupID := r.D()
	r.D()
	r.D()
	itemID := r.D()
	r.C()
	npcID := r.D()
	distribution := r.C()
	choice := r.D()
	bid := r.Q()
	if r.Err != nil || (distribution != distributionRoll && distribution != distributionBid) ||
		(distribution == distributionRoll && (choice < 0 || choice > 1)) {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		o := s.byID[npcID]
		if o == nil || o.loot == nil || o.loot.active == nil {
			return
		}
		roll := o.loot.active
		if p.group != roll.group || groupID != roll.group.id || itemID != roll.item.item || !roll.pending[p] ||
			distribution != roll.distribution {
			return
		}
		value := int64(0)
		switch {
		case distribution == distributionBid && bid > 0 && bid <= p.kinah.Count:
			value = bid
		case distribution == distributionRoll && choice == 1:
			value = int64(rnd(1, 100))
		}
		s.answerLootRoll(o, p, value)
	})
}

func groupLootPacket(groupID, itemID, npcID int32, distribution byte) *wire.Writer {
	w := wire.Packet(smGroupLoot)
	w.D(groupID)
	w.D(1)
	w.D(1)
	w.D(itemID)
	w.C(0)
	w.D(npcID)
	w.C(distribution)
	w.D(0)
	w.D(1)
	return w
}

func (s *Server) beginLootRoll(o *object, item *dropItem) bool {
	loot := o.loot
	t := s.data.Items[item.item]
	if t == nil || t.ID == data.Kinah {
		return false
	}
	distribution := byte(loot.group.qualityDistribution(t.Quality))
	if distribution != distributionRoll && distribution != distributionBid {
		return false
	}
	roll := &lootRoll{distribution: distribution, group: loot.group, item: item, pending: map[*player]bool{}}
	for _, p := range loot.eligible {
		if s.spawned[p.ID] == p && p.group == loot.group && p.conn != nil {
			roll.participants = append(roll.participants, p)
			roll.pending[p] = true
		}
	}
	if len(roll.participants) < 2 {
		return false
	}
	item.roll = roll
	loot.active = roll
	if s.lootRolls == nil {
		s.lootRolls = map[*object]bool{}
	}
	s.lootRolls[o] = true
	for _, p := range roll.participants {
		p.conn.send(groupLootPacket(roll.group.id, item.item, o.id, distribution))
	}
	return true
}

func (s *Server) answerLootRoll(o *object, p *player, value int64) {
	roll := o.loot.active
	if roll == nil || !roll.pending[p] {
		return
	}
	delete(roll.pending, p)
	roll.replies = append(roll.replies, lootReply{player: p, value: value})
	// Java tells the members of rolls only; a bid is secret.
	for _, m := range roll.participants {
		if roll.distribution != distributionRoll {
			break
		}
		if s.spawned[m.ID] != m || m.group != roll.group || m.conn == nil {
			continue
		}
		switch {
		case value == 0 && m == p:
			m.conn.send(systemMessage(msgRollPassMe))
		case value == 0:
			m.conn.send(systemMessage(msgRollPassOther, p.Name))
		case m == p:
			m.conn.send(systemMessage(msgRollMe, strconv.FormatInt(value, 10)))
		default:
			m.conn.send(systemMessage(msgRollOther, p.Name, strconv.FormatInt(value, 10)))
		}
	}
	s.finishLootRoll(o, roll)
}

func (s *Server) finishLootRoll(o *object, roll *lootRoll) {
	if len(roll.pending) != 0 {
		return
	}
	o.loot.active = nil
	roll.best = 0
	roll.winner = nil
	for _, reply := range roll.replies {
		p := reply.player
		// Strictly greater preserves Java's first-response tie winner.
		if reply.value > roll.best && s.spawned[p.ID] == p && p.group == roll.group {
			roll.best, roll.winner = reply.value, p
		}
	}
	if roll.winner == nil {
		roll.item.free = true
		roll.item.roll = nil
		s.untrackLootRoll(o)
		return
	}
	s.awardLootRoll(o, roll)
}

func (s *Server) awardLootRoll(o *object, roll *lootRoll) {
	winner := roll.winner
	if winner == nil || s.spawned[winner.ID] != winner || winner.group != roll.group {
		return
	}
	if roll.distribution == distributionBid {
		if !s.awardLootBid(roll) {
			s.untrackLootRoll(o)
			return
		}
	} else {
		if !s.receiveRolledLoot(winner, roll.item, nil) {
			return
		}
		t := s.data.Items[roll.item.item]
		for _, p := range roll.participants {
			if s.spawned[p.ID] != p || p.group != roll.group || p.conn == nil {
				continue
			}
			if p == winner {
				p.conn.send(systemMessage(msgRollWonMe, descriptionID(t.NameID)))
			} else {
				p.conn.send(systemMessage(msgRollWonOther, winner.Name, descriptionID(t.NameID)))
			}
		}
	}
	loot := o.loot
	loot.items = slices.DeleteFunc(loot.items, func(item *dropItem) bool { return item == roll.item })
	s.untrackLootRoll(o)
	looter := s.spawned[loot.looting]
	if len(loot.items) > 0 {
		if looter != nil {
			looter.conn.send(lootItemList(o.id, loot.items))
		}
		return
	}
	if looter != nil {
		looter.conn.send(lootStatus(o.id, 3))
		looter.state &^= stateLooting
		looter.state |= stateActive
		looter.broadcast(s.playerEmotionTo(looter, emoteEndLoot, 0, o.id, 0, 0, 0, 0), true)
	}
	s.despawnNpc(o, true)
}

// leaveLootRolls prevents a departing member from holding the remaining group indefinitely.
func (s *Server) leaveLootRolls(p *player) {
	for o := range s.lootRolls {
		loot := o.loot
		if loot == nil {
			delete(s.lootRolls, o)
			continue
		}
		if loot.looting == p.ID {
			s.closeLoot(p, o.id)
		}
		if roll := loot.active; roll != nil {
			delete(roll.pending, p)
			s.finishLootRoll(o, roll)
		}
		if o.loot == nil {
			continue
		}
		for _, item := range loot.items {
			if item.roll != nil && item.roll.winner == p {
				item.roll = nil
				item.free = true
			}
		}
		s.untrackLootRoll(o)
	}
}

func (s *Server) untrackLootRoll(o *object) {
	if o.loot != nil {
		for _, item := range o.loot.items {
			if item.roll != nil {
				return
			}
		}
	}
	delete(s.lootRolls, o)
}

func (s *Server) cancelLootRolls(o *object) {
	delete(s.lootRolls, o)
	o.loot = nil
}

// awardLootBid is DropService.winningBidActions: the winner pays its bid, shared among the other members who were asked,
// when it receives the item. A winner who no longer has the kinah loses the item to everyone.
func (s *Server) awardLootBid(roll *lootRoll) bool {
	winner, bid := roll.winner, roll.best
	if winner.kinah.Count < bid {
		roll.item.roll = nil
		roll.item.free = true
		return false
	}
	// Java divides by the members asked less the winner, and pays those still here.
	share := bid / int64(len(roll.participants)-1)
	var others []*player
	payments := []store.KinahChange{{Before: *winner.kinah, Count: winner.kinah.Count - bid}}
	for _, p := range roll.participants {
		if p == winner || s.spawned[p.ID] != p || p.group != roll.group || p.conn == nil {
			continue
		}
		others = append(others, p)
		if share > 0 {
			payments = append(payments, store.KinahChange{Before: *p.kinah, Count: p.kinah.Count + share})
		}
	}
	if !s.receiveRolledLoot(winner, roll.item, payments) {
		return false
	}
	winner.kinah.Count -= bid
	winner.conn.send(s.updateItemPacket(winner, winner.kinah))
	total := strconv.FormatInt(bid, 10)
	winner.conn.send(systemMessage(msgPayAccountMe, total))
	for _, p := range others {
		if share > 0 {
			p.kinah.Count += share
			p.conn.send(s.updateItemPacket(p, p.kinah))
		}
		p.conn.send(systemMessage(msgPayAccountOthr, winner.Name, total))
		p.conn.send(systemMessage(msgPayDistribute, total, strconv.Itoa(len(roll.participants)-1), strconv.FormatInt(share, 10)))
	}
	return true
}

type lootReceiver interface {
	ReceiveLoot(context.Context, int32, []store.LootStack, []store.Item) error
}

type bidReceiver interface {
	ReceiveBidLoot(context.Context, int32, []store.LootStack, []store.Item, []store.KinahChange) error
}

// receiveRolledLoot plans the whole drop before committing, with a winning bid's payments if any.
// A full inventory or failed write leaves the corpse's item reserved.
func (s *Server) receiveRolledLoot(p *player, drop *dropItem, payments []store.KinahChange) bool {
	t := s.data.Items[drop.item]
	if t == nil || drop.count < 1 || drop.count > math.MaxInt32 {
		return false
	}
	remaining := drop.count
	var updates []store.LootStack
	var existing []*store.Item
	for _, item := range p.cube {
		if item.ItemID != drop.item || item.Equipped || item.Location != 0 {
			continue
		}
		free := int64(t.MaxStack) - item.Count
		if free <= 0 || remaining == 0 {
			continue
		}
		count := min(free, remaining)
		updates = append(updates, store.LootStack{Before: *item, Count: item.Count + count})
		existing = append(existing, item)
		remaining -= count
	}
	stack := int64(t.MaxStack)
	if stack < 1 {
		stack = remaining
	}
	slots := int64(0)
	if remaining > 0 {
		slots = (remaining-1)/stack + 1
	}
	if slots > int64(p.cubeLimit()-len(p.cube)) {
		p.conn.send(systemMessage(msgInventoryFull))
		return false
	}
	var inserts []store.Item
	for remaining > 0 {
		count := min(stack, remaining)
		inserts = append(inserts, *s.newItem(p, drop.item, count))
		remaining -= count
	}
	// Connections do not yet expose a lifecycle context; bound the atomic receipt meanwhile.
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	err := errors.New("group loot persistence unavailable")
	if db, ok := s.items.(bidReceiver); ok && payments != nil {
		err = db.ReceiveBidLoot(ctx, p.ID, updates, inserts, payments)
	} else if db, ok := s.items.(lootReceiver); ok && payments == nil {
		err = db.ReceiveLoot(ctx, p.ID, updates, inserts)
	}
	if err != nil {
		for _, item := range inserts {
			s.ids.release(item.UniqueID)
		}
		s.log.Error("receiving rolled loot", "err", err)
		return false
	}
	for index, update := range updates {
		existing[index].Count = update.Count
		p.conn.send(s.updateItemPacket(p, existing[index]))
	}
	for _, item := range inserts {
		added := new(store.Item)
		*added = item
		p.cube = append(p.cube, added)
		p.conn.send(s.addItemsPacket(p, added))
	}
	return true
}
