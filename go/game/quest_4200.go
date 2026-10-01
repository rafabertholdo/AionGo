package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	suspiciousCallItem     = 182209097 // Teleport Scroll, looted from Haorunerk's Bag (700522); 3200 uses 182209082
	suspiciousCallInstance = 300100000
	suspiciousCallDelay    = 3000 * time.Millisecond
)

// A Suspicious Call (Java _4200ASuspiciousCall; _3200PriceOfGoodwill is the same handler): Uikinerk sends the player into
// an instance, Haorunerk, the bag's Teleport Scroll (item use, 3s) takes the
// player out to Beluslan, Garkbinerk finishes, Payrinrinerk rewards.
var suspiciousCallChain = talkChain{
	startPage: 4762,
	endPage:   10002,
	special:   suspiciousCallUikinerk,
	steps: []talkStep{
		{npc: 798332, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 279006, vars: 3, page: 2034, advance: []uint16{10255}, toReward: true},
	},
}

// suspiciousCallUikinerk handles Uikinerk's extra dialogs; the teleport is
// Java's InstanceService.registerPlayerWithInstance + teleportTo.
func suspiciousCallUikinerk(c *conn, o *object, script *data.QuestScript, dialogID uint16) bool {
	if o.npc.ID != script.StartNPC {
		return false
	}
	p := c.player
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 1003, script.ID))
	case 1011:
		c.send(dialogWindow(o.id, 1011, script.ID))
	case 10000:
		q := p.quest(script.ID)
		if questVar(q.Vars, 0) == 0 && c.customQuestProgress(script.ID, 1, "") {
			in := c.s.newInstance(suspiciousCallInstance)
			in.registered[p.ID] = true
			c.s.teleportToInstance(p, suspiciousCallInstance, in.id, 403.55, 508.11, 885.77, 0, 0)
		}
	default:
		return false
	}
	return true
}

// suspiciousCallItemUse is onItemUseEvent: with variable 2 the scroll animates
// for three seconds, then suspiciousCallItemDone runs.
func (c *conn) suspiciousCallItemUse(item *store.Item, script *data.QuestScript) bool {
	p := c.player
	q := p.quest(script.ID)
	if q == nil || q.Status != "START" || questVar(q.Vars, 0) != 2 || item.ItemID != script.ItemID || p.cubeItem(item.UniqueID) != item {
		return false
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, int32(suspiciousCallDelay/time.Millisecond), 0, 0), true)
	c.s.later(suspiciousCallDelay, func() { c.suspiciousCallItemDone(item, script) })
	return true
}

func (c *conn) suspiciousCallItemDone(item *store.Item, script *data.QuestScript) {
	p := c.player
	q := p.quest(script.ID)
	if p.conn != c || q == nil || q.Status != "START" || questVar(q.Vars, 0) != 2 || p.cubeItem(item.UniqueID) != item {
		return
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
	if c.customQuestProgress(script.ID, 3, "") {
		c.s.removeItemsByID(p, script.ItemID, 1)
		c.s.teleportToInstance(p, 400010000, 0, 3419.16, 2445.43, 2766.54, 57, 0)
	}
}
