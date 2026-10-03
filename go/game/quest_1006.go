package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

const (
	ascensionQuestID       int32 = 1006
	ascensionJournalItem   int32 = 182200007
	ascensionProofItem     int32 = 182200008
	ascensionTestimonyItem int32 = 182200009
	ascensionStartNPC      int32 = 790001
	ascensionElimNPC       int32 = 730008
	ascensionTransportNPC  int32 = 205000
	ascensionMinionNPC     int32 = 211042
	ascensionBossNPC       int32 = 211043
	ascensionFailMessage   int32 = 0x13D886
	ascensionInstanceMap   int32 = 310010000
)

// ascensionDialog ports the level-nine class trial's four NPCs and reward
// choice. The plain-click behavior is handled separately by the dialog router.
func (c *conn) ascensionDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || script == nil || script.ID != ascensionQuestID {
		return false
	}
	if o == nil || o.npc == nil || o.dead {
		return false
	}
	p := c.player
	quest := p.quest(ascensionQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != ascensionStartNPC {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, ascensionQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	variable := quest.Vars
	switch o.npc.ID {
	case ascensionStartNPC:
		// Java's cases fall through from 25 down to 10011; a select that no var matches ends in the next case.
		setClass := func(class string) bool {
			p.Class = class
			c.s.levelUp(p)
			if c.customQuestProgress(ascensionQuestID, quest.Vars, "REWARD") {
				c.send(dialogWindow(o.id, 5, ascensionQuestID))
			}
			return true
		}
		switch dialogID {
		case 25:
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, ascensionQuestID))
				return true
			case 3:
				c.send(dialogWindow(o.id, 1693, ascensionQuestID))
				return true
			case 5:
				c.send(dialogWindow(o.id, 2034, ascensionQuestID))
				return true
			}
			fallthrough
		case 10000:
			if variable == 0 {
				if !c.addQuestItems([]data.QuestItem{{ID: ascensionJournalItem, Count: 1}}) {
					return true
				}
				if c.customQuestProgress(ascensionQuestID, 1, "") {
					c.send(dialogWindow(o.id, 10, 0))
				}
				return true
			}
			fallthrough
		case 10002:
			if variable == 3 {
				c.jRemoveAll(ascensionTestimonyItem)
				// Ascension uses the whole quest value for its special step 99.
				// Packing it into a six-bit subvariable would truncate it to 35.
				if c.customQuestProgress(ascensionQuestID, 99, "") {
					c.send(dialogWindow(o.id, 0, 0))
					instance := c.s.newInstance(ascensionInstanceMap)
					instance.registered[p.ID] = true
					c.s.teleportToInstance(p, ascensionInstanceMap, instance.id, 52, 174, 229, 0, 0)
				}
				return true
			}
			fallthrough
		case 10003:
			if variable == 5 {
				if page := map[string]uint16{"WARRIOR": 2375, "SCOUT": 2716, "MAGE": 3057, "PRIEST": 3398}[p.Class]; page != 0 {
					c.send(dialogWindow(o.id, page, ascensionQuestID))
					return true
				}
			}
			fallthrough
		case 10004:
			if variable == 5 {
				return setClass("GLADIATOR")
			}
			fallthrough
		case 10005:
			if variable == 5 {
				return setClass("TEMPLAR")
			}
			fallthrough
		case 10006:
			if variable == 5 {
				return setClass("ASSASSIN")
			}
			fallthrough
		case 10007:
			if variable == 5 {
				return setClass("RANGER")
			}
			fallthrough
		case 10008:
			if variable == 5 {
				return setClass("SORCERER")
			}
			fallthrough
		case 10009:
			if variable == 5 {
				return setClass("SPIRIT_MASTER")
			}
			fallthrough
		case 10010:
			if variable == 5 {
				return setClass("CLERIC")
			}
			fallthrough
		case 10011:
			if variable == 5 {
				return setClass("CHANTER")
			}
		}
	case ascensionElimNPC:
		if dialogID == 25 && variable == 2 {
			page := uint16(1354)
			if c.s.countItems(p, ascensionProofItem) > 0 {
				page = 1352
			}
			c.send(dialogWindow(o.id, page, ascensionQuestID))
			return true
		}
		if dialogID == 1353 || dialogID == 25 {
			if variable == 2 {
				c.send(ascensionMovie(14))
				c.s.removeItemsByID(p, ascensionProofItem, 1)
				c.addQuestItems([]data.QuestItem{{ID: ascensionTestimonyItem, Count: 1}})
			}
			return false // Java returns false after the movie; the framework echoes the page.
		}
		if dialogID == 10001 && variable == 2 && c.customQuestProgress(ascensionQuestID, 3, "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case ascensionTransportNPC:
		if dialogID == 25 && variable == 99 {
			p.broadcast(c.s.playerEmotionTo(p, emoteStartFlyTele, 1001, 0, 0, 0, 0, 0), true)
			if !c.customQuestProgress(ascensionQuestID, 50, "") {
				return true
			}
			world, instanceID := p.WorldID, p.instance
			c.s.later(43*time.Second, func() {
				current := p.quest(ascensionQuestID)
				if p.conn != c || !p.spawned || p.WorldID != world || p.instance != instanceID || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 50 {
					return
				}
				if c.customQuestProgress(ascensionQuestID, 51, "") {
					for _, position := range [][3]float32{{224.073, 239.1, 206.7}, {233.5, 241.04, 206.365}, {229.6, 265.7, 205.7}, {222.8, 262.5, 205.7}} {
						c.spawnAscensionNPC(ascensionMinionNPC, world, instanceID, position[0], position[1], position[2], 0, true)
					}
				}
			})
			return true
		}
	}
	return false
}

