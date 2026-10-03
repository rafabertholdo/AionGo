package game

import (
	"errors"
	"slices"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

type restoredSkills struct {
	noSkills
	writes []store.Skill
	err    error
}

func (db *restoredSkills) SaveSkill(_ int32, skill store.Skill) error {
	if db.err != nil {
		return db.err
	}
	db.writes = append(db.writes, skill)
	return nil
}

func TestRestoreAutolearnSkillsAllClasses(t *testing.T) {
	d := staticDataOrSkip(t)
	for index, class := range classNames {
		for _, race := range []string{"ELYOS", "ASMODIANS"} {
			t.Run(class+"/"+race, func(t *testing.T) {
				s := testServer(d)
				db := &restoredSkills{}
				s.skillDB = db
				level := 1
				if index%3 != 0 {
					level = 20
				}
				p := &player{character: &character{Character: &store.Character{ID: 42, Class: class, Race: race}}, level: level}
				if err := s.restoreAutolearnSkills(p); err != nil {
					t.Fatal(err)
				}
				want := map[int32]int32{}
				for l := 0; l <= level; l++ {
					classes := []string{class}
					if index%3 != 0 && l < 10 {
						classes = append(classes, classNames[index-index%3])
					}
					for _, candidate := range classes {
						for _, learn := range d.SkillsAt(candidate, race, l) {
							if !learn.Autolearn || learn.Stigma {
								continue
							}
							id := learn.SkillID
							if id == 30001 && level >= 10 {
								id = 30002
							}
							want[id] = max(want[id], learn.SkillLevel)
						}
					}
				}
				if len(p.skills) != len(want) {
					t.Fatalf("got %d skills, want %d", len(p.skills), len(want))
				}
				for _, skill := range p.skills {
					if want[skill.ID] != skill.Level {
						t.Errorf("unexpected skill %+v; want level %d", skill, want[skill.ID])
					}
				}
				writes := len(db.writes)
				if err := s.restoreAutolearnSkills(p); err != nil || len(db.writes) != writes {
					t.Fatalf("second login rewrites skills: %v, %d writes", err, len(db.writes)-writes)
				}
			})
		}
	}
}

func TestRestoreSkillsPreservesLearningRules(t *testing.T) {
	s := testServer(&data.Data{SkillTree: []data.SkillLearn{
		{Class: "PRIEST", SkillID: 965, SkillLevel: 1, MinLevel: 1, Autolearn: true},
		{Class: "PRIEST", SkillID: 975, SkillLevel: 2, MinLevel: 2, Autolearn: true},
		{Class: "PRIEST", SkillID: 100, SkillLevel: 1, MinLevel: 1},
		{Class: "PRIEST", SkillID: 101, SkillLevel: 1, MinLevel: 1, Autolearn: true, Stigma: true},
		{Class: "PRIEST", Race: "ASMODIANS", SkillID: 102, MinLevel: 1, Autolearn: true},
		{Class: "CLERIC", SkillID: 103, MinLevel: 1, Autolearn: true},
		{Class: "PRIEST", SkillID: 104, MinLevel: 3, Autolearn: true},
	}})
	db := &restoredSkills{}
	s.skillDB = db
	p := &player{character: &character{Character: &store.Character{Class: "PRIEST", Race: "ELYOS"}}, level: 2,
		skills: []store.Skill{{ID: 965, Level: 5}, {ID: 975, Level: 1}, {ID: 999, Level: 10}}}
	if err := s.restoreAutolearnSkills(p); err != nil {
		t.Fatal(err)
	}
	want := []store.Skill{{ID: 965, Level: 5}, {ID: 975, Level: 2}, {ID: 999, Level: 10}}
	if !slices.Equal(p.skills, want) || !slices.Equal(db.writes, []store.Skill{{ID: 975, Level: 2}}) {
		t.Fatalf("skills %v, writes %v", p.skills, db.writes)
	}
}

func TestRestoreSkillSaveFailure(t *testing.T) {
	s := testServer(&data.Data{SkillTree: []data.SkillLearn{{Class: "PRIEST", SkillID: 965, SkillLevel: 1, MinLevel: 1, Autolearn: true}}})
	failure := errors.New("database unavailable")
	s.skillDB = &restoredSkills{err: failure}
	for _, skills := range [][]store.Skill{nil, {{ID: 965, Level: 0}}} {
		p := &player{character: &character{Character: &store.Character{Class: "PRIEST"}}, level: 1, skills: slices.Clone(skills)}
		if err := s.restoreAutolearnSkills(p); !errors.Is(err, failure) {
			t.Fatalf("got %v, want database failure", err)
		}
		if !slices.Equal(p.skills, skills) {
			t.Fatalf("failed save changed skills: %v", p.skills)
		}
	}
}
