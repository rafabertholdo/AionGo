package game

import (
	"slices"

	"aionlightning/wire"
)

// Shops: an npc's dialog offers to buy from it (dialog 2) or sell to it (dialog 3), and the client then sends
// CM_BUY_ITEM with the goods.

// The prices are AL-Game's default prices.properties.
const (
	priceDefault      = 100
	priceModifier     = 100
	priceTaxes        = 100
	priceVendorBuy    = 100
	priceVendorSell   = 20
	dialogBuy         = 2
	dialogSell        = 3
	shopSellToPlayers = 0
	shopSellToShop    = 1
	shopBuyFromShop   = 13
	shopBuyAbyss      = 14
)

func init() {
	handlers[cmBuyItem] = (*conn).buyItem
	// The dialogs of the npcs' services are answered here; the rest are the quests'.
	quests := handlers[cmDialogSelect]
	handlers[cmDialogSelect] = func(c *conn, r *wire.Reader) {
		peek := wire.NewReader(slices.Clone(r.Data))
		id, dialog := peek.D(), peek.H()
		if peek.Err == nil && c.serviceDialog(id, dialog) {
			return
		}
		quests(c, r)
	}
}

// shopDialog is NpcController.onDialogSelect for the shop dialogs: the client is shown the goods or the sell window.
func (c *conn) shopDialog(id int32, dialog uint16) bool {
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := c.dialogNpc(id)
	if o == nil {
		return false
	}
	switch dialog {
	case dialogBuy:
		list := s.data.TradeLists[o.npc.ID]
		if list == nil || len(list.Tabs) == 0 {
			return false
		}
		c.send(tradeListPacket(o.id, list.Abyss, priceVendorBuy*list.SellRate/100, list.Tabs))
	case dialogSell:
		w := wire.Packet(smSellItem)
		w.D(o.id)
		w.D(vendorSellModifier())
		c.send(w)
	}
	return true
}

// tradeListPacket is SM_TRADELIST: which goods lists the npc sells.
func tradeListPacket(npc int32, abyss bool, modifier int32, tabs []int32) *wire.Writer {
	w := wire.Packet(smTradelist)
	w.D(npc)
	if abyss {
		w.C(2)
	} else {
		w.C(1)
	}
	w.D(modifier)
	w.H(uint16(len(tabs)))
	for _, tab := range tabs {
		w.D(tab)
	}
	return w
}

// vendorSellModifier is Prices.getVendorSellModifier.
func vendorSellModifier() int32 {
	return int32(float32(int32(float32(int32(float32(priceVendorSell)*priceDefault/100))*priceModifier/100)) * priceTaxes / 100)
}

// kinahForBuy is Prices.getKinahForBuy.
func kinahForBuy(required int64) int64 {
	return int64(float64(int64(float64(int64(float64(int64(float64(required)*priceVendorBuy/100))*priceDefault/100))*priceModifier/100)) * priceTaxes / 100)
}

// goods is one line of a buy or sell: the item id (the object id when selling) and how many.
type goods struct {
	id    int32
	count int64
}

// buyItem is CM_BUY_ITEM.
func (c *conn) buyItem(r *wire.Reader) {
	p := c.player
	seller := r.D()
	action := r.H()
	amount := int(r.H())
	var list []goods
	for range amount {
		id, count := r.D(), r.D()
		r.D()
		if count >= 1 {
			list = append(list, goods{id, int64(count)})
		}
	}
	if p == nil || r.Err != nil || len(list) == 0 {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.dead {
		return
	}
	switch action {
	case shopSellToPlayers:
		if seller := s.spawned[seller]; seller != nil {
			s.sellFromStore(seller, p, list)
		}
	case shopSellToShop:
		if c.dialogNpc(seller) != nil {
			s.sellToShop(p, list)
		}
	case shopBuyFromShop:
		if o := c.dialogNpc(seller); o != nil {
			s.buyFromShop(p, o, list)
		}
	}
}

// buyFromShop is TradeService.performBuyFromShop.
func (s *Server) buyFromShop(p *player, o *object, list []goods) bool {
	trade := s.data.TradeLists[o.npc.ID]
	if trade == nil {
		return false
	}
	allowed := map[int32]bool{}
	for _, tab := range trade.Tabs {
		for _, id := range s.data.GoodsLists[tab] {
			allowed[id] = true
		}
	}
	var required int64
	for _, g := range list {
		t := s.data.Items[g.id]
		if t == nil || !allowed[g.id] {
			return false
		}
		required += kinahForBuy(int64(t.Price)) * g.count * int64(trade.SellRate) / 100
	}
	required = kinahForBuy(required)
	if p.kinah.Count < required {
		return false
	}
	if p.cubeLimit()-len(p.cube)+1 < len(list) {
		return false
	}
	for _, g := range list {
		if !s.addItem(p, g.id, g.count) {
			s.decreaseKinah(p, required)
			return false
		}
	}
	s.decreaseKinah(p, required)
	return true
}

// sellToShop is TradeService.performSellToShop: the items are given for a part of their price.
func (s *Server) sellToShop(p *player, list []goods) bool {
	var reward int64
	for _, g := range list {
		item := p.cubeItem(g.id)
		if item == nil || item.Count < g.count {
			return false
		}
		t := s.template(item)
		if s.decreaseItemCount(p, item, g.count) == 0 && t != nil {
			reward += int64(t.Price) * g.count
		}
	}
	s.increaseKinah(p, int64(float64(reward)*float64(vendorSellModifier())/100))
	return true
}
