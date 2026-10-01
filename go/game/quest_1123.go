package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

// wheresTuttyDialog handles Pernos's offer and the report after entering Q1123.
func (c *conn) wheresTuttyDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if o.npc == nil || o.npc.ID != 790001 || script.ID != 1123 {
		return
	}
	q := c.player.quest(script.ID)
	if q == nil || q.Status == "NONE" {
		c.customQuestStart(o, script, dialogID, 0)
		return
	}
	if q.Status != "REWARD" {
		return
	}
	switch dialogID {
	case 1009:
		c.send(dialogWindow(o.id, 5, script.ID))
	case 17:
		c.finishQuest(script, o.id, dialogID)
	}
}

// wheresTuttyEnterZone is Java's onEnterZoneEvent for ZoneName.Q1123.
// The caller must invoke it only on entry into this zone, not on every move.
func (c *conn) wheresTuttyEnterZone(zoneName string) bool {
	if zoneName != "Q1123" || !c.customQuestProgress(1123, 0, "REWARD") {
		return false
	}
	c.send(wheresTuttyMovie())
	return true
}

// Java sends SM_PLAY_MOVIE(0, 11), whose type differs from the prologue movie.
func wheresTuttyMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(11)
	w.D(0)
	return w
}
