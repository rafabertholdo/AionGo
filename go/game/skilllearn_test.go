package game

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"aionlightning/game/store"
)

// TestLevelUpTeachesSkills has the level 1 mage gain levels and learn the skills its class gets.
func TestLevelUpTeachesSkills(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	p.skills = nil
	s.visMu.Lock()
	defer s.visMu.Unlock()
	// The exp of level 5 in one go: the skills of level 5 are learned (and those of the levels between, when they
	// come one at a time, but a jump learns the skills of the level it lands on).
	s.setExp(p, d.ExpStart(5))
	if p.level != 5 {
		t.Fatalf("level %d, want 5", p.level)
	}
	want := 0
	for _, learn := range d.SkillsAt(p.Class, p.Race, 5) {
		if learn.Autolearn {
			want++
			if !p.hasSkill(learn.SkillID) {
				t.Errorf("skill %d wasn't learned", learn.SkillID)
			}
		}
	}
	if want == 0 {
		t.Skip("the class learns nothing at level 5")
	}
	if got := tap.count(smSkillList); got != want {
		t.Errorf("%d skill lists sent, want %d", got, want)
	}
}

// TestLearnedSkillPacketMatchesClient19 rebuilds the SM_SKILL_LIST of the skills Wrathchild learned on level 2.
func TestLearnedSkillPacketMatchesClient19(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	checked := 0
	for _, row := range playSession(t) {
		if row[0] != "server" || row[1] != "SM_SKILL_LIST" || len(row[2]) != 58 {
			continue
		}
		payload, _ := hex.DecodeString(row[2])
		skill := store.Skill{ID: int32(binary.LittleEndian.Uint16(payload[2:])), Level: int32(binary.LittleEndian.Uint16(payload[4:]))}
		message := int32(binary.LittleEndian.Uint32(payload[13:]))
		if got := hex.EncodeToString(s.skillListAdded(skill, message).Data[1:]); got != row[2] {
			t.Errorf("skill %d differs from the 1.9 server's:\n%s", skill.ID, diffHex(got, row[2]))
		}
		checked++
	}
	if checked == 0 {
		t.Error("no learned skill in the recording")
	}
}
