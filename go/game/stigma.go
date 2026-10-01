package game

import (
	"slices"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// Stigma stones: worn in the stigma slots, they teach a skill for as long as they are worn (AL-Game's StigmaService).
// The skill is shown to the client as a stigma skill and isn't saved.

const (
	stigmaShard        = 141000001
	msgStigmaLearned   = 1300401
	msgStigmaForgotten = 1300403
	msgStigmaNeeded    = 1300410
)

func (p *player) stigmaCount(s *Server) int {
	n := 0
	for _, item := range p.equipment {
		if t := s.template(item); t != nil && t.IsStigma() {
			n++
		}
	}
	return n
}

// stigmaEquip is StigmaService.notifyEquipAction: whether the stone may be worn, and the skill it teaches is learned.
func (s *Server) stigmaEquip(p *player, item *store.Item) bool {
	t := s.template(item)
	if t == nil || !t.IsStigma() {
		return true
	}
	if p.level/10+p.StigmaSlots <= p.stigmaCount(s) || t.Stigma == nil || !t.ClassSpecific(int(classIDs[p.Class])) {
		return false
	}
	if s.countItems(p, stigmaShard) < int64(t.Stigma.Shard) {
		return false
	}
	s.decreaseItemsByID(p, stigmaShard, int64(t.Stigma.Shard))
	s.learnStigmaSkill(p, t.Stigma.SkillID, t.Stigma.SkillLvl, true)
	return true
}

// learnStigmaSkill puts the skill among the player's, as a stigma one.
func (s *Server) learnStigmaSkill(p *player, id, level int32, message bool) {
	if p.stigma == nil {
		p.stigma = map[int32]bool{}
	}
	p.stigma[id] = true
	skill := store.Skill{ID: id, Level: level}
	if i := slices.IndexFunc(p.skills, func(k store.Skill) bool { return k.ID == id }); i >= 0 {
		p.skills[i] = skill
	} else {
		p.skills = append(p.skills, skill)
	}
	if message {
		p.conn.send(s.skillListAdded(skill, msgStigmaLearned))
	}
}

// stigmaUnequip is StigmaService.notifyUnequipAction: the skill is forgotten, unless a stone that stays needs it.
func (s *Server) stigmaUnequip(p *player, item *store.Item) bool {
	t := s.template(item)
	if t == nil || !t.IsStigma() || t.Stigma == nil {
		return true
	}
	skill := t.Stigma.SkillID
	for _, other := range p.equipment {
		ot := s.template(other)
		if other == item || ot == nil || ot.Stigma == nil {
			continue
		}
		for _, require := range ot.Stigma.Require {
			if slices.Contains(require.SkillIDs, skill) {
				p.conn.send(systemMessage(msgStigmaNeeded, descriptionID(t.NameID), descriptionID(ot.NameID)))
				return false
			}
		}
	}
	delete(p.stigma, skill)
	if !s.removeStigmaSkill(p, skill) {
		return true
	}
	name := int32(0)
	if sk := s.data.Skills[skill]; sk != nil {
		name = sk.NameID
	}
	p.conn.send(systemMessage(msgStigmaForgotten, descriptionID(name)))
	return true
}

// removeStigmaSkill takes a skill off the player's list without touching the database.
func (s *Server) removeStigmaSkill(p *player, id int32) bool {
	i := slices.IndexFunc(p.skills, func(k store.Skill) bool { return k.ID == id })
	if i < 0 {
		return false
	}
	p.skills = slices.Delete(p.skills, i, i+1)
	w := wire.Packet(smSkillRemove)
	w.H(uint16(id))
	p.conn.send(w)
	return true
}

// stigmaLogin is StigmaService.onPlayerLogin: the stones the player wears teach their skills again.
func (s *Server) stigmaLogin(p *player) {
	for _, item := range p.equipment {
		if t := s.template(item); t != nil && t.IsStigma() && t.Stigma != nil {
			s.learnStigmaSkill(p, t.Stigma.SkillID, t.Stigma.SkillLvl, false)
		}
	}
}
