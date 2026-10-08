package game

import "aionlightning/game/data"

// [Group] General Destruction (Elyos 3702, Nereus) and General Death (Asmodian 4702,
// Lisya): the Java tree leaves both as TODO. The giver hands over the faction's siege
// weapon item; var 0 goes 0 -> 1 when the fortress gate falls (for every holder in the
// run, as period guides describe), and killing the General then readies the reward.
// ponytail: dialog pages follow the shared 1.9 conventions (1011 start, window 5 on
// reward); the client's encoded quest_q3702/4702 pages were not decoded to confirm them.
var nochsanaGeneralQuests = map[int32]int32{3702: 182202179, 4702: 182205676} // quest -> siege weapon item

func (c *conn) nochsanaGeneralQuestDialog(o *object, script *data.QuestScript, d int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || o.npc.ID != script.StartNPC {
		return false
	}
	q := c.player.quest(script.ID)
	switch {
	case q != nil && q.Status == "REWARD":
		if d == -1 || d == 1009 {
			c.send(dialogWindow(o.id, 5, script.ID))
			return true
		}
		if d >= 8 && d <= 17 {
			c.customQuestEnd(o, script, uint16(d))
			return true
		}
		return false
	case q != nil && q.Status == "START":
		return false
	case d == 25 || d == 1002 || d == 1003 || d == 1007: // the pages defaultQuestStartDialog answers
		return c.customQuestStart(o, script, uint16(d), nochsanaGeneralQuests[script.ID])
	}
	return false
}

// nochsanaGeneralGateCredit gives the gate step to every player of the run on the quest.
func (s *Server) nochsanaGeneralGateCredit(gate *object) {
	for _, p := range s.spawned {
		if p.conn == nil || p.WorldID != gate.worldID || p.instance != gate.instance {
			continue
		}
		for id := range nochsanaGeneralQuests {
			if q := p.quest(id); q != nil && q.Status == "START" && questVar(q.Vars, 0) == 0 {
				p.conn.customQuestProgress(id, setQuestVar(q.Vars, 0, 1), "")
			}
		}
	}
}

// nochsanaGeneralKill readies the reward once the gate step is done.
func (c *conn) nochsanaGeneralKill(id int32, dead *object) {
	q := c.player.quest(id)
	if dead.npc.ID != nochsanaGeneral || q == nil || q.Status != "START" || questVar(q.Vars, 0) != 1 {
		return
	}
	next := *q
	next.Status = "REWARD"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("readying the Nochsana General quest", "quest", id, "err", err)
		return
	}
	*q = next
	c.send(questAccepted(2, next))
}
