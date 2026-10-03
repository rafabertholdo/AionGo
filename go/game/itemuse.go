package game

import (
	"strconv"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() {
	handlers[cmUseItem] = (*conn).useItem
}

const msgItemUseDelay = 1300494 // "You can't use that until the delay time is over."

// useItem is CM_USE_ITEM: the player uses an item of its cube.
func (c *conn) useItem(r *wire.Reader) {
	p := c.player
	id := r.D()
	useType := r.C()
	var targetID int32
	if useType == 2 {
		targetID = r.D()
	}
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.dead || p.cast != nil || s.restrictedInPrison(p, "use item") {
		return
	}
	for _, item := range p.cube {
		if item.UniqueID == id {
			if c.javaItemUse(item) {
				return
			}
			if item.ItemID == flyingReconnaissancePotionID {
				c.flyingReconnaissancePotionUse(item)
				return
			}
			for _, script := range s.data.QuestItemUses[item.ItemID] {
				if script.ID == fearThisQuestID {
					c.fearThisItemUse(item)
					return
				}
				if script.ID == 1006 {
					c.ascensionItemUse(item)
					return
				}
				if script.ID == 1032 {
					c.rulersDutyItemUse(item)
					return
				}
				if script.ID == 1039 {
					c.somethingInTheWaterItemUse(item)
					return
				}
				if script.ID == 1042 {
					c.keeperKaidanKeyItemUse(item)
					return
				}
				if script.ID == archonOfStormsQuestID {
					c.archonOfStormsItemUse(item)
					return
				}
				if script.ID == speakingBalaurQuestID {
					c.speakingBalaurItemUse(item)
					return
				}
				if script.ID == fragmentOfMemory2QuestID {
					c.fragmentOfMemory2ItemUse(item)
					return
				}
				if script.ItemUseDelay > 0 {
					c.delayedItemQuestUse(item, script)
					return
				}
				if script.ID == 4200 || script.ID == 3200 {
					c.suspiciousCallItemUse(item, script)
					return
				}
				if script.ID == 2122 {
					c.ashesToAshesEvent(nil, item, script, -1)
					return
				}
				if script.ID == 1114 {
					c.nymphsGownItemUse(item, script)
					return
				}
				if script.ID == dukakiOdiumQuestID {
					c.dukakiOdiumItemUse(item, script)
					return
				}
				if script.ID == flyingReconnaissanceQuestID {
					c.flyingReconnaissanceItemUse(item)
					return
				}
				if script.ID == 1107 || script.ID == 2107 || script.ID == lostAxeQuestID || script.ID == 3914 {
					c.questStartItemUse(item, script)
					return
				}
			}
			s.useItem(p, item, s.itemOf(p, targetID))
			return
		}
	}
}

// useItem is CM_USE_ITEM's handler: it does what the item's actions say, if the player may use it now.
func (s *Server) useItem(p *player, item *store.Item, target *store.Item) {
	t := s.template(item)
	if t == nil || len(t.Actions) == 0 {
		return
	}
	race, _ := raceGender(p.Character)
	if (t.Race == "ASMODIANS" && race != 1) || (t.Race == "ELYOS" && race != 0) || !t.AllowedFor(int(classIDs[p.Class]), p.level) {
		return
	}
	var actions []*data.Node
	for _, action := range t.Actions {
		if s.canAct(p, action, target) {
			actions = append(actions, action)
		}
	}
	if len(actions) == 0 {
		return
	}
	if t.UseDelay > 0 && time.Now().Before(p.itemCooldowns[t.UseDelayID]) {
		p.conn.send(systemMessage(msgItemUseDelay))
		return
	}
	p.setItemCooldown(t)
	for _, action := range actions {
		s.act(p, item, t, action, target)
	}
}

// setItemCooldown is Player.addItemCoolDown: the item's use delay group is busy, and the client is told.
func (p *player) setItemCooldown(t *data.ItemTemplate) {
	if t.UseDelay <= 0 {
		return
	}
	if p.itemCooldowns == nil {
		p.itemCooldowns = map[int32]time.Time{}
	}
	p.itemCooldowns[t.UseDelayID] = time.Now().Add(time.Duration(t.UseDelay) * time.Millisecond)
	p.conn.send(itemCooldown(t.UseDelayID, t.UseDelay))
}

// itemCooldown is SM_ITEM_COOLDOWN for one use delay group that has just begun.
func itemCooldown(delayID, delay int32) *wire.Writer {
	w := wire.Packet(smItemCooldown)
	w.H(1)
	w.H(uint16(delayID))
	w.D(delay / 1000)
	w.D(delay)
	return w
}

// startingClasses maps each class to the one a player starts as.
var startingClasses = map[string]string{"GLADIATOR": "WARRIOR", "TEMPLAR": "WARRIOR", "ASSASSIN": "SCOUT", "RANGER": "SCOUT",
	"SORCERER": "MAGE", "SPIRIT_MASTER": "MAGE", "CLERIC": "PRIEST", "CHANTER": "PRIEST"}

// canAct is AbstractItemAction.canAct.
func (s *Server) canAct(p *player, a *data.Node, target *store.Item) bool {
	switch a.Name {
	case "skilluse":
		tmpl := s.data.Skills[a.Int("skillid")]
		if tmpl == nil {
			return false
		}
		sk := &skill{s: s, tmpl: tmpl, effector: p, level: a.Int("level"), first: s.creatureByID(p.targetID)}
		return s.canUseSkill(p, sk) && sk.canUse()
	case "queststart":
		return true // the quest handlers offer it
	case "toypetspawn":
		return s.canSpawnKisk(p)
	case "enchant":
		if target == nil {
			p.conn.send(systemMessage(msgItemError))
			return false
		}
		return true
	case "extract":
		if target == nil {
			p.conn.send(systemMessage(msgItemError))
			return false
		}
		return true
	case "craftlearn":
		return s.canLearnRecipe(p, a.Int("recipeid"))
	case "dye":
		if target == nil {
			p.conn.send(systemMessage(msgItemError))
			return false
		}
		return true
	case "skilllearn":
		if p.level < int(a.Int("level")) || p.hasSkill(a.Int("skillid")) {
			return false
		}
		if race := a.Str("race"); race != "ALL" && race != p.Race {
			return false
		}
		class := a.Str("class")
		return class == "ALL" || class == p.Class || startingClasses[p.Class] == class
	}
	return false
}

// act is AbstractItemAction.act, for the actions ported so far.
func (s *Server) act(p *player, item *store.Item, t *data.ItemTemplate, a *data.Node, target *store.Item) {
	switch a.Name {
	case "skilluse":
		sk := &skill{s: s, tmpl: s.data.Skills[a.Int("skillid")], effector: p, level: a.Int("level"), first: s.creatureByID(p.targetID)}
		sk.item = t
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 1), true)
		sk.use()
		s.decreaseItemCount(p, item, 1)
	case "toypetspawn":
		if s.spawnKisk(p, a.Int("npcid")) != nil {
			p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 1), true)
			s.decreaseItemCount(p, item, 1)
		}
	case "craftlearn":
		p.conn.send(systemMessage(msgUseItem, descriptionID(t.NameID)))
		p.conn.send(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 1))
		if s.decreaseItemCount(p, item, 1) == 0 {
			s.learnRecipe(p, s.data.Recipes[a.Int("recipeid")])
		}
	case "extract":
		s.extractItem(p, item, target)
	case "dye":
		s.dye(p, item, target, a.Str("color"))
	case "enchant":
		s.startEnchanting(p, item, target, nil)
	case "skilllearn":
		s.removeItem(p, item)
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 1), true)
		s.addSkill(p, a.Int("skillid"), 1, true)
	}
}

// itemOf is the item of the player's cube or equipment with the unique id, or nil.
func (s *Server) itemOf(p *player, id int32) *store.Item {
	if id == 0 {
		return nil
	}
	if item := p.cubeItem(id); item != nil {
		return item
	}
	for _, item := range p.equipment {
		if item.UniqueID == id {
			return item
		}
	}
	return nil
}

// dye is DyeAction.act: the item takes the colour, and the scroll is used up.
func (s *Server) dye(p *player, scroll, target *store.Item, color string) {
	t := s.template(target)
	if t == nil || !t.Dye {
		return
	}
	if color == "no" {
		target.Color = 0
	} else {
		rgb, err := strconv.ParseInt(color, 16, 32)
		if err != nil {
			return
		}
		// The client's colours are BGRA.
		target.Color = int32(0xFF | (rgb&0xFF)<<24 | (rgb&0xFF00)<<8 | (rgb&0xFF0000)>>8)
	}
	s.saveItem(target)
	if target.Equipped {
		p.broadcast(s.appearancePacket(p), true)
	}
	p.conn.send(s.updateItemPacket(p, target))
	s.decreaseItemCount(p, scroll, 1)
}
