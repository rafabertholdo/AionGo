package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const (
	maxAdminItemCount = math.MaxInt32
	adminItemIDFloor  = 2_000_000
	adminItemIDStart  = 3_000_000
	adminItemSlot     = 65535
	maxCubeExpansion  = 9
)

var (
	adminItemIDMu sync.Mutex
	adminItemID   int64 = adminItemIDStart
)

func characterToken(u *user) string {
	mac := hmac.New(sha256.New, sessionKey)
	fmt.Fprintf(mac, "character-edit:%d", u.ID)
	return hex.EncodeToString(mac.Sum(nil))
}

type ascensionPreparation struct {
	firstQuest, quest, variable, world int
	x, y, z                            float64
	npc                                string
}

func prepareAscension(race, class, schema string) (ascensionPreparation, error) {
	switch class {
	case "WARRIOR", "SCOUT", "MAGE", "PRIEST":
	default:
		return ascensionPreparation{}, errors.New("This character already has an advanced class. Ascension preparation requires a base class.")
	}
	switch race {
	case "ELYOS":
		return ascensionPreparation{1000, 1006, 3, 210010000, 242, 1638, 100, "Pernos"}, nil
	case "ASMODIANS":
		if schema != "au_server_gs_java" {
			return ascensionPreparation{}, errors.New("Asmodian Ascension is not implemented on the Go server yet.")
		}
		return ascensionPreparation{2000, 2008, 4, 220010000, 380, 1895, 330, "Munin"}, nil
	default:
		return ascensionPreparation{}, errors.New("Unsupported character race.")
	}
}

func (a *assets) experienceForLevel(level int) (int64, error) {
	if level < 1 || level > len(a.exp) || level > 50 {
		return 0, errors.New("Choose a level from 1 to 50. The server's experience table must be installed.")
	}
	return a.exp[level-1], nil
}

