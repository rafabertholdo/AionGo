package game

import (
	"testing"
	"time"

	"aionlightning/game/data"
)

// TestMonsterCastsSkills gives the striped kerub a skill it always uses and has it fight: the player sees it cast.
func TestMonsterCastsSkills(t *testing.T) {
	d := staticDataOrSkip(t)
	var skillID int32
	for _, n := range d.NpcSkills[201010] {
		if tmpl := d.Skills[n.SkillID]; tmpl != nil && tmpl.IsActive() {
			skillID = n.SkillID
		}
	}
	if skillID == 0 {
		t.Skip("no npc skill to try")
	}
	// A copy of the data, so that the other tests' monsters don't learn the skill.
	own := *d
	own.NpcSkills = map[int32][]data.NpcSkill{210133: {{SkillID: skillID, Level: 1, Probability: 100}}}
	s := testServer(&own)
	s.startWorldTasks()
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	s.visMu.Lock()
	s.updateNpcKnown(o)
	p.lastAttack = time.Time{}
	s.playerAttack(p, o)
	s.visMu.Unlock()
	for range 40 {
		time.Sleep(500 * time.Millisecond)
		if tap.count(smCastspell) > 0 {
			return
		}
		// Keep the monster angry, and the player alive.
		s.visMu.Lock()
		p.life.HP = p.stats.current(data.MaxHP)
		p.lastAttack = time.Time{}
		if !o.dead && !p.dead {
			s.playerAttack(p, o)
		}
		s.visMu.Unlock()
	}
	t.Errorf("the monster never cast skill %d", skillID)
}
