package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// Trading between two players: AL-Game's ExchangeService. Each side offers items and kinah, locks its offer, and
// confirms; when both have, the goods change hands.

func init() {
	handlers[cmExchangeRequest] = (*conn).exchangeRequest
	handlers[cmExchangeAddItem] = (*conn).exchangeAddItem
	handlers[cmExchangeAddKinah] = (*conn).exchangeAddKinah
	handlers[cmExchangeLock] = (*conn).exchangeLock
	handlers[cmExchangeCancel] = (*conn).exchangeCancel
	handlers[cmExchangeOk] = (*conn).exchangeOK
}

const (
	questionExchange       = 0x15f91
	msgRequestTrade        = 1300353
	msgTradeRejected       = 1300354
	msgTradeDenied         = 1390115
	deniedTrade      int32 = 2 // DeniedStatus.TRADE
	itemTradeable          = 1 << 1
)

// exchange is one side of a trade: what the player offers.
type exchange struct {
	me, partner *player
	confirmed   bool
	locked      bool
	kinah       int64
	items       map[int32]*offer // by the object id of the item it is taken from
	order       []int32
}

// offer is an item that is offered, or a part of a stack.
type offer struct {
	count int64
	shown *store.Item // what the partner is shown: the item itself, or a new one with the count
}

const exchangeSlots = 9

func (s *Server) exchangeOf(p *player) *exchange { return s.exchanges[p.ID] }

// exchangeRequest is CM_EXCHANGE_REQUEST: the player asks another to trade.
func (c *conn) exchangeRequest(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		target := s.spawned[id]
		if target == nil || target == p {
			return
		}
		if target.settings.Deny&deniedTrade != 0 {
			p.conn.send(systemMessage(msgTradeDenied, target.Name))
			return
		}
		p.conn.send(systemMessage(msgRequestTrade, target.Name))
		asked := target.putRequest(questionExchange, func(accepted bool) {
			if !accepted {
				p.conn.send(systemMessage(msgTradeRejected, target.Name))
				return
			}
			s.registerExchange(p, target)
		})
		if asked {
			target.conn.send(questionWindow(questionExchange, 0, p.Name))
		}
	})
}

// registerExchange is ExchangeService.registerExchange.
func (s *Server) registerExchange(a, b *player) {
	if s.spawned[a.ID] == nil || s.spawned[b.ID] == nil || a.trading || b.trading {
		return
	}
	a.trading, b.trading = true, true
	if s.exchanges == nil {
		s.exchanges = map[int32]*exchange{}
	}
	s.exchanges[a.ID] = &exchange{me: a, partner: b, items: map[int32]*offer{}}
	s.exchanges[b.ID] = &exchange{me: b, partner: a, items: map[int32]*offer{}}
	for _, pair := range [][2]*player{{a, b}, {b, a}} {
		w := wire.Packet(smExchangeRequest)
		w.S(pair[1].Name)
		pair[0].conn.send(w)
	}
}

func exchangeKinahPacket(count int64, other byte) *wire.Writer {
	w := wire.Packet(smExchangeAddKinah)
	w.C(other)
	w.D(int32(count))
	w.D(0)
	return w
}

// exchangeConfirmation is SM_EXCHANGE_CONFIRMATION: 0 done, 1 cancelled, 2 the partner confirmed, 3 the partner locked.
func exchangeConfirmation(action byte) *wire.Writer {
	w := wire.Packet(smExchangeConfirmation)
	w.C(action)
	return w
}

// exchangeItemPacket is SM_EXCHANGE_ADD_ITEM: an item a player offers, for it (0) or for the other (1).
func (s *Server) exchangeItemPacket(p *player, other byte, item *store.Item) *wire.Writer {
	t := s.data.Items[item.ItemID]
	if t == nil {
		t = &data.ItemTemplate{ID: item.ItemID}
	}
	w := wire.Packet(smExchangeAddItem)
	w.C(other)
	w.D(t.ID)
	w.D(item.UniqueID)
	w.H(0x24)
	w.D(t.NameID)
	w.H(0)
	s.writeItemDetails(w, p, item, t)
	if t.ID != data.Kinah && !t.IsWeapon() && !t.IsArmor() {
		w.C(0)
	}
	return w
}

// exchangeAddKinah is CM_EXCHANGE_ADD_KINAH.
func (c *conn) exchangeAddKinah(r *wire.Reader) {
	count := int64(r.D())
	r.D()
	if r.Err != nil || count < 1 {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		e := s.exchangeOf(p)
		if e == nil || e.locked {
			return
		}
		add := min(count, p.kinah.Count-e.kinah)
		if add <= 0 {
			return
		}
		p.conn.send(exchangeKinahPacket(add, 0))
		e.partner.conn.send(exchangeKinahPacket(add, 1))
		e.kinah += add
	})
}

