package game

import (
	"slices"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// Private stores: a player sells items from its cube at its own prices (AL-Game's PrivateStoreService).
// ponytail: the item details in SM_PRIVATE_STORE are the cube's; AL-Game writes them a little differently for a store.

func init() {
	handlers[cmPrivateStore] = (*conn).privateStore
	handlers[cmPrivateStoreName] = (*conn).privateStoreName
}

// privateStore is a player's PrivateStore.
type privateStore struct {
	message string
	items   []*storeItem
}

// storeItem is TradePSItem: an item of the cube for sale, how many and for how much each.
type storeItem struct {
	objID, itemID int32
	count         int64
	price         int32
}

func (ps *privateStore) find(objID int32) *storeItem {
	for _, it := range ps.items {
		if it.objID == objID {
			return it
		}
	}
	return nil
}

// privateStore is CM_PRIVATE_STORE: the items the player puts up for sale, or none to close the store.
func (c *conn) privateStore(r *wire.Reader) {
	n := int(r.H())
	var list []storeItem
	for range n {
		list = append(list, storeItem{objID: r.D(), itemID: r.D(), count: int64(r.H()), price: r.D()})
	}
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if n == 0 {
			s.closeStore(p)
			return
		}
		s.addToStore(p, list)
	})
}

// addToStore is PrivateStoreService.addItem.
func (s *Server) addToStore(p *player, list []storeItem) {
	if p.store == nil {
		p.store = &privateStore{}
		p.state |= statePrivateShop
		p.broadcast(s.playerEmotion(p, emoteOpenShop, 0, 0, 0, 0, 0), true)
	}
	for _, it := range list {
		item := p.cubeItem(it.objID)
		if item == nil || item.SoulBound {
			continue
		}
		if t := s.template(item); t == nil || t.Mask&itemTradeable == 0 {
			continue
		}
		if item.ItemID != it.itemID || it.count > item.Count || it.count < 1 {
			return
		}
		if old := p.store.find(it.objID); old != nil {
			*old = it
		} else {
			it := it
			p.store.items = append(p.store.items, &it)
		}
	}
}

func (s *Server) closeStore(p *player) {
	p.store = nil
	p.state &^= statePrivateShop
	p.broadcast(s.playerEmotion(p, emoteCloseShop, 0, 0, 0, 0, 0), true)
}

// privateStoreName is CM_PRIVATE_STORE_NAME: what the store's sign says.
func (c *conn) privateStoreName(r *wire.Reader) {
	name := r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if p.store == nil {
			return
		}
		p.store.message = name
		w := wire.Packet(smPrivateStoreName)
		w.D(p.ID)
		w.S(name)
		p.broadcast(w, true)
	})
}

// privateStorePacket is SM_PRIVATE_STORE: what a store sells.
func (s *Server) privateStorePacket(seller *player) *wire.Writer {
	w := wire.Packet(smPrivateStore)
	w.D(seller.ID)
	w.H(uint16(len(seller.store.items)))
	for _, it := range seller.store.items {
		item := seller.cubeItem(it.objID)
		if item == nil {
			continue
		}
		t := s.template(item)
		if t == nil {
			t = &data.ItemTemplate{ID: item.ItemID}
		}
		w.D(it.objID)
		w.D(t.ID)
		w.H(uint16(it.count))
		w.D(it.price)
		s.writeItemDetails(w, seller, item, t)
	}
	return w
}

// sellFromStore is PrivateStoreService.sellStoreItem: the buyer takes goods of the seller's store, by their place
// in the store, and pays for them.
func (s *Server) sellFromStore(seller, buyer *player, list []goods) {
	if seller.store == nil || seller == buyer {
		return
	}
	type pick struct {
		it    *storeItem
		count int64
	}
	var picks []pick
	var price int64
	for _, g := range list {
		if g.id < 0 || int(g.id) >= len(seller.store.items) {
			return
		}
		it := seller.store.items[g.id]
		item := seller.cubeItem(it.objID)
		if item == nil || g.count > it.count || g.count > item.Count {
			return
		}
		picks = append(picks, pick{it, g.count})
		price += int64(it.price) * g.count
	}
	if len(picks) == 0 || buyer.cubeLimit()-len(buyer.cube)+1 < len(picks) || !s.decreaseKinah(buyer, price) {
		return
	}
	s.increaseKinah(seller, price)
	for _, pk := range picks {
		item := seller.cubeItem(pk.it.objID)
		if item == nil {
			continue
		}
		if item.Count == pk.count {
			seller.cube = removeFromCube(seller.cube, item)
			s.sendDeleted(seller, storageCube, item.UniqueID)
			item.Owner = buyer.ID
			s.putItem(buyer, storageCube, item, s.template(item))
		} else {
			s.decreaseItemCount(seller, item, pk.count)
			s.addItem(buyer, item.ItemID, pk.count)
		}
		pk.it.count -= pk.count
		if pk.it.count == 0 {
			seller.store.items = slices.DeleteFunc(seller.store.items, func(o *storeItem) bool { return o == pk.it })
		}
	}
	if len(seller.store.items) == 0 {
		s.closeStore(seller)
	} else {
		buyer.conn.send(s.privateStorePacket(seller))
	}
}
