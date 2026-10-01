package store

import (
	"database/sql"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// TestPunishmentQueries runs the prison and petition queries against a real database, when AION_TEST_DB names its
// host. It touches one character's punishment row and petitions of a player id far above any real one, and cleans up.
func TestPunishmentQueries(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("AION_TEST_DB isn't set")
	}
	config := mysql.NewConfig()
	config.Net, config.Addr, config.User, config.Passwd, config.DBName, config.ParseTime = "tcp", host+":3306", "root", "aion", "au_server_gs", true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := Store{DB: db}
	var id int32
	var name string
	var account int32
	if err := db.QueryRow(`SELECT id, name, account_id FROM players LIMIT 1`).Scan(&id, &name, &account); err != nil {
		t.Skip("the database has no characters")
	}
	const petitionID = 2000000000
	clean := func() {
		s.UnpunishPlayer(id)
		db.Exec(`DELETE FROM petitions WHERE id IN (?, ?)`, petitionID, petitionID+1)
	}
	clean()
	t.Cleanup(clean)

	if got, err := s.AccountIDByName(name); err != nil || got != account {
		t.Fatalf("account of %s: %d, %v", name, got, err)
	}
	if got, err := s.PlayerName(id); err != nil || got != name {
		t.Fatalf("name of %d: %q, %v", id, got, err)
	}
	if ms, err := s.PrisonTimer(id); err != nil || ms != 0 {
		t.Fatalf("a free player has %d ms: %v", ms, err)
	}
	if err := s.PunishPlayer(id, 1, 90000); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePunishment(id, 1, 60000); err != nil {
		t.Fatal(err)
	}
	if ms, err := s.PrisonTimer(id); err != nil || ms != 60000 {
		t.Fatalf("prison timer %d: %v", ms, err)
	}
	if err := s.UnpunishPlayer(id); err != nil {
		t.Fatal(err)
	}
	if ms, _ := s.PrisonTimer(id); ms != 0 {
		t.Fatalf("the sentence stayed")
	}

	if next, err := s.NextPetitionID(); err != nil || next < 1 {
		t.Fatalf("next petition id %d: %v", next, err)
	}
	p := &Petition{ID: petitionID, PlayerID: petitionID, Type: 768, Title: "t", Message: "m", Extra: "a/b"}
	if err := s.InsertPetition(p); err != nil {
		t.Fatal(err)
	}
	open, err := s.Petitions()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range open {
		found = found || *o == *p
	}
	if got, err := s.PetitionByID(petitionID); err != nil || got == nil || *got != *p || !found {
		t.Fatalf("petition %v, %v, listed %v", got, err, found)
	}
	if err := s.SetPetitionReplied(petitionID); err != nil {
		t.Fatal(err)
	}
	open, _ = s.Petitions()
	for _, o := range open {
		if o.ID == petitionID {
			t.Fatalf("a replied petition is still open")
		}
	}
	if err := s.InsertPetition(&Petition{ID: petitionID + 1, PlayerID: petitionID}); err == nil {
		s.DeletePetition(petitionID) // only pending ones, so the replied row above is left to the cleanup
	}
	if got, _ := s.PetitionByID(petitionID + 1); got != nil {
		t.Fatalf("the pending petition wasn't deleted")
	}
	if got, _ := s.PetitionByID(12345678); got != nil {
		t.Fatalf("a petition that isn't there")
	}
}
