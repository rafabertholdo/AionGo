package game

import (
	"math"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// The quest_script_data templates, ported from questEngine/handlers/template/*.java branch by branch. Each returns
// Java's onDialogEvent result; questDialog turns it into the dialog framework's answered / echo.

// questOffered is the templates' "qs == null || NONE || COMPLETE and repeatable" test.
func (c *conn) questOffered(q *store.Quest, id int32) bool {
	if q == nil || q.Status == "NONE" {
		return true
	}
	template := c.s.data.Quests[id]
	return q.Status == "COMPLETE" && template != nil && q.CompleteCount <= int32(template.MaxRepeatCount)
}

// defaultQuestStartDialog is QuestHandler.defaultQuestStartDialog.
func (c *conn) defaultQuestStartDialog(o *object, script *data.QuestScript, d int32) bool {
	switch d {
	case 1007:
		c.send(dialogWindow(o.id, 4, script.ID))
		return true
	case 1002:
		return c.startQuest(script, o.id)
	case 1003:
		c.send(dialogWindow(o.id, 1004, script.ID))
		return true
	}
	return false
}

// defaultQuestEndDialog is QuestHandler.defaultQuestEndDialog.
func (c *conn) defaultQuestEndDialog(o *object, script *data.QuestScript, d int32) bool {
	switch {
	case d >= 8 && d <= 17:
		q := c.player.quest(script.ID)
		if q == nil || q.Status != "REWARD" {
			return false
		}
		c.finishQuest(script, o.id, uint16(d))
		return true
	case d == 1009 || d == -1:
		if q := c.player.quest(script.ID); q != nil && q.Status == "REWARD" {
			c.send(dialogWindow(o.id, 5, script.ID))
			return true
		}
	}
	return false
}

// reportToDialog is template/ReportTo.onDialogEvent.
func (c *conn) reportToDialog(o *object, script *data.QuestScript, d int32) bool {
	p := c.player
	q := p.quest(script.ID)
	switch {
	case o.npc.ID == script.StartNPC && (c.questOffered(q, script.ID) || script.StartNPC != script.EndNPC):
		// Java tests the start npc first: when it is also the end npc, a started quest never reaches the end branch
		// and could not be handed in. Go lets it through (see parityDeviation).
		if !c.questOffered(q, script.ID) {
			return false
		}
		if d == 25 {
			c.send(dialogWindow(o.id, 1011, script.ID))
			return true
		}
		if d == 1002 && script.ItemID != 0 {
			if c.addQuestItems([]data.QuestItem{{ID: script.ItemID, Count: 1}}) {
				return c.defaultQuestStartDialog(o, script, d)
			}
			return true
		}
		return c.defaultQuestStartDialog(o, script, d)
	case o.npc.ID == script.EndNPC:
		// Java takes any status here; a finished quest must not become REWARD again (it would pay out twice).
		if q == nil || q.Status != "START" && q.Status != "REWARD" {
			return false
		}
		if d == 25 && q.Status == "START" {
			c.send(dialogWindow(o.id, 2375, script.ID))
			return true
		}
		if d == 1009 {
			if script.ItemID != 0 {
				c.s.removeItemsByID(p, script.ItemID, math.MaxInt64) // removeItemFromInventoryByItemId: every one
			}
			c.updateQuest(script.ID, func(q *store.Quest) { q.Vars, q.Status = 1, "REWARD" })
		}
		return c.defaultQuestEndDialog(o, script, d)
	}
	return false
}

// monsterHuntDialog is template/MonsterHunt.onDialogEvent, except that a hunt is only handed in once every count is
// reached (Java lets the turn-in page and 1009 through with any count below the maximum).
func (c *conn) monsterHuntDialog(o *object, script *data.QuestScript, d int32) bool {
	q := c.player.quest(script.ID)
	switch {
	case c.questOffered(q, script.ID):
		if o.npc.ID != script.StartNPC {
			return false
		}
		if d == 25 {
			c.send(dialogWindow(o.id, 1011, script.ID))
			return true
		}
		return c.defaultQuestStartDialog(o, script, d)
	case q.Status == "START":
		for _, m := range script.MonsterInfos {
			if m.MaxKill < questVar(q.Vars, m.VarID) {
				return false
			}
		}
		if o.npc.ID != script.EndNPC {
			return false
		}
		if (d == 25 || d == 1009) && !monsterHuntComplete(q, script) {
			return false // Go rule: no turn-in before the kills are done
		}
		switch d {
		case 25:
			c.send(dialogWindow(o.id, 1352, script.ID))
			return true
		case 1009:
			c.updateQuest(script.ID, func(q *store.Quest) { q.Status, q.Vars = "REWARD", setQuestVar(q.Vars, 0, questVar(q.Vars, 0)+1) })
			c.send(dialogWindow(o.id, 5, script.ID))
			return true
		}
		return c.defaultQuestEndDialog(o, script, d)
	case q.Status == "REWARD" && o.npc.ID == script.EndNPC:
		return c.defaultQuestEndDialog(o, script, d)
	}
	return false
}

// itemCollectingDialog is template/ItemCollecting.onDialogEvent.
func (c *conn) itemCollectingDialog(o *object, script *data.QuestScript, d int32) bool {
	q := c.player.quest(script.ID)
	switch {
	case c.questOffered(q, script.ID):
		if o.npc.ID != script.StartNPC {
			return false
		}
		if d == 25 {
			c.send(dialogWindow(o.id, 1011, script.ID))
			return true
		}
		return c.defaultQuestStartDialog(o, script, d)
	case q.Status == "START":
		if o.npc.ID == script.EndNPC {
			switch d {
			case 25:
				c.send(dialogWindow(o.id, 2375, script.ID))
				return true
			case 33:
				if c.collectQuestItems(script.ID) {
					c.updateQuest(script.ID, func(q *store.Quest) { q.Status, q.Vars = "REWARD", setQuestVar(q.Vars, 0, questVar(q.Vars, 0)+1) })
					c.send(dialogWindow(o.id, 5, script.ID))
				} else {
					c.send(dialogWindow(o.id, 2716, script.ID))
				}
				return true
			}
		} else if o.npc.ID == script.ActionNPC && script.ActionNPC != 0 {
			return true
		}
	case q.Status == "REWARD" && o.npc.ID == script.EndNPC:
		return c.defaultQuestEndDialog(o, script, d)
	}
	return false
}

// xmlQuestFallback is the part of template/XmlQuest.onDialogEvent after its talk events.
func (c *conn) xmlQuestFallback(o *object, script *data.QuestScript, d int32) bool {
	q := c.player.quest(script.ID)
	switch {
	case c.questOffered(q, script.ID):
		if o.npc.ID != script.StartNPC {
			return false
		}
		if d == 25 {
			c.send(dialogWindow(o.id, 1011, script.ID))
			return true
		}
		return c.defaultQuestStartDialog(o, script, d)
	case q.Status == "REWARD" && o.npc.ID == script.EndNPC:
		return c.defaultQuestEndDialog(o, script, d)
	}
	return false
}

// collectQuestItems is QuestService.collectItemCheck(env, true): whether the quest's collect items are all there,
// taking them when they are.
func (c *conn) collectQuestItems(id int32) bool {
	p := c.player
	if p.quest(id) == nil {
		return false
	}
	template := c.s.data.Quests[id]
	if template == nil {
		return true
	}
	if !c.s.hasQuestItems(p, template) {
		return false
	}
	for _, item := range template.CollectItems {
		c.s.removeItemsByID(p, item.ID, item.Count)
	}
	return true
}

// addQuestItems is ItemService.addItems: every item or, when the cube lacks room, none and the full-inventory message.
func (c *conn) addQuestItems(items []data.QuestItem) bool {
	if !c.s.questRewardsFit(c.player, items) {
		c.send(systemMessage(msgInventoryFull))
		return false
	}
	for _, item := range items {
		c.s.addItem(c.player, item.ID, c.s.javaItemCount(item))
	}
	return true
}

// javaItemCount is the count ItemService.addItem really gives: ItemService.newItem caps it at the item's stack size.
func (s *Server) javaItemCount(item data.QuestItem) int64 {
	if t := s.data.Items[item.ID]; t != nil && t.MaxStack > 0 && item.Count > int64(t.MaxStack) {
		return int64(t.MaxStack)
	}
	return item.Count
}

// updateQuest changes the quest's state, saves it and sends it (qs.set... then updateQuestStatus).
func (c *conn) updateQuest(id int32, change func(*store.Quest)) {
	q := c.player.quest(id)
	if q == nil {
		return
	}
	next := *q
	change(&next)
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating quest", "quest", id, "err", err)
		return
	}
	*q = next
	c.send(questAccepted(2, next))
	if next.Status == "COMPLETE" {
		c.send(c.s.nearbyQuests(c.player))
	}
}

