package store

// AutoAnnouncement is an announcements row: a message the server repeats every Delay seconds.
type AutoAnnouncement struct {
	ID      int32
	Text    string
	Faction string // ALL, ASMODIANS or ELYOS
	Type    string // ANNOUNCE, SHOUT, ORANGE, YELLOW or NORMAL
	Delay   int32
}

// AutoAnnouncements is AnnouncementsDAO.getAnnouncements.
func (s Store) AutoAnnouncements() ([]AutoAnnouncement, error) {
	rows, err := s.DB.Query("SELECT id, announce, faction, `type`, `delay` FROM announcements ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []AutoAnnouncement
	for rows.Next() {
		var a AutoAnnouncement
		if err := rows.Scan(&a.ID, &a.Text, &a.Faction, &a.Type, &a.Delay); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}
