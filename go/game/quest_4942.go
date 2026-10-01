package game

// Proving Proficiency (Java _4942ProvingProficiency): Kvasir picks a craft
// (any of dialogs 10000-10005), the six masters each hand over a quest item,
// Usener takes the crafted heart, Balder purifies, Kvasir rewards.
var provingProficiencyChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 204053, vars: 0, page: 1011, advance: []uint16{10000, 10001, 10002, 10003, 10004, 10005}},
		{npc: 204104, vars: 1, page: 1352, advance: []uint16{10006}, give: 152201596},
		{npc: 204108, vars: 2, page: 1693, advance: []uint16{10006}, give: 152201639},
		{npc: 204106, vars: 3, page: 2034, advance: []uint16{10006}, give: 152201615},
		{npc: 204110, vars: 4, page: 2375, advance: []uint16{10006}, give: 152201632},
		{npc: 204100, vars: 5, page: 2716, advance: []uint16{10006}, give: 152201644},
		{npc: 204102, vars: 6, page: 3057, advance: []uint16{10006}, give: 152201643},
		{npc: 798317, vars: 7, page: 3398, turnIn: 186000077, turnInCount: 1},
		{npc: 204075, vars: 8, page: 3739, pageMissing: 3825, holyItem: 186000084},
	},
}
