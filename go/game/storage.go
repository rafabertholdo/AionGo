package game

import (
	"slices"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// The storages an item is in (StorageType) and moving items between them: CM_MOVE_ITEM, CM_SPLIT_ITEM, CM_REPLACE_ITEM
// and CM_DELETE_ITEM. The account warehouse (2) is in accountwh.go; the legion's (3) isn't ported.

func init() {
	handlers[cmMoveItem] = (*conn).moveItem
	handlers[cmSplitItem] = (*conn).splitItem
	handlers[cmReplaceItem] = (*conn).replaceItem
	handlers[cmDeleteItem] = (*conn).deleteItem
}

const (
	storageCube      = 0
	storageWarehouse = 1
	storageAccount   = 2
	warehouseBase    = 104 // the slots of a warehouse that hasn't been expanded
	warehouseStep    = 8
)

func (p *player) warehouseLimit() int { return warehouseBase + p.WarehouseSize*warehouseStep }

// storage is the items of the storage and how many it holds at most; false for a storage that isn't ported.
func (p *player) storage(kind byte) (items *[]*store.Item, limit int, ok bool) {
	switch kind {
	case storageCube:
		return &p.cube, p.cubeLimit(), true
	case storageWarehouse:
		return &p.warehouse, p.warehouseLimit(), true
	case storageAccount:
		if p.acctWH != nil {
			return &p.acctWH.items, accountWarehouseSlots, true
		}
	}
	return nil, 0, false
}

// kinahOf is the money item of a storage; the warehouse's is made when something is first put in it.
func (s *Server) kinahOf(p *player, kind byte, create bool) *store.Item {
	if kind == storageCube {
		return p.kinah
	}
	slot := &p.warehouseKinah
	if kind == storageAccount {
		if p.acctWH == nil {
			return nil
		}
		slot = &p.acctWH.kinah
	}
	if *slot == nil && create {
		k := s.newItem(p, data.Kinah, 0)
		k.Owner = p.ownerOf(kind)
		k.Location = int8(kind)
		k.Slot = 0
		k.Count = 0
		if err := s.items.InsertItem(k); err != nil {
			s.log.Error("saving item", "item", data.Kinah, "err", err)
		}
		*slot = k
	}
	return *slot
}

// ownerOf is who the items of a storage belong to: the character, or the account for its warehouse.
func (p *player) ownerOf(kind byte) int32 {
	if kind == storageAccount {
		return p.AccountID
	}
	return p.ID
}

func itemIn(items []*store.Item, id int32) *store.Item {
	for _, item := range items {
		if item.UniqueID == id {
			return item
		}
	}
	return nil
}

// sendAdded, sendUpdated and sendDeleted tell the client what happened to an item of a storage.
func (s *Server) sendAdded(p *player, kind byte, item *store.Item) {
	if kind == storageCube {
		p.conn.send(s.addItemsPacket(p, item))
		return
	}
	w := wire.Packet(smWarehouseUpdate)
	w.C(kind)
	w.H(13)
	w.H(1)
	s.writeItem(w, p, item)
	p.conn.send(w)
}

func (s *Server) sendUpdated(p *player, kind byte, item *store.Item) {
	if kind == storageCube {
		p.conn.send(s.updateItemPacket(p, item))
		return
	}
	t := s.data.Items[item.ItemID]
	if t == nil {
		t = &data.ItemTemplate{ID: item.ItemID}
	}
	w := wire.Packet(smUpdateWarehouseItem)
	w.D(item.UniqueID)
	w.C(kind)
	w.H(0x24)
	w.D(t.NameID)
	w.H(0)
	s.writeItemDetails(w, p, item, t)
	p.conn.send(w)
}

func (s *Server) sendDeleted(p *player, kind byte, id int32) {
	if kind == storageCube {
		p.conn.send(deleteItemPacket(id))
		return
	}
	w := wire.Packet(smDeleteWarehouseItem)
	w.C(kind)
	w.D(id)
	w.C(14)
	p.conn.send(w)
}

// warehouseInfo is SM_WAREHOUSE_INFO: some of the items of a warehouse, ten at a time.
func (s *Server) warehouseInfo(p *player, items []*store.Item, kind byte, expand int, first bool) *wire.Writer {
	w := wire.Packet(smWarehouseInfo)
	w.C(kind)
	w.Bool(first)
	w.C(byte(expand))
	w.H(0)
	w.H(uint16(len(items)))
	for _, item := range items {
		s.writeItem(w, p, item)
	}
	return w
}

// sendWarehouseInfo is WarehouseService.sendWarehouseInfo: the regular warehouse, and the account's.
func (s *Server) sendWarehouseInfo(p *player, account bool) {
	items := p.warehouse
	if k := p.warehouseKinah; k != nil {
		items = append(slices.Clone(items), k)
	}
	first := true
	index := 0
	for ; index+10 < len(items); index += 10 {
		p.conn.send(s.warehouseInfo(p, items[index:index+10], storageWarehouse, p.WarehouseSize, first))
		first = false
	}
	if len(items) != 0 {
		p.conn.send(s.warehouseInfo(p, items[index:], storageWarehouse, p.WarehouseSize, first))
	}
	p.conn.send(s.warehouseInfo(p, nil, storageWarehouse, p.WarehouseSize, false))
	if account && p.acctWH != nil {
		p.conn.send(s.warehouseInfo(p, p.acctWH.all(), storageAccount, 0, true))
	} else if account {
		p.conn.send(s.warehouseInfo(p, nil, storageAccount, 0, true))
	}
	p.conn.send(s.warehouseInfo(p, nil, storageAccount, 0, false))
}

// moveItem is CM_MOVE_ITEM: the client puts an item in a slot, of the same storage or another.
func (c *conn) moveItem(r *wire.Reader) {
	p := c.player
	id, source, destination, slot := r.D(), r.C(), r.C(), r.H()
	if p == nil || r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	c.s.moveItem(p, id, source, destination, int32(slot))
}

// moveItem is ItemService.moveItem.
func (s *Server) moveItem(p *player, id int32, source, destination byte, slot int32) {
	from, _, ok := p.storage(source)
	if !ok {
		return
	}
	item := itemIn(*from, id)
	if item == nil {
		return
	}
	item.Slot = slot
	if source == destination {
		s.saveItem(item)
		s.sendUpdated(p, source, item)
		return
	}
	to, limit, ok := p.storage(destination)
	if !ok {
		return
	}
	t := s.template(item)
	if t == nil || len(*to) >= limit && !s.canMerge(*to, item, t) {
		p.conn.send(systemMessage(msgInventoryFull))
		return
	}
	*from = slices.DeleteFunc(*from, func(other *store.Item) bool { return other == item })
	s.sendDeleted(p, source, item.UniqueID)
	s.putItem(p, destination, item, t)
}

// canMerge is whether the item's stack fits in a stack the storage already has.
func (s *Server) canMerge(items []*store.Item, item *store.Item, t *data.ItemTemplate) bool {
	if t.MaxStack <= 1 {
		return false
	}
	free := int64(0)
	for _, other := range items {
		if other.ItemID == item.ItemID {
			free += int64(t.MaxStack) - other.Count
		}
	}
	return free >= item.Count
}

// putItem is ItemService.addFullItem: the item joins the stacks of its kind the storage has, and takes a place if
// there is anything left of it.
func (s *Server) putItem(p *player, kind byte, item *store.Item, t *data.ItemTemplate) {
	list, _, _ := p.storage(kind)
	if t.MaxStack > 1 {
		for _, other := range *list {
			if item.Count == 0 {
				break
			}
			if other.ItemID != item.ItemID || other.Count >= int64(t.MaxStack) {
				continue
			}
			take := min(item.Count, int64(t.MaxStack)-other.Count)
			other.Count += take
			item.Count -= take
			s.saveItem(other)
			s.sendUpdated(p, kind, other)
		}
	}
	if item.Count == 0 {
		if err := s.items.DeleteItem(item.UniqueID); err != nil {
			s.log.Error("deleting item", "item", item.ItemID, "err", err)
		}
		s.ids.release(item.UniqueID)
		return
	}
	item.Location = int8(kind)
	item.Owner = p.ownerOf(kind)
	*list = append(*list, item)
	s.saveItem(item)
	s.sendAdded(p, kind, item)
}

// splitItem is CM_SPLIT_ITEM: part of a stack is moved into a new item, or into a stack of the same kind.
func (c *conn) splitItem(r *wire.Reader) {
	id, amount := r.D(), int64(r.D())
	r.B(4)
	source := r.C()
	target := r.D()
	destination := r.C()
	slot := r.H()
	p := c.player
	if p == nil || r.Err != nil || amount < 1 {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if target == 0 {
		c.s.split(p, id, amount, int32(slot), source, destination)
	} else {
		c.s.merge(p, id, amount, target, source, destination)
	}
}

// split is ItemService.splitItem.
func (s *Server) split(p *player, id int32, amount int64, slot int32, source, destination byte) {
	from, _, ok := p.storage(source)
	to, limit, ok2 := p.storage(destination)
	if !ok || !ok2 {
		return
	}
	item := itemIn(*from, id)
	if item == nil {
		// The client moves kinah between storages by splitting it.
		if k := s.kinahOf(p, source, false); k != nil && k.UniqueID == id {
			s.moveKinah(p, source, destination, amount)
		}
		return
	}
	if item.Count <= amount || len(*to) >= limit {
		return
	}
	t := s.template(item)
	if t == nil {
		return
	}
	created := s.newItem(p, item.ItemID, amount)
	created.Slot = slot
	if !s.decreaseItemCountIn(p, source, item, amount) {
		return
	}
	created.SoulBound = item.SoulBound
	if err := s.items.InsertItem(created); err != nil {
		s.log.Error("saving item", "item", created.ItemID, "err", err)
	}
	s.putItemNew(p, destination, created, t)
}

// putItemNew is putItem for an item that hasn't a place in the storage or the database yet: it isn't merged.
func (s *Server) putItemNew(p *player, kind byte, item *store.Item, t *data.ItemTemplate) {
	list, _, _ := p.storage(kind)
	item.Location = int8(kind)
	item.Owner = p.ownerOf(kind)
	*list = append(*list, item)
	s.saveItem(item)
	s.sendAdded(p, kind, item)
}

// decreaseItemCountIn is ItemService.decreaseItemCount for any storage: the item loses count, and is removed if that is all.
func (s *Server) decreaseItemCountIn(p *player, kind byte, item *store.Item, count int64) bool {
	if item.Count < count {
		return false
	}
	item.Count -= count
	list, _, _ := p.storage(kind)
	if item.Count == 0 {
		*list = slices.DeleteFunc(*list, func(other *store.Item) bool { return other == item })
		if err := s.items.DeleteItem(item.UniqueID); err != nil {
			s.log.Error("deleting item", "item", item.ItemID, "err", err)
		}
		s.sendDeleted(p, kind, item.UniqueID)
		s.ids.release(item.UniqueID)
		return true
	}
	s.saveItem(item)
	s.sendUpdated(p, kind, item)
	return true
}

// moveKinah moves money between the cube and the warehouse.
func (s *Server) moveKinah(p *player, source, destination byte, amount int64) {
	from := s.kinahOf(p, source, false)
	if from == nil || from.Count < amount || source == destination {
		return
	}
	to := s.kinahOf(p, destination, true)
	from.Count -= amount
	to.Count += amount
	for _, side := range []struct {
		kind byte
		item *store.Item
	}{{source, from}, {destination, to}} {
		s.saveItem(side.item)
		s.sendUpdated(p, side.kind, side.item)
	}
}

// merge is ItemService.mergeItems: part of a stack is added to another stack of the same kind.
func (s *Server) merge(p *player, id int32, amount int64, targetID int32, source, destination byte) {
	from, _, ok := p.storage(source)
	to, _, ok2 := p.storage(destination)
	if !ok || !ok2 {
		return
	}
	item, target := itemIn(*from, id), itemIn(*to, targetID)
	if item == nil || target == nil || item == target || item.ItemID != target.ItemID || item.Count < amount {
		return
	}
	t := s.template(item)
	if t == nil || target.Count+amount > int64(t.MaxStack) {
		return
	}
	target.Count += amount
	s.saveItem(target)
	s.sendUpdated(p, destination, target)
	s.decreaseItemCountIn(p, source, item, amount)
}

// replaceItem is CM_REPLACE_ITEM: two items of a storage change places.
func (c *conn) replaceItem(r *wire.Reader) {
	sourceKind, sourceID, replaceKind, replaceID := r.C(), r.D(), r.C(), r.D()
	p := c.player
	if p == nil || r.Err != nil || sourceKind != replaceKind {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	list, _, ok := p.storage(sourceKind)
	if !ok {
		return
	}
	a, b := itemIn(*list, sourceID), itemIn(*list, replaceID)
	if a == nil || b == nil {
		return
	}
	a.Slot, b.Slot = b.Slot, a.Slot
	for _, item := range []*store.Item{a, b} {
		c.s.saveItem(item)
		c.s.sendUpdated(p, sourceKind, item)
	}
}

// deleteItem is CM_DELETE_ITEM: the player throws an item of its cube away.
func (c *conn) deleteItem(r *wire.Reader) {
	id := r.D()
	p := c.player
	if p == nil || r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if item := p.cubeItem(id); item != nil {
		c.s.removeItem(p, item)
	}
}
