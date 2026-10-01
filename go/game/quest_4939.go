package game

// Proving Ground (Java _4939ProvingGround): Njord, Sichel, Skadi, 30 medals to
// Skadi, then Balder's purification and Kvasir's reward. Java lets Njord advance
// at any variable; it is guarded to variable 0 here.
var provingGroundChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 204055, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 204273, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 204054, vars: 2, page: 1693, advance: []uint16{10002}},
		{npc: 204054, vars: 3, page: 2034, turnIn: 186000079, turnInCount: 30},
		{npc: 204075, vars: 4, page: 2375, pageMissing: 2461, holyItem: 186000085},
	},
}
