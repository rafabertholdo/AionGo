package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpawnLookupPreservesFileOrderAndReload(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.xml": `<spawns><spawn map="300000000" npcid="123456" pool="0"><object x="1.5" y="-2.25" z="3.75"/></spawn><spawn map="300000000" npcid="123456" pool="1"><object x="2"/></spawn></spawns>`,
		"b.xml": `<spawns><spawn map="100000000" npcid="123456" pool="1"><object x="3"/></spawn></spawns>`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	d := &Data{}
	if err := d.loadSpawns(dir); err != nil {
		t.Fatal(err)
	}
	groups := d.SpawnsByNPC[123456]
	if len(groups) != 3 || groups[0].Map != 300000000 || groups[0].Spots[0].X != 1.5 || groups[1].Spots[0].X != 2 || groups[2].Map != 100000000 {
		t.Fatalf("NPC groups lost file order: %+v", groups)
	}
	if groups[0] != d.Spawns[300000000][0] {
		t.Fatal("world and NPC lookup must share spawn groups")
	}
	second := groups[1]
	d.RemoveSpawnGroup(groups[0])
	if len(d.Spawns[300000000]) != 1 || d.SpawnsByNPC[123456][0] != second {
		t.Fatal("removing a spawn group did not update both lookups")
	}
	added := &SpawnGroup{Map: 200000000, NpcID: 123456, Spots: []Spot{{X: 4}}}
	d.AddSpawnGroup(added)
	if d.SpawnsByNPC[123456][2] != added || d.Spawns[200000000][0] != added {
		t.Fatal("admin spawn was not appended to both lookups")
	}
	if err := d.loadSpawns(dir); err != nil {
		t.Fatal(err)
	}
	if len(d.SpawnsByNPC[123456]) != 3 || len(d.Spawns[200000000]) != 0 || d.SpawnsByNPC[123456][0].Spots[0].X != 1.5 {
		t.Fatal("reload retained modified spawn data")
	}
}