func (c *conn) ascensionItemUse(item *store.Item) bool {
	if c == nil || c.player == nil || item == nil || item.ItemID != ascensionJournalItem || c.player.cubeItem(item.UniqueID) != item {
		return false
	}
	p := c.player
	quest := p.quest(ascensionQuestID)
	if !c.s.insideZone(p, "ITEMUSE_Q1006") || quest == nil || quest.Status != "START" {
		return false
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0), true)
	c.s.later(3*time.Second, func() {
		current := p.quest(ascensionQuestID)
		if p.conn != c || p.cubeItem(item.UniqueID) != item || !c.s.insideZone(p, "ITEMUSE_Q1006") || current == nil || current.Status != "START" {
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.s.removeItemsByID(p, ascensionJournalItem, 1)
		c.s.addItem(p, ascensionProofItem, 1)
		c.customQuestProgress(ascensionQuestID, 2, "")
	})
	return true
}

func (c *conn) ascensionKill(dead *object) {
	if c == nil || c.player == nil || dead == nil || dead.npc == nil || dead.npc.ID != ascensionMinionNPC {
		return
	}
	quest := c.player.quest(ascensionQuestID)
	if quest == nil || quest.Status != "START" {
		return
	}
	variable := quest.Vars
	if variable >= 51 && variable <= 53 {
		c.customQuestProgress(ascensionQuestID, quest.Vars+1, "")
		return
	}
	if variable != 54 || !c.customQuestProgress(ascensionQuestID, 4, "") {
		return
	}
	spawned := c.spawnAscensionNPC(ascensionBossNPC, ascensionInstanceMap, c.player.instance, 226.7, 251.5, 205.5, 0, false)
	if spawned != nil {
		spawned.stats.set(data.MainHandPower, spawned.stats.current(data.MainHandPower)/3, false)
		c.s.addDamage(spawned, c.player, 1000)
	}
}

