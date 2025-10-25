package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"backend_mini/internal/util"
)

type DB struct {
	SQL *sql.DB
}

type Parent struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Email            string      `json:"email"`
	KidsList         []ParentKid `json:"kids_list"`
	RegistrationDate string      `json:"registration_date"`
	Wallet           float64     `json:"wallet"`
}

type Child struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	ParentID string  `json:"parent_id"`
	Wallet   float64 `json:"wallet"`
}

type ParentKid struct {
	Email  string  `json:"email"`
	Wallet float64 `json:"wallet"`
}

func Open(ctx context.Context, path string) (*DB, error) {
	d, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := d.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = d.Close()
		return nil, err
	}
	return &DB{SQL: d}, nil
}

func (d *DB) Close() error { return d.SQL.Close() }

func (d *DB) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS parents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			kids_list TEXT NOT NULL DEFAULT '[]',
			registration_date TEXT NOT NULL,
			wallet REAL NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS children (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			parent_id TEXT NOT NULL,
			wallet REAL NOT NULL DEFAULT 0,
			FOREIGN KEY(parent_id) REFERENCES parents(id) ON UPDATE CASCADE ON DELETE CASCADE
		);`,
	}
	for _, s := range stmts {
		if _, err := d.SQL.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) GetParentByEmail(ctx context.Context, email string) (*Parent, bool, error) {
	row := d.SQL.QueryRowContext(ctx, `SELECT id, name, email, kids_list, registration_date, wallet FROM parents WHERE lower(email)=?`, strings.ToLower(email))
	var p Parent
	var kidsRaw string
	if err := row.Scan(&p.ID, &p.Name, &p.Email, &kidsRaw, &p.RegistrationDate, &p.Wallet); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if kidsRaw == "" {
		p.KidsList = []ParentKid{}
	} else {
		var kids []ParentKid
		if err := json.Unmarshal([]byte(kidsRaw), &kids); err == nil {
			p.KidsList = kids
		} else {
			// fallback to empty if malformed
			p.KidsList = []ParentKid{}
		}
	}
	return &p, true, nil
}

func (d *DB) GetChildByEmail(ctx context.Context, email string) (*Child, bool, error) {
	row := d.SQL.QueryRowContext(ctx, `SELECT id, name, email, parent_id, wallet FROM children WHERE lower(email)=?`, strings.ToLower(email))
	var c Child
	if err := row.Scan(&c.ID, &c.Name, &c.Email, &c.ParentID, &c.Wallet); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &c, true, nil
}

func (d *DB) CreateParent(ctx context.Context, name, email string) (*Parent, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	// try multiple times in case of rare id collisions
	for i := 0; i < 10; i++ {
		id, err := util.GenerateShortID()
		if err != nil {
			return nil, err
		}
		_, err = d.SQL.ExecContext(ctx, `INSERT INTO parents (id, name, email, kids_list, registration_date, wallet) VALUES (?, ?, ?, '[]', ?, 0)`, id, name, strings.ToLower(email), now)
		if err == nil {
			return &Parent{ID: id, Name: name, Email: strings.ToLower(email), KidsList: []ParentKid{}, RegistrationDate: now, Wallet: 0}, nil
		}
		// unique collision on id or email -> retry id only when it's id collision; email collision will fail again but caller path should avoid create if exists
		// continue loop to retry id; if email duplicate, next attempt will still fail and we will return the error after attempts
	}
	return nil, errors.New("failed to generate unique id for parent")
}

func (d *DB) UpdateParentByEmail(ctx context.Context, email string, name *string, wallet *float64) (*Parent, error) {
	sets := []string{}
	args := []any{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if wallet != nil {
		sets = append(sets, "wallet = ?")
		args = append(args, *wallet)
	}
	if len(sets) == 0 {
		// no-op, just return current
		p, found, err := d.GetParentByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, sql.ErrNoRows
		}
		return p, nil
	}
	args = append(args, strings.ToLower(email))
	q := "UPDATE parents SET " + strings.Join(sets, ", ") + " WHERE lower(email)=?"
	if _, err := d.SQL.ExecContext(ctx, q, args...); err != nil {
		return nil, err
	}
	return d.mustGetParentByEmail(ctx, email)
}

func (d *DB) mustGetParentByEmail(ctx context.Context, email string) (*Parent, error) {
	p, found, err := d.GetParentByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, sql.ErrNoRows
	}
	return p, nil
}

func (d *DB) CreateChild(ctx context.Context, name, email, parentID string) (*Child, error) {
	// ensure parent exists
	if _, found, err := d.GetParentByID(ctx, parentID); err != nil {
		return nil, err
	} else if !found {
		return nil, errors.New("parent_id not found")
	}

	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var child *Child
	for i := 0; i < 10; i++ {
		id, genErr := util.GenerateShortID()
		if genErr != nil {
			return nil, genErr
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO children (id, name, email, parent_id, wallet) VALUES (?, ?, ?, ?, 0)`, id, name, strings.ToLower(email), parentID)
		if err == nil {
			child = &Child{ID: id, Name: name, Email: strings.ToLower(email), ParentID: parentID, Wallet: 0}
			break
		}
	}
	if child == nil {
		return nil, errors.New("failed to generate unique id for child")
	}

	if err := addChildToParentKidsListTx(ctx, tx, parentID, child.Email, child.Wallet); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return child, nil
}

