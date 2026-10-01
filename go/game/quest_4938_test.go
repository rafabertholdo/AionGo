package game

import "testing"

func TestWorkOfTheFenrisFangs(t *testing.T) {
	f := newChainFixture(t, 4938)
	f.start(4762)
	f.talk(798369, 10002) // out of order
	f.expectVars(0)
	npcs := []int32{798367, 798368, 798369, 798370, 798371, 798372, 798373, 798374}
	for i, npc := range npcs {
		f.talk(npc, uint16(10000+i))
		f.expectVars(int32(i + 1))
	}
	f.purify(204075, 186000085, 3825, 3739)
	f.finish(204053)
}
