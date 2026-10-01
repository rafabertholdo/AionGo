package game

// Secret of the Greater Stigma (Java _4936SecretOfTheGreaterStigma): Hresvelgr,
// then Vergelmir collects 100 Aether Crystals and the Greater Stigma.
var secretOfTheGreaterStigmaChain = talkChain{
	steps: []talkStep{
		{npc: 204837, vars: 0, page: 1352, advance: []uint16{10000}},
		{npc: 204051, vars: 1, page: 2375, collectMissing: 2716, collectDone: 5, toReward: true},
	},
}
