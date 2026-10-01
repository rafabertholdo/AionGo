package store

import (
	"database/sql"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// TestSocialQueries runs the friends and blocks queries against a real database, when AION_TEST_DB names its host.
// It uses two characters the database has, and leaves it as it found it.
func TestSocialQueries(t *testing.T) {
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
	rows, err := db.Query(`SELECT id FROM players LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int32
	for rows.Next() {
		var id int32
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) < 2 {
		t.Skip("the database has fewer than two characters")
	}
	a, b := ids[0], ids[1]
	t.Cleanup(func() { s.DelFriends(a, b); s.DelBlock(a, b) })

	if err := s.AddFriends(a, b); err != nil {
		t.Fatal(err)
	}
	if friends, err := s.Friends(a); err != nil || len(friends) != 1 || friends[0].ID != b {
		t.Fatalf("friends of a: %v, %v", friends, err)
	}
	if friends, err := s.Friends(b); err != nil || len(friends) != 1 || friends[0].ID != a {
		t.Fatalf("friends of b: %v, %v", friends, err)
	}
	if err := s.DelFriends(a, b); err != nil {
		t.Fatal(err)
	}
	if friends, _ := s.Friends(b); len(friends) != 0 {
		t.Fatalf("the friendship stayed")
	}
	if err := s.AddBlock(a, b, "rude"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBlockReason(a, b, "very rude"); err != nil {
		t.Fatal(err)
	}
	if blocks, err := s.Blocks(a); err != nil || len(blocks) != 1 || blocks[0].Reason != "very rude" || blocks[0].Name == "" {
		t.Fatalf("blocks: %v, %v", blocks, err)
	}
	if err := s.DelBlock(a, b); err != nil {
		t.Fatal(err)
	}
	// The skills the level up teaches.
	t.Cleanup(func() { s.DeleteSkill(a, 99999) })
	if err := s.SaveSkill(a, Skill{ID: 99999, Level: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSkill(a, Skill{ID: 99999, Level: 2}); err != nil {
		t.Fatal(err)
	}
	skills, err := s.Skills(a)
	found := false
	for _, k := range skills {
		found = found || k.ID == 99999 && k.Level == 2
	}
	if err != nil || !found {
		t.Fatalf("skills: %v, %v", skills, err)
	}
	if err := s.DeleteSkill(a, 99999); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DeleteRecipe(a, 99999) })
	if err := s.AddRecipe(a, 99999); err != nil {
		t.Fatal(err)
	}
	if recipes, err := s.Recipes(a); err != nil || !slices.Contains(recipes, 99999) {
		t.Fatalf("recipes: %v, %v", recipes, err)
	}
	if err := s.DeleteRecipe(a, 99999); err != nil {
		t.Fatal(err)
	}
	// Mail.
	letter := &Letter{ID: 2000000001, Recipient: b, Sender: "Sender", Title: "hi", Message: "text", Unread: true, Kinah: 5, Received: time.Now()}
	t.Cleanup(func() { s.DeleteLetter(letter.ID) })
	if err := s.InsertLetter(letter); err != nil {
		t.Fatal(err)
	}
	letter.Unread, letter.Kinah = false, 0
	if err := s.UpdateLetter(letter); err != nil {
		t.Fatal(err)
	}
	if letters, err := s.Letters(b); err != nil || len(letters) == 0 || letters[len(letters)-1].Unread {
		t.Fatalf("letters: %v, %v", letters, err)
	}
	if err := s.DeleteLetter(letter.ID); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM players WHERE id = ?`, a).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if c, err := s.CharacterByName(name); err != nil || c == nil || c.ID != a {
		t.Fatalf("character by name: %v, %v", c, err)
	}
	if err := s.SetMailboxLetters(a, 0); err != nil {
		t.Fatal(err)
	}
	// The broker, and macros.
	row := &BrokerRow{Pointer: 2000000003, ItemID: 160000001, Count: 3, Seller: "Seller", SellerID: a, Price: 50, Race: "ELYOS",
		Expire: time.Now().Add(time.Hour).Truncate(time.Second), Settle: time.Now().Truncate(time.Second)}
	t.Cleanup(func() { s.DeleteBroker(row) })
	if err := s.InsertBroker(row); err != nil {
		t.Fatal(err)
	}
	row.Sold = true
	if err := s.UpdateBroker(row); err != nil {
		t.Fatal(err)
	}
	if rows, _, err := s.Broker(); err != nil || len(rows) == 0 {
		t.Fatalf("broker: %v, %v", rows, err)
	}
	if err := s.DeleteBroker(row); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DeleteMacro(a, 250) })
	if err := s.AddMacro(a, 250, "<macro/>"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteMacro(a, 250); err != nil {
		t.Fatal(err)
	}
	// Legions.
	const legionID = 2000000002
	t.Cleanup(func() { s.DeleteLegion(legionID) })
	if err := s.InsertLegion(legionID, "TestLegionX"); err != nil {
		t.Fatal(err)
	}
	if used, err := s.LegionNameUsed("TestLegionX"); err != nil || !used {
		t.Fatalf("name used: %v, %v", used, err)
	}
	if err := s.InsertLegionMember(legionID, a, "BRIGADE_GENERAL"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateLegionMember(a, "nick", "BRIGADE_GENERAL", "hello"); err != nil {
		t.Fatal(err)
	}
	if m, err := s.LegionMemberOf(a); err != nil || m == nil || m.LegionID != legionID || m.Nickname != "nick" || m.Name == "" {
		t.Fatalf("member: %+v, %v", m, err)
	}
	if members, err := s.LegionMembers(legionID); err != nil || len(members) != 1 {
		t.Fatalf("members: %v, %v", members, err)
	}
	if err := s.InsertAnnouncement(legionID, "Hi", time.Now()); err != nil {
		t.Fatal(err)
	}
	if notices, err := s.LegionAnnouncements(legionID); err != nil || len(notices) != 1 {
		t.Fatalf("notices: %v, %v", notices, err)
	}
	if err := s.InsertHistory(legionID, HistoryEntry{Type: "JOIN", Name: "Botty", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if h, err := s.LegionHistory(legionID); err != nil || len(h) != 1 || h[0].Name != "Botty" {
		t.Fatalf("history: %v, %v", h, err)
	}
	l, err := s.LegionByID(legionID)
	if err != nil || l == nil || l.Name != "TestLegionX" {
		t.Fatalf("legion: %+v, %v", l, err)
	}
	l.Level = 2
	if err := s.UpdateLegion(l); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLegionMember(a); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLegion(legionID); err != nil {
		t.Fatal(err)
	}
	if items, err := s.Items(a, 1, false); err != nil {
		t.Fatalf("warehouse items: %v (%d)", err, len(items))
	}
}
