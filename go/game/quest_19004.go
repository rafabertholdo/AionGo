package game

// Perikles' Insight (Java _19004PeriklessInsight): Jucleas, Lavirintos, then
// Mysteris moves the quest to REWARD; every chain NPC shows page 10002 then.
var periklessInsightChain = talkChain{
	rewardPage: 10002,
	steps: []talkStep{
		{npc: 203752, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 203701, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 798500, vars: 2, page: 1693, advance: []uint16{10255}, toReward: true},
	},
}