// The Java quest API that scripts/quest-java-port.py translates to (quest_java_dialogs.go). Java changes a quest
// state in memory and tells the client only on updateQuestStatus; here each change is also saved at once.

// jPage is QuestHandler.sendQuestDialog.
func (c *conn) jPage(objectID, questID int32, page uint16) bool {
	c.send(dialogWindow(objectID, page, questID))
	return true
}

// jUpdate is QuestHandler.updateQuestStatus.
func (c *conn) jUpdate(q *store.Quest) {
	c.send(questAccepted(2, *q))
	if q.Status == "COMPLETE" {
		c.send(c.s.nearbyQuests(c.player))
	}
}

func (c *conn) jSave(q *store.Quest) {
	if err := c.s.quests.SaveQuest(c.player.ID, *q); err != nil {
		c.s.log.Error("updating quest", "quest", q.ID, "err", err)
	}
}

// jSetVarByID is QuestState.setQuestVarById.
func (c *conn) jSetVarByID(q *store.Quest, id int, value int32) {
	q.Vars = setQuestVar(q.Vars, id, value)
	c.jSave(q)
}

// jSetVar is QuestState.setQuestVar.
func (c *conn) jSetVar(q *store.Quest, value int32) {
	q.Vars = value
	c.jSave(q)
}

