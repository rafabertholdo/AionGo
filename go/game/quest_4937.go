package game

// Recognition of the Preceptors (Java _4937RecognitionOfThePreceptors): four
// preceptors, Balder's purification, Kvasir rewards.
var recognitionOfThePreceptorsChain = talkChain{
	startPage: 4762,
	steps: []talkStep{
		{npc: 204059, vars: 0, page: 1011, advance: []uint16{10000}},
		{npc: 204058, vars: 1, page: 1352, advance: []uint16{10001}},
		{npc: 204057, vars: 2, page: 1693, advance: []uint16{10002}},
		{npc: 204056, vars: 3, page: 2034, advance: []uint16{10003}},
		{npc: 204075, vars: 4, page: 2375, pageMissing: 2461, holyItem: 186000085},
	},
}
