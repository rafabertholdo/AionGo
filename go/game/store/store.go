// Package store is the game server's database, au_server_gs, read and written
// the way AL-Game's MySQL5 DAO scripts do.
package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Store is au_server_gs.
type Store struct{ DB *sql.DB }

// Character is a players row.
type Character struct {
	ID            int32
	Name          string
	AccountID     int32
	AccountName   string
	Exp           int64
	RecoverExp    int64
	X, Y, Z       float32
	Heading       int32
	WorldID       int32
	Gender        string // MALE, FEMALE
	Race          string // ELYOS, ASMODIANS
	Class         string // WARRIOR, GLADIATOR, …
	Created       time.Time
	Deletion      *time.Time
	LastOnline    time.Time
	CubeSize      int
	StigmaSlots   int
	WarehouseSize int
	Letters       int
	BindPoint     int32
	TitleID       int32
	Note          string
}

// Appearance is a player_appearance row, in the table's column order.
type Appearance struct {
	Face, Hair, Deco, Tattoo                                                   int32
	SkinRGB, HairRGB, LipRGB, EyeRGB                                           int32
	FaceShape, Forehead, EyeHeight, EyeSpace, EyeWidth, EyeSize, EyeShape      int32
	EyeAngle, BrowHeight, BrowAngle, BrowShape, Nose, NoseBridge, NoseWidth    int32
	NoseTip, Cheek, LipHeight, MouthSize, LipSize, Smile, LipShape, JawHeight  int32
	ChinJut, EarShape, HeadSize, Neck, NeckLength, Shoulders, ShoulderSize     int32
	Torso, Chest, Waist, Hips, ArmThickness, ArmLength, HandSize, LegThickness int32
	LegLength, FootSize, FacialRate, Voice                                     int32
	Height                                                                     float32
}

func (a *Appearance) fields() []any {
	return []any{&a.Face, &a.Hair, &a.Deco, &a.Tattoo, &a.SkinRGB, &a.HairRGB, &a.LipRGB, &a.EyeRGB,
		&a.FaceShape, &a.Forehead, &a.EyeHeight, &a.EyeSpace, &a.EyeWidth, &a.EyeSize, &a.EyeShape,
		&a.EyeAngle, &a.BrowHeight, &a.BrowAngle, &a.BrowShape, &a.Nose, &a.NoseBridge, &a.NoseWidth,
		&a.NoseTip, &a.Cheek, &a.LipHeight, &a.MouthSize, &a.LipSize, &a.Smile, &a.LipShape, &a.JawHeight,
		&a.ChinJut, &a.EarShape, &a.HeadSize, &a.Neck, &a.NeckLength, &a.Shoulders, &a.ShoulderSize,
		&a.Torso, &a.Chest, &a.Waist, &a.Hips, &a.ArmThickness, &a.ArmLength, &a.HandSize, &a.LegThickness,
		&a.LegLength, &a.FootSize, &a.FacialRate, &a.Voice, &a.Height}
}

const appearanceColumns = `face, hair, deco, tattoo, skin_rgb, hair_rgb, lip_rgb, eye_rgb, face_shape,
	forehead, eye_height, eye_space, eye_width, eye_size, eye_shape, eye_angle,
	brow_height, brow_angle, brow_shape, nose, nose_bridge, nose_width, nose_tip,
	cheek, lip_height, mouth_size, lip_size, smile, lip_shape, jaw_height, chin_jut,
	ear_shape, head_size, neck, neck_length, shoulders, shoulder_size, torso, chest,
	waist, hips, arm_thickness, arm_length, hand_size, leg_thickness, leg_length,
	foot_size, facial_rate, voice, height`

// Item is an inventory row.
type Item struct {
	UniqueID  int32
	ItemID    int32
	Count     int64
	Color     int32
	Owner     int32
	Equipped  bool
	SoulBound bool
	Slot      int32
	Location  int8 // 0 cube, 1 regular warehouse, 2 account warehouse, 127 mail
	Enchant   int8
	Skin      int32
	Fusioned  int32
	Godstone  int32 // item_stones category 1, slot 0; not an inventory column
}

// SkinID is the item the character appears to wear.
func (i *Item) SkinID() int32 {
	if i.Skin != 0 {
		return i.Skin
	}
	return i.ItemID
}