func (d *DB) UpdateChildByEmail(ctx context.Context, email string, name *string, parentID *string, wallet *float64) (*Child, error) {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT id, name, email, parent_id, wallet FROM children WHERE lower(email)=?`, strings.ToLower(email))
	var existing Child
	if err := row.Scan(&existing.ID, &existing.Name, &existing.Email, &existing.ParentID, &existing.Wallet); err != nil {
		return nil, err
	}

	sets := []string{}
	args := []any{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	var newParentID string
	changingParent := false
	if parentID != nil && *parentID != "" && *parentID != existing.ParentID {
		// validate target parent exists
		pRow := tx.QueryRowContext(ctx, `SELECT id FROM parents WHERE id=?`, *parentID)
		var tmp string
		if err := pRow.Scan(&tmp); err != nil {
			return nil, errors.New("parent_id not found")
		}
		sets = append(sets, "parent_id = ?")
		args = append(args, *parentID)
		newParentID = *parentID
		changingParent = true
	}
	if wallet != nil {
		sets = append(sets, "wallet = ?")
		args = append(args, *wallet)
	}
	if len(sets) > 0 {
		args = append(args, strings.ToLower(email))
		q := "UPDATE children SET " + strings.Join(sets, ", ") + " WHERE lower(email)=?"
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return nil, err
		}
	}

	if changingParent {
		if err := removeChildFromParentKidsListTx(ctx, tx, existing.ParentID, existing.Email); err != nil {
			return nil, err
		}
		// fetch current wallet (may have changed above)
		effectiveWallet := existing.Wallet
		if wallet != nil {
			effectiveWallet = *wallet
		}
		if err := addChildToParentKidsListTx(ctx, tx, newParentID, existing.Email, effectiveWallet); err != nil {
			return nil, err
		}
	}

	// If wallet changed but parent not changed, update parent's kids_list wallet entry
	if !changingParent && wallet != nil {
		if err := upsertChildInParentKidsListTx(ctx, tx, existing.ParentID, existing.Email, *wallet); err != nil {
			return nil, err
		}
	}

	row2 := tx.QueryRowContext(ctx, `SELECT id, name, email, parent_id, wallet FROM children WHERE lower(email)=?`, strings.ToLower(email))
	var out Child
	if err := row2.Scan(&out.ID, &out.Name, &out.Email, &out.ParentID, &out.Wallet); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (d *DB) GetParentByID(ctx context.Context, id string) (*Parent, bool, error) {
	row := d.SQL.QueryRowContext(ctx, `SELECT id, name, email, kids_list, registration_date, wallet FROM parents WHERE id=?`, id)
	var p Parent
	var kidsRaw string
	if err := row.Scan(&p.ID, &p.Name, &p.Email, &kidsRaw, &p.RegistrationDate, &p.Wallet); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if kidsRaw == "" {
		p.KidsList = []ParentKid{}
	} else {
		var kids []ParentKid
		if err := json.Unmarshal([]byte(kidsRaw), &kids); err == nil {
			p.KidsList = kids
		} else {
			p.KidsList = []ParentKid{}
		}
	}
	return &p, true, nil
}

func addChildToParentKidsListTx(ctx context.Context, tx *sql.Tx, parentID, childEmail string, childWallet float64) error {
	return upsertChildInParentKidsListTx(ctx, tx, parentID, childEmail, childWallet)
}

func upsertChildInParentKidsListTx(ctx context.Context, tx *sql.Tx, parentID, childEmail string, childWallet float64) error {
	row := tx.QueryRowContext(ctx, `SELECT kids_list FROM parents WHERE id=?`, parentID)
	var kidsRaw string
	if err := row.Scan(&kidsRaw); err != nil {
		return err
	}
	var kids []ParentKid
	if kidsRaw != "" {
		_ = json.Unmarshal([]byte(kidsRaw), &kids)
	}
	updated := false
	for i := range kids {
		if strings.EqualFold(kids[i].Email, childEmail) {
			kids[i].Wallet = childWallet
			updated = true
			break
		}
	}
	if !updated {
		kids = append(kids, ParentKid{Email: strings.ToLower(childEmail), Wallet: childWallet})
	}
	buf, _ := json.Marshal(kids)
	_, err := tx.ExecContext(ctx, `UPDATE parents SET kids_list=? WHERE id=?`, string(buf), parentID)
	return err
}

func removeChildFromParentKidsListTx(ctx context.Context, tx *sql.Tx, parentID, childEmail string) error {
	row := tx.QueryRowContext(ctx, `SELECT kids_list FROM parents WHERE id=?`, parentID)
	var kidsRaw string
	if err := row.Scan(&kidsRaw); err != nil {
		return err
	}
	var kids []ParentKid
	if kidsRaw != "" {
		_ = json.Unmarshal([]byte(kidsRaw), &kids)
	}
	out := make([]ParentKid, 0, len(kids))
	for _, k := range kids {
		if !strings.EqualFold(k.Email, childEmail) {
			out = append(out, k)
		}
	}
	buf, _ := json.Marshal(out)
	_, err := tx.ExecContext(ctx, `UPDATE parents SET kids_list=? WHERE id=?`, string(buf), parentID)
	return err
}