// jSetStatus is QuestState.setStatus.
func (c *conn) jSetStatus(q *store.Quest, status string) {
	q.Status = status
	c.jSave(q)
}

// jRemoveAll is ItemService.removeItemFromInventoryByItemId: every one of the item.
// It says whether anything was removed.
func (c *conn) jRemoveAll(itemID int32) bool {
	n := c.s.countItems(c.player, itemID)
	return n > 0 && c.s.removeItemsByID(c.player, itemID, n)
}

// jQuestFinish is QuestService.questFinish(env): reward 0, the selectable item from dialog id - 8 (none for 17).
func (c *conn) jQuestFinish(script *data.QuestScript, d int32) bool {
	return c.questFinish(script, uint16(d), 0)
}

// jStartLocked is QuestService.startQuest(env, LOCKED) for another quest: no start checks, the quest update, the row
// (reset to LOCKED/0 when it exists and may be repeated), then the nearby quests.
func (c *conn) jStartLocked(id int32) {
	p := c.player
	locked := store.Quest{ID: id, Status: "LOCKED"}
	c.send(questAccepted(1, locked))
	if q := p.quest(id); q == nil {
		c.jSave(&locked)
		p.quests = append(p.quests, locked)
	} else if template := c.s.data.Quests[id]; template != nil && int32(template.MaxRepeatCount) >= q.CompleteCount {
		q.Status, q.Vars = "LOCKED", 0
		c.jSave(q)
	}
	c.send(c.s.nearbyQuests(p))
}

// jLater is ThreadPoolManager.schedule from a quest handler: run after ms with the world locked, while the player is
// still in the game on this connection.
func (c *conn) jLater(ms int32, do func()) {
	p := c.player
	c.s.later(time.Duration(ms)*time.Millisecond, func() {
		if p.conn == c && p.spawned {
			c.javaPort(func() bool { do(); return true })
		}
	})
}

// jTarget is Player.getTarget: the object the player has selected, or nil.
func (c *conn) jTarget() *object {
	p := c.player
	if p.targetID == 0 {
		return nil
	}
	return p.seen[p.targetID]
}

// jTargetEmotion is broadcastPacket(player.getTarget(), new SM_EMOTION(target, kind, emote, targetObjectId)).
func (c *conn) jTargetEmotion(kind byte, emote, target int32) {
	if o := c.jTarget(); o != nil {
		o.broadcast(emotionPacket(o.id, kind, 0, 0, emote, target, 0, 0, 0, 0, 0, 0), true)
	}
}

// jTeleport is TeleportService.teleportTo(player, world, instance, x, y, z, heading, delay). The translator writes
// Java's default instance 1 as this server's instance 0.
func (c *conn) jTeleport(world, instance int32, x, y, z float32, heading byte, delay int32) bool {
	return c.s.teleportToInstance(c.player, world, instance, x, y, z, heading, time.Duration(delay)*time.Millisecond)
}

// jAddNewSpawn is QuestService.addNewSpawn: the npc, spawned once (no respawn) at the place.
func (c *conn) jAddNewSpawn(world, instance, npcID int32, x, y, z float32, heading byte) *object {
	template := c.s.data.Npcs[npcID]
	if template == nil {
		return nil
	}
	o := &object{id: c.s.ids.nextID(), worldID: world, instance: instance, x: x, y: y, z: z, heading: heading,
		homeX: x, homeY: y, homeZ: z, npc: template}
	c.s.initNpc(o)
	c.s.byID[o.id] = o
	c.s.addObject(o)
	return o
}

// jDistance is MathUtil.getDistance between two points.
func jDistance(x1, y1, z1, x2, y2, z2 float32) float32 {
	dx, dy, dz := x1-x2, y1-y2, z1-z2
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}

// jUseSkill is SkillEngine.getSkill(player, id, level, player).useSkill(): the player casts the skill on itself.
func (c *conn) jUseSkill(id, level int32) {
	tmpl := c.s.data.Skills[id]
	if tmpl == nil {
		return
	}
	sk := &skill{s: c.s, tmpl: tmpl, effector: c.player, level: level, first: c.player}
	sk.use()
}

// javaPort runs a translated Java handler. Where the Java throws (a missing target or quest state), its packet handler
// logs the exception and the client gets nothing more; here the panic is recovered the same way, and the dialog counts
// as answered so that nothing is echoed.
func (c *conn) javaPort(run func() bool) (answered bool) {
	defer func() {
		if r := recover(); r != nil {
			c.s.log.Error("quest handler failed", "err", r)
			answered = true
		}
	}()
	return run()
}
