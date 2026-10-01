package game

import (
	"slices"
	"testing"
)

// TestStigma has a mage wear a stigma stone: it costs shards and teaches its skill, which goes when the stone is taken off.
func TestStigma(t *testing.T) {
	d := staticDataOrSkip(t)
	var stone int32
	for _, id := range slices.Sorted(mapKeys(d.Items)) {
		it := d.Items[id]
		if it.IsStigma() && it.Stigma != nil && len(it.Stigma.Require) == 0 && it.ClassSpecific(int(classIDs["MAGE"])) && it.Level <= 20 && it.Stigma.Shard < 100 {
			stone = id
			break
		}
	}
	if stone == 0 || d.Items[stigmaShard] == nil {
		t.Skip("no stigma stone for a mage in the data")
	}
	tmpl := d.Items[stone]
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	p.level = 20
	p.Class = "MAGE"
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.addItem(p, stone, 1)
	s.addItem(p, stigmaShard, int64(tmpl.Stigma.Shard))
	var item int32
	for _, it := range p.cube {
		if it.ItemID == stone {
			item = it.UniqueID
		}
	}
	if s.equipItem(p, item, 1<<19) == nil {
		t.Fatalf("the stone wasn't worn")
	}
	if !p.hasSkill(tmpl.Stigma.SkillID) || !p.stigma[tmpl.Stigma.SkillID] || s.countItems(p, stigmaShard) != 0 {
		t.Errorf("skill %v, shards %d", p.hasSkill(tmpl.Stigma.SkillID), s.countItems(p, stigmaShard))
	}
	if s.unequipItem(p, item) == nil || p.hasSkill(tmpl.Stigma.SkillID) {
		t.Errorf("the skill stayed")
	}
}
