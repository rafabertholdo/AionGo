package game

import "aionlightning/game/data"

// Sanctum talk-chain quests (Java quest/sanctum handlers 3934-3939, 3965-3967).
// Holy-item purification steps follow the shared talkStep shape.

// The Quest for Templars (Java _3934TheQuestForTemplars): eight officers, Jucleas purifies.
var questForTemplarsChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 798359, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 798360, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 798361, vars: 2, page: 1693, advance: []uint16{10002}},
		{npc: 798362, vars: 3, page: 2034, advance: []uint16{10003}},
		{npc: 798363, vars: 4, page: 2375, advance: []uint16{10004}},
		{npc: 798364, vars: 5, page: 2716, advance: []uint16{10005}},
		{npc: 798365, vars: 6, page: 3057, advance: []uint16{10006}},
		{npc: 798366, vars: 7, page: 3398, advance: []uint16{10007}},
		{npc: 203752, vars: 8, page: 3739, pageMissing: 3825, holyItem: 186000080},
	},
}

// Shoulder the Burden (Java _3935ShoulderTheBurden).
var shoulderTheBurdenChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 203316, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 203702, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 203329, vars: 2, page: 1693, advance: []uint16{10002}},
		{npc: 203329, vars: 3, page: 2034, turnIn: 186000079, turnInCount: 30},
		{npc: 203752, vars: 4, page: 2375, pageMissing: 2461, holyItem: 186000080},
	},
}

// Decorations of Sanctum (Java _3936DecorationsOfSanctum): Dairos alone; dialog 10000
// only shows page 1352, dialog 33 takes the four collect items and moves to REWARD.
var decorationsOfSanctumChain = talkChain{
	startPage: 4762,
	special: func(c *conn, o *object, script *data.QuestScript, dialogID uint16) bool {
		if dialogID != 10000 || o.npc.ID != script.StartNPC {
			return false
		}
		c.send(dialogWindow(o.id, 1352, script.ID))
		return true
	},
	steps: []talkStep{
		{npc: 203710, vars: 0, page: 1011, collectMissing: 10001, collectDone: 5, toReward: true},
	},
}

// Well Rounded (Java _3938WellRounded): Lavirintos, six masters, Anusis, Jucleas purifies.
var wellRoundedChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 203701, vars: 0, page: 1011, advance: []uint16{10000, 10001, 10002, 10003, 10004, 10005}},
		{npc: 203788, vars: 1, page: 1352, advance: []uint16{10006}, give: 152201596},
		{npc: 203792, vars: 2, page: 1693, advance: []uint16{10006}, give: 152201639},
		{npc: 203790, vars: 3, page: 2034, advance: []uint16{10006}, give: 152201615},
		{npc: 203793, vars: 4, page: 2375, advance: []uint16{10006}, give: 152201632},
		{npc: 203784, vars: 5, page: 2716, advance: []uint16{10006}, give: 152201644},
		{npc: 203786, vars: 6, page: 3057, advance: []uint16{10006}, give: 152201643},
		{npc: 798316, vars: 7, page: 3398, turnIn: 186000077, turnInCount: 1},
		{npc: 203752, vars: 8, page: 3739, pageMissing: 3825, holyItem: 186000081},
	},
}

// Persistence and Luck (Java _3939PersistenceAndLuck): Sabotes takes 3,400,000 kinah.
var persistenceAndLuckChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 203780, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 203781, vars: 1, page: 1352, kinahDialog: 10001, kinahCost: 3400000, kinahShort: 1438},
		{npc: 203780, vars: 2, page: 1693, turnIn: 182206098, turnInCount: 20},
		{npc: 203752, vars: 3, page: 2034, pageMissing: 2120, holyItem: 186000080},
	},
}

// To the Galleria of Grandeur (Java _3965): two vouchers on accept, Andu takes one,
// Palentine takes the other. Java's Andu removes every voucher (so Palentine's removal
// could never succeed); this port takes one.
var galleriaOfGrandeurChain = talkChain{
	startItem:  182206120,
	startCount: 2,
	endPage:    2375,
	special: func(c *conn, o *object, script *data.QuestScript, dialogID uint16) bool {
		p := c.player
		vars := questVar(p.quest(script.ID).Vars, 0)
		switch {
		case o.npc.ID == 798391 && vars == 0 && dialogID == 10000:
			if c.s.countItems(p, 182206120) > 1 && c.customQuestProgress(script.ID, setQuestVar(p.quest(script.ID).Vars, 0, 1), "") {
				c.s.removeItemsByID(p, 182206120, 1)
			}
			return true
		case o.npc.ID == 798390 && vars == 1 && dialogID == 1009:
			if c.s.countItems(p, 182206120) > 0 && c.customQuestProgress(script.ID, setQuestVar(p.quest(script.ID).Vars, 0, 2), "REWARD") {
				c.s.removeItemsByID(p, 182206120, c.s.countItems(p, 182206120))
				c.send(dialogWindow(o.id, 5, script.ID))
			}
			return true
		}
		return false
	},
	steps: []talkStep{
		{npc: 798391, vars: 0, page: 1352},
		{npc: 798390, vars: 1, page: 2375},
	},
}

// Salute a New Uniform (Java _3966SaluteANewUniform).
var saluteANewUniformChain = talkChain{
	endPage: 2375,
	steps: []talkStep{
		{npc: 203994, vars: 0, page: 1352, advance: []uint16{10000}},
		{npc: 204030, vars: 1, page: 1693, advance: []uint16{10001}},
		{npc: 204568, vars: 2, page: 2034, advance: []uint16{10002}, toReward: true},
	},
}

// Andu's Dye Box (Java _3967AndusDyeBox, after 3966): Arenzes hands over the box.
var andusDyeBoxChain = talkChain{
	endPage:    2375,
	rewardTake: []int32{182206122},
	steps: []talkStep{
		{npc: 798309, vars: 0, page: 1352, advance: []uint16{10000}, give: 182206122, toReward: true},
	},
}
