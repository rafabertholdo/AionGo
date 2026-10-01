package game

import (
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// The broker: players put items up for sale for a price, other players of their race buy them, and the sellers
// settle their account for the kinah (AL-Game's BrokerService).
// ponytail: items are sorted by name id, as the template's name isn't kept; the lists aren't cached per player.

func init() {
	handlers[cmBrokerList] = (*conn).brokerList
	handlers[cmBrokerRegistered] = func(c *conn, r *wire.Reader) { r.D(); c.withPlayer(func(s *Server, p *player) { s.showRegistered(p) }) }
	handlers[cmBrokerSettleList] = func(c *conn, r *wire.Reader) { r.D(); c.withPlayer(func(s *Server, p *player) { s.showSettled(p) }) }
	handlers[cmBrokerSettleAccount] = func(c *conn, r *wire.Reader) { r.D(); c.withPlayer(func(s *Server, p *player) { s.settleAccount(p) }) }
	handlers[cmBuyBrokerItem] = (*conn).buyBrokerItem
	handlers[cmRegisterBrokerItem] = (*conn).registerBrokerItem
	handlers[cmBrokerCancelRegistered] = (*conn).cancelBrokerItem
}

// brokerSaver keeps the broker; store.Store does.
type brokerSaver interface {
	InsertBroker(*store.BrokerRow) error
	UpdateBroker(*store.BrokerRow) error
	DeleteBroker(*store.BrokerRow) error
}

const (
	brokerDays       = 8
	brokerCheckEvery = time.Minute
	brokerPage       = 9
	brokerNoKinah    = 1
	dialogBroker     = 27
)

// brokerItem is a BrokerItem.
type brokerItem struct {
	store.BrokerRow
	item *store.Item // what is for sale; nil once sold
}

// board is the broker of one race: what is for sale, and what is settled.
type board struct {
	items   map[int32]*brokerItem
	settled map[int32]*brokerItem
}

func (s *Server) boardOf(race string) *board {
	key := "ELYOS"
	if race == "ASMODIANS" || race == "ASMODIAN" {
		key = "ASMODIAN"
	}
	if s.boards == nil {
		s.boards = map[string]*board{}
	}
	if s.boards[key] == nil {
		s.boards[key] = &board{items: map[int32]*brokerItem{}, settled: map[int32]*brokerItem{}}
	}
	return s.boards[key]
}

func brokerRace(p *player) string {
	if p.Race == "ASMODIANS" {
		return "ASMODIAN"
	}
	return "ELYOS"
}

// loadBroker is BrokerService.initBrokerService.
func (s *Server) loadBroker() error {
	rows, items, err := s.store.Broker()
	if err != nil {
		return err
	}
	for _, row := range rows {
		b := &brokerItem{BrokerRow: row}
		if !row.Sold {
			for _, it := range items {
				if it.UniqueID == row.Pointer {
					b.item = it
				}
			}
		}
		if b.item == nil {
			b.Sold, b.Settled = true, true
		}
		board := s.boardOf(row.Race)
		if b.Settled {
			board.settled[row.Pointer] = b
		} else {
			board.items[row.Pointer] = b
		}
	}
	s.every(brokerCheckEvery, brokerCheckEvery, s.checkExpiredBroker)
	return nil
}

func (s *Server) saveBroker(b *brokerItem, remove bool) {
	var err error
	switch {
	case remove:
		err = s.brokerDB.DeleteBroker(&b.BrokerRow)
	case b.Settled:
		err = s.brokerDB.UpdateBroker(&b.BrokerRow)
	default:
		err = s.brokerDB.InsertBroker(&b.BrokerRow)
	}
	if err != nil {
		s.log.Error("saving the broker", "item", b.ItemID, "err", err)
	}
}

// brokerMatches is BrokerItemMask.isMatches, by the mask's id: items are grouped by the thousands and hundreds of their ids.
func brokerMatches(mask int, itemID int32) bool {
	group := int(itemID / 100000)
	between := func(lo, hi int) bool { return itemID >= int32(lo*100000) && itemID < int32(hi*100000) }
	one := func(groups ...int) bool { return slices.Contains(groups, group) }
	switch mask {
	case 9010:
		return between(1000, 1018)
	case 1000, 1001, 1002, 1005, 1006, 1009, 1013, 1015, 1017:
		return group == mask
	case 9020:
		return between(1101, 1160)
	case 8010:
		return one(1100, 1110, 1120, 1130, 1140)
	case 8020:
		return one(1101, 1111, 1121, 1131, 1141)
	case 8030:
		return one(1103, 1113, 1123, 1133, 1143)
	case 8040:
		return one(1105, 1115, 1125, 1135, 1145)
	case 8050:
		return one(1106, 1116, 1126, 1136, 1146)
	case 1100, 1101, 1103, 1105, 1106, 1110, 1111, 1113, 1115, 1116, 1120, 1121, 1123, 1125, 1126, 1130, 1131, 1133, 1135, 1136,
		1140, 1141, 1143, 1145, 1146, 1150, 1200, 1210, 1220, 1230:
		return group == mask
	case 9030:
		return between(1200, 1270)
	case 7030:
		return between(1250, 1270)
	case 9040:
		return one(1400, 1695)
	case 1400, 1695, 1520, 1522, 1600, 1620, 1660, 1670, 1680:
		return group == mask
	case 9050:
		return one(1520, 1522)
	case 6030:
		return int(itemID/10000) == 15200
	case 6031:
		return int(itemID/10000) == 15201
	case 6032:
		return int(itemID/10000) == 15202
	case 9060:
		return one(1410, 1600, 1620, 1640, 1690, 1694)
	case 7060:
		return group == 1640
	case 8060:
		return one(1660, 1670, 1680, 1692)
	case 7061:
		return group == 1692
	case 7062:
		return one(1410, 1690, 1694)
	case 7070:
		return one(1860, 1880)
	}
	return false
}

// brokerList is CM_BROKER_LIST: a page of what is for sale of a kind.
func (c *conn) brokerList(r *wire.Reader) {
	r.D()
	sortType, page, mask := int(r.C()), int(r.H()), int(r.H())
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		var found []*brokerItem
		for _, id := range sortedKeys(s.boardOf(p.Race).items) {
			if b := s.boardOf(p.Race).items[id]; b.item != nil && brokerMatches(mask, b.ItemID) {
				found = append(found, b)
			}
		}
		total := len(found)
		s.sortBroker(found, sortType)
		start := min(page*brokerPage, len(found))
		found = found[start:min(start+45, len(found))]
		p.conn.send(s.brokerItems(p, found, total, page))
	})
}

