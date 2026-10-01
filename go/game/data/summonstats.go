package data

import (
	"encoding/xml"
	"os"
	"path/filepath"
)

// SummonStats is the stats_template of a summon of a level.
type SummonStats struct {
	MaxHP, MaxMP   int32
	PDefense       int32
	MResist        int32
	MainHandAttack int32
	RunSpeed       float32
}

type summonKey struct{ npc, level int32 }

func loadSummonStats(dir string) (map[summonKey]*SummonStats, error) {
	files, err := filepath.Glob(filepath.Join(dir, "stats/summon/*.xml"))
	if err != nil {
		return nil, err
	}
	stats := map[summonKey]*SummonStats{}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		var doc struct {
			List []struct {
				Light int32 `xml:"npc_id_light,attr"`
				Dark  int32 `xml:"npc_id_dark,attr"`
				Level int32 `xml:"level,attr"`
				Stats struct {
					MaxHP          int32   `xml:"maxHp,attr"`
					MaxMP          int32   `xml:"maxMp,attr"`
					PDefense       int32   `xml:"pdefense,attr"`
					MResist        int32   `xml:"mresist,attr"`
					MainHandAttack int32   `xml:"main_hand_attack,attr"`
					RunSpeed       float32 `xml:"run_speed,attr"`
				} `xml:"stats_template"`
			} `xml:"summon_stats"`
		}
		err = xml.NewDecoder(f).Decode(&doc)
		f.Close()
		if err != nil {
			return nil, err
		}
		for _, e := range doc.List {
			st := &SummonStats{MaxHP: e.Stats.MaxHP, MaxMP: e.Stats.MaxMP, PDefense: e.Stats.PDefense, MResist: e.Stats.MResist,
				MainHandAttack: e.Stats.MainHandAttack, RunSpeed: e.Stats.RunSpeed}
			stats[summonKey{e.Light, e.Level}] = st
			stats[summonKey{e.Dark, e.Level}] = st
		}
	}
	return stats, nil
}

// SummonStatsFor is SummonStatsData.getSummonTemplate: the stats of the summon npc at a level, or nil.
func (d *Data) SummonStatsFor(npc, level int32) *SummonStats {
	return d.summonStats[summonKey{npc, level}]
}
