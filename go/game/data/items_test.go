package data

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillBookClassAndRaceOrdinals(t *testing.T) {
	classes := []struct{ legacy, player string }{
		{"WARRIOR", "WARRIOR"}, {"FIGHTER", "GLADIATOR"}, {"KNIGHT", "TEMPLAR"},
		{"SCOUT", "SCOUT"}, {"ASSASSIN", "ASSASSIN"}, {"RANGER", "RANGER"},
		{"MAGE", "MAGE"}, {"WIZARD", "SORCERER"}, {"ELEMENTALLIST", "SPIRIT_MASTER"},
		{"CLERIC", "PRIEST"}, {"PRIEST", "CLERIC"}, {"CHANTER", "CHANTER"}, {"ALL", "ALL"},
	}
	races := []struct{ legacy, player string }{
		{"PC_LIGHT", "ELYOS"}, {"PC_DARK", "ASMODIANS"}, {"ALL", "ALL"},
	}
	for _, class := range classes {
		for _, race := range races {
			t.Run(class.legacy+"/"+race.legacy, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "items.xml")
				xml := fmt.Sprintf(`<items><item_template id="1"><actions><skilllearn skillid="962" class="%s" level="3" race="%s"/></actions></item_template></items>`, class.legacy, race.legacy)
				if err := os.WriteFile(path, []byte(xml), 0600); err != nil {
					t.Fatal(err)
				}
				items, err := loadItems(path)
				if err != nil {
					t.Fatal(err)
				}
				action := items[1].Actions[0]
				if action.Str("class") != class.player || action.Str("race") != race.player {
					t.Fatalf("class/race = %s/%s, want %s/%s", action.Str("class"), action.Str("race"), class.player, race.player)
				}
				if action.Int("skillid") != 962 || action.Int("level") != 3 {
					t.Fatal("skill ID or required level changed")
				}
			})
		}
	}
}