func (s *Server) sortBroker(list []*brokerItem, sortType int) {
	level := func(b *brokerItem) int32 {
		if t := s.data.Items[b.ItemID]; t != nil {
			return t.Level
		}
		return 0
	}
	name := func(b *brokerItem) int32 {
		if t := s.data.Items[b.ItemID]; t != nil {
			return t.NameID
		}
		return 0
	}
	piece := func(b *brokerItem) int64 { return b.Price / max(b.Count, 1) }
	cmp := func(a, b int64, desc bool) int {
		switch {
		case a == b:
			return 0
		case (a > b) != desc:
			return 1
		}
		return -1
	}
	slices.SortStableFunc(list, func(a, b *brokerItem) int {
		switch sortType {
		case 0, 1:
			return cmp(int64(name(a)), int64(name(b)), false)
		case 2, 3:
			return cmp(int64(level(a)), int64(level(b)), sortType == 3)
		case 4, 5:
			return cmp(a.Price, b.Price, sortType == 5)
		case 6, 7:
			return cmp(piece(a), piece(b), sortType == 7)
		}
		return 0
	})
}

func (s *Server) brokerItems(p *player, items []*brokerItem, total, page int) *wire.Writer {
	w := wire.Packet(smBrokerItems)
	w.D(int32(total))
	w.C(0)
	w.H(uint16(page))
	w.H(uint16(len(items)))
	for _, b := range items {
		t := s.data.Items[b.ItemID]
		if t == nil {
			t = &data.ItemTemplate{ID: b.ItemID}
		}
		w.D(b.item.UniqueID)
		w.D(t.ID)
		w.Q(b.Price)
		w.Q(b.item.Count)
		if t.IsArmor() || t.IsWeapon() {
			w.C(0)
			w.C(byte(b.item.Enchant))
			w.D(b.item.SkinID())
			w.C(0)
			s.writeStones(w, nil)
			w.D(0) // god stone
			w.C(0)
			w.D(0)
			w.D(0)
		} else {
			for range 9 {
				w.D(0)
			}
			w.H(0)
		}
		w.S(b.Seller)
		w.S("")
	}
	return w
}

