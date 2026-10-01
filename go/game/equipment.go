package game

import (
	"cmp"
	"slices"
	"strconv"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() {
	handlers[cmEquipItem] = (*conn).equip
}

// What each weapon type needs to be worn: one of the skills, and how many hands (WeaponType).
var weaponRules = map[string]struct {
	skills []int32
	hands  int
}{
	"DAGGER_1H": {[]int32{30, 9}, 1}, "MACE_1H": {[]int32{3, 10}, 1}, "SWORD_1H": {[]int32{1, 8}, 1},
	"TOOLHOE_1H": {nil, 1}, "BOOK_2H": {[]int32{64}, 2}, "ORB_2H": {[]int32{64}, 2}, "POLEARM_2H": {[]int32{16}, 2},
	"STAFF_2H": {[]int32{53}, 2}, "SWORD_2H": {[]int32{15}, 2}, "TOOLPICK_2H": {nil, 2}, "TOOLROD_2H": {nil, 2},
	"BOW": {[]int32{17}, 2},
}

// The skills that let a class wear an armor type (ArmorType).
var armorSkills = map[string][]int32{
	"CHAIN": {6, 13}, "CLOTHES": {4}, "LEATHER": {5, 12}, "PLATE": {18}, "ROBE": {67, 70}, "SHIELD": {7, 14},
}

// Messages of equipping.
const (
	msgItemTooLowLevel = 1300372 // "You must be level %0 to use %1."
)

// equip is CM_EQUIP_ITEM: the player puts an item on or takes it off.
func (c *conn) equip(r *wire.Reader) {
	p := c.player
	action, slot, id := r.C(), r.D(), r.D()
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.dead || s.restrictedInPrison(p, "equip / unequip item") {
		return
	}
	var result *store.Item
	switch action {
	case 0:
		result = s.equipItem(p, id, slot)
	case 1:
		result = s.unequipItem(p, id)
	}
	if result != nil {
		p.broadcast(s.appearancePacket(p), true)
	}
}

// appearancePacket is SM_UPDATE_PLAYER_APPEARANCE: what the player wears, for everyone who sees it.
func (s *Server) appearancePacket(p *player) *wire.Writer {
	w := wire.Packet(smUpdatePlayerAppearance)
	w.D(p.ID)
	var mask uint16
	var worn []*store.Item
	for _, item := range p.equipment {
		if item.Slot < 1<<19 {
			mask |= uint16(item.Slot)
			worn = append(worn, item)
		}
	}
	w.H(mask)
	for _, item := range worn {
		w.D(item.SkinID())
		w.D(0) // god stone
		w.D(item.Color)
		w.H(0)
	}
	return w
}

func (p *player) hasSkill(id int32) bool {
	return slices.ContainsFunc(p.skills, func(sk store.Skill) bool { return sk.ID == id })
}

func (p *player) hasAnySkill(ids []int32) bool {
	return len(ids) == 0 || slices.ContainsFunc(ids, p.hasSkill)
}

func (p *player) equippedIn(slot int32) *store.Item {
	for _, item := range p.equipment {
		if item.Slot == slot {
			return item
		}
	}
	return nil
}

func (s *Server) template(item *store.Item) *data.ItemTemplate { return s.data.Items[item.ItemID] }

// slotsFor is ItemSlot.getSlotsFor: the single slots a slot mask lets an item go in, in order.
func slotsFor(mask int32) []int32 {
	var slots []int32
	for bit := range 31 {
		if mask&(1<<bit) != 0 {
			slots = append(slots, 1<<bit)
		}
	}
	return slots
}

// equipItem is Equipment.equipItem.
func (s *Server) equipItem(p *player, id, slotRead int32) *store.Item {
	item := p.cubeItem(id)
	if item == nil {
		return nil
	}
	t := s.template(item)
	if t == nil {
		return nil
	}
	if t.Level > int32(p.level) {
		p.conn.send(systemMessage(msgItemTooLowLevel, strconv.Itoa(int(t.Level)), descriptionID(t.NameID)))
		return nil
	}
	if p.conn.account.accessLevel == 0 {
		race, _ := raceGender(p.Character)
		if (t.Race == "ASMODIANS" && race != 1) || (t.Race == "ELYOS" && race != 0) {
			return nil
		}
		if !t.AllowedFor(int(classIDs[p.Class]), p.level) {
			return nil
		}
	}
	marked := map[int32]bool{}
	switch t.EquipmentType {
	case "ARMOR":
		if !s.validateArmor(p, item, t, true, marked) {
			return nil
		}
	case "WEAPON":
		if !s.validateWeapon(p, item, t, true, marked) {
			return nil
		}
	}
	mask := t.Slot
	if t.EquipmentType == "STIGMA" || t.IsStigma() {
		mask = slotRead
	}
	possible := slotsFor(mask)
	if len(possible) == 0 {
		return nil
	}
	slot := possible[0]
	for _, candidate := range possible {
		if p.equippedIn(candidate) == nil || marked[candidate] {
			slot = candidate
			break
		}
	}
	if t.SoulBound() && !item.SoulBound {
		s.soulBind(p, item, t, slot)
		return nil
	}
	if !s.stigmaEquip(p, item) {
		return nil
	}
	return s.equipTo(p, slot, item, t)
}

// equipTo is Equipment.equip.
func (s *Server) equipTo(p *player, slot int32, item *store.Item, t *data.ItemTemplate) *store.Item {
	p.cube = slices.DeleteFunc(p.cube, func(other *store.Item) bool { return other == item })
	if s.unequipSlot(p, slot) {
		// what was there is in the cube now
	}
	switch t.EquipmentType {
	case "ARMOR":
		s.validateArmor(p, item, t, false, nil)
	case "WEAPON":
		s.validateWeapon(p, item, t, false, nil)
	}
	if p.equippedIn(slot) != nil {
		s.log.Error("putting an item in an equipped slot", "slot", slot, "item", item.ItemID)
		p.cube = append(p.cube, item)
		return nil
	}
	item.Equipped = true
	item.Slot = slot
	p.equipment = append(p.equipment, item)
	slices.SortStableFunc(p.equipment, func(a, b *store.Item) int { return cmp.Compare(a.Slot, b.Slot) })
	s.saveItem(item)
	p.conn.send(s.updateItemPacket(p, item))
	s.equipmentChanged(p)
	return item
}

// unequipSlot is Equipment.unEquip: what is worn in the slot goes to the cube.
func (s *Server) unequipSlot(p *player, slot int32) bool {
	item := p.equippedIn(slot)
	if item == nil {
		return false
	}
	p.equipment = slices.DeleteFunc(p.equipment, func(other *store.Item) bool { return other == item })
	item.Equipped = false
	p.cube = append(p.cube, item)
	s.saveItem(item)
	s.equipmentChanged(p)
	p.conn.send(s.updateItemPacket(p, item))
	return true
}

// equipmentChanged is what follows a change of what the player wears: its stats are worked out again.
func (s *Server) equipmentChanged(p *player) {
	before := p.stats
	p.stats = s.playerStats(p)
	// The life the player has can't be over what it can now have.
	maxHP, maxMP := p.stats.current(data.MaxHP), p.stats.current(data.MaxMP)
	p.life.HP, p.life.MP = min(p.life.HP, maxHP), min(p.life.MP, maxMP)
	p.dirtyHP, p.dirtyMP = true, true
	if p.life.HP != maxHP || p.life.MP != maxMP {
		s.triggerRestore(p)
	}
	if before.current(data.Speed) != p.stats.current(data.Speed) || before.current(data.AttackSpeed) != p.stats.current(data.AttackSpeed) {
		p.broadcast(s.playerEmotion(p, emoteStartEmote2, 0, 0, 0, 0, 0), true)
	}
	p.conn.send(s.statsInfo(p))
}

// unequipItem is Equipment.unEquipItem.
func (s *Server) unequipItem(p *player, id int32) *store.Item {
	if p.cubeFull() {
		return nil
	}
	var item *store.Item
	for _, worn := range p.equipment {
		if worn.UniqueID == id {
			item = worn
		}
	}
	if item == nil {
		return nil
	}
	t := s.template(item)
	if t == nil || !s.stigmaUnequip(p, item) {
		return nil
	}
	free := p.cubeLimit() - len(p.cube)
	if t.WeaponType == "BOW" {
		if arrows := p.equippedIn(data.SlotSubHand); arrows != nil && s.template(arrows).ArmorType == "ARROW" {
			if free < 1 {
				return nil
			}
			s.unequipSlot(p, data.SlotSubHand)
			free--
		}
	}
	if item.Slot == data.SlotMainHand {
		if off := p.equippedIn(data.SlotSubHand); off != nil && s.template(off).IsWeapon() {
			if free < 2 {
				return nil
			}
			s.unequipSlot(p, data.SlotSubHand)
		}
	}
	if t.IsArmor() && t.ArmorType == "SHARD" {
		p.state &^= statePowershard
		p.conn.send(s.playerEmotion(p, emotePowershardOff, 0, 0, 0, 0, 0))
	}
	s.unequipSlot(p, item.Slot)
	return item
}

// validateWeapon is Equipment.validateEquippedWeapon: the player needs the skill for it and room for what has to come off.
func (s *Server) validateWeapon(p *player, item *store.Item, t *data.ItemTemplate, validateOnly bool, marked map[int32]bool) bool {
	rules := weaponRules[t.WeaponType]
	if !p.hasAnySkill(rules.skills) {
		return false
	}
	main, sub := p.equippedIn(data.SlotMainHand), p.equippedIn(data.SlotSubHand)
	needed := 0
	take := func(slot int32) {
		if validateOnly {
			needed++
			marked[slot] = true
		} else {
			s.unequipSlot(p, slot)
		}
	}
	if rules.hands == 2 {
		if t.WeaponType == "BOW" {
			if sub != nil && s.template(sub).ArmorType != "ARROW" {
				take(data.SlotSubHand)
			}
		} else if sub != nil {
			take(data.SlotSubHand)
		}
	}
	// Two handed weapons go on to the main hand too, and so do the ones a dual wielder puts there.
	if rules.hands >= 1 {
		switch {
		case main != nil && !p.hasSkill(19) && !p.hasSkill(360):
			take(data.SlotMainHand)
		case main != nil && weaponRules[s.template(main).WeaponType].hands == 2:
			take(data.SlotMainHand)
		}
		if arrows := p.equippedIn(data.SlotSubHand); arrows != nil && s.template(arrows).ArmorType == "ARROW" {
			take(data.SlotSubHand)
		}
	}
	return p.cubeLimit()-len(p.cube) >= needed-1
}

// validateArmor is Equipment.validateEquippedArmor.
func (s *Server) validateArmor(p *player, item *store.Item, t *data.ItemTemplate, validateOnly bool, marked map[int32]bool) bool {
	if t.ArmorType == "" {
		return true
	}
	if !p.hasAnySkill(armorSkills[t.ArmorType]) {
		return false
	}
	main := p.equippedIn(data.SlotMainHand)
	switch t.ArmorType {
	case "ARROW":
		if (main == nil || s.template(main).WeaponType != "BOW") && validateOnly {
			return false
		}
	case "SHIELD":
		if main != nil && weaponRules[s.template(main).WeaponType].hands == 2 {
			if validateOnly {
				if p.cubeFull() {
					return false
				}
				marked[data.SlotMainHand] = true
			} else {
				s.unequipSlot(p, data.SlotMainHand)
			}
		}
	}
	return true
}

// soulBind is Equipment.soulBindItem: the player is asked whether it wants the item bound to it, and if it does,
// after a few seconds standing still the item is its own and worn.
func (s *Server) soulBind(p *player, item *store.Item, t *data.ItemTemplate, slot int32) {
	handler := func(accepted bool) {
		if !accepted {
			p.conn.send(systemMessage(msgSoulBoundCancelled, descriptionID(t.NameID)))
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 5000, 4, 0), true)
		x, y, z, world := p.X, p.Y, p.Z, p.WorldID
		s.later(5100*time.Millisecond, func() {
			p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 6, 0), true)
			if p.X != x || p.Y != y || p.Z != z || p.WorldID != world || p.cubeItem(item.UniqueID) == nil {
				return
			}
			p.conn.send(systemMessage(msgSoulBoundDone, descriptionID(t.NameID)))
			item.SoulBound = true
			if s.equipTo(p, slot, item, t) != nil {
				p.broadcast(s.appearancePacket(p), true)
			}
		})
	}
	if p.putRequest(questionSoulBound, handler) {
		p.conn.send(questionWindow(questionSoulBound, 0, descriptionID(t.NameID)))
	}
}

// Messages of soul binding.
const (
	msgSoulBoundDone      = 1300485
	msgSoulBoundCancelled = 1300487
)
