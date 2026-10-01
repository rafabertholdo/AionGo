package game

// The Shulack's Stigma (Java _4934TheShulacksStigma): Moreinen, Teirunerk twice
// (the second time with the Heavy Surkana looted from object 700562), Vergelmir
// rewards. The object goes through the shared quest-object loot path.
var shulacksStigmaChain = talkChain{
	startPage: 4762,
	endPage:   10002,
	steps: []talkStep{
		{npc: 204211, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 204285, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 204285, vars: 2, page: 1693, collectMissing: 10001, collectDone: 10000, toReward: true},
	},
}