// registerBrokerItem is CM_REGISTER_BROKER_ITEM.
func (c *conn) registerBrokerItem(r *wire.Reader) {
	r.D()
	id, price := r.D(), r.D()
	r.D()
	r.H()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) { s.registerBrokerItem(p, id, int64(price)) })
}

func brokerRegistration(b *brokerItem) *wire.Writer {
	w := wire.Packet(smBrokerRegistrationService)
	w.H(0)
	w.D(b.item.UniqueID)
	w.D(b.ItemID)
	w.Q(b.Price)
	w.Q(b.item.Count)
	w.Q(b.item.Count)
	w.H(brokerDays)
	w.C(0)
	w.D(b.ItemID)
	for range 8 {
		w.D(0)
	}
	w.H(0)
	return w
}

// registerBrokerItem is BrokerService.registerItem.
func (s *Server) registerBrokerItem(p *player, id int32, price int64) {
	item := p.cubeItem(id)
	if item == nil || item.SoulBound {
		return
	}
	if t := s.template(item); t == nil || t.Mask&itemTradeable == 0 {
		return
	}
	commission := max(mailRound(float32(price)*0.02), 10)
	if p.kinah.Count < commission {
		w := wire.Packet(smBrokerRegistrationService)
		w.H(brokerNoKinah)
		p.conn.send(w)
		return
	}
	s.decreaseKinah(p, commission)
	p.cube = removeFromCube(p.cube, item)
	s.sendDeleted(p, storageCube, item.UniqueID)
	item.Location = store.BrokerLocation
	s.saveItem(item)
	now := time.Now()
	b := &brokerItem{BrokerRow: store.BrokerRow{Pointer: item.UniqueID, ItemID: item.ItemID, Count: item.Count, Seller: p.Name, SellerID: p.ID,
		Price: price, Race: brokerRace(p), Expire: now.Add(brokerDays * 24 * time.Hour), Settle: now}, item: item}
	s.boardOf(p.Race).items[item.UniqueID] = b
	s.saveBroker(b, false)
	p.conn.send(brokerRegistration(b))
}

// buyBrokerItem is CM_BUY_BROKER_ITEM.
func (c *conn) buyBrokerItem(r *wire.Reader) {
	r.D()
	id := r.D()
	r.H()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) { s.buyFromBroker(p, id) })
}

// buyFromBroker is BrokerService.buyBrokerItem.
func (s *Server) buyFromBroker(p *player, id int32) {
	if p.cubeFull() {
		p.conn.send(systemMessage(msgInventoryFull))
		return
	}
	board := s.boardOf(p.Race)
	b := board.items[id]
	if b == nil || b.item == nil || p.kinah.Count < b.Price {
		return
	}
	delete(board.items, id)
	item := b.item
	s.settle(p.Race, b, true)
	s.decreaseKinah(p, b.Price)
	item.Owner = p.ID
	s.putItem(p, storageCube, item, s.template(item))
}

// settle is BrokerService.putToSettled: the item is sold, or came back unsold, and its seller may collect it.
func (s *Server) settle(race string, b *brokerItem, sold bool) {
	if sold {
		b.item = nil
		b.Sold, b.Settled = true, true
	} else {
		b.Settled = true
	}
	b.Settle = time.Now()
	s.boardOf(race).settled[b.Pointer] = b
	s.saveBroker(b, false)
	if seller := s.spawned[b.SellerID]; seller != nil {
		seller.conn.send(brokerIcon(true))
	}
}

func brokerIcon(items bool) *wire.Writer {
	w := wire.Packet(smBrokerSettledList)
	w.D(0)
	w.Bool(items)
	w.D(0)
	w.H(1)
	w.D(0)
	w.C(1)
	w.H(0)
	return w
}

