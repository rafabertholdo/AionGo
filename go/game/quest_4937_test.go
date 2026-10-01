package game

import "testing"

func TestRecognitionOfThePreceptors(t *testing.T) {
	f := newChainFixture(t, 4937)
	f.start(4762)
	for i, npc := range []int32{204059, 204058, 204057, 204056} {
		f.talk(npc, uint16(10000+i))
		f.expectVars(int32(i + 1))
	}
	f.purify(204075, 186000085, 2461, 2375)
	f.finish(204053)
}
