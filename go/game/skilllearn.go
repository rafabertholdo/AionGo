package game

import (
	"slices"
	"strconv"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// skillSaver keeps a player's skills, recipes and abyss rank; store.Store does.
type skillSaver interface {
	SaveSkill(playerID int32, skill store.Skill) error
	DeleteSkill(playerID, skillID int32) error
	AddRecipe(playerID, recipeID int32) error
	DeleteRecipe(playerID, recipeID int32) error
	SaveAbyssRank(playerID int32, a *store.AbyssRank) error
}

// System messages of learning skills (SkillList.sendMessage).
const (
	msgSkillLearned  = 1300050 // "You learned %0 level %1."
	msgCraftLearned  = 1330005
	msgGatherLearned = 1330061
)

// skillListAdded is SM_SKILL_LIST for one skill the player just learned, with the message that goes with it.
func (s *Server) skillListAdded(skill store.Skill, message int32) *wire.Writer {
	w := wire.Packet(smSkillList)
	w.H(1)
	w.H(uint16(skill.ID))
	w.H(uint16(skill.Level))
	w.C(0)
	w.C(0) // extra level
	w.D(0)
	w.C(0) // stigma
	w.D(message)
	w.H(0x24)
	if t := s.data.Skills[skill.ID]; t != nil {
		w.D(t.NameID)
	} else {
		w.D(0)
	}
	w.H(0)
	w.S(strconv.Itoa(int(skill.Level)))
	return w
}

// addSkill is SkillList.addSkill: the player learns the skill, or its level of it rises. False if it knew it as well or better.
func (s *Server) addSkill(p *player, id, level int32, message bool) bool {
	i := slices.IndexFunc(p.skills, func(k store.Skill) bool { return k.ID == id })
	if i >= 0 {
		if p.skills[i].Level >= level {
			return false
		}
		p.skills[i].Level = level
	} else {
		p.skills = append(p.skills, store.Skill{ID: id, Level: level})
		i = len(p.skills) - 1
	}
	if err := s.skillDB.SaveSkill(p.ID, p.skills[i]); err != nil {
		s.log.Error("saving skill", "skill", id, "err", err)
	}
	if message {
		msg := int32(msgSkillLearned)
		switch id {
		case 30001, 30002, 30003:
			msg = msgCraftLearned
		case 40001, 40002, 40003, 40004, 40005, 40006, 40007, 40008, 40009:
			msg = msgGatherLearned
		}
		p.conn.send(s.skillListAdded(p.skills[i], msg))
	}
	return true
}

// removeSkill is SkillList.removeSkill.
func (s *Server) removeSkill(p *player, id int32) bool {
	i := slices.IndexFunc(p.skills, func(k store.Skill) bool { return k.ID == id })
	if i < 0 {
		return false
	}
	p.skills = slices.Delete(p.skills, i, i+1)
	if err := s.skillDB.DeleteSkill(p.ID, id); err != nil {
		s.log.Error("deleting skill", "skill", id, "err", err)
	}
	w := wire.Packet(smSkillRemove)
	w.H(uint16(id))
	p.conn.send(w)
	return true
}

// learnSkillsFor is SkillLearnService.addSkills: the skills a class learns at a level.
func (s *Server) learnSkillsFor(p *player, level int, class string, message bool) {
	for _, learn := range s.data.SkillsAt(class, p.Race, level) {
		if p.hasSkill(learn.SkillID) || learn.Autolearn {
			s.addSkill(p, learn.SkillID, learn.SkillLevel, message)
		}
	}
}

// learnNewSkills is SkillLearnService.addNewSkills for a player who reached its level.
func (s *Server) learnNewSkills(p *player) {
	before := len(p.skills)
	s.learnSkillsFor(p, p.level, p.Class, true)
	if len(p.skills) != before {
		// A passive skill it learned changes its stats.
		p.stats = s.playerStats(p)
		p.conn.send(s.statsInfo(p))
	}
}

// switchCraftingSkill is what reaching level 10 does to the first crafting skill: 30001 becomes 30002.
func (s *Server) switchCraftingSkill(p *player) {
	i := slices.IndexFunc(p.skills, func(k store.Skill) bool { return k.ID == 30001 })
	if i < 0 {
		return
	}
	level := p.skills[i].Level
	s.removeSkill(p, 30001)
	p.conn.send(skillList(p))
	s.addSkill(p, 30002, level, true)
}
