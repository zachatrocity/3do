package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

var ErrNotFound = sql.ErrNoRows

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			display_name TEXT NOT NULL,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'member',
			active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (role IN ('admin', 'member'))
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS printers (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			location TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'idle',
			capabilities TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS queue_items (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'backlog',
			priority TEXT NOT NULL DEFAULT 'normal',
			requested_by TEXT NOT NULL DEFAULT '',
			owner TEXT NOT NULL DEFAULT '',
			printing_by TEXT NOT NULL DEFAULT '',
			printer_id INTEGER REFERENCES printers(id) ON DELETE SET NULL,
			quantity INTEGER NOT NULL DEFAULT 1,
			material TEXT NOT NULL DEFAULT '',
			color TEXT NOT NULL DEFAULT '',
			estimated_minutes INTEGER,
			due_at TEXT,
			reprint_of_id INTEGER REFERENCES queue_items(id) ON DELETE SET NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (status IN ('backlog', 'queued', 'printing', 'blocked', 'done', 'cancelled')),
			CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
			CHECK (quantity > 0)
		)`,
		`CREATE TABLE IF NOT EXISTS item_links (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			url TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'other',
			title TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS item_files (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			storage_path TEXT NOT NULL,
			original_name TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			checksum TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT 'other',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS item_notes (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			author TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS status_events (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			old_status TEXT NOT NULL DEFAULT '',
			new_status TEXT NOT NULL,
			actor TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS queue_items_status_idx ON queue_items(status)`,
		`CREATE INDEX IF NOT EXISTS queue_items_printer_idx ON queue_items(printer_id)`,
		`CREATE INDEX IF NOT EXISTS item_files_checksum_idx ON item_files(checksum)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_idx ON users(lower(email))`,
		`CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS sessions_expires_idx ON sessions(expires_at)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := s.addColumnIfMissing(ctx, "users", "password_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *Store) addColumnIfMissing(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

func (s *Store) UserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Store) BootstrapAdmin(ctx context.Context, input UserInput) (User, error) {
	input.Role = RoleAdmin
	input.Active = true
	normalizeUserInput(&input)
	if err := validateUserInput(input, true); err != nil {
		return User{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return User{}, err
	}
	if count > 0 {
		return User{}, errors.New("admin bootstrap is already complete")
	}
	row := tx.QueryRowContext(ctx, `INSERT INTO users (display_name, email, password_hash, role, active)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, display_name, email, password_hash, role, active, created_at, updated_at`,
		input.DisplayName, input.Email, input.PasswordHash, input.Role, boolInt(input.Active))
	user, err := scanUser(row)
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) CreateUser(ctx context.Context, input UserInput) (User, error) {
	normalizeUserInput(&input)
	if err := validateUserInput(input, true); err != nil {
		return User{}, err
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO users (display_name, email, password_hash, role, active)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, display_name, email, password_hash, role, active, created_at, updated_at`,
		input.DisplayName, input.Email, input.PasswordHash, input.Role, boolInt(input.Active))
	return scanUser(row)
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, display_name, email, password_hash, role, active, created_at, updated_at
		FROM users ORDER BY active DESC, display_name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, display_name, email, password_hash, role, active, created_at, updated_at
		FROM users WHERE lower(email) = lower(?)`, strings.TrimSpace(email))
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, display_name, email, password_hash, role, active, created_at, updated_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) UpdateUser(ctx context.Context, id int64, input UserUpdate) (User, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Role == "" {
		input.Role = RoleMember
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return User{}, errors.New("display_name is required")
	}
	if strings.TrimSpace(input.Email) == "" {
		return User{}, errors.New("email is required")
	}
	if !validRole(input.Role) {
		return User{}, errors.New("role must be admin or member")
	}
	if err := s.ensureAdminCanChange(ctx, id, input.Role, input.Active); err != nil {
		return User{}, err
	}
	if input.PasswordHash == "" {
		row := s.db.QueryRowContext(ctx, `UPDATE users
			SET display_name = ?, email = ?, role = ?, active = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
			RETURNING id, display_name, email, password_hash, role, active, created_at, updated_at`,
			input.DisplayName, input.Email, input.Role, boolInt(input.Active), id)
		return scanUser(row)
	}
	row := s.db.QueryRowContext(ctx, `UPDATE users
		SET display_name = ?, email = ?, password_hash = ?, role = ?, active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		RETURNING id, display_name, email, password_hash, role, active, created_at, updated_at`,
		input.DisplayName, input.Email, input.PasswordHash, input.Role, boolInt(input.Active), id)
	return scanUser(row)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	if err := s.ensureAdminCanChange(ctx, id, RoleMember, false); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ensureAdminCanChange(ctx context.Context, id int64, nextRole UserRole, nextActive bool) error {
	var currentRole UserRole
	var currentActive int
	if err := s.db.QueryRowContext(ctx, `SELECT role, active FROM users WHERE id = ?`, id).Scan(&currentRole, &currentActive); err != nil {
		return err
	}
	if currentRole != RoleAdmin || currentActive != 1 || (nextRole == RoleAdmin && nextActive) {
		return nil
	}
	var otherAdmins int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id != ? AND role = ? AND active = 1`, id, RoleAdmin).Scan(&otherAdmins); err != nil {
		return err
	}
	if otherAdmins == 0 {
		return errors.New("at least one active admin is required")
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES (?, ?, ?)`, tokenHash, userID, expiresAt.Format(time.RFC3339))
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT users.id, users.display_name, users.email, users.password_hash,
		users.role, users.active, users.created_at, users.updated_at
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > ? AND users.active = 1`,
		tokenHash, now.Format(time.RFC3339))
	return scanUser(row)
}

func (s *Store) PruneExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Format(time.RFC3339))
	return err
}

func (s *Store) CreateQueueItem(ctx context.Context, input QueueItemInput) (QueueItem, error) {
	normalizeQueueInput(&input)
	if strings.TrimSpace(input.Title) == "" {
		return QueueItem{}, errors.New("title is required")
	}
	if err := validateQueueValues(input.Status, input.Priority, input.Quantity); err != nil {
		return QueueItem{}, err
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO queue_items (
		title, description, status, priority, requested_by, owner, printing_by, printer_id,
		quantity, material, color, estimated_minutes, due_at, reprint_of_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id, title, description, status, priority, requested_by, owner, printing_by, printer_id,
		quantity, material, color, estimated_minutes, due_at, reprint_of_id, created_at, updated_at`,
		input.Title, input.Description, input.Status, input.Priority, input.RequestedBy, input.Owner, input.PrintingBy,
		input.PrinterID, input.Quantity, input.Material, input.Color, input.EstimatedMinutes, timePtrString(input.DueAt), input.ReprintOfID)
	item, err := scanQueueItem(row)
	if err != nil {
		return QueueItem{}, err
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO status_events (queue_item_id, new_status, actor, note) VALUES (?, ?, ?, ?)`,
		item.ID, item.Status, item.Owner, "created")
	return item, nil
}

func (s *Store) DeleteQueueItem(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM queue_items WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) GetQueueItem(ctx context.Context, id int64) (QueueItemDetail, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, title, description, status, priority, requested_by, owner, printing_by,
		printer_id, quantity, material, color, estimated_minutes, due_at, reprint_of_id, created_at, updated_at
		FROM queue_items WHERE id = ?`, id)
	item, err := scanQueueItem(row)
	if err != nil {
		return QueueItemDetail{}, err
	}
	return s.hydrateQueueItemDetail(ctx, item)
}

func (s *Store) ListQueueItems(ctx context.Context) ([]QueueItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, title, description, status, priority, requested_by, owner, printing_by,
		printer_id, quantity, material, color, estimated_minutes, due_at, reprint_of_id, created_at, updated_at
		FROM queue_items ORDER BY
		CASE status
			WHEN 'printing' THEN 1
			WHEN 'queued' THEN 2
			WHEN 'blocked' THEN 3
			WHEN 'backlog' THEN 4
			WHEN 'done' THEN 5
			ELSE 6
		END, priority DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		item, err := scanQueueItem(rows)
		if err != nil {
			return nil, err
		}
		item.Links, err = s.ListLinks(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Files, err = s.ListFiles(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateQueueItem(ctx context.Context, id int64, input QueueItemUpdate) (QueueItemDetail, error) {
	normalizeQueueUpdate(&input)
	if err := validateQueueValues(input.Status, input.Priority, input.Quantity); err != nil {
		return QueueItemDetail{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return QueueItemDetail{}, err
	}
	defer tx.Rollback()

	var oldStatus QueueStatus
	if err := tx.QueryRowContext(ctx, `SELECT status FROM queue_items WHERE id = ?`, id).Scan(&oldStatus); err != nil {
		return QueueItemDetail{}, err
	}

	row := tx.QueryRowContext(ctx, `UPDATE queue_items
		SET status = ?, priority = ?, owner = ?, printing_by = ?, quantity = ?,
			material = ?, color = ?, estimated_minutes = ?, due_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		RETURNING id, title, description, status, priority, requested_by, owner, printing_by,
			printer_id, quantity, material, color, estimated_minutes, due_at, reprint_of_id, created_at, updated_at`,
		input.Status, input.Priority, input.Owner, input.PrintingBy, input.Quantity, input.Material, input.Color,
		input.EstimatedMinutes, timePtrString(input.DueAt), id)
	item, err := scanQueueItem(row)
	if err != nil {
		return QueueItemDetail{}, err
	}
	if oldStatus != input.Status {
		if _, err := tx.ExecContext(ctx, `INSERT INTO status_events (queue_item_id, old_status, new_status, actor, note)
			VALUES (?, ?, ?, ?, ?)`, id, oldStatus, input.Status, input.Actor, input.Note); err != nil {
			return QueueItemDetail{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return QueueItemDetail{}, err
	}
	return s.hydrateQueueItemDetail(ctx, item)
}

func (s *Store) AddNote(ctx context.Context, queueItemID int64, author, body string) (ItemNote, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return ItemNote{}, errors.New("note body is required")
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO item_notes (queue_item_id, author, body)
		VALUES (?, ?, ?)
		RETURNING id, queue_item_id, author, body, created_at`, queueItemID, strings.TrimSpace(author), body)
	return scanNote(row)
}

func (s *Store) AddLink(ctx context.Context, queueItemID int64, rawURL string) (ItemLink, error) {
	normalizedURL, sourceType, err := NormalizeLink(rawURL)
	if err != nil {
		return ItemLink{}, err
	}
	if normalizedURL == "" {
		return ItemLink{}, errors.New("url is required")
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO item_links (queue_item_id, url, source_type)
		VALUES (?, ?, ?)
		RETURNING id, queue_item_id, url, source_type, title, created_at`, queueItemID, normalizedURL, sourceType)
	return scanLink(row)
}

func (s *Store) ListLinks(ctx context.Context, queueItemID int64) ([]ItemLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, queue_item_id, url, source_type, title, created_at
		FROM item_links WHERE queue_item_id = ? ORDER BY created_at ASC`, queueItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []ItemLink
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) AddFile(ctx context.Context, file ItemFile) (ItemFile, error) {
	row := s.db.QueryRowContext(ctx, `INSERT INTO item_files
		(queue_item_id, storage_path, original_name, size_bytes, checksum, content_type, kind)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, queue_item_id, storage_path, original_name, size_bytes, checksum, content_type, kind, created_at`,
		file.QueueItemID, file.StoragePath, file.OriginalName, file.SizeBytes, file.Checksum, file.ContentType, file.Kind)
	return scanFile(row)
}

func (s *Store) FileByChecksum(ctx context.Context, checksum string) (ItemFile, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, queue_item_id, storage_path, original_name, size_bytes, checksum, content_type, kind, created_at
		FROM item_files WHERE checksum = ? ORDER BY created_at ASC LIMIT 1`, checksum)
	return scanFile(row)
}

func (s *Store) ListFiles(ctx context.Context, queueItemID int64) ([]ItemFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, queue_item_id, storage_path, original_name, size_bytes, checksum, content_type, kind, created_at
		FROM item_files WHERE queue_item_id = ? ORDER BY created_at ASC`, queueItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []ItemFile
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (s *Store) ListNotes(ctx context.Context, queueItemID int64) ([]ItemNote, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, queue_item_id, author, body, created_at
		FROM item_notes WHERE queue_item_id = ? ORDER BY created_at ASC, id ASC`, queueItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []ItemNote
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func (s *Store) ListStatusEvents(ctx context.Context, queueItemID int64) ([]StatusEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, queue_item_id, old_status, new_status, actor, note, created_at
		FROM status_events WHERE queue_item_id = ? ORDER BY created_at ASC, id ASC`, queueItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []StatusEvent
	for rows.Next() {
		event, err := scanStatusEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) hydrateQueueItemDetail(ctx context.Context, item QueueItem) (QueueItemDetail, error) {
	var err error
	item.Links, err = s.ListLinks(ctx, item.ID)
	if err != nil {
		return QueueItemDetail{}, err
	}
	item.Files, err = s.ListFiles(ctx, item.ID)
	if err != nil {
		return QueueItemDetail{}, err
	}
	notes, err := s.ListNotes(ctx, item.ID)
	if err != nil {
		return QueueItemDetail{}, err
	}
	events, err := s.ListStatusEvents(ctx, item.ID)
	if err != nil {
		return QueueItemDetail{}, err
	}
	return QueueItemDetail{QueueItem: item, Notes: notes, StatusEvents: events}, nil
}

func (s *Store) CreatePrinter(ctx context.Context, input PrinterInput) (Printer, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Printer{}, errors.New("name is required")
	}
	if input.Status == "" {
		input.Status = "idle"
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO printers (name, location, status, capabilities, notes, active)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id, name, location, status, capabilities, notes, active, created_at, updated_at`,
		input.Name, input.Location, input.Status, input.Capabilities, input.Notes, boolInt(input.Active))
	return scanPrinter(row)
}

func (s *Store) ListPrinters(ctx context.Context) ([]Printer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, location, status, capabilities, notes, active, created_at, updated_at
		FROM printers ORDER BY active DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var printers []Printer
	for rows.Next() {
		printer, err := scanPrinter(rows)
		if err != nil {
			return nil, err
		}
		printers = append(printers, printer)
	}
	return printers, rows.Err()
}

func normalizeQueueInput(input *QueueItemInput) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Status == "" {
		input.Status = StatusBacklog
	}
	if input.Priority == "" {
		input.Priority = PriorityNormal
	}
	if input.Quantity < 1 {
		input.Quantity = 1
	}
}

func normalizeQueueUpdate(input *QueueItemUpdate) {
	if input.Status == "" {
		input.Status = StatusBacklog
	}
	if input.Priority == "" {
		input.Priority = PriorityNormal
	}
	if input.Quantity < 1 {
		input.Quantity = 1
	}
	input.Owner = strings.TrimSpace(input.Owner)
	input.PrintingBy = strings.TrimSpace(input.PrintingBy)
	input.Material = strings.TrimSpace(input.Material)
	input.Color = strings.TrimSpace(input.Color)
	input.Actor = strings.TrimSpace(input.Actor)
	input.Note = strings.TrimSpace(input.Note)
}

func validateQueueValues(status QueueStatus, priority Priority, quantity int) error {
	if !validQueueStatus(status) {
		return errors.New("status must be backlog, queued, printing, blocked, done, or cancelled")
	}
	if !validPriority(priority) {
		return errors.New("priority must be low, normal, high, or urgent")
	}
	if quantity < 1 {
		return errors.New("quantity must be greater than zero")
	}
	return nil
}

func validQueueStatus(status QueueStatus) bool {
	switch status {
	case StatusBacklog, StatusQueued, StatusPrinting, StatusBlocked, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}

func validPriority(priority Priority) bool {
	switch priority {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}

func normalizeUserInput(input *UserInput) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Role == "" {
		input.Role = RoleMember
	}
}

func validateUserInput(input UserInput, requirePassword bool) error {
	if input.DisplayName == "" {
		return errors.New("display_name is required")
	}
	if input.Email == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(input.Email, "@") {
		return errors.New("email must be valid")
	}
	if !validRole(input.Role) {
		return errors.New("role must be admin or member")
	}
	if requirePassword && input.PasswordHash == "" {
		return errors.New("password_hash is required")
	}
	return nil
}

func validRole(role UserRole) bool {
	return role == RoleAdmin || role == RoleMember
}

func NormalizeLink(rawURL string) (string, string, error) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return "", "", errors.New("url is required")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", "", fmt.Errorf("url is invalid: %w", err)
	}
	isURLLike := parsed.Scheme != "" || strings.Contains(parsed.Path, ".")
	if isURLLike && strings.ContainsAny(value, " \t\r\n") {
		return "", "", errors.New("url must not contain whitespace")
	}
	if parsed.Scheme == "" && strings.Contains(parsed.Path, ".") {
		parsed, err = url.Parse("https://" + value)
		if err != nil {
			return "", "", fmt.Errorf("url is invalid: %w", err)
		}
	}
	sourceType := detectSourceType(parsed)
	if parsed.Scheme == "" {
		return value, sourceType, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", errors.New("url must use http or https")
	}
	if parsed.Hostname() == "" {
		return "", "", errors.New("url host is required")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	return parsed.String(), sourceType, nil
}

func detectSourceType(parsed *url.URL) string {
	host := strings.ToLower(parsed.Hostname())
	switch {
	case hostMatches(host, "printables.com"):
		return "printables"
	case hostMatches(host, "makerworld.com"):
		return "makerworld"
	case hostMatches(host, "thingiverse.com"):
		return "thingiverse"
	case hostMatches(host, "github.com"):
		return "github"
	case parsed.Scheme == "http" || parsed.Scheme == "https":
		return "direct"
	default:
		return "other"
	}
}

func hostMatches(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func DetectFileKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".stl":
		return "stl"
	case ".3mf":
		return "3mf"
	case ".gcode":
		return "gcode"
	case ".step", ".stp":
		return "step"
	case ".obj":
		return "obj"
	case ".png", ".jpg", ".jpeg", ".webp":
		return "image"
	default:
		return "other"
	}
}

func AllowedUpload(name string) bool {
	switch DetectFileKind(name) {
	case "stl", "3mf", "gcode", "step", "obj", "image":
		return true
	default:
		return strings.EqualFold(filepath.Ext(name), ".zip")
	}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanQueueItem(row scanner) (QueueItem, error) {
	var item QueueItem
	var printerID, estimatedMinutes, reprintOfID sql.NullInt64
	var dueAt sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&item.ID, &item.Title, &item.Description, &item.Status, &item.Priority,
		&item.RequestedBy, &item.Owner, &item.PrintingBy, &printerID, &item.Quantity, &item.Material,
		&item.Color, &estimatedMinutes, &dueAt, &reprintOfID, &createdAt, &updatedAt)
	if err != nil {
		return QueueItem{}, err
	}
	if printerID.Valid {
		value := printerID.Int64
		item.PrinterID = &value
	}
	if estimatedMinutes.Valid {
		value := int(estimatedMinutes.Int64)
		item.EstimatedMinutes = &value
	}
	if reprintOfID.Valid {
		value := reprintOfID.Int64
		item.ReprintOfID = &value
	}
	if dueAt.Valid {
		parsed, err := time.Parse(time.RFC3339, dueAt.String)
		if err == nil {
			item.DueAt = &parsed
		}
	}
	item.CreatedAt = parseSQLiteTime(createdAt)
	item.UpdatedAt = parseSQLiteTime(updatedAt)
	return item, nil
}

func scanLink(row scanner) (ItemLink, error) {
	var link ItemLink
	var createdAt string
	if err := row.Scan(&link.ID, &link.QueueItemID, &link.URL, &link.SourceType, &link.Title, &createdAt); err != nil {
		return ItemLink{}, err
	}
	link.CreatedAt = parseSQLiteTime(createdAt)
	return link, nil
}

func scanFile(row scanner) (ItemFile, error) {
	var file ItemFile
	var createdAt string
	if err := row.Scan(&file.ID, &file.QueueItemID, &file.StoragePath, &file.OriginalName, &file.SizeBytes, &file.Checksum, &file.ContentType, &file.Kind, &createdAt); err != nil {
		return ItemFile{}, err
	}
	file.CreatedAt = parseSQLiteTime(createdAt)
	return file, nil
}

func scanNote(row scanner) (ItemNote, error) {
	var note ItemNote
	var createdAt string
	if err := row.Scan(&note.ID, &note.QueueItemID, &note.Author, &note.Body, &createdAt); err != nil {
		return ItemNote{}, err
	}
	note.CreatedAt = parseSQLiteTime(createdAt)
	return note, nil
}

func scanStatusEvent(row scanner) (StatusEvent, error) {
	var event StatusEvent
	var createdAt string
	if err := row.Scan(&event.ID, &event.QueueItemID, &event.OldStatus, &event.NewStatus, &event.Actor, &event.Note, &createdAt); err != nil {
		return StatusEvent{}, err
	}
	event.CreatedAt = parseSQLiteTime(createdAt)
	return event, nil
}

func scanPrinter(row scanner) (Printer, error) {
	var printer Printer
	var active int
	var createdAt, updatedAt string
	if err := row.Scan(&printer.ID, &printer.Name, &printer.Location, &printer.Status, &printer.Capabilities, &printer.Notes, &active, &createdAt, &updatedAt); err != nil {
		return Printer{}, err
	}
	printer.Active = active == 1
	printer.CreatedAt = parseSQLiteTime(createdAt)
	printer.UpdatedAt = parseSQLiteTime(updatedAt)
	return printer, nil
}

func scanUser(row scanner) (User, error) {
	var user User
	var active int
	var createdAt, updatedAt string
	if err := row.Scan(&user.ID, &user.DisplayName, &user.Email, &user.PasswordHash, &user.Role, &active, &createdAt, &updatedAt); err != nil {
		return User{}, err
	}
	user.Active = active == 1
	user.CreatedAt = parseSQLiteTime(createdAt)
	user.UpdatedAt = parseSQLiteTime(updatedAt)
	return user, nil
}

func timePtrString(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}

func parseSQLiteTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) DebugString() string {
	return fmt.Sprintf("store{%p}", s.db)
}
