package login

import (
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

// Account is a row of au_server_ls.account_data, with its account_time row.
type Account struct {
	ID           int32
	Name         string
	PasswordHash string
	AccessLevel  byte
	Membership   byte
	Activated    byte
	LastServer   byte
	LastIP       string
	IPForce      string
	Time         *AccountTime
}

// AccountTime tracks play and rest time, account expiry and bans. Durations are milliseconds.
type AccountTime struct {
	LastActive        time.Time
	Expiration        *time.Time
	SessionDuration   int64
	AccumulatedOnline int64
	AccumulatedRest   int64
	PenaltyEnd        *time.Time
}

// permanentPenalty is how a permanent ban is stored: one second after the epoch.
var permanentPenalty = time.UnixMilli(1000)

// GameServerRow is a registered game server: the id it logs in with, the IPs
// it may connect from, and its password.
type GameServerRow struct {
	ID       byte
	Mask     string
	Password string
}

// Ban is an IP ban; a nil End never ends.
type Ban struct {
	Mask string
	End  *time.Time
}

func (b Ban) active(now time.Time) bool {
	return b.End == nil || b.End.After(now)
}

// Store is the login server's database: au_server_ls, as AL-Login keeps it.
type Store interface {
	Account(name string) (*Account, error)
	CreateAccount(name, passwordHash string) (*Account, error)
	UpdateAccess(account *Account) error
	UpdateLastServer(id int32, server byte) error
	UpdateLastIP(id int32, ip string) error
	LastIP(id int32) (string, error)
	AccountTime(id int32) (*AccountTime, error)
	SaveAccountTime(id int32, t *AccountTime) error
	GameServers() ([]GameServerRow, error)
	Bans() ([]Ban, error)
	AddBan(mask string, end *time.Time) error
	RemoveBan(mask string) error
}

// HashPassword is AL-Login's password hash: SHA-1 of the UTF-8 password, in standard Base64.
func HashPassword(password string) string {
	sum := sha1.Sum([]byte(password))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// SQLStore is Store on MariaDB or MySQL.
type SQLStore struct{ DB *sql.DB }

func (s SQLStore) Account(name string) (*Account, error) {
	a := &Account{}
	var lastIP, ipForce sql.NullString
	err := s.DB.QueryRow(`SELECT id, name, password, access_level, membership, activated, last_server, last_ip, ip_force
		FROM account_data WHERE name = ?`, name).
		Scan(&a.ID, &a.Name, &a.PasswordHash, &a.AccessLevel, &a.Membership, &a.Activated, &a.LastServer, &lastIP, &ipForce)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.LastIP, a.IPForce = lastIP.String, ipForce.String
	a.Time, err = s.AccountTime(a.ID)
	return a, err
}

func (s SQLStore) CreateAccount(name, passwordHash string) (*Account, error) {
	result, err := s.DB.Exec(`INSERT INTO account_data (name, password, access_level, membership, activated, last_server, last_ip, ip_force)
		VALUES (?, ?, 0, 0, 1, 0, NULL, NULL)`, name, passwordHash)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Account{ID: int32(id), Name: name, PasswordHash: passwordHash, Activated: 1}, nil
}

func (s SQLStore) UpdateAccess(a *Account) error {
	_, err := s.DB.Exec(`UPDATE account_data SET access_level = ?, membership = ? WHERE id = ?`, a.AccessLevel, a.Membership, a.ID)
	return err
}

func (s SQLStore) UpdateLastServer(id int32, server byte) error {
	_, err := s.DB.Exec(`UPDATE account_data SET last_server = ? WHERE id = ?`, int8(server), id)
	return err
}

func (s SQLStore) UpdateLastIP(id int32, ip string) error {
	_, err := s.DB.Exec(`UPDATE account_data SET last_ip = ? WHERE id = ?`, ip, id)
	return err
}

func (s SQLStore) LastIP(id int32) (string, error) {
	var ip sql.NullString
	err := s.DB.QueryRow(`SELECT last_ip FROM account_data WHERE id = ?`, id).Scan(&ip)
	return ip.String, err
}

func (s SQLStore) AccountTime(id int32) (*AccountTime, error) {
	t := &AccountTime{}
	var expiration, penalty sql.NullTime
	var session, online, rest sql.NullInt64
	err := s.DB.QueryRow(`SELECT last_active, expiration_time, session_duration, accumulated_online, accumulated_rest, penalty_end
		FROM account_time WHERE account_id = ?`, id).
		Scan(&t.LastActive, &expiration, &session, &online, &rest, &penalty)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.SessionDuration, t.AccumulatedOnline, t.AccumulatedRest = session.Int64, online.Int64, rest.Int64
	if expiration.Valid {
		t.Expiration = &expiration.Time
	}
	if penalty.Valid {
		t.PenaltyEnd = &penalty.Time
	}
	return t, nil
}

func (s SQLStore) SaveAccountTime(id int32, t *AccountTime) error {
	_, err := s.DB.Exec(`REPLACE INTO account_time
		(account_id, last_active, expiration_time, session_duration, accumulated_online, accumulated_rest, penalty_end)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, t.LastActive, t.Expiration, t.SessionDuration, t.AccumulatedOnline, t.AccumulatedRest, t.PenaltyEnd)
	return err
}

func (s SQLStore) GameServers() ([]GameServerRow, error) {
	rows, err := s.DB.Query(`SELECT id, mask, password FROM gameservers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []GameServerRow
	for rows.Next() {
		var g GameServerRow
		if err := rows.Scan(&g.ID, &g.Mask, &g.Password); err != nil {
			return nil, err
		}
		servers = append(servers, g)
	}
	return servers, rows.Err()
}

func (s SQLStore) Bans() ([]Ban, error) {
	rows, err := s.DB.Query(`SELECT mask, time_end FROM banned_ip`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bans []Ban
	for rows.Next() {
		var b Ban
		var end sql.NullTime
		if err := rows.Scan(&b.Mask, &end); err != nil {
			return nil, err
		}
		if end.Valid {
			b.End = &end.Time
		}
		bans = append(bans, b)
	}
	return bans, rows.Err()
}

func (s SQLStore) AddBan(mask string, end *time.Time) error {
	_, err := s.DB.Exec(`INSERT INTO banned_ip (mask, time_end) VALUES (?, ?)`, mask, end)
	return err
}

func (s SQLStore) RemoveBan(mask string) error {
	_, err := s.DB.Exec(`DELETE FROM banned_ip WHERE mask = ?`, mask)
	return err
}
