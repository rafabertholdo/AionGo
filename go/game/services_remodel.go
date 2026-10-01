package game

import (
	"aionlightning/wire"
)

// Item remodelling and arms fusion: AL-Game's ItemRemodelService and ArmsfusionService, driven by
// CM_ITEM_REMODEL, CM_FUSION_WEAPONS and CM_BREAK_WEAPONS.
// ponytail: a fused weapon keeps the second weapon's id (inventory.fusionedItem); as in AL-Game its manastones are not
// carried over, and its stats are not added here (AL-Game reads it only in StatFunctions' attack calculation).

func init() {
	handlers[cmItemRemodel] = (*conn).itemRemodel
	handlers[cmFusionWeapons] = (*conn).fuseWeapons
	handlers[cmBreakWeapons] = (*conn).breakWeapons
}

const (
	patternReshaper = 168100000
	remodelMinLevel = 20
	remodelPrice    = 1000 // Prices.getPriceForService(1000), all the modifiers being 100%
	fusionPrice     = 50000

	msgSkinLevelLimit = 1300476
	msgSkinNotAllowed = 1300478
	msgSkinNotFit     = 1300480
	msgSkinNoGold     = 1300481
	msgSkinNoExtract  = 1300482
	msgSkinSucceed    = 1300483

	msgFuseHigherLevel = 1400288
	msgUnfused         = 1400335
	msgFused           = 1400336
	msgFuseNoGold      = 1400337
	msgFuseDifferent   = 1400364
	msgUnfuseNone      = 1400373
)

// itemRemodel is CM_ITEM_REMODEL: the keep item takes the look of the extract item, or loses it to a pattern reshaper.
func (c *conn) itemRemodel(r *wire.Reader) {
	r.D() // the npc
	keep, extract := r.D(), r.D()
	if c.player == nil || r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if !c.player.dead {
		c.s.remodelItem(c.player, keep, extract)
	}
}

func (s *Server) remodelItem(p *player, keepID, extractID int32) {
	keep, extract := p.cubeItem(keepID), p.cubeItem(extractID)
	if keep == nil || extract == nil || keep == extract {
		return
	}
	kt, et := s.template(keep), s.template(extract)
	kst, est := s.data.Items[keep.SkinID()], s.data.Items[extract.SkinID()]
	if kt == nil || et == nil || kst == nil || est == nil {
		return
	}
	if p.level < remodelMinLevel {
		p.conn.send(systemMessage(msgSkinLevelLimit))
		return
	}
	if et.ID == patternReshaper {
		if keep.SkinID() == keep.ItemID {
			s.tell(p, "That item does not have a remodeled skin to remove.")
			return
		}
		if !s.decreaseKinah(p, remodelPrice) {
			p.conn.send(systemMessage(msgSkinNoGold, descriptionID(kt.NameID)))
			return
		}
		s.decreaseItemCount(p, extract, 1)
		keep.Skin = 0
		if !kt.Dye {
			keep.Color = 0
		}
		s.itemChanged(p, keep)
		p.conn.send(systemMessage(msgSkinSucceed, descriptionID(kt.NameID)))
		return
	}
	if kt.WeaponType != est.WeaponType || (est.ArmorType != "CLOTHES" && kt.ArmorType != est.ArmorType) ||
		kt.ArmorType == "CLOTHES" || kt.Slot != est.Slot {
		p.conn.send(systemMessage(msgSkinNotFit, descriptionID(kt.NameID), descriptionID(est.NameID)))
		return
	}
	if kt.Quality == "EPIC" || kt.Quality == "MYTHIC" {
		p.conn.send(systemMessage(msgSkinNotAllowed, descriptionID(kt.NameID)))
		return
	}
	if et.Quality == "EPIC" || et.Quality == "MYTHIC" {
		p.conn.send(systemMessage(msgSkinNoExtract, descriptionID(et.NameID)))
		return
	}
	if !s.decreaseKinah(p, remodelPrice) {
		p.conn.send(systemMessage(msgSkinNoGold, descriptionID(kt.NameID)))
		return
	}
	skin, color := extract.SkinID(), extract.Color
	s.decreaseItemCount(p, extract, 1)
	keep.Skin, keep.Color = skin, color
	if skin == keep.ItemID {
		keep.Skin = 0
	}
	s.itemChanged(p, keep)
	p.conn.send(systemMessage(msgSkinSucceed, descriptionID(kt.NameID)))
}

// targetsNpc is Player.getTarget() instanceof Npc.
func (s *Server) targetsNpc(p *player) bool {
	o, ok := s.creatureByID(p.targetID).(*object)
	return ok && o.npc != nil
}

// fuseWeapons is CM_FUSION_WEAPONS: at an npc, the first weapon takes the second's stats, which is used up.
func (c *conn) fuseWeapons(r *wire.Reader) {
	r.D()
	first, second := r.D(), r.D()
	if c.player == nil || r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if !c.player.dead {
		c.s.fuseWeapons(c.player, first, second)
	}
}

func (s *Server) fuseWeapons(p *player, firstID, secondID int32) {
	first, second := s.itemOf(p, firstID), s.itemOf(p, secondID)
	// ponytail: AL-Game would also take an equipped second weapon; here it must be in the cube to be used up.
	if first == nil || second == nil || first == second || second.Equipped || !s.targetsNpc(p) {
		return
	}
	ft, st := s.template(first), s.template(second)
	if ft == nil || st == nil {
		return
	}
	switch {
	case p.kinah.Count < fusionPrice:
		p.conn.send(systemMessage(msgFuseNoGold, descriptionID(ft.NameID), descriptionID(st.NameID)))
	case ft.WeaponType != st.WeaponType:
		p.conn.send(systemMessage(msgFuseDifferent))
	case st.Level > ft.Level:
		p.conn.send(systemMessage(msgFuseHigherLevel))
	default:
		// AL-Game checks the kinah but never charges it.
		first.Fusioned = second.ItemID
		s.decreaseItemCount(p, second, 1)
		s.itemChanged(p, first)
		p.conn.send(systemMessage(msgFused, descriptionID(ft.NameID), descriptionID(st.NameID)))
	}
}

// breakWeapons is CM_BREAK_WEAPONS: a fused weapon is separated from what it took (AL-Game only lets that be done
// with no npc targeted).
func (c *conn) breakWeapons(r *wire.Reader) {
	r.D()
	id := r.D()
	if c.player == nil || r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if !c.player.dead {
		c.s.breakWeapons(c.player, id)
	}
}

func (s *Server) breakWeapons(p *player, id int32) {
	item := s.itemOf(p, id)
	if item == nil || s.targetsNpc(p) {
		return
	}
	t := s.template(item)
	if t == nil {
		return
	}
	if item.Fusioned == 0 {
		p.conn.send(systemMessage(msgUnfuseNone, descriptionID(t.NameID)))
		return
	}
	item.Fusioned = 0
	s.itemChanged(p, item)
	p.conn.send(systemMessage(msgUnfused, descriptionID(t.NameID)))
}
