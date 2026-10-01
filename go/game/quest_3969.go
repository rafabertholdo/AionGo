package game

// The Sexiest Man Alive (Java _3969SexiestManAlive, after 3968): Palentine hands
// over a letter on acceptance, Andu takes it. Every dialog here is a plain
// click in Java, hence clickPages.
var sexiestManAliveChain = talkChain{
	startPage:  1011,
	startItem:  182206126,
	endPage:    2375,
	clickPages: true,
	steps: []talkStep{
		{npc: 798391, vars: 0, page: 1352, advance: []uint16{10000}, take: 182206126, toReward: true},
	},
}
