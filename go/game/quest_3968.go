package game

// Palentine's Request (Java _3968PalentinesRequest): three NPCs each hand over a
// quest item; Palentine takes them back when the reward is chosen.
var palentinesRequestChain = talkChain{
	endPage:    2375,
	rewardTake: []int32{182206123, 182206124, 182206125},
	steps: []talkStep{
		{npc: 798176, vars: 0, page: 1352, advance: []uint16{10000}, give: 182206123},
		{npc: 204528, vars: 1, page: 1693, advance: []uint16{10001}, give: 182206124},
		{npc: 203927, vars: 2, page: 2034, advance: []uint16{10002}, give: 182206125, toReward: true},
	},
}