func (s *Server) showRegistered(p *player) {
	w := wire.Packet(smBrokerRegisteredList)
	w.D(0)
	var mine []*brokerItem
	for _, id := range sortedKeys(s.boardOf(p.Race).items) {
		if b := s.boardOf(p.Race).items[id]; b.item != nil && b.SellerID == p.ID {
			mine = append(mine, b)
		}
	}
	w.H(uint16(len(mine)))
	for _, b := range mine {
		w.D(b.item.UniqueID)
		w.D(b.ItemID)
		w.Q(b.Price)
		w.Q(b.item.Count)
		w.Q(b.item.Count)
		w.H(uint16(max(int(time.Until(b.Expire)/(24*time.Hour)+0), 0)))
		w.C(0)
		w.D(b.ItemID)
		for range 8 {
			w.D(0)
		}
		w.H(0)
	}
	p.conn.send(w)
}

// cancelBrokerItem is CM_BROKER_CANCEL_REGISTERED: an item for sale comes back to its seller.
func (c *conn) cancelBrokerItem(r *wire.Reader) {
	r.D()
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		board := s.boardOf(p.Race)
		if b := board.items[id]; b != nil && b.item != nil && b.SellerID == p.ID {
			item := b.item
			item.Owner = p.ID
			s.putItem(p, storageCube, item, s.template(item))
			delete(board.items, id)
			s.saveBroker(b, true)
		}
		s.showRegistered(p)
	})
}

func (s *Server) showSettled(p *player) {
	var mine []*brokerItem
	var total int64
	for _, id := range sortedKeys(s.boardOf(p.Race).settled) {
		if b := s.boardOf(p.Race).settled[id]; b.SellerID == p.ID {
			mine = append(mine, b)
			if b.Sold {
				total += b.Price
			}
		}
	}
	w := wire.Packet(smBrokerSettledList)
	w.Q(total)
	w.H(1)
	w.D(0)
	w.C(0)
	w.H(uint16(len(mine)))
	for _, b := range mine {
		w.D(b.ItemID)
		if b.Sold {
			w.Q(b.Price)
		} else {
			w.Q(0)
		}
		w.Q(b.Count)
		w.Q(b.Count)
		w.D(int32(b.Settle.UnixMilli() / 60000))
		w.H(0)
		w.D(b.ItemID)
		for range 8 {
			w.D(0)
		}
		w.H(0)
	}
	p.conn.send(w)
}

// settleAccount is BrokerService.settleAccount: the seller takes the kinah of what sold, and what didn't back.
func (s *Server) settleAccount(p *player) {
	board := s.boardOf(p.Race)
	var kinah int64
	itemsLeft := false
	for _, id := range sortedKeys(board.settled) {
		b := board.settled[id]
		if b.SellerID != p.ID {
			continue
		}
		if b.Sold {
			delete(board.settled, id)
			s.saveBroker(b, true)
			kinah += b.Price
			continue
		}
		if b.item == nil {
			continue
		}
		if p.cubeFull() {
			itemsLeft = true
			continue
		}
		item := b.item
		item.Owner = p.ID
		delete(board.settled, id)
		s.saveBroker(b, true)
		s.putItem(p, storageCube, item, s.template(item))
	}
	s.increaseKinah(p, kinah)
	s.showSettled(p)
	if !itemsLeft {
		p.conn.send(brokerIcon(false))
	}
}

// checkExpiredBroker is BrokerService.checkExpiredItems: what has been for sale too long comes back to its seller.
func (s *Server) checkExpiredBroker() {
	now := time.Now()
	for race, board := range s.boards {
		for id, b := range board.items {
			if !b.Expire.After(now) {
				delete(board.items, id)
				s.settle(race, b, false)
			}
		}
	}
}

// brokerLogin is BrokerService.onPlayerLogin: a seller with something to collect is shown the icon.
func (s *Server) brokerLogin(p *player) {
	for _, b := range s.boardOf(p.Race).settled {
		if b.SellerID == p.ID {
			p.conn.send(brokerIcon(true))
			return
		}
	}
}
