package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AzizARSLN/drydock/internal/models"
	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database and exposes typed data access helpers.
type Store struct {
	db *sql.DB
}

// Open initialises the SQLite file and runs the schema migrations.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite writes are serialized; avoids "database is locked"
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role          TEXT NOT NULL DEFAULT 'viewer',
  full_name     TEXT NOT NULL DEFAULT '',
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_login    DATETIME
);

CREATE TABLE IF NOT EXISTS servers (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  name              TEXT NOT NULL,
  host              TEXT NOT NULL,
  port              INTEGER NOT NULL DEFAULT 22,
  username          TEXT NOT NULL,
  auth_type         TEXT NOT NULL DEFAULT 'key',
  private_key_enc   TEXT NOT NULL DEFAULT '',
  password_enc      TEXT NOT NULL DEFAULT '',
  passphrase_enc    TEXT NOT NULL DEFAULT '',
  sudo_password_enc TEXT NOT NULL DEFAULT '',
  tags              TEXT NOT NULL DEFAULT '',
  description       TEXT NOT NULL DEFAULT '',
  host_key          TEXT NOT NULL DEFAULT '',
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME
);

CREATE TABLE IF NOT EXISTS snapshots (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id  INTEGER NOT NULL,
  kind       TEXT NOT NULL,
  path       TEXT NOT NULL,
  content    TEXT NOT NULL,
  note       TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_snapshots_server ON snapshots(server_id, kind, path);

CREATE TABLE IF NOT EXISTS audit_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL DEFAULT 0,
  username    TEXT NOT NULL DEFAULT '',
  server_id   INTEGER NOT NULL DEFAULT 0,
  server_name TEXT NOT NULL DEFAULT '',
  action      TEXT NOT NULL,
  target      TEXT NOT NULL DEFAULT '',
  detail      TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL DEFAULT '',
  ip          TEXT NOT NULL DEFAULT '',
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);
`

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Idempotent column additions for databases created by older builds.
	alters := []string{
		`ALTER TABLE servers ADD COLUMN host_key TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range alters {
		// A "duplicate column name" error simply means the migration already ran.
		_, _ = s.db.Exec(stmt)
	}
	return nil
}

// --- Users -----------------------------------------------------------------

// CountUsers returns how many operator accounts exist.
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser inserts a new operator account.
func (s *Store) CreateUser(u *models.User) error {
	res, err := s.db.Exec(
		`INSERT INTO users(username, password_hash, role, full_name) VALUES(?,?,?,?)`,
		u.Username, u.PasswordHash, u.Role, u.FullName,
	)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

// GetUserByUsername loads an account by its login name.
func (s *Store) GetUserByUsername(username string) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, role, full_name, created_at, last_login
		 FROM users WHERE username = ?`, username)
	return scanUser(row)
}

// GetUserByID loads an account by primary key.
func (s *Store) GetUserByID(id int64) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, role, full_name, created_at, last_login
		 FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// ListUsers returns every operator account.
func (s *Store) ListUsers() ([]models.User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, password_hash, role, full_name, created_at, last_login
		 FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// UpdateUser persists role/full name changes.
func (s *Store) UpdateUser(id int64, role, fullName string) error {
	_, err := s.db.Exec(`UPDATE users SET role = ?, full_name = ? WHERE id = ?`, role, fullName, id)
	return err
}

// UpdatePassword replaces the password hash of an account.
func (s *Store) UpdatePassword(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// DeleteUser removes an account.
func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

// TouchLastLogin records the most recent successful authentication.
func (s *Store) TouchLastLogin(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET last_login = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(r rowScanner) (*models.User, error) {
	var u models.User
	var last sql.NullTime
	err := r.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.FullName, &u.CreatedAt, &last)
	if err != nil {
		return nil, err
	}
	if last.Valid {
		u.LastLogin = &last.Time
	}
	return &u, nil
}

// --- Servers ---------------------------------------------------------------

const serverCols = `id, name, host, port, username, auth_type, private_key_enc, password_enc,
	passphrase_enc, sudo_password_enc, tags, description, host_key, created_at, updated_at`

// CreateServer inserts a managed host.
func (s *Store) CreateServer(sv *models.Server) error {
	res, err := s.db.Exec(
		`INSERT INTO servers(name, host, port, username, auth_type, private_key_enc,
			password_enc, passphrase_enc, sudo_password_enc, tags, description, host_key)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		sv.Name, sv.Host, sv.Port, sv.Username, sv.AuthType, sv.PrivateKeyEnc,
		sv.PasswordEnc, sv.PassphraseEnc, sv.SudoPasswordEnc, sv.Tags, sv.Description,
		sv.HostKeyFingerprint)
	if err != nil {
		return err
	}
	sv.ID, _ = res.LastInsertId()
	return nil
}

