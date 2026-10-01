package game

import "aionlightning/game/data"

// More Sanctum, Eltnen and Theobomos talk-chain quests on the talkChain shape.

// Class Preceptor's Consent (Java _3933ClassPreceptorConsent): four masters sign, Jucleas purifies.
var classPreceptorConsentChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 203704, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 203705, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 203706, vars: 2, page: 1693, advance: []uint16{10002}},
		{npc: 203707, vars: 3, page: 2034, advance: []uint16{10003}},
		{npc: 203752, vars: 4, page: 2375, pageMissing: 2461, holyItem: 186000080},
	},
}

// Stop the Shulacks (Java _3932StopTheShulacks): Maloren, then Miriya collects the drops.
var stopTheShulacksChain = talkChain{
	startPage: 1011,
	steps: []talkStep{
		{npc: 204656, vars: 0, page: 1352, advance: []uint16{10000}},
		{npc: 203711, vars: 1, page: 2375, collectMissing: 2716, collectDone: 5, toReward: true},
	},
}

// How to Use Stigma (Java _3931HowToUseStigma): Koruchinerk twice, Kohrunerk swaps the belt for the manual.
var howToUseStigmaChain = talkChain{
	startPage: 4762,
	endPage:   10002,
	steps: []talkStep{
		{npc: 798321, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 798321, vars: 1, page: 1352, collectMissing: 10001, collectDone: 10000, give: 182206080},
		{npc: 279005, vars: 2, page: 1693, advance: []uint16{10255}, take: 182206080, give: 182206081, toReward: true},
	},
}

// Secret of the Shattered Stigma (Java _3930): the strongbox (700562, quest_drop) holds the item Koruchinerk collects.
var shatteredStigmaChain = talkChain{
	startPage: 4762,
	endPage:   10002,
	steps: []talkStep{
		{npc: 203833, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 798321, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 798321, vars: 2, page: 1693, collectMissing: 10001, collectDone: 10000, toReward: true},
	},
}

// The Balaur Report (Java _3914TheBalaurReport): started by using the report item, Jucleas hands it in.
var balaurReportChain = talkChain{
	endPage: 2375,
	steps: []talkStep{
		{npc: 203752, vars: 0, page: 1352, advance: []uint16{10000}, toReward: true},
	},
}

// An Order for Gojirunerk (Java _3319AnOrderforGojirunerk).
var orderForGojirunerkChain = talkChain{
	steps: []talkStep{
		{npc: 798138, vars: 0, page: 1352, advance: []uint16{10000}},
		{npc: 798050, vars: 1, page: 2375, reward1009: true},
	},
}

// Secret Dumpling Recipe (Java _3093RecetteSecretedeQuenelles). Java's start dialog
// NPEs (it bumps the variable of a quest that does not exist yet); this port does what
// it plainly meant: hand over the recipe and start at variable 1. Java lets any of the
// four NPCs pay out the reward, hence rewardPage.
var secretDumplingRecipeChain = talkChain{
	startItem:  182206062,
	startVar:   1,
	rewardPage: 2375,
	steps: []talkStep{
		{npc: 798177, vars: 1, page: 1352, advance: []uint16{10000}},
		{npc: 798179, vars: 2, page: 1693, advance: []uint16{10001}},
		{npc: 203784, vars: 3, page: 2034, advance: []uint16{10002}, give: 182208052, toReward: true},
	},
}

// The Shugo Menace (Java _3326TheShugoMenace): 20 kills of five Shugo variants (MonsterInfos
// count them into variable 0), then Sedona's page 10002 and dialog 1009 hand in. Java lets
// dialog 1009 in early; this port needs the 20 kills.
const shugoMenaceKills = 20

var shugoMenaceChain = talkChain{
	startPage: 4,
	endPage:   5,
	special: func(c *conn, o *object, script *data.QuestScript, dialogID uint16) bool {
		q := c.player.quest(script.ID)
		if o.npc.ID != script.StartNPC {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 10002, script.ID))
		case 1009:
			if questVar(q.Vars, 0) >= shugoMenaceKills && c.customQuestProgress(script.ID, q.Vars, "REWARD") {
				c.send(dialogWindow(o.id, 5, script.ID))
			}
		default:
			return false
		}
		return true
	},
}