// Skill is a player_skills row.
type Skill struct {
	ID, Level int32
}

const characterColumns = `id, name, account_id, account_name, exp, recoverexp, x, y, z, heading, world_id,
	gender, race, player_class, creation_date, deletion_date, last_online, cube_size,
	advenced_stigma_slot_size, warehouse_size, mailboxLetters, bind_point, title_id, note`

func scanCharacter(row interface{ Scan(...any) error }) (*Character, error) {
	c := &Character{}
	var created, lastOnline, deletion sql.NullTime
	var note sql.NullString
	err := row.Scan(&c.ID, &c.Name, &c.AccountID, &c.AccountName, &c.Exp, &c.RecoverExp, &c.X, &c.Y, &c.Z,
		&c.Heading, &c.WorldID, &c.Gender, &c.Race, &c.Class, &created, &deletion, &lastOnline, &c.CubeSize,
		&c.StigmaSlots, &c.WarehouseSize, &c.Letters, &c.BindPoint, &c.TitleID, &note)
	if err != nil {
		return nil, err
	}
	c.Created, c.LastOnline, c.Note = created.Time, lastOnline.Time, note.String
	if deletion.Valid {
		c.Deletion = &deletion.Time
	}
	return c, nil
}

// Characters is an account's characters.
func (s Store) Characters(accountID int32) ([]*Character, error) {
	rows, err := s.DB.Query(`SELECT `+characterColumns+` FROM players WHERE account_id = ?`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var characters []*Character
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		characters = append(characters, c)
	}
	return characters, rows.Err()
}

