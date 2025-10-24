package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Database struct {
	DB *sql.DB
}

type Kid struct {
	ID       int64  `json:"id"`
	UID      string `json:"uid"`
	ParentID int64  `json:"-"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Balance  int64  `json:"balance"`
	// internal fields
	HPKEPrivDER  []byte `json:"-"`
	HPKEPubDER   []byte `json:"-"`
	GridUserID   string `json:"grid_user_id,omitempty"`
	GridWalletID string `json:"grid_wallet_id,omitempty"`
}

type Parent struct {
	ID      int64
	UID     string
	Name    string
	Email   string
	Balance int64
	// The following fields are internal and should not be exposed via JSON
	HPKEPrivDER  []byte `json:"-"`
	HPKEPubDER   []byte `json:"-"`
	GridUserID   string `json:"grid_user_id,omitempty"`
	GridWalletID string `json:"grid_wallet_id,omitempty"`
}

func Open(path string) (*Database, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&cache=shared", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxIdleTime(0)
	db.SetConnMaxLifetime(0)

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Database{DB: db}, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		`CREATE TABLE IF NOT EXISTS parents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uid TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
            email TEXT UNIQUE,
			balance INTEGER NOT NULL DEFAULT 0,
            hpke_priv_der BLOB,
            hpke_pub_der BLOB,
            grid_user_id TEXT,
            grid_wallet_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS kids (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            uid TEXT UNIQUE,
            parent_id INTEGER NOT NULL,
            email TEXT UNIQUE,
            name TEXT NOT NULL,
            balance INTEGER NOT NULL DEFAULT 0,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(parent_id) REFERENCES parents(id) ON DELETE CASCADE ON UPDATE CASCADE
        );`,
		"CREATE INDEX IF NOT EXISTS idx_kids_parent_id ON kids(parent_id);",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_kids_uid ON kids(uid);",
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	// Best-effort schema evolution for older databases without the new columns
	alters := []string{
		"ALTER TABLE parents ADD COLUMN email TEXT UNIQUE;",
		"ALTER TABLE parents ADD COLUMN hpke_priv_der BLOB;",
		"ALTER TABLE parents ADD COLUMN hpke_pub_der BLOB;",
		"ALTER TABLE parents ADD COLUMN grid_user_id TEXT;",
		"ALTER TABLE parents ADD COLUMN grid_wallet_id TEXT;",
		"ALTER TABLE kids ADD COLUMN uid TEXT UNIQUE;",
		"ALTER TABLE kids ADD COLUMN email TEXT UNIQUE;",
		"ALTER TABLE kids ADD COLUMN hpke_priv_der BLOB;",
		"ALTER TABLE kids ADD COLUMN hpke_pub_der BLOB;",
		"ALTER TABLE kids ADD COLUMN grid_user_id TEXT;",
		"ALTER TABLE kids ADD COLUMN grid_wallet_id TEXT;",
	}
	for _, s := range alters {
		if _, err := db.Exec(s); err != nil {
			// ignore duplicate column errors (SQLite: "duplicate column name")
			if !isSQLiteDuplicateColumnError(err) {
				// continue on other errors to avoid breaking startup
				_ = err
			}
		}
	}
	// Backfill missing kid uids
	rows, err := db.Query(`SELECT id, IFNULL(uid, '') FROM kids`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var uid string
			if err := rows.Scan(&id, &uid); err == nil {
				if strings.TrimSpace(uid) == "" {
					_, _ = db.Exec(`UPDATE kids SET uid = ? WHERE id = ?`, uuid.NewString(), id)
				}
			}
		}
		_ = rows.Err()
	}
	return nil
}

func isSQLiteDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	// SQLite error message contains this substring when adding existing column
	return strings.Contains(err.Error(), "duplicate column name")
}

func (d *Database) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	return d.DB.Close()
}

func (d *Database) CreateParent(name string) (int64, string, error) {
	uid := uuid.NewString()
	res, err := d.DB.Exec(`INSERT INTO parents(uid, name) VALUES(?, ?)`, uid, name)
	if err != nil {
		return 0, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	return id, uid, nil
}

func (d *Database) CreateParentWithEmail(name, email string) (int64, string, error) {
	uid := uuid.NewString()
	res, err := d.DB.Exec(`INSERT INTO parents(uid, name, email) VALUES(?, ?, ?)`, uid, name, email)
	if err != nil {
		return 0, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	return id, uid, nil
}

func (d *Database) CreateKid(name, email string, parentID int64) (int64, string, error) {
	kidUID := uuid.NewString()
	res, err := d.DB.Exec(`INSERT INTO kids(uid, parent_id, email, name) VALUES(?, ?, ?, ?)`, kidUID, parentID, email, name)
	if err != nil {
		return 0, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	return id, kidUID, nil
}

func (d *Database) ListKids(parentID int64) ([]Kid, error) {
	rows, err := d.DB.Query(`SELECT id, name, balance FROM kids WHERE parent_id = ? ORDER BY id ASC`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Kid, 0)
	for rows.Next() {
		var k Kid
		if err := rows.Scan(&k.ID, &k.Name, &k.Balance); err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *Database) GetParentBalance(parentID int64) (int64, error) {
	var bal int64
	err := d.DB.QueryRow(`SELECT balance FROM parents WHERE id = ?`, parentID).Scan(&bal)
	if err != nil {
		return 0, err
	}
	return bal, nil
}

func (d *Database) GetKidBalance(kidID int64) (int64, error) {
	var bal int64
	err := d.DB.QueryRow(`SELECT balance FROM kids WHERE id = ?`, kidID).Scan(&bal)
	if err != nil {
		return 0, err
	}
	return bal, nil
}

func (d *Database) GetParentByID(id int64) (*Parent, error) {
	p := &Parent{}
	var gridUserNS, gridWalletNS sql.NullString
	err := d.DB.QueryRow(`SELECT id, uid, name, email, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id FROM parents WHERE id = ?`, id).Scan(&p.ID, &p.UID, &p.Name, &p.Email, &p.Balance, &p.HPKEPrivDER, &p.HPKEPubDER, &gridUserNS, &gridWalletNS)
	if err != nil {
		return nil, err
	}
	if gridUserNS.Valid {
		p.GridUserID = gridUserNS.String
	}
	if gridWalletNS.Valid {
		p.GridWalletID = gridWalletNS.String
	}
	return p, nil
}

func (d *Database) GetParentByName(name string) (*Parent, error) {
	p := &Parent{}
	var gridUserNS, gridWalletNS sql.NullString
	err := d.DB.QueryRow(`SELECT id, uid, name, email, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id FROM parents WHERE name = ?`, name).Scan(&p.ID, &p.UID, &p.Name, &p.Email, &p.Balance, &p.HPKEPrivDER, &p.HPKEPubDER, &gridUserNS, &gridWalletNS)
	if err != nil {
		return nil, err
	}
	if gridUserNS.Valid {
		p.GridUserID = gridUserNS.String
	}
	if gridWalletNS.Valid {
		p.GridWalletID = gridWalletNS.String
	}
	return p, nil
}

func (d *Database) GetParentByEmail(email string) (*Parent, error) {
	p := &Parent{}
	var gridUserNS, gridWalletNS sql.NullString
	err := d.DB.QueryRow(`SELECT id, uid, name, email, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id FROM parents WHERE email = ?`, email).Scan(&p.ID, &p.UID, &p.Name, &p.Email, &p.Balance, &p.HPKEPrivDER, &p.HPKEPubDER, &gridUserNS, &gridWalletNS)
	if err != nil {
		return nil, err
	}
	if gridUserNS.Valid {
		p.GridUserID = gridUserNS.String
	}
	if gridWalletNS.Valid {
		p.GridWalletID = gridWalletNS.String
	}
	return p, nil
}

func (d *Database) GetKidByID(id int64) (*Kid, error) {
	k := &Kid{}
	err := d.DB.QueryRow(`SELECT id, uid, parent_id, email, name, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id FROM kids WHERE id = ?`, id).Scan(&k.ID, &k.UID, &k.ParentID, &k.Email, &k.Name, &k.Balance, &k.HPKEPrivDER, &k.HPKEPubDER, &k.GridUserID, &k.GridWalletID)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (d *Database) GetKidByName(name string) (*Kid, error) {
	k := &Kid{}
	err := d.DB.QueryRow(`SELECT id, uid, parent_id, email, name, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id FROM kids WHERE name = ?`, name).Scan(&k.ID, &k.UID, &k.ParentID, &k.Email, &k.Name, &k.Balance, &k.HPKEPrivDER, &k.HPKEPubDER, &k.GridUserID, &k.GridWalletID)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (d *Database) GetKidByNameAndParent(name string, parentID int64) (*Kid, error) {
	k := &Kid{}
	err := d.DB.QueryRow(`SELECT id, uid, parent_id, email, name, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id FROM kids WHERE name = ? AND parent_id = ?`, name, parentID).Scan(&k.ID, &k.UID, &k.ParentID, &k.Email, &k.Name, &k.Balance, &k.HPKEPrivDER, &k.HPKEPubDER, &k.GridUserID, &k.GridWalletID)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (d *Database) GetOrCreateKidForParent(name, email string, parentID int64) (*Kid, error) {
	k, err := d.GetKidByNameAndParent(name, parentID)
	if err == nil {
		if k.Email == "" && strings.TrimSpace(email) != "" {
			_ = d.UpdateKidEmail(k.ID, email)
			k.Email = email
		}
		return k, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		id, _, errCreate := d.CreateKid(name, email, parentID)
		if errCreate != nil {
			return nil, errCreate
		}
		return d.GetKidByID(id)
	}
	return nil, err
}

func (d *Database) UpdateKidEmail(id int64, email string) error {
	_, err := d.DB.Exec(`UPDATE kids SET email = ? WHERE id = ?`, email, id)
	return err
}

func (d *Database) GetKidByEmail(email string) (*Kid, error) {
	k := &Kid{}
	var gridUserNS, gridWalletNS sql.NullString
	err := d.DB.QueryRow(`SELECT id, uid, parent_id, email, name, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id FROM kids WHERE email = ?`, email).Scan(&k.ID, &k.UID, &k.ParentID, &k.Email, &k.Name, &k.Balance, &k.HPKEPrivDER, &k.HPKEPubDER, &gridUserNS, &gridWalletNS)
	if err != nil {
		return nil, err
	}
	if gridUserNS.Valid {
		k.GridUserID = gridUserNS.String
	}
	if gridWalletNS.Valid {
		k.GridWalletID = gridWalletNS.String
	}
	return k, nil
}

func (d *Database) SetKidHPKEKeys(kidID int64, privDER, pubDER []byte) error {
	_, err := d.DB.Exec(`UPDATE kids SET hpke_priv_der = ?, hpke_pub_der = ? WHERE id = ?`, privDER, pubDER, kidID)
	return err
}

func (d *Database) UpdateKidGridIDs(kidID int64, gridUserID, gridWalletID string) error {
	_, err := d.DB.Exec(`UPDATE kids SET grid_user_id = ?, grid_wallet_id = ? WHERE id = ?`, gridUserID, gridWalletID, kidID)
	return err
}

func (d *Database) GetOrCreateParentByName(name string) (*Parent, error) {
	p, err := d.GetParentByName(name)
	if err == nil {
		return p, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		id, _, errCreate := d.CreateParent(name)
		if errCreate != nil {
			return nil, errCreate
		}
		return d.GetParentByID(id)
	}
	return nil, err
}

func (d *Database) GetOrCreateParentByEmail(name, email string) (*Parent, error) {
	p, err := d.GetParentByEmail(email)
	if err == nil {
		return p, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		id, _, errCreate := d.CreateParentWithEmail(name, email)
		if errCreate != nil {
			return nil, errCreate
		}
		return d.GetParentByID(id)
	}
	return nil, err
}

func (d *Database) SetParentHPKEKeys(parentID int64, privDER, pubDER []byte) error {
	_, err := d.DB.Exec(`UPDATE parents SET hpke_priv_der = ?, hpke_pub_der = ? WHERE id = ?`, privDER, pubDER, parentID)
	return err
}

func (d *Database) UpdateParentGridIDs(parentID int64, gridUserID, gridWalletID string) error {
	_, err := d.DB.Exec(`UPDATE parents SET grid_user_id = ?, grid_wallet_id = ? WHERE id = ?`, gridUserID, gridWalletID, parentID)
	return err
}

func (d *Database) TopUpParent(parentID int64, amount int64) (int64, error) {
	_, err := d.DB.Exec(`UPDATE parents SET balance = balance + ? WHERE id = ?`, amount, parentID)
	if err != nil {
		return 0, err
	}
	return d.GetParentBalance(parentID)
}

func (d *Database) SendKidMoney(parentID, kidID, amount int64) (int64, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM kids WHERE id = ? AND parent_id = ?`, kidID, parentID).Scan(&exists); err != nil {
		return 0, 0, err
	}
	if exists == 0 {
		return 0, 0, errors.New("kid does not belong to parent")
	}

	res, err := tx.ExecContext(ctx, `UPDATE parents SET balance = balance - ? WHERE id = ? AND balance >= ?`, amount, parentID, amount)
	if err != nil {
		return 0, 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	if affected == 0 {
		return 0, 0, errors.New("insufficient funds or parent not found")
	}

	if _, err := tx.ExecContext(ctx, `UPDATE kids SET balance = balance + ? WHERE id = ?`, amount, kidID); err != nil {
		return 0, 0, err
	}

	var parentBal int64
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM parents WHERE id = ?`, parentID).Scan(&parentBal); err != nil {
		return 0, 0, err
	}
	var kidBal int64
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM kids WHERE id = ?`, kidID).Scan(&kidBal); err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return parentBal, kidBal, nil
}
