package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

// Effects persistence tests require a disposable database with the repository schema.
func TestSaveEffectsReplacesAndReloads(t *testing.T) {
	dsn := os.Getenv("AION_EFFECTS_TEST_DSN")
	if dsn == "" {
		t.Skip("AION_EFFECTS_TEST_DSN isn't set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	// One connection, so the fixture's player needs no players row.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`SET FOREIGN_KEY_CHECKS = 0`); err != nil {
		t.Fatal(err)
	}
	s := Store{DB: db}
	const player = 2000000400
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM player_effects WHERE player_id = ?`, player); err != nil {
			t.Error(err)
		}
	})
	reuse := time.UnixMilli(time.Now().Add(time.Hour).UnixMilli())
	ctx := context.Background()
	if err := s.SaveEffects(ctx, player, []SavedEffect{{SkillID: 1, Level: 2, Current: 3}, {SkillID: 4, Reuse: reuse}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveEffects(ctx, player, []SavedEffect{{SkillID: 8845, Level: 3, Current: 100000, Reuse: reuse}, {SkillID: 1801, Reuse: reuse}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Effects(player)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int32]SavedEffect{8845: {SkillID: 8845, Level: 3, Current: 100000, Reuse: reuse}, 1801: {SkillID: 1801, Reuse: reuse}}
	if len(got) != len(want) {
		t.Fatalf("reloaded %+v, want %+v", got, want)
	}
	for _, e := range got {
		w := want[e.SkillID]
		if e.Level != w.Level || e.Current != w.Current || !e.Reuse.Equal(w.Reuse) {
			t.Fatalf("reloaded %+v, want %+v", e, w)
		}
	}
	// A failed insert leaves the earlier rows.
	if err := s.SaveEffects(ctx, player, []SavedEffect{{SkillID: 1}, {SkillID: 1}}); err == nil {
		t.Fatal("duplicate skill rows were saved")
	}
	if got, err := s.Effects(player); err != nil || len(got) != 2 {
		t.Fatalf("failed save changed the rows: %+v %v", got, err)
	}
	if err := s.SaveEffects(ctx, player, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Effects(player); err != nil || len(got) != 0 {
		t.Fatalf("empty save kept rows: %+v %v", got, err)
	}
}
