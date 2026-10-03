package game

import "aionlightning/game/store"

// QuestEngine's event dispatch for the translated Java handlers (quest_java_dialogs.go): the handlers registered for
// the event run in register() order, and where Java stops at the first one that returns true, so does this.

func (c *conn) javaEventFirst(ids []int32, run func(id int32) bool) bool {
	for _, id := range ids {
		if c.s.data.QuestScripts[id] != nil && c.javaPort(func() bool { return run(id) }) {
			return true
		}
	}
	return false
}

// javaKill is QuestEngine.onKill: the npc o died and the player has the kill.
func (c *conn) javaKill(o *object) bool {
	return c.javaEventFirst(javaKills[o.npc.ID], func(id int32) bool {
		return javaKillHandlers[id](c, o, c.s.data.QuestScripts[id], 0)
	})
}

// javaAttack is QuestEngine.onAttack, after the player's damage on o.
func (c *conn) javaAttack(o *object) bool {
	return c.javaEventFirst(javaAttacks[o.npc.ID], func(id int32) bool {
		return javaAttackHandlers[id](c, o, c.s.data.QuestScripts[id], 0)
	})
}

// javaItemUse is QuestEngine.onItemUseEvent; true stops CM_USE_ITEM there.
func (c *conn) javaItemUse(item *store.Item) bool {
	return c.javaEventFirst(javaItemUses[item.ItemID], func(id int32) bool {
		return javaItemUseHandlers[id](c, nil, c.s.data.QuestScripts[id], 0, item)
	})
}

// javaEnterZone is QuestEngine.onEnterZone.
func (c *conn) javaEnterZone(zone string) bool {
	return c.javaEventFirst(javaZones[zone], func(id int32) bool {
		return javaEnterZoneHandlers[id](c, nil, c.s.data.QuestScripts[id], 0, zone)
	})
}

// javaMovieEnd is QuestEngine.onMovieEnd.
func (c *conn) javaMovieEnd(movieID int32) bool {
	return c.javaEventFirst(javaMovieEnds[movieID], func(id int32) bool {
		return javaMovieEndHandlers[id](c, nil, c.s.data.QuestScripts[id], 0, movieID)
	})
}

// javaEnterWorld is QuestEngine.onEnterWorld: every handler.
func (c *conn) javaEnterWorld() {
	for _, id := range javaEnterWorld {
		if script := c.s.data.QuestScripts[id]; script != nil {
			c.javaPort(func() bool { return javaEnterWorldHandlers[id](c, nil, script, 0) })
		}
	}
}

// javaDie is QuestEngine.onDie: every handler.
func (c *conn) javaDie() {
	for _, id := range javaDie {
		if script := c.s.data.QuestScripts[id]; script != nil {
			c.javaPort(func() bool { return javaDieHandlers[id](c, nil, script, 0) })
		}
	}
}

// javaQuestFinish is QuestEngine.onQuestFinish, run by QuestService.questFinish before the quest completes.
func (c *conn) javaQuestFinish() {
	for _, id := range javaQuestFinish {
		if script := c.s.data.QuestScripts[id]; script != nil {
			c.javaPort(func() bool { return javaQuestFinishHandlers[id](c, nil, script, 0) })
		}
	}
}