// Character is one character, or nil.
func (s Store) Character(id int32) (*Character, error) {
	c, err := scanCharacter(s.DB.QueryRow(`SELECT `+characterColumns+` FROM players WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// Appearance is a character's appearance.
func (s Store) Appearance(playerID int32) (*Appearance, error) {
	a := &Appearance{}
	err := s.DB.QueryRow(`SELECT `+appearanceColumns+` FROM player_appearance WHERE player_id = ?`, playerID).Scan(a.fields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// Items is a character's items in a location (0 is the cube), equipped or not.
func (s Store) Items(owner int32, location int8, equipped bool) ([]*Item, error) {
	rows, err := s.DB.Query(`SELECT itemUniqueId, itemId, itemCount, itemColor, isEquiped, isSoulBound, slot, enchant, itemSkin, fusionedItem,
		COALESCE((SELECT itemId FROM item_stones WHERE itemUniqueId = inventory.itemUniqueId AND category = 1 AND slot = 0), 0)
		FROM inventory WHERE itemOwner = ? AND itemLocation = ? AND isEquiped = ? ORDER BY itemUniqueId`, owner, location, equipped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*Item
	for rows.Next() {
		i := &Item{Owner: owner, Location: location}
		if err := rows.Scan(&i.UniqueID, &i.ItemID, &i.Count, &i.Color, &i.Equipped, &i.SoulBound, &i.Slot, &i.Enchant, &i.Skin, &i.Fusioned, &i.Godstone); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// LegionID is the legion a character belongs to, or 0.
func (s Store) LegionID(playerID int32) (int32, error) {
	var id int32
	err := s.DB.QueryRow(`SELECT legion_id FROM legion_members WHERE player_id = ?`, playerID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// NameUsed reports whether a character already has name (compared as the database collation does).
func (s Store) NameUsed(name string) (bool, error) {
	var count int
	err := s.DB.QueryRow(`SELECT count(id) FROM players WHERE name = ?`, name).Scan(&count)
	return count > 0, err
}

// UsedIDs is every object id taken by players, items, legions and mail.
func (s Store) UsedIDs() ([]int32, error) {
	var ids []int32
	for _, query := range []string{`SELECT id FROM players`, `SELECT itemUniqueId FROM inventory`, `SELECT id FROM legions`, `SELECT mailUniqueId FROM mail`} {
		rows, err := s.DB.Query(query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int32
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
	}
	return ids, nil
}

// CreateCharacter stores a new character with its appearance, skills and items, all or nothing.
func (s Store) CreateCharacter(c *Character, a *Appearance, skills []Skill, items []*Item) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO players (id, name, account_id, account_name, x, y, z, heading, world_id, gender, race, player_class, cube_size, warehouse_size, online, creation_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		c.ID, c.Name, c.AccountID, c.AccountName, c.X, c.Y, c.Z, c.Heading, c.WorldID, c.Gender, c.Race, c.Class,
		c.CubeSize, c.WarehouseSize, c.Created); err != nil {
		return err
	}
	values := append([]any{c.ID}, a.fields()...)
	for i := range values[1:] {
		values[i+1] = deref(values[i+1])
	}
	if _, err := tx.Exec(`INSERT INTO player_appearance (player_id, `+appearanceColumns+`)
		VALUES (?`+strings.Repeat(", ?", 50)+`)`, values...); err != nil {
		return err
	}
	for _, skill := range skills {
		if _, err := tx.Exec(`INSERT INTO player_skills (player_id, skillId, skillLevel) VALUES (?, ?, ?)`, c.ID, skill.ID, skill.Level); err != nil {
			return err
		}
	}
	for _, i := range items {
		if _, err := tx.Exec(insertItem, i.row()...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const insertItem = `INSERT INTO inventory (itemUniqueId, itemId, itemCount, itemColor, itemOwner, isEquiped,
	isSoulBound, slot, itemLocation, enchant, itemSkin, fusionedItem) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func (i *Item) row() []any {
	return []any{i.UniqueID, i.ItemID, i.Count, i.Color, i.Owner, i.Equipped, i.SoulBound, i.Slot, i.Location, i.Enchant, i.Skin, i.Fusioned}
}

// InsertItem stores a new item.
func (s Store) InsertItem(i *Item) error {
	_, err := s.DB.Exec(insertItem, i.row()...)
	return err
}

// UpdateItem saves an item's count, slot and the rest that changes.
func (s Store) UpdateItem(i *Item) error {
	_, err := s.DB.Exec(`UPDATE inventory SET itemCount = ?, itemColor = ?, itemOwner = ?, isEquiped = ?, isSoulBound = ?,
		slot = ?, itemLocation = ?, enchant = ?, itemSkin = ?, fusionedItem = ? WHERE itemUniqueId = ?`,
		i.Count, i.Color, i.Owner, i.Equipped, i.SoulBound, i.Slot, i.Location, i.Enchant, i.Skin, i.Fusioned, i.UniqueID)
	return err
}

// DeleteItem removes an item, and the stones socketed in it.
func (s Store) DeleteItem(id int32) error {
	for _, query := range []string{`DELETE FROM item_stones WHERE itemUniqueId = ?`, `DELETE FROM inventory WHERE itemUniqueId = ?`} {
		if _, err := s.DB.Exec(query, id); err != nil {
			return err
		}
	}
	return nil
}

// Drop is a droplist row: what a monster may drop.
type Drop struct {
	ItemID   int32
	Min, Max int32
	Chance   float32
}

// DropList is every monster's drops, by npc id.
func (s Store) DropList() (map[int32][]Drop, error) {
	rows, err := s.DB.Query(`SELECT mobId, itemId, ` + "`min`, `max`" + `, chance FROM droplist ORDER BY Id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	drops := map[int32][]Drop{}
	for rows.Next() {
		var mob int32
		var d Drop
		if err := rows.Scan(&mob, &d.ItemID, &d.Min, &d.Max, &d.Chance); err != nil {
			return nil, err
		}
		drops[mob] = append(drops[mob], d)
	}
	return drops, rows.Err()
}

// SetDeletion schedules a character's deletion, or cancels it with nil.
func (s Store) SetDeletion(id int32, at *time.Time) error {
	_, err := s.DB.Exec(`UPDATE players SET deletion_date = ? WHERE id = ?`, at, id)
	return err
}

// DeleteCharacter removes a character and what AL-Game deletes with it: its items and life stats.
// The other tables go with it by foreign key.
func (s Store) DeleteCharacter(id int32) error {
	for _, query := range []string{
		`DELETE FROM inventory WHERE itemOwner = ? AND (itemLocation = 0 OR itemLocation = 1)`,
		`DELETE FROM player_life_stats WHERE player_id = ?`,
		`DELETE FROM players WHERE id = ?`,
	} {
		if _, err := s.DB.Exec(query, id); err != nil {
			return err
		}
	}
	return nil
}

func deref(p any) any {
	switch v := p.(type) {
	case *int32:
		return *v
	case *float32:
		return *v
	}
	return p
}
