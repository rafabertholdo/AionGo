package data

import "path/filepath"

// NpcSkill is an npcskill of npc_skills.xml: a skill an npc may use while it fights.
type NpcSkill struct {
	SkillID     int32 `xml:"skillid,attr"`
	Level       int32 `xml:"skilllevel,attr"`
	Probability int32 `xml:"probability,attr"`
	AboutHP     bool  `xml:"abouthp,attr"`
}

// loadNpcSkills reads npc_skills.xml by npc template id.
func loadNpcSkills(dir string) (map[int32][]NpcSkill, error) {
	var file struct {
		Npcs []struct {
			ID     int32      `xml:"npcid,attr"`
			Skills []NpcSkill `xml:"npcskill"`
		} `xml:"npcskills"`
	}
	if err := loadXML(filepath.Join(dir, "npc_skills/npc_skills.xml"), &file); err != nil {
		return nil, err
	}
	skills := map[int32][]NpcSkill{}
	for _, n := range file.Npcs {
		skills[n.ID] = n.Skills
	}
	return skills, nil
}
