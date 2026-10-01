package game

// Luck and Persistence (Java _4943LuckandPersistence): Latatusk, Relir sells
// the Light of Luck vessel for 3,400,000 kinah, 20 lights back to Latatusk,
// Balder purifies, Kvasir rewards.
var luckAndPersistenceChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 204096, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 204097, vars: 1, page: 1352, kinahDialog: 1354, kinahCost: 3400000, kinahShort: 1438, give: 182207123},
		{npc: 204096, vars: 2, page: 1693, turnIn: 182207124, turnInCount: 20},
		{npc: 204075, vars: 3, page: 2034, pageMissing: 2120, holyItem: 186000085},
	},
}