func characterBack(w http.ResponseWriter, r *http.Request, name, message string, failed bool) {
	target := "/character?name=" + template.URLQueryEscaper(name) + "&m=" + template.URLQueryEscaper(message)
	if failed {
		target += "&e"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// editCharacter requires offline characters for inventory and progression edits.
// Account access changes take effect at the next account login.
func (p *panel) editCharacter(u *user, w http.ResponseWriter, r *http.Request) {
	if !u.Admin() || !hmac.Equal([]byte(r.FormValue("token")), []byte(characterToken(u))) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	action := r.FormValue("action")
	var message string
	var err error
	switch action {
	case "promote_admin", "remove_gm":
		level := adminLevel
		if action == "remove_gm" {
			level = 0
		}
		err = p.setCharacterAccess(r, u, name, level)
		message = fmt.Sprintf("Account access set to %d for all characters on this account. Sign out of the game completely and log back in to apply the change.", level)
	case "level", "ascension":
		var level int
		level, err = strconv.Atoi(r.FormValue("level"))
		if action == "ascension" {
			level, err = 9, nil
		}
		if err != nil {
			characterBack(w, r, name, "Choose a valid level.", true)
			return
		}
		var exp int64
		exp, err = p.assets.experienceForLevel(level)
		if err == nil {
			err = p.updateCharacter(r, name, action, exp)
		}
		message = fmt.Sprintf("Level set to %d. Log in to apply the change.", level)
		if action == "ascension" {
			message = "Ready for Ascension: level 9, previous campaign quests completed. Log in and speak to Pernos (Elyos) or Munin (Asmodian) to enter the Ascension trial instance. Complete the trial there to choose your class. Quest rewards were not granted."
		}
	case "kinah":
		var amount int64
		amount, err = positiveAmount(r.FormValue("amount"), math.MaxInt64)
		if err == nil {
			err = p.grantKinah(r, name, amount)
		}
		message = fmt.Sprintf("Added %s kinah. Log in to apply the change.", commas(amount))
	case "item":
		var itemID int64
		itemID, err = strconv.ParseInt(r.FormValue("item_id"), 10, 32)
		var amount int64
		if err == nil {
			amount, err = positiveAmount(r.FormValue("amount"), maxAdminItemCount)
		}
		var item itemInfo
		if err == nil {
			item, err = p.grantItem(r, name, int32(itemID), amount)
		}
		itemName := item.Name
		if itemName == "" && itemID > 0 {
			itemName = fmt.Sprintf("item %d", itemID)
		}
		message = fmt.Sprintf("Granted %s × %s. Log in to apply the change.", commas(amount), itemName)
	case "set":
		var set gearSetInfo
		set, err = p.grantGearSet(r, name, r.FormValue("set_id"))
		message = fmt.Sprintf("Granted %s. Log in to apply the change.", set.Name)
	case "expand_cube":
		var alreadyExpanded bool
		alreadyExpanded, err = p.expandCube(r, name)
		message = "Expanded the character's cube to 108 slots. Log in to apply the change."
		if alreadyExpanded {
			message = "This character's cube is already fully expanded."
		}
	default:
		http.Error(w, "unknown character action", http.StatusBadRequest)
		return
	}
	if err != nil {
		p.log.Warn("character edit rejected", "by", u.Name, "character", name, "err", err)
		characterBack(w, r, name, err.Error(), true)
		return
	}
	p.log.Info("character edited", "by", u.Name, "character", name, "action", action)
	characterBack(w, r, name, message, false)
}

func positiveAmount(raw string, maximum int64) (int64, error) {
	amount, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || amount < 1 || amount > maximum {
		return 0, fmt.Errorf("Choose an amount from 1 to %s.", commas(maximum))
	}
	return amount, nil
}

type characterEditRecord struct {
	id       int32
	race     string
	class    string
	cubeSize int
}

func (p *panel) lockOfflineCharacter(ctx context.Context, tx *sql.Tx, name string) (characterEditRecord, error) {
	var record characterEditRecord
	var online bool
	err := tx.QueryRowContext(ctx, `SELECT id, race, player_class, online, cube_size FROM `+p.gsDB+
		`.players WHERE name = ? AND deletion_date IS NULL FOR UPDATE`, name).
		Scan(&record.id, &record.race, &record.class, &online, &record.cubeSize)
	if errors.Is(err, sql.ErrNoRows) {
		return characterEditRecord{}, errors.New("Character not found.")
	}
	if err != nil {
		p.log.Error("reading character for edit", "err", err)
		return characterEditRecord{}, errors.New("Unable to load the character.")
	}
	if online {
		return characterEditRecord{}, errors.New("Log out of this character before making changes.")
	}
	return record, nil
}

func (p *panel) grantKinah(r *http.Request, name string, amount int64) error {
	if amount < 1 {
		return errors.New("Choose a positive kinah amount.")
	}
	tx, err := p.db.BeginTx(r.Context(), nil)
	if err != nil {
		return errors.New("Unable to save the character.")
	}
	defer tx.Rollback()
	record, err := p.lockOfflineCharacter(r.Context(), tx, name)
	if err != nil {
		return err
	}
	var itemID int32
	var current int64
	err = tx.QueryRowContext(r.Context(), `SELECT itemUniqueId, itemCount FROM `+p.gsDB+
		`.inventory WHERE itemOwner = ? AND itemId = ? AND isEquiped = 0 AND COALESCE(itemLocation, 0) = 0 ORDER BY itemUniqueId LIMIT 1 FOR UPDATE`,
		record.id, kinahID).Scan(&itemID, &current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return p.editDatabaseError(err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		ids, idErr := p.reserveAdminItemIDs(r.Context(), tx, 1)
		if idErr != nil {
			return idErr
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO `+p.gsDB+`.inventory
			(itemUniqueId, itemId, itemCount, itemColor, itemOwner, isEquiped, isSoulBound, slot, itemLocation, enchant, itemSkin, fusionedItem)
			VALUES (?, ?, ?, 0, ?, 0, 0, ?, 0, 0, 0, 0)`, ids[0], kinahID, amount, record.id, adminItemSlot); err != nil {
			return p.editDatabaseError(err)
		}
	} else {
		if current > math.MaxInt64-amount {
			return errors.New("The character's kinah balance is too high for that grant.")
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE `+p.gsDB+`.inventory SET itemCount = ? WHERE itemUniqueId = ?`, current+amount, itemID); err != nil {
			return p.editDatabaseError(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return p.editDatabaseError(err)
	}
	return nil
}

type itemStackUpdate struct {
	id    int32
	count int64
}

func (p *panel) grantItem(r *http.Request, name string, itemID int32, amount int64) (itemInfo, error) {
	item, ok := p.assets.items[itemID]
	if !ok || itemID == kinahID {
		return itemInfo{}, errors.New("Choose an item from the search results.")
	}
	if amount < 1 || amount > maxAdminItemCount {
		return itemInfo{}, fmt.Errorf("Choose an item quantity from 1 to %s.", commas(maxAdminItemCount))
	}
	maxStack := item.MaxStack
	if maxStack < 1 {
		maxStack = 1
	}

	tx, err := p.db.BeginTx(r.Context(), nil)
	if err != nil {
		return itemInfo{}, errors.New("Unable to save the character.")
	}
	defer tx.Rollback()
	record, err := p.lockOfflineCharacter(r.Context(), tx, name)
	if err != nil {
		return itemInfo{}, err
	}
	var usedSlots int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM `+p.gsDB+
		`.inventory WHERE itemOwner = ? AND isEquiped = 0 AND COALESCE(itemLocation, 0) = 0 AND itemId <> ?`,
		record.id, kinahID).Scan(&usedSlots); err != nil {
		return itemInfo{}, p.editDatabaseError(err)
	}

	var updates []itemStackUpdate
	remaining := amount
	if maxStack > 1 {
		rows, queryErr := tx.QueryContext(r.Context(), `SELECT itemUniqueId, itemCount FROM `+p.gsDB+
			`.inventory WHERE itemOwner = ? AND itemId = ? AND isEquiped = 0 AND COALESCE(itemLocation, 0) = 0 ORDER BY itemUniqueId FOR UPDATE`,
			record.id, itemID)
		if queryErr != nil {
			return itemInfo{}, p.editDatabaseError(queryErr)
		}
		for rows.Next() {
			var existing itemStackUpdate
			if err = rows.Scan(&existing.id, &existing.count); err != nil {
				rows.Close()
				return itemInfo{}, p.editDatabaseError(err)
			}
			free := maxStack - existing.count
			if free <= 0 || remaining == 0 {
				continue
			}
			added := min(free, remaining)
			existing.count += added
			remaining -= added
			updates = append(updates, existing)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return itemInfo{}, p.editDatabaseError(err)
		}
		if err = rows.Close(); err != nil {
			return itemInfo{}, p.editDatabaseError(err)
		}
	}
	newStacks := remaining / maxStack
	if remaining%maxStack > 0 {
		newStacks++
	}
	limit := baseCubeSlots + record.cubeSize*9
	freeSlots := limit - usedSlots
	if freeSlots < 0 {
		freeSlots = 0
	}
	if newStacks > int64(freeSlots) {
		return itemInfo{}, fmt.Errorf("The character's cube has room for %d more item stacks.", freeSlots)
	}
	for _, update := range updates {
		if _, err = tx.ExecContext(r.Context(), `UPDATE `+p.gsDB+`.inventory SET itemCount = ? WHERE itemUniqueId = ?`, update.count, update.id); err != nil {
			return itemInfo{}, p.editDatabaseError(err)
		}
	}
	if newStacks > 0 {
		ids, idErr := p.reserveAdminItemIDs(r.Context(), tx, int(newStacks))
		if idErr != nil {
			return itemInfo{}, idErr
		}
		for _, uniqueID := range ids {
			stackCount := min(remaining, maxStack)
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO `+p.gsDB+`.inventory
				(itemUniqueId, itemId, itemCount, itemColor, itemOwner, isEquiped, isSoulBound, slot, itemLocation, enchant, itemSkin, fusionedItem)
				VALUES (?, ?, ?, 0, ?, 0, 0, ?, 0, 0, 0, 0)`, uniqueID, itemID, stackCount, record.id, adminItemSlot); err != nil {
				return itemInfo{}, p.editDatabaseError(err)
			}
			remaining -= stackCount
		}
	}
	if err = tx.Commit(); err != nil {
		return itemInfo{}, p.editDatabaseError(err)
	}
	return item, nil
}

func (p *panel) grantGearSet(r *http.Request, name, key string) (gearSetInfo, error) {
	var set gearSetInfo
	for _, candidate := range p.assets.sets {
		if candidate.Key == key {
			set = candidate
			break
		}
	}
	if set.Key == "" || len(set.ItemIDs) == 0 {
		return gearSetInfo{}, errors.New("Choose a gear set from the list.")
	}
	for _, itemID := range set.ItemIDs {
		if _, ok := p.assets.items[itemID]; !ok || itemID == kinahID {
			return gearSetInfo{}, errors.New("This gear set is missing item data. Refresh the panel assets before granting it.")
		}
	}

	tx, err := p.db.BeginTx(r.Context(), nil)
	if err != nil {
		return gearSetInfo{}, errors.New("Unable to save the character.")
	}
	defer tx.Rollback()
	record, err := p.lockOfflineCharacter(r.Context(), tx, name)
	if err != nil {
		return gearSetInfo{}, err
	}
	var usedSlots int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM `+p.gsDB+
		`.inventory WHERE itemOwner = ? AND isEquiped = 0 AND COALESCE(itemLocation, 0) = 0 AND itemId <> ?`,
		record.id, kinahID).Scan(&usedSlots); err != nil {
		return gearSetInfo{}, p.editDatabaseError(err)
	}
	freeSlots := baseCubeSlots + record.cubeSize*9 - usedSlots
	if freeSlots < len(set.ItemIDs) {
		return gearSetInfo{}, fmt.Errorf("The character's cube needs %d free slots for %s; it has %d.", len(set.ItemIDs), set.Name, max(freeSlots, 0))
	}
	ids, err := p.reserveAdminItemIDs(r.Context(), tx, len(set.ItemIDs))
	if err != nil {
		return gearSetInfo{}, err
	}
	for i, itemID := range set.ItemIDs {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO `+p.gsDB+`.inventory
			(itemUniqueId, itemId, itemCount, itemColor, itemOwner, isEquiped, isSoulBound, slot, itemLocation, enchant, itemSkin, fusionedItem)
			VALUES (?, ?, 1, 0, ?, 0, 0, ?, 0, 0, 0, 0)`, ids[i], itemID, record.id, adminItemSlot); err != nil {
			return gearSetInfo{}, p.editDatabaseError(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return gearSetInfo{}, p.editDatabaseError(err)
	}
	return set, nil
}

func (p *panel) expandCube(r *http.Request, name string) (bool, error) {
	tx, err := p.db.BeginTx(r.Context(), nil)
	if err != nil {
		return false, errors.New("Unable to save the character.")
	}
	defer tx.Rollback()
	record, err := p.lockOfflineCharacter(r.Context(), tx, name)
	if err != nil {
		return false, err
	}
	if record.cubeSize >= maxCubeExpansion {
		return true, nil
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE `+p.gsDB+`.players SET cube_size = ? WHERE id = ?`, maxCubeExpansion, record.id); err != nil {
		return false, p.editDatabaseError(err)
	}
	if err = tx.Commit(); err != nil {
		return false, p.editDatabaseError(err)
	}
	return false, nil
}

func (p *panel) reserveAdminItemIDs(ctx context.Context, tx *sql.Tx, count int) ([]int32, error) {
	// Stay above normal startup object IDs without forcing Java's BitSet allocator to grow near int32's limit.
	adminItemIDMu.Lock()
	defer adminItemIDMu.Unlock()
	ids := make([]int32, 0, count)
	for len(ids) < count {
		if adminItemID < adminItemIDFloor {
			return nil, errors.New("Unable to allocate a unique item ID.")
		}
		candidate := int32(adminItemID)
		adminItemID--
		var exists int32
		err := tx.QueryRowContext(ctx, `SELECT itemUniqueId FROM `+p.gsDB+`.inventory WHERE itemUniqueId = ?`, candidate).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			ids = append(ids, candidate)
			continue
		}
		if err != nil {
			return nil, p.editDatabaseError(err)
		}
	}
	return ids, nil
}

func (p *panel) updateCharacter(r *http.Request, name, action string, exp int64) error {
	tx, err := p.db.BeginTx(r.Context(), nil)
	if err != nil {
		return errors.New("Unable to save the character.")
	}
	defer tx.Rollback()
	record, err := p.lockOfflineCharacter(r.Context(), tx, name)
	if err != nil {
		return err
	}
	if action == "ascension" {
		plan, err := prepareAscension(record.race, record.class, p.gsDB)
		if err != nil {
			return err
		}
		for quest := plan.firstQuest; quest <= plan.quest; quest++ {
			status, variable, count := "COMPLETE", 0, 1
			if quest == plan.quest {
				status, variable, count = "START", plan.variable, 0
			}
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO `+p.gsDB+`.player_quests (player_id, quest_id, status, quest_vars, complete_count) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE status = VALUES(status), quest_vars = VALUES(quest_vars), complete_count = VALUES(complete_count)`, record.id, quest, status, variable, count); err != nil {
				return p.editDatabaseError(err)
			}
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE `+p.gsDB+`.players SET world_id = ?, x = ?, y = ?, z = ?, heading = 0 WHERE id = ?`, plan.world, plan.x, plan.y, plan.z, record.id); err != nil {
			return p.editDatabaseError(err)
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE `+p.gsDB+`.players SET exp = ?, recoverexp = 0 WHERE id = ?`, exp, record.id); err != nil {
		return p.editDatabaseError(err)
	}
	if err = tx.Commit(); err != nil {
		return p.editDatabaseError(err)
	}
	return nil
}

func (p *panel) editDatabaseError(err error) error {
	p.log.Error("saving character edit", "err", err)
	return errors.New("Unable to save the character. No changes were applied.")
}

// GM permissions come from the login account, not the character row.
func (p *panel) setCharacterAccess(r *http.Request, u *user, name string, level int) error {
	if !u.Admin() || (level != 0 && level != adminLevel) || level > u.AccessLevel {
		return errors.New("You cannot assign that access level.")
	}
	tx, err := p.db.BeginTx(r.Context(), nil)
	if err != nil {
		return p.editDatabaseError(err)
	}
	defer tx.Rollback()
	var accountID int32
	err = tx.QueryRowContext(r.Context(), `SELECT account_id FROM `+p.gsDB+
		`.players WHERE name = ? AND deletion_date IS NULL`, name).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("Character not found.")
	}
	if err != nil {
		return p.editDatabaseError(err)
	}
	var current int
	err = tx.QueryRowContext(r.Context(), `SELECT access_level FROM account_data WHERE id = ? FOR UPDATE`, accountID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("Character account not found.")
	}
	if err != nil {
		return p.editDatabaseError(err)
	}
	if accountID == u.ID {
		return errors.New("You cannot change your own account's access here.")
	}
	if current > u.AccessLevel {
		return errors.New("You cannot change an account with a higher access level than yours.")
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE account_data SET access_level = ? WHERE id = ?`, level, accountID); err != nil {
		return p.editDatabaseError(err)
	}
	if err := tx.Commit(); err != nil {
		return p.editDatabaseError(err)
	}
	return nil
}
