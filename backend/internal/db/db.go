package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Database struct {
	DB *sql.DB
}

type Kid struct {
	ID      int64
	Name    string
	Balance int64
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
			balance INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS kids (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			parent_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			balance INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(parent_id) REFERENCES parents(id) ON DELETE CASCADE ON UPDATE CASCADE
		);`,
		"CREATE INDEX IF NOT EXISTS idx_kids_parent_id ON kids(parent_id);",
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
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

func (d *Database) CreateKid(name string, parentID int64) (int64, error) {
	res, err := d.DB.Exec(`INSERT INTO kids(parent_id, name) VALUES(?, ?)`, parentID, name)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
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