func (c *conn) ascensionBossDamage(o *object, damage int32) int32 {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != ascensionBossNPC || o.maxHP <= 1 {
		return damage
	}
	quest := c.player.quest(ascensionQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 || o.hp-damage >= o.maxHP/2 {
		return damage
	}
	return max(int32(1), o.hp-o.maxHP/2+1)
}

func (c *conn) ascensionAttack(o *object) {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != ascensionBossNPC || o.dead || o.hp >= o.maxHP/2 {
		return
	}
	quest := c.player.quest(ascensionQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 {
		return
	}
	c.send(ascensionMovie(151))
	c.s.despawnNpc(o, true)
}

func (c *conn) ascensionMovieEnd(movieID uint16) bool {
	if c == nil || c.player == nil || movieID != 151 || c.player.WorldID != ascensionInstanceMap {
		return false
	}
	quest := c.player.quest(ascensionQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 {
		return false
	}
	if !c.customQuestProgress(ascensionQuestID, 5, "") {
		return false
	}
	c.spawnAscensionNPC(ascensionStartNPC, ascensionInstanceMap, c.player.instance, 220.6, 247.8, 206, 0, true)
	return true
}

func (c *conn) ascensionDeath() {
	if c == nil || c.player == nil {
		return
	}
	quest := c.player.quest(ascensionQuestID)
	if quest == nil || quest.Status != "START" {
		return
	}
	variable := quest.Vars
	if variable != 4 && (variable < 50 || variable > 55) {
		return
	}
	if c.customQuestProgress(ascensionQuestID, 3, "") {
		nameID := int32(0)
		if template := c.s.data.Quests[ascensionQuestID]; template != nil {
			nameID = template.NameID
		}
		c.send(systemMessage(ascensionFailMessage, descriptionID(nameID)))
	}
}

func (c *conn) ascensionEnterWorld() {
	if c == nil || c.player == nil {
		return
	}
	quest := c.player.quest(ascensionQuestID)
	if quest == nil || quest.Status != "START" {
		return
	}
	variable := quest.Vars
	if variable != 4 && (variable < 50 || variable > 55) && variable != 99 {
		return
	}
	if c.player.WorldID != ascensionInstanceMap {
		if c.customQuestProgress(ascensionQuestID, 3, "") {
			c.send(systemMessage(ascensionFailMessage, descriptionID(c.s.data.Quests[ascensionQuestID].NameID)))
		}
		return
	}
	w := wire.Packet(smAscensionMorph)
	w.D(1)
	c.send(w)
}

func (c *conn) ascensionQuestFinish() {
	if c == nil || c.player == nil {
		return
	}
	c.s.teleportTo(c.player, 210010000, 242, 1638, 100, 20, 0)
}

func (c *conn) spawnAscensionNPC(npcID, worldID, instanceID int32, x, y, z float32, heading byte, weaken bool) *object {
	template := c.s.data.Npcs[npcID]
	if template == nil {
		return nil
	}
	if weaken && npcID == ascensionMinionNPC && worldID == ascensionInstanceMap {
		// Zero attack power reaches the combat formula's minimum of one damage.
		// Use a private template so recalculating stats preserves the trial damage.
		trialTemplate := *template
		trialTemplate.Stats.Power = 0
		template = &trialTemplate
	}
	o := &object{id: c.s.ids.nextID(), worldID: worldID, instance: instanceID,
		x: x, y: y, z: z, heading: heading, homeX: x, homeY: y, homeZ: z, npc: template}
	c.s.initNpc(o)
	if weaken {
		o.stats.set(data.MainHandPower, o.stats.current(data.MainHandPower)/3, false)
		o.stats.set(data.PhysicalDefense, 0, false)
	}
	c.s.byID[o.id] = o
	c.s.addObject(o)
	if c.player != nil && c.player.WorldID == worldID && c.player.instance == instanceID {
		c.s.addDamage(o, c.player, 1000)
	}
	return o
}

func ascensionMovie(id uint16) *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(id)
	w.D(0)
	return w
}
