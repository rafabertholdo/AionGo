package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// itemSaver is what keeps items: store.Store.
type itemSaver interface {
	InsertItem(*store.Item) error
	UpdateItem(*store.Item) error
	DeleteItem(int32) error
	AddStone(itemUniqueID int32, stone store.Stone) error
	DeleteStone(itemUniqueID, slot int32) error
	DeleteStones(itemUniqueID int32) error
}

// firstAvailableSlot is ItemStorage.FIRST_AVAILABLE_SLOT: where a new item is until the client puts it somewhere.
const firstAvailableSlot = 65535

// cubeLimit is how many items the cube holds (CM_ENTER_WORLD's 27 + 9 for each expansion).
func (p *player) cubeLimit() int { return 27 + p.CubeSize*9 }

func (p *player) cubeFull() bool { return len(p.cube) >= p.cubeLimit() }

// cubeItem is the item of the cube with the unique id, or nil.
func (p *player) cubeItem(id int32) *store.Item {
	for _, item := range p.cube {
		if item.UniqueID == id {
			return item
		}
	}
	return nil
}

// System messages of the inventory.
const (
	msgInventoryFull = 1390182 // STR_MSG_DICE_INVEN_ERROR
)

// updateItemPacket is SM_UPDATE_ITEM: an item's count or slot changed.
func (s *Server) updateItemPacket(p *player, item *store.Item) *wire.Writer {
	t := s.data.Items[item.ItemID]
	if t == nil {
		t = &data.ItemTemplate{ID: item.ItemID}
	}
	w := wire.Packet(smUpdateItem)
	w.D(item.UniqueID)
	w.H(0x24)
	w.D(t.NameID)
	w.H(0)
	switch {
	case t.ID == data.Kinah:
		w.H(0x16)
		w.C(0)
		w.H(uint16(t.Mask))
		w.Q(item.Count)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		w.C(0x1a)
		w.C(0)
	case t.IsWeapon() || t.IsArmor():
		s.writeItemDetails(w, p, item, t)
	case t.IsStigma():
		w.H(0x05)
		w.C(0x06)
		if item.Equipped {
			w.D(item.Slot)
		} else {
			w.D(0)
		}
	default:
		w.H(0x16)
		w.C(0)
		w.H(uint16(t.Mask))
		w.D(int32(item.Count))
		w.D(0)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		w.H(0)
		w.H(uint16(item.Slot))
	}
	return w
}

// addItemsPacket is SM_ADD_ITEMS: items that came into the cube.
func (s *Server) addItemsPacket(p *player, items ...*store.Item) *wire.Writer {
	w := wire.Packet(smAddItems)
	w.H(25)
	w.H(uint16(len(items)))
	for _, item := range items {
		s.writeItem(w, p, item)
	}
	return w
}

func deleteItemPacket(id int32) *wire.Writer {
	w := wire.Packet(smDeleteItem)
	w.D(id)
	w.C(0)
	return w
}

// newItem is ItemService.newItem: an item that isn't anywhere yet, with at most a stack's count.
func (s *Server) newItem(p *player, itemID int32, count int64) *store.Item {
	t := s.data.Items[itemID]
	if t == nil {
		s.log.Error("no item template", "item", itemID)
		return nil
	}
	if t.MaxStack != 0 && count > int64(t.MaxStack) {
		count = int64(t.MaxStack)
	}
	return &store.Item{UniqueID: s.ids.nextID(), ItemID: itemID, Count: count, Owner: p.ID, Slot: firstAvailableSlot}
}

// addItem is ItemService.addItem: the player gets the items, in the stacks of the ones it has and then in new
// places, or as much as fits. False if none did.
func (s *Server) addItem(p *player, itemID int32, count int64) bool {
	if count < 1 {
		return false
	}
	if itemID == data.Kinah {
		return s.increaseKinah(p, count)
	}
	t := s.data.Items[itemID]
	if t == nil {
		return false
	}
	stack := int64(t.MaxStack)
	// Merge into the stacks it has.
	for _, existing := range p.cube {
		if count < 1 {
			break
		}
		if existing.ItemID != itemID {
			continue
		}
		free := stack - existing.Count
		if free <= 0 {
			continue
		}
		take := min(count, free)
		s.increaseItemCount(p, existing, take)
		count -= take
	}
	for count > 0 && !p.cubeFull() {
		n := count
		if stack > 0 && n > stack {
			n = stack
		}
		item := s.newItem(p, itemID, n)
		if item == nil {
			return false
		}
		p.cube = append(p.cube, item)
		if err := s.items.InsertItem(item); err != nil {
			s.log.Error("saving item", "item", itemID, "err", err)
		}
		p.conn.send(s.addItemsPacket(p, item))
		count -= n
	}
	if count > 0 {
		p.conn.send(systemMessage(msgInventoryFull))
		return false
	}
	return true
}

// increaseItemCount is ItemService.increaseItemCount.
func (s *Server) increaseItemCount(p *player, item *store.Item, count int64) {
	item.Count += count
	s.saveItem(item)
	p.conn.send(s.updateItemPacket(p, item))
}

// decreaseItemCount is ItemService.decreaseItemCount: the item loses count, is removed if that is all of it,
// and what is left of count is returned.
func (s *Server) decreaseItemCount(p *player, item *store.Item, count int64) int64 {
	take := min(count, item.Count)
	item.Count -= take
	count -= take
	if item.Count == 0 {
		s.removeItem(p, item)
	} else {
		s.saveItem(item)
	}
	p.conn.send(s.updateItemPacket(p, item))
	return count
}

// removeItem is ItemService.removeItem: the item is no longer in the cube, nor anywhere.
func (s *Server) removeItem(p *player, item *store.Item) {
	for i, other := range p.cube {
		if other == item {
			p.cube = append(p.cube[:i], p.cube[i+1:]...)
			break
		}
	}
	if err := s.items.DeleteItem(item.UniqueID); err != nil {
		s.log.Error("deleting item", "item", item.ItemID, "err", err)
	}
	p.conn.send(deleteItemPacket(item.UniqueID))
	s.ids.release(item.UniqueID)
}

func (s *Server) saveItem(item *store.Item) {
	if err := s.items.UpdateItem(item); err != nil {
		s.log.Error("saving item", "item", item.ItemID, "err", err)
	}
}

// increaseKinah is ItemService.increaseKinah.
func (s *Server) increaseKinah(p *player, amount int64) bool {
	updated := *p.kinah
	updated.Count += amount
	if err := s.items.UpdateItem(&updated); err != nil {
		s.log.Error("saving kinah", "err", err)
		return false
	}
	*p.kinah = updated
	p.conn.send(s.updateItemPacket(p, p.kinah))
	return true
}

// decreaseKinah is ItemService.decreaseKinah: false if the player hasn't that much.
func (s *Server) decreaseKinah(p *player, amount int64) bool {
	if p.kinah.Count < amount {
		return false
	}
	updated := *p.kinah
	updated.Count -= amount
	if err := s.items.UpdateItem(&updated); err != nil {
		s.log.Error("saving kinah", "err", err)
		return false
	}
	*p.kinah = updated
	p.conn.send(s.updateItemPacket(p, p.kinah))
	return true
}
