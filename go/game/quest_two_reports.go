package game

import "aionlightning/game/data"

// twoReportsQuestDialog handles the five Java quests that visit two report
// NPCs before the final reward NPC. Both reports use dialog 10000.
func (c *conn) twoReportsQuestDialog(o *object, script *data.QuestScript, d int32) bool {
	var first, second, final int32
	switch script.ID {
	case 1553:
		first, second, final = 730051, 204500, 204584
	case 1620:
		first, second, final = 790000, 730001, 203125
	case 1578:
		first, second, final = 730024, 204560, 204579
	case 1605:
		first, second, final = 204530, 204501, 204577
	case 1483:
		first, second, final = 203940, 203944, 798127
	default:
		return false
	}
	secondAfter, endVars := uint16(1693), int32(3)
	if script.ID == 1553 {
		secondAfter, endVars = 10, 2
	}
	return c.reportsQuestDialog(o, script, d, reportsShape{noneStarts: true, end: final, endVars: endVars,
		reports: []questReport{{first, 1352, 10000, 10}, {second, 1352, 10000, secondAfter}}})
}
