package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	sealingAbyssGateQuestID    int32 = 1020
	sealingAbyssGatePernosID   int32 = 203098
	sealingAbyssGateEntryID    int32 = 700141
	sealingAbyssGateGuardianID int32 = 700142
	sealingAbyssGateSealID     int32 = 700551
	sealingAbyssGateBossID     int32 = 210753
	sealingAbyssGateMapID      int32 = 310030000
	sealingAbyssGateItemID     int32 = 182200024
)

var sealingAbyssGatePrerequisites = [...]int32{1130, 1023, 1022, 1021, 1019, 1018, 1017, 1016, 1015, 1014, 1013, 1012, 1011}

func (c *conn) sealingAbyssGatePrerequisitesComplete() bool {
	if c == nil || c.player == nil {
		return false
	}
	for _, questID := range sealingAbyssGatePrerequisites {
		quest := c.player.quest(questID)
		if quest == nil || quest.Status != "COMPLETE" {
			return false
		}
	}
	return true
}

func (c *conn) sealingAbyssGateStart() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(sealingAbyssGateQuestID)
	template := c.s.data.Quests[sealingAbyssGateQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel || !c.sealingAbyssGatePrerequisitesComplete() {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Sealing the Abyss Gate", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// sealingAbyssGateEnterWorld handles Java's login recovery and late quest start.
func (c *conn) sealingAbyssGateEnterWorld() bool {
	if c == nil || c.player == nil {
		return false
	}
	player := c.player
	quest := player.quest(sealingAbyssGateQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "START" {
		stage := questVar(quest.Vars, 0)
		if (stage == 2 || stage == 3) && player.WorldID != sealingAbyssGateMapID {
			if !c.customQuestProgress(sealingAbyssGateQuestID, 1, "") {
				return false
			}
			c.s.removeItemsByID(player, sealingAbyssGateItemID, 1)
			return true
		}
		return false
	}
	if quest.Status != "LOCKED" || player.level <= 15 || !c.sealingAbyssGatePrerequisitesComplete() {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(player.ID, next); err != nil {
		c.s.log.Error("starting Sealing the Abyss Gate on world entry", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// sealingAbyssGateDeath resets the instance stage and removes its carried key.
func (c *conn) sealingAbyssGateDeath() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(sealingAbyssGateQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	stage := questVar(quest.Vars, 0)
	if stage != 2 && stage != 3 {
		return false
	}
	if !c.customQuestProgress(sealingAbyssGateQuestID, 1, "") {
		return false
	}
	c.s.removeItemsByID(c.player, sealingAbyssGateItemID, 1)
	return true
}

func (c *conn) sealingAbyssGateDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != sealingAbyssGateQuestID {
		return false
	}
	quest := c.player.quest(sealingAbyssGateQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != sealingAbyssGatePernosID {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 1352, sealingAbyssGateQuestID))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, sealingAbyssGateQuestID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" {
		return false
	}
	stage := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case sealingAbyssGatePernosID:
		if dialogID == 25 && stage == 0 {
			c.send(dialogWindow(o.id, 1011, sealingAbyssGateQuestID))
			return true
		}
		if dialogID == 10000 && stage == 0 && c.customQuestProgress(sealingAbyssGateQuestID, 1, "") {
			c.send(dialogWindow(o.id, 0, 0))
			return true
		}
	case sealingAbyssGateEntryID:
		if stage == 1 {
			return c.sealingAbyssGateEnterInstance(o)
		}
		if stage == 3 && c.s.countItems(c.player, sealingAbyssGateItemID) > 0 {
			return c.sealingAbyssGateLeaveInstance(o)
		}
	case sealingAbyssGateSealID:
		if stage == 2 && c.s.countItems(c.player, sealingAbyssGateItemID) > 0 {
			return c.sealingAbyssGateUseSeal(o)
		}
		fallthrough
	case sealingAbyssGateGuardianID:
		if stage == 2 {
			return c.sealingAbyssGateSpawnGuardian(o)
		}
	}
	return false
}

func (c *conn) sealingAbyssGateEnterInstance(o *object) bool {
	player := c.player
	if o.useTask != nil || player.targetID != o.id {
		return false
	}
	player.broadcast(c.s.playerEmotionTo(player, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		quest := player.quest(sealingAbyssGateQuestID)
		if player.conn != c || !player.spawned || player.seen[o.id] != o || o.dead || player.targetID != o.id || quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 1 {
			return
		}
		if !c.customQuestProgress(sealingAbyssGateQuestID, 2, "") {
			return
		}
		instance := c.s.newInstance(sealingAbyssGateMapID)
		instance.registered[player.ID] = true
		c.s.teleportToInstance(player, sealingAbyssGateMapID, instance.id, 270.5, 174.3, 204.3, 0, 0)
	})
	return true
}

func (c *conn) sealingAbyssGateSpawnGuardian(o *object) bool {
	player := c.player
	if o.useTask != nil || player.targetID != o.id {
		return false
	}
	instanceID := player.instance
	player.broadcast(c.s.playerEmotionTo(player, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		quest := player.quest(sealingAbyssGateQuestID)
		if player.conn != c || !player.spawned || player.seen[o.id] != o || o.dead || player.targetID != o.id || player.WorldID != sealingAbyssGateMapID || player.instance != instanceID || quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 2 {
			return
		}
		player.broadcast(c.s.playerEmotionTo(player, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		if c.s.data.Npcs[sealingAbyssGateBossID] == nil {
			return
		}
		spawn := &object{id: c.s.ids.nextID(), worldID: sealingAbyssGateMapID, instance: instanceID,
			x: 258.89917, y: 237.20166, z: 217.06035, npc: c.s.data.Npcs[sealingAbyssGateBossID]}
		c.s.initNpc(spawn)
		c.s.byID[spawn.id] = spawn
		c.s.addObject(spawn)
	})
	return true
}

func (c *conn) sealingAbyssGateUseSeal(o *object) bool {
	player := c.player
	if o.useTask != nil || player.targetID != o.id {
		return false
	}
	instanceID := player.instance
	player.broadcast(c.s.playerEmotionTo(player, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		quest := player.quest(sealingAbyssGateQuestID)
		if player.conn != c || !player.spawned || player.seen[o.id] != o || o.dead || player.targetID != o.id || player.WorldID != sealingAbyssGateMapID || player.instance != instanceID || quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 2 || c.s.countItems(player, sealingAbyssGateItemID) < 1 {
			return
		}
		if c.customQuestProgress(sealingAbyssGateQuestID, 3, "") {
			c.send(playMovie(153))
		}
	})
	return true
}

func (c *conn) sealingAbyssGateLeaveInstance(o *object) bool {
	player := c.player
	if o.useTask != nil || player.targetID != o.id {
		return false
	}
	instanceID := player.instance
	player.broadcast(c.s.playerEmotionTo(player, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		quest := player.quest(sealingAbyssGateQuestID)
		if player.conn != c || !player.spawned || player.seen[o.id] != o || o.dead || player.targetID != o.id || player.WorldID != sealingAbyssGateMapID || player.instance != instanceID || quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 3 || c.s.countItems(player, sealingAbyssGateItemID) < 1 {
			return
		}
		if !c.customQuestProgress(sealingAbyssGateQuestID, quest.Vars, "REWARD") {
			return
		}
		c.s.removeItemsByID(player, sealingAbyssGateItemID, 1)
		c.s.teleportToInstance(player, 210030000, 0, 2684.308, 1068.7382, 199.375, 0, 0)
	})
	return true
}

func (c *conn) sealingAbyssGateShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != sealingAbyssGatePernosID || script == nil || script.ID != sealingAbyssGateQuestID {
		return false
	}
	quest := c.player.quest(sealingAbyssGateQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 1352, sealingAbyssGateQuestID))
	return true
}