// UpdateServer persists edited connection details.
func (s *Store) UpdateServer(sv *models.Server) error {
	_, err := s.db.Exec(
		`UPDATE servers SET name=?, host=?, port=?, username=?, auth_type=?,
			private_key_enc=?, password_enc=?, passphrase_enc=?, sudo_password_enc=?,
			tags=?, description=?, host_key=?, updated_at=? WHERE id=?`,
		sv.Name, sv.Host, sv.Port, sv.Username, sv.AuthType, sv.PrivateKeyEnc,
		sv.PasswordEnc, sv.PassphraseEnc, sv.SudoPasswordEnc, sv.Tags, sv.Description,
		sv.HostKeyFingerprint, time.Now().UTC(), sv.ID)
	return err
}

// SetHostKey pins the host key fingerprint captured during the first connection.
func (s *Store) SetHostKey(serverID int64, fingerprint string) error {
	_, err := s.db.Exec(`UPDATE servers SET host_key = ? WHERE id = ?`, fingerprint, serverID)
	return err
}

// DeleteServer removes a host and its snapshots.
func (s *Store) DeleteServer(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM snapshots WHERE server_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM servers WHERE id = ?`, id)
	return err
}

// GetServer loads a single host by id.
func (s *Store) GetServer(id int64) (*models.Server, error) {
	row := s.db.QueryRow(`SELECT `+serverCols+` FROM servers WHERE id = ?`, id)
	return scanServer(row)
}

// ListServers returns all managed hosts.
func (s *Store) ListServers() ([]models.Server, error) {
	rows, err := s.db.Query(`SELECT ` + serverCols + ` FROM servers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Server
	for rows.Next() {
		sv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sv)
	}
	return out, rows.Err()
}

func scanServer(r rowScanner) (*models.Server, error) {
	var sv models.Server
	var updated sql.NullTime
	err := r.Scan(&sv.ID, &sv.Name, &sv.Host, &sv.Port, &sv.Username, &sv.AuthType,
		&sv.PrivateKeyEnc, &sv.PasswordEnc, &sv.PassphraseEnc, &sv.SudoPasswordEnc,
		&sv.Tags, &sv.Description, &sv.HostKeyFingerprint, &sv.CreatedAt, &updated)
	if err != nil {
		return nil, err
	}
	if updated.Valid {
		sv.UpdatedAt = &updated.Time
	}
	return &sv, nil
}

// --- Snapshots -------------------------------------------------------------

// SaveSnapshot records the previous content of a file.
func (s *Store) SaveSnapshot(sn *models.Snapshot) error {
	res, err := s.db.Exec(
		`INSERT INTO snapshots(server_id, kind, path, content, note, created_by)
		 VALUES(?,?,?,?,?,?)`,
		sn.ServerID, sn.Kind, sn.Path, sn.Content, sn.Note, sn.CreatedBy)
	if err != nil {
		return err
	}
	sn.ID, _ = res.LastInsertId()
	return nil
}

// ListSnapshots returns snapshots for a server, optionally filtered by path.
func (s *Store) ListSnapshots(serverID int64, path string) ([]models.Snapshot, error) {
	q := `SELECT id, server_id, kind, path, note, created_by, created_at
	      FROM snapshots WHERE server_id = ?`
	args := []any{serverID}
	if path != "" {
		q += ` AND path = ?`
		args = append(args, path)
	}
	q += ` ORDER BY id DESC LIMIT 200`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Snapshot
	for rows.Next() {
		var sn models.Snapshot
		if err := rows.Scan(&sn.ID, &sn.ServerID, &sn.Kind, &sn.Path, &sn.Note,
			&sn.CreatedBy, &sn.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// GetSnapshot loads a single snapshot including its content.
func (s *Store) GetSnapshot(id int64) (*models.Snapshot, error) {
	var sn models.Snapshot
	err := s.db.QueryRow(
		`SELECT id, server_id, kind, path, content, note, created_by, created_at
		 FROM snapshots WHERE id = ?`, id).
		Scan(&sn.ID, &sn.ServerID, &sn.Kind, &sn.Path, &sn.Content, &sn.Note,
			&sn.CreatedBy, &sn.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sn, nil
}

// --- Audit -----------------------------------------------------------------

// AddAudit appends an entry to the audit trail.
func (s *Store) AddAudit(e *models.AuditEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_log(user_id, username, server_id, server_name, action,
			target, detail, status, ip) VALUES(?,?,?,?,?,?,?,?,?)`,
		e.UserID, e.Username, e.ServerID, e.ServerName, e.Action, e.Target,
		e.Detail, e.Status, e.IP)
	return err
}

// ListAudit returns the most recent audit entries, newest first.
func (s *Store) ListAudit(limit, serverID int64) ([]models.AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	q := `SELECT id, user_id, username, server_id, server_name, action, target,
	             detail, status, ip, created_at FROM audit_log`
	args := []any{}
	if serverID > 0 {
		q += ` WHERE server_id = ?`
		args = append(args, serverID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AuditEntry
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Username, &e.ServerID, &e.ServerName,
			&e.Action, &e.Target, &e.Detail, &e.Status, &e.IP, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
