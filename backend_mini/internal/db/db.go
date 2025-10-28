package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite"

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
	Wallet           string      `json:"wallet"`
}

type Child struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	ParentID string `json:"parent_id"`
	Wallet   string `json:"wallet"`
}

type ParentKid struct {
	Email  string `json:"email"`
	Wallet string `json:"wallet"`
}

type Chore struct {
	ChoreID          string `json:"chore_id"`
	ParentWallet     string `json:"parent_wallet"`
	ChildWallet      string `json:"child_wallet"`
	ChoreName        string `json:"chore_name"`
	ChoreDescription string `json:"chore_description"`
	BountyAmount     uint64 `json:"bounty_amount"`
	ChoreStatus      int    `json:"chore_status"`
}

type AppLimit struct {
	LimitID      string `json:"limit_id"`
	ParentEmail  string `json:"parent_email"`
	KidEmail     string `json:"kid_email"`
	App          string `json:"app"`
	TimePerDay   int    `json:"time_per_day"`
	FeeExtraHour uint64 `json:"fee_extra_hour"`
	CreatedAt    string `json:"created_at"`
}

func Open(ctx context.Context, path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
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
			wallet TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS children (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			parent_id TEXT NOT NULL,
			wallet TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(parent_id) REFERENCES parents(id) ON UPDATE CASCADE ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS chores (
			chore_id TEXT PRIMARY KEY,
			parent_wallet TEXT NOT NULL,
			child_wallet TEXT NOT NULL,
			chore_name TEXT NOT NULL,
			chore_description TEXT NOT NULL,
			bounty_amount INTEGER NOT NULL,
			chore_status INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS app_limits (
			limit_id TEXT PRIMARY KEY,
			parent_email TEXT NOT NULL,
			kid_email TEXT NOT NULL,
			app TEXT NOT NULL,
			time_per_day INTEGER NOT NULL,
			fee_extra_hour INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(parent_email, kid_email, app)
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
		_, err = d.SQL.ExecContext(ctx, `INSERT INTO parents (id, name, email, kids_list, registration_date, wallet) VALUES (?, ?, ?, '[]', ?, '')`, id, name, strings.ToLower(email), now)
		if err == nil {
			return &Parent{ID: id, Name: name, Email: strings.ToLower(email), KidsList: []ParentKid{}, RegistrationDate: now, Wallet: ""}, nil
		}
		// unique collision on id or email -> retry id only when it's id collision; email collision will fail again but caller path should avoid create if exists
		// continue loop to retry id; if email duplicate, next attempt will still fail and we will return the error after attempts
	}
	return nil, errors.New("failed to generate unique id for parent")
}

func (d *DB) UpdateParentByEmail(ctx context.Context, email string, name *string, wallet *string) (*Parent, error) {
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
		_, err = tx.ExecContext(ctx, `INSERT INTO children (id, name, email, parent_id, wallet) VALUES (?, ?, ?, ?, '')`, id, name, strings.ToLower(email), parentID)
		if err == nil {
			child = &Child{ID: id, Name: name, Email: strings.ToLower(email), ParentID: parentID, Wallet: ""}
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

func (d *DB) UpdateChildByEmail(ctx context.Context, email string, name *string, parentID *string, wallet *string) (*Child, error) {
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

func addChildToParentKidsListTx(ctx context.Context, tx *sql.Tx, parentID, childEmail string, childWallet string) error {
	return upsertChildInParentKidsListTx(ctx, tx, parentID, childEmail, childWallet)
}

func upsertChildInParentKidsListTx(ctx context.Context, tx *sql.Tx, parentID, childEmail string, childWallet string) error {
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

func (d *DB) CreateChore(ctx context.Context, parentWallet, childWallet, choreName, choreDescription string, bountyAmount uint64) (*Chore, error) {
	id, err := util.GenerateShortID()
	if err != nil {
		return nil, err
	}

	_, err = d.SQL.ExecContext(ctx, `INSERT INTO chores (chore_id, parent_wallet, child_wallet, chore_name, chore_description, bounty_amount, chore_status) VALUES (?, ?, ?, ?, ?, ?, 0)`,
		id, parentWallet, childWallet, choreName, choreDescription, bountyAmount)
	if err != nil {
		return nil, err
	}

	return &Chore{
		ChoreID:          id,
		ParentWallet:     parentWallet,
		ChildWallet:      childWallet,
		ChoreName:        choreName,
		ChoreDescription: choreDescription,
		BountyAmount:     bountyAmount,
		ChoreStatus:      0,
	}, nil
}

func (d *DB) UpdateChoreStatus(ctx context.Context, choreID string, newStatus int) (*Chore, error) {
	result, err := d.SQL.ExecContext(ctx, `UPDATE chores SET chore_status=? WHERE chore_id=?`, newStatus, choreID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, sql.ErrNoRows
	}

	row := d.SQL.QueryRowContext(ctx, `SELECT chore_id, parent_wallet, child_wallet, chore_name, chore_description, bounty_amount, chore_status FROM chores WHERE chore_id=?`, choreID)
	var c Chore
	if err := row.Scan(&c.ChoreID, &c.ParentWallet, &c.ChildWallet, &c.ChoreName, &c.ChoreDescription, &c.BountyAmount, &c.ChoreStatus); err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) GetChores(ctx context.Context, wallet string) ([]Chore, error) {
	rows, err := d.SQL.QueryContext(ctx, `SELECT chore_id, parent_wallet, child_wallet, chore_name, chore_description, bounty_amount, chore_status FROM chores WHERE parent_wallet=? OR child_wallet=?`, wallet, wallet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chores []Chore
	for rows.Next() {
		var c Chore
		if err := rows.Scan(&c.ChoreID, &c.ParentWallet, &c.ChildWallet, &c.ChoreName, &c.ChoreDescription, &c.BountyAmount, &c.ChoreStatus); err != nil {
			return nil, err
		}
		chores = append(chores, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chores, nil
}

func (d *DB) CreateOrUpdateAppLimit(ctx context.Context, parentEmail, kidEmail, app string, timePerDay int, feeExtraHour uint64) (*AppLimit, error) {
	limitID, err := util.GenerateShortID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = d.SQL.ExecContext(ctx, `
		INSERT INTO app_limits (limit_id, parent_email, kid_email, app, time_per_day, fee_extra_hour, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(parent_email, kid_email, app) DO UPDATE SET
			time_per_day = excluded.time_per_day,
			fee_extra_hour = excluded.fee_extra_hour
	`, limitID, strings.ToLower(parentEmail), strings.ToLower(kidEmail), app, timePerDay, feeExtraHour, now)

	if err != nil {
		return nil, err
	}

	row := d.SQL.QueryRowContext(ctx, `
		SELECT limit_id, parent_email, kid_email, app, time_per_day, fee_extra_hour, created_at
		FROM app_limits 
		WHERE parent_email=? AND kid_email=? AND app=?
	`, strings.ToLower(parentEmail), strings.ToLower(kidEmail), app)

	var limit AppLimit
	if err := row.Scan(&limit.LimitID, &limit.ParentEmail, &limit.KidEmail, &limit.App, &limit.TimePerDay, &limit.FeeExtraHour, &limit.CreatedAt); err != nil {
		return nil, err
	}

	return &limit, nil
}

func (d *DB) GetAppLimitsByKidEmail(ctx context.Context, kidEmail string) ([]AppLimit, error) {
	rows, err := d.SQL.QueryContext(ctx, `
		SELECT limit_id, parent_email, kid_email, app, time_per_day, fee_extra_hour, created_at
		FROM app_limits 
		WHERE kid_email=?
		ORDER BY created_at DESC
	`, strings.ToLower(kidEmail))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var limits []AppLimit
	for rows.Next() {
		var limit AppLimit
		if err := rows.Scan(&limit.LimitID, &limit.ParentEmail, &limit.KidEmail, &limit.App, &limit.TimePerDay, &limit.FeeExtraHour, &limit.CreatedAt); err != nil {
			return nil, err
		}
		limits = append(limits, limit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return limits, nil
}
