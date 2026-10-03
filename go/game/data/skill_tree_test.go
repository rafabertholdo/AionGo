package data

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSkillTreesMatchJavaClassOrdinals(t *testing.T) {
	if staticData == "" {
		t.Skip("AION_DATA is not set")
	}
	d, err := Load(staticData)
	if err != nil {
		t.Fatal(err)
	}
	var raw []SkillLearn
	for _, name := range []string{"craft_skill_tree.xml", "skill_tree.xml"} {
		var tree struct {
			Skills []SkillLearn `xml:"skill"`
		}
		if err := loadXML(filepath.Join(staticData, "skill_tree", name), &tree); err != nil {
			t.Fatal(err)
		}
		raw = append(raw, tree.Skills...)
	}
	// SkillClass and PlayerClass have corresponding enum positions in Java.
	javaClasses := []string{"WARRIOR", "FIGHTER", "KNIGHT", "SCOUT", "ASSASSIN", "RANGER", "MAGE", "WIZARD", "ELEMENTALLIST", "CLERIC", "PRIEST", "CHANTER"}
	playerClasses := []string{"WARRIOR", "GLADIATOR", "TEMPLAR", "SCOUT", "ASSASSIN", "RANGER", "MAGE", "SORCERER", "SPIRIT_MASTER", "PRIEST", "CLERIC", "CHANTER"}
	maxLevel := 0
	for _, entry := range raw {
		maxLevel = max(maxLevel, entry.MinLevel)
		if entry.Class != "ALL" && !slices.Contains(javaClasses, entry.Class) {
			t.Fatalf("unrecognized Java skill class %q", entry.Class)
		}
	}
	for index, class := range playerClasses {
		t.Run(class, func(t *testing.T) {
			for _, race := range []string{"ELYOS", "ASMODIANS"} {
				for level := 0; level <= maxLevel; level++ {
					var want []SkillLearn
					for _, entry := range raw {
						if entry.MinLevel == level && (entry.Class == javaClasses[index] || entry.Class == "ALL") &&
							(entry.Race == "" || entry.Race == "ALL" || entry.Race == race) {
							if entry.Class != "ALL" {
								entry.Class = class
							}
							want = append(want, entry)
						}
					}
					if got := d.SkillsAt(class, race, level); !slices.Equal(got, want) {
						t.Fatalf("%s level %d: got %v, want Java %v", race, level, got, want)
					}
				}
			}
		})
	}
}

func TestStartingClassSkills(t *testing.T) {
	if staticData == "" {
		t.Skip("AION_DATA is not set")
	}
	if _, err := os.Stat(staticData); err != nil {
		t.Fatal(err)
	}
	d, err := Load(staticData)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		class string
		ids   []int32
	}{
		{"WARRIOR", []int32{1, 3, 4, 5, 6, 7, 67, 122, 169, 1801, 1803, 30001}},
		{"SCOUT", []int32{1, 4, 5, 30, 67, 564, 572, 1801, 1803, 30001}},
		{"MAGE", []int32{4, 64, 67, 1351, 1373, 1801, 1803, 30001}},
		{"PRIEST", []int32{3, 4, 5, 67, 965, 975, 1801, 1803, 30001}},
	} {
		for _, race := range []string{"ELYOS", "ASMODIANS"} {
			t.Run(tc.class+"/"+race, func(t *testing.T) {
				var ids []int32
				for _, learn := range d.SkillsAt(tc.class, race, 1) {
					if learn.Autolearn {
						if learn.SkillLevel != 1 {
							t.Errorf("skill %d starts at level %d", learn.SkillID, learn.SkillLevel)
						}
						ids = append(ids, learn.SkillID)
					}
				}
				slices.Sort(ids)
				if !slices.Equal(ids, tc.ids) {
					t.Fatalf("starting skills %v, want %v", ids, tc.ids)
				}
			})
		}
	}
}
