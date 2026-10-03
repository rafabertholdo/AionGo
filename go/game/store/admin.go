package store

import "database/sql"

// Bookmark is a player's saved teleport destination.
type Bookmark struct {
	Name    string
	WorldID int32
	X, Y, Z float32
}

func (s Store) AddTitle(playerID, titleID int32) error {
	_, err := s.DB.Exec("INSERT IGNORE INTO player_titles (player_id, title_id) VALUES (?, ?)", playerID, titleID)
	return err
}
func (s Store) AddDrop(npcID int32, d Drop) error {
	_, err := s.DB.Exec("INSERT INTO droplist (mobId,itemId,`min`,`max`,chance) VALUES (?,?,?,?,?)", npcID, d.ItemID, d.Min, d.Max, d.Chance)
	return err
}
func (s Store) Bookmarks(playerID int32) ([]Bookmark, error) {
	return queryAll(s.DB, "SELECT name,world_id,x,y,z FROM bookmark WHERE char_id=? ORDER BY name", []any{playerID}, func(r *sql.Rows) (Bookmark, error) {
		var b Bookmark
		err := r.Scan(&b.Name, &b.WorldID, &b.X, &b.Y, &b.Z)
		return b, err
	})
}
func (s Store) AddBookmark(playerID int32, b Bookmark) error {
	_, err := s.DB.Exec("INSERT INTO bookmark (name,char_id,world_id,x,y,z) VALUES (?,?,?,?,?,?)", b.Name, playerID, b.WorldID, b.X, b.Y, b.Z)
	return err
}
func (s Store) DeleteBookmark(playerID int32, name string) error {
	_, err := s.DB.Exec("DELETE FROM bookmark WHERE char_id=? AND name=?", playerID, name)
	return err
}
func (s Store) SaveAnnouncement(a AutoAnnouncement) error {
	_, err := s.DB.Exec("INSERT INTO announcements (announce,faction,`type`,`delay`) VALUES (?,?,?,?)", a.Text, a.Faction, a.Type, a.Delay)
	return err
}
func (s Store) DeleteAnnouncement(id int32) error {
	_, err := s.DB.Exec("DELETE FROM announcements WHERE id=?", id)
	return err
}
func (s Store) LegionIDByName(name string) (int32, error) {
	var id int32
	err := s.DB.QueryRow("SELECT id FROM legions WHERE name=?", name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}
