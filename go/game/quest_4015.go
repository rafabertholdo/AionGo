package game

import (
	"time"

	"aionlightning/game/data"
)

// The Missing Laborers (Java _4015TheMissingLaborers): object 730107 is
// searched for three seconds (plain click, variable 0 -> 1), then the start NPC
// 205130 takes the report.
var missingLaborersChain = talkChain{
	special: missingLaborersSearch,
	steps: []talkStep{
		{npc: 205130, vars: 1, page: 2375, reward1009: true},
	},
}

func missingLaborersSearch(c *conn, o *object, script *data.QuestScript, dialogID uint16) bool {
	p := c.player
	q := p.quest(script.ID)
	if o.npc.ID != 730107 || dialogID != ^uint16(0) || questVar(q.Vars, 0) != 0 || o.useTask != nil {
		return false
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		current := p.quest(script.ID)
		if p.conn != c || !p.spawned || p.seen[o.id] != o || p.targetID != o.id || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 0 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.customQuestProgress(script.ID, 1, "")
	})
	return true
}
