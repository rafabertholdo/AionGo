package game

// A Booklet on Stigma (Java _4935ABookletOnStigma): Teirunerk wants four
// research logs and hands over a letter, Kohrunerk swaps it for the Tattered
// Booklet. Java also demands the booklet at the reward; that is not checked here.
var bookletOnStigmaChain = talkChain{
	startPage: 4762,
	endPage:   10002,
	steps: []talkStep{
		{npc: 204285, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 204285, vars: 1, page: 1352, collectMissing: 10001, collectDone: 10000, give: 182207107},
		{npc: 279005, vars: 2, page: 1693, advance: []uint16{10255}, give: 182207108, take: 182207107, toReward: true},
	},
}
