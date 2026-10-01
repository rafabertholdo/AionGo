package game

// The Zombie's Descendant (Java _4060TheZombiesDescendant): started from a
// quest item (ItemUseDelay path), three talks, then the end NPC takes the item at
// dialog 1009 and moves to REWARD.
var zombiesDescendantChain = talkChain{
	steps: []talkStep{
		{npc: 205156, vars: 0, page: 1352, advance: []uint16{10000}},
		{npc: 204143, vars: 1, page: 1693, advance: []uint16{10001}},
		{npc: 204731, vars: 2, page: 2034, advance: []uint16{10002}},
		{npc: 205204, vars: 3, page: 2375, reward1009: true, take: 182209037},
	},
}