// exchangeAddItem is CM_EXCHANGE_ADD_ITEM.
func (c *conn) exchangeAddItem(r *wire.Reader) {
	id, count := r.D(), int64(r.D())
	if r.Err != nil || count < 1 {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		item := p.cubeItem(id)
		e := s.exchangeOf(p)
		if item == nil || e == nil || e.locked || count > item.Count || item.SoulBound {
			return
		}
		if t := s.template(item); t == nil || t.Mask&itemTradeable == 0 {
			return
		}
		o := e.items[id]
		switch {
		case o == nil:
			if len(e.items) >= exchangeSlots {
				return
			}
			shown := item
			if count != item.Count {
				shown = &store.Item{UniqueID: s.ids.nextID(), ItemID: item.ItemID, Count: count, Owner: p.ID}
			}
			o = &offer{count: count, shown: shown}
			e.items[id] = o
			e.order = append(e.order, id)
		default:
			if o.count == item.Count {
				return
			}
			o.count += min(count, item.Count-o.count)
			o.shown.Count = o.count
		}
		p.conn.send(s.exchangeItemPacket(p, 0, o.shown))
		e.partner.conn.send(s.exchangeItemPacket(p, 1, o.shown))
	})
}

// exchangeLock is CM_EXCHANGE_LOCK: the player's offer is final, and its partner is told.
func (c *conn) exchangeLock(*wire.Reader) {
	c.withPlayer(func(s *Server, p *player) {
		if e := s.exchangeOf(p); e != nil {
			e.locked = true
			e.partner.conn.send(exchangeConfirmation(3))
		}
	})
}

// exchangeCancel is CM_EXCHANGE_CANCEL.
func (c *conn) exchangeCancel(*wire.Reader) {
	c.withPlayer(func(s *Server, p *player) {
		e := s.exchangeOf(p)
		var partner *player
		if e != nil {
			partner = e.partner
		}
		s.cleanupExchange(p, partner)
		if partner != nil {
			partner.conn.send(exchangeConfirmation(1))
		}
	})
}

// exchangeOK is CM_EXCHANGE_OK: the player confirms; when its partner has too, the trade is made.
func (c *conn) exchangeOK(*wire.Reader) {
	c.withPlayer(func(s *Server, p *player) {
		e := s.exchangeOf(p)
		if e == nil {
			return
		}
		e.confirmed = true
		e.partner.conn.send(exchangeConfirmation(2))
		if theirs := s.exchangeOf(e.partner); theirs != nil && theirs.confirmed {
			s.performTrade(p, e.partner)
		}
	})
}

// performTrade is ExchangeService.performTrade.
func (s *Server) performTrade(a, b *player) {
	ea, eb := s.exchangeOf(a), s.exchangeOf(b)
	free := func(p *player) int { return p.cubeLimit() - len(p.cube) }
	if free(a) < len(eb.items) || free(b) < len(ea.items) {
		return
	}
	a.conn.send(exchangeConfirmation(0))
	b.conn.send(exchangeConfirmation(0))
	s.handOver(ea, b)
	s.handOver(eb, a)
	s.cleanupExchange(a, b)
}

// handOver is ExchangeService.doExchanges: what a player offered goes to the other.
func (s *Server) handOver(e *exchange, to *player) {
	from := e.me
	for _, id := range e.order {
		item := from.cubeItem(id)
		if item == nil {
			continue
		}
		o := e.items[id]
		if o.count == item.Count {
			from.cube = removeFromCube(from.cube, item)
			s.sendDeleted(from, storageCube, item.UniqueID)
			item.Owner = to.ID
			s.putItem(to, storageCube, item, s.template(item))
		} else {
			s.decreaseItemCount(from, item, o.count)
			s.addItem(to, item.ItemID, o.count)
		}
	}
	if e.kinah > 0 && s.decreaseKinah(from, e.kinah) {
		s.increaseKinah(to, e.kinah)
	}
}

func removeFromCube(list []*store.Item, item *store.Item) []*store.Item {
	for i, other := range list {
		if other == item {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// cleanupExchange is ExchangeService.cleanupExchanges.
func (s *Server) cleanupExchange(a, b *player) {
	for _, p := range []*player{a, b} {
		if p == nil {
			continue
		}
		if e := s.exchanges[p.ID]; e != nil {
			for id, o := range e.items {
				if o.shown.UniqueID != id {
					s.ids.release(o.shown.UniqueID)
				}
			}
		}
		delete(s.exchanges, p.ID)
		p.trading = false
	}
}
