package store

import (
	"database/sql"
	"time"
)

// Letter is a mail row.
type Letter struct {
	ID, Recipient int32
	Sender, Title string
	Message       string
	Unread        bool
	ItemID        int32 // the unique id of the attached item, or 0
	Kinah         int64
	Express       bool
	Received      time.Time
}

// MailboxLocation is the location of the items attached to letters.
const MailboxLocation = 127

// Letters is a player's mailbox.
func (s Store) Letters(recipient int32) ([]Letter, error) {
	return queryAll(s.DB, `SELECT mailUniqueId, mailRecipientId, senderName, mailTitle, mailMessage, unread, attachedItemId,
		attachedKinahCount, express, recievedTime FROM mail WHERE mailRecipientId = ?`, []any{recipient},
		func(r *sql.Rows) (Letter, error) {
			var l Letter
			var received sql.NullTime
			err := r.Scan(&l.ID, &l.Recipient, &l.Sender, &l.Title, &l.Message, &l.Unread, &l.ItemID, &l.Kinah, &l.Express, &received)
			l.Received = received.Time
			return l, err
		})
}

// InsertLetter stores a new letter.
func (s Store) InsertLetter(l *Letter) error {
	_, err := s.DB.Exec(`INSERT INTO mail (mailUniqueId, mailRecipientId, senderName, mailTitle, mailMessage, unread, attachedItemId,
		attachedKinahCount, express, recievedTime) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.Recipient, l.Sender, l.Title, l.Message, l.Unread, l.ItemID, l.Kinah, l.Express, l.Received)
	return err
}

// UpdateLetter saves what changes in a letter: read, and what is still attached.
func (s Store) UpdateLetter(l *Letter) error {
	_, err := s.DB.Exec(`UPDATE mail SET unread = ?, attachedItemId = ?, attachedKinahCount = ?, recievedTime = ? WHERE mailUniqueId = ?`,
		l.Unread, l.ItemID, l.Kinah, l.Received, l.ID)
	return err
}

// DeleteLetter removes a letter.
func (s Store) DeleteLetter(id int32) error {
	_, err := s.DB.Exec(`DELETE FROM mail WHERE mailUniqueId = ?`, id)
	return err
}

// CharacterByName is the character with the name, or nil.
func (s Store) CharacterByName(name string) (*Character, error) {
	c, err := scanCharacter(s.DB.QueryRow(`SELECT `+characterColumns+` FROM players WHERE name = ?`, name))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// SetMailboxLetters is how many letters a player who is away has waiting.
func (s Store) SetMailboxLetters(playerID int32, n int) error {
	_, err := s.DB.Exec(`UPDATE players SET mailboxLetters = ? WHERE id = ?`, n, playerID)
	return err
}
